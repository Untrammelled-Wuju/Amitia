import { createHash, randomUUID } from "node:crypto";
import type { LoadedInstallation, RuntimeAction } from "./resource-loader";
import type {
  DesktopPetPlayerPort,
  PlayerLifecyclePort,
  PlayerState,
  PlayerSubmissionIdentity,
  PlayerSwitchContext,
} from "./player-port";
import type { AnimationIpcAdapter, RendererDeliveryResult } from "./animation-ipc";
import type {
  PlayActionCommand,
  LoopType,
  ReturnTarget,
  PlaybackEvent,
  PlaybackSnapshot,
} from "../../desktop-pet/animation/contracts";


function buildReturnTarget(action: RuntimeAction): ReturnTarget {
  if (action.returnTo) {
    switch (action.returnTo.type) {
      case "action":
        return { type: "action", actionKey: action.returnTo.actionKey };
      case "previous":
        return { type: "previous" };
      case "current_activity":
        return { type: "current_activity" };
      case "none":
        return { type: "none" };
      case "default":
      default:
        return { type: "default" };
    }
  }
  if (action.returnAction) {
    return { type: "action", actionKey: action.returnAction };
  }
  return { type: "default" };
}

function finiteNonNegative(value: number | null | undefined): number | undefined {
  if (value === null || value === undefined || !Number.isFinite(value) || value < 0) return undefined;
  return Math.floor(value);
}

export interface AnimationPlayerBridgeCallbacks {
  onActionSwitch?: (newActionKey: string, oldActionKey: string | null, playbackId: string) => void;
  onActionCompleted?: (actionKey: string, loopCount: number, playbackId: string) => void;
  onActionInterrupted?: (actionKey: string, loopCount: number, playbackId: string) => void;
  onActionFailed?: (actionKey: string, reason?: string, playbackId?: string) => void;
  onError?: (error: Error) => void;
}

export class AnimationPlayerBridge implements DesktopPetPlayerPort, PlayerLifecyclePort {
  private pendingAction: RuntimeAction | null = null;
  private pendingPlaybackId: string | null = null;
  private pendingCommandId: string | null = null;
  private currentAction: RuntimeAction | null = null;
  private state: PlayerState = "idle";
  private loopCount = 0;
  private currentFrameIndex = 0;
  private loaded: LoadedInstallation | null = null;
  private readonly sustainedActionMap: Map<string, string> = new Map();
  private callbacks: AnimationPlayerBridgeCallbacks;
  private animationIpc: AnimationIpcAdapter | null = null;
  private installationId = "";
  private petInstanceId = "";
  private packageRevision = 0;
  private currentPlaybackId: string | null = null;
  private currentCommandId: string | null = null;
  private readonly submittedPlaybacks = new Map<string, {
    action: RuntimeAction;
    playbackId: string | null;
  }>();

  constructor(callbacks?: AnimationPlayerBridgeCallbacks) {
    this.callbacks = callbacks ?? {};
  }

  setAnimationIpc(ipc: AnimationIpcAdapter): void {
    this.animationIpc = ipc;
  }

  setInstallationContext(installationId: string, petInstanceId: string, packageRevision: number): void {
    this.installationId = installationId;
    this.petInstanceId = petInstanceId;
    this.packageRevision = packageRevision;
  }

  attachLoaded(loaded: LoadedInstallation): void {
    this.loaded = loaded;
  }

  detachLoaded(): void {
    this.loaded = null;
    this.clearAllPlaybackState();
  }

  setSustainedActionMap(map: Record<string, string>): void {
    this.sustainedActionMap.clear();
    for (const [key, value] of Object.entries(map)) {
      if (key && value) {
        this.sustainedActionMap.set(key, value);
      }
    }
  }

  play(action: RuntimeAction, context?: PlayerSwitchContext): PlayerSubmissionIdentity | void {
    if (!action) {
      this.reportError(new Error("ACTION_REQUIRED"));
      return;
    }
    if (!action.available) {
      this.reportError(
        new Error(
          `ACTION_UNAVAILABLE: ${action.key}${action.loadError ? ` (${action.loadError})` : ""}`,
        ),
      );
      return;
    }

    const commandId = context?.commandId?.trim() || randomUUID();
    const oldKey = this.currentAction?.key ?? null;

    this.pendingAction = action;
    this.pendingPlaybackId = null;
    this.pendingCommandId = commandId;
    this.loopCount = 0;
    this.currentFrameIndex = 0;
    this.state = "loading";

    // Playback identity is renderer-authoritative. Keep every in-flight
    // submission, not only the latest one: Renderer -> Main events can be delayed
    // while Main has already submitted a newer replacement.
    void oldKey;
    this.submittedPlaybacks.set(commandId, { action, playbackId: null });
    const delivered = this.sendPlayCommand(action, commandId, context);
    if (!delivered) {
      this.submittedPlaybacks.delete(commandId);
      return;
    }
    return { commandId };
  }

  pause(): void {
    if (this.state !== "playing" && this.state !== "loading") return;
    this.state = "paused";
    this.animationIpc?.sendPause();
  }

  resume(): void {
    if (this.state !== "paused") return;
    if (!this.currentAction && !this.pendingAction) return;
    this.state = "playing";
    this.animationIpc?.sendResume();
  }

  stop(reason = "user_disabled"): void {
    const action = this.currentAction ?? this.pendingAction;
    const playbackId = this.currentPlaybackId ?? this.pendingPlaybackId ?? undefined;
    this.state = "stopped";
    this.loopCount = 0;
    this.currentFrameIndex = 0;
    // Deliver the interruption reason before clearing local mirrors. Renderer
    // playback identity remains authoritative for the terminal event when IPC
    // succeeds; on delivery failure, surface a bridge failure so Manager can
    // terminalize the already-accepted Runtime v2 play command itself.
    const result = this.animationIpc?.sendStop(reason);
    if (result && result.status !== "delivered" && action) {
      this.callbacks.onActionFailed?.(
        action.key,
        `STOP_DELIVERY_FAILED:${result.reason ?? "send_failed"}`,
        playbackId,
      );
    }
    this.clearAllPlaybackState();
  }

  switchAction(action: RuntimeAction, context?: PlayerSwitchContext): PlayerSubmissionIdentity | void {
    return this.play(action, context);
  }

  getCurrentAction(): RuntimeAction | null {
    return this.currentAction ?? this.pendingAction;
  }

  getCurrentFrameIndex(): number {
    return this.currentFrameIndex;
  }

  getCurrentPlaybackId(): string | null {
    return this.currentPlaybackId ?? this.pendingPlaybackId;
  }

  getState(): PlayerState {
    return this.state;
  }

  getLoopCount(): number {
    return this.loopCount;
  }

  getFallbackChain(
    action: RuntimeAction,
    loaded?: LoadedInstallation | null,
  ): RuntimeAction[] {
    const result: RuntimeAction[] = [];
    const seen = new Set<string>();
    const loadedRef = loaded ?? this.loaded;
    if (!action || !loadedRef) return result;

    if (action.returnAction) {
      const r = loadedRef.actions.get(action.returnAction);
      if (r && r.available && !seen.has(r.key)) {
        result.push(r);
        seen.add(r.key);
      }
    }

    const sustainedKey = this.sustainedActionMap.get(action.key);
    if (sustainedKey) {
      const s = loadedRef.actions.get(sustainedKey);
      if (s && s.available && !seen.has(s.key)) {
        result.push(s);
        seen.add(s.key);
      }
    }

    if (
      loadedRef.defaultAction &&
      loadedRef.defaultAction.available &&
      !seen.has(loadedRef.defaultAction.key)
    ) {
      result.push(loadedRef.defaultAction);
      seen.add(loadedRef.defaultAction.key);
    }

    for (const candidate of loadedRef.actions.values()) {
      if (!candidate.available) continue;
      if (candidate.key === action.key) continue;
      if (seen.has(candidate.key)) continue;
      result.push(candidate);
      seen.add(candidate.key);
    }

    return result;
  }

  handleSubmissionFailure(commandId: string, reason: string): boolean {
    if (!commandId) return false;
    const submitted = this.submittedPlaybacks.get(commandId);
    if (!submitted) return false;
    this.submittedPlaybacks.delete(commandId);
    const actionKey = submitted.action.key;
    if (commandId === this.pendingCommandId) {
      this.clearPendingPlaybackState();
    }
    this.state = this.pendingAction ? "loading" : this.currentAction ? "playing" : "stopped";
    this.reportError(new Error(`DELIVERY_FAILED: ${reason}${actionKey ? ` (${actionKey})` : ""}`));
    return true;
  }

  handlePlaybackEvent(event: PlaybackEvent): void {
    const eventPlaybackId = event.playbackInstanceId ?? "";
    const eventCommandId = event.commandId ?? "";
    const submitted = eventCommandId ? this.submittedPlaybacks.get(eventCommandId) : undefined;

    switch (event.type) {
      case "playback.command_accepted": {
        if (!submitted || !eventPlaybackId) return;
        if (submitted.playbackId && submitted.playbackId !== eventPlaybackId) return;
        submitted.playbackId = eventPlaybackId;
        if (eventCommandId === this.pendingCommandId) {
          this.pendingPlaybackId = eventPlaybackId;
        }
        const oldKey = this.currentAction?.key ?? null;
        const pendingKey = event.actionKey ?? submitted.action.key;
        if (pendingKey) this.callbacks.onActionSwitch?.(pendingKey, oldKey, eventPlaybackId);
        break;
      }
      case "playback.action_started": {
        if (!submitted || !eventCommandId || !eventPlaybackId) return;
        if (!submitted.playbackId || eventPlaybackId !== submitted.playbackId) return;

        const newAction = (event.actionKey ? this.loaded?.actions.get(event.actionKey) : undefined)
          ?? submitted.action;
        this.currentAction = newAction;
        this.currentPlaybackId = eventPlaybackId;
        this.currentCommandId = eventCommandId;
        this.submittedPlaybacks.delete(eventCommandId);

        if (eventCommandId === this.pendingCommandId) {
          this.clearPendingPlaybackState();
        }
        // A newer replacement may already be in flight. The physical playback is
        // still authoritative, but the bridge remains loading until that latest
        // submission resolves.
        this.state = this.pendingAction ? "loading" : "playing";
        break;
      }
      case "playback.action_completed": {
        if (this.matchesTerminalPlayback(event)) {
          this.loopCount += 1;
          const key = event.actionKey ?? this.currentAction?.key ?? "";
          if (key) {
            this.callbacks.onActionCompleted?.(key, this.loopCount, eventPlaybackId || this.currentPlaybackId || "");
          }
          this.clearCurrentPlaybackState();
          this.state = this.pendingAction ? "loading" : "stopped";
          break;
        }
        // `already_satisfied` completes after command_accepted without a new
        // physical start. Resolve only that submitted command and preserve the
        // playback that is already visible.
        if (submitted && submitted.playbackId === eventPlaybackId) {
          this.submittedPlaybacks.delete(eventCommandId);
          if (eventCommandId === this.pendingCommandId) this.clearPendingPlaybackState();
          const key = event.actionKey ?? submitted.action.key;
          if (key) this.callbacks.onActionCompleted?.(key, this.loopCount, eventPlaybackId);
          this.state = this.pendingAction ? "loading" : this.currentAction ? "playing" : "stopped";
        }
        break;
      }
      case "playback.action_interrupted": {
        if (this.matchesTerminalPlayback(event)) {
          const key = event.actionKey ?? this.currentAction?.key ?? "";
          if (key) {
            this.callbacks.onActionInterrupted?.(key, this.loopCount, eventPlaybackId || this.currentPlaybackId || "");
          }
          this.clearCurrentPlaybackState();
          this.state = this.pendingAction ? "loading" : "stopped";
          break;
        }
        if (submitted && submitted.playbackId === eventPlaybackId) {
          this.submittedPlaybacks.delete(eventCommandId);
          if (eventCommandId === this.pendingCommandId) this.clearPendingPlaybackState();
          const key = event.actionKey ?? submitted.action.key;
          if (key) this.callbacks.onActionInterrupted?.(key, this.loopCount, eventPlaybackId);
          this.state = this.pendingAction ? "loading" : this.currentAction ? "playing" : "stopped";
        }
        break;
      }
      case "playback.action_failed": {
        // A command may fail before command_accepted, after acceptance but before
        // first frame, or after becoming the physical current playback. Resolve
        // only the exact known command/playback pair at each phase.
        const isCurrent = this.matchesTerminalPlayback(event);
        const isSubmitted = !!submitted && (!submitted.playbackId || submitted.playbackId === eventPlaybackId);
        if (!isCurrent && !isSubmitted) return;

        if (isSubmitted) {
          this.submittedPlaybacks.delete(eventCommandId);
          if (eventCommandId === this.pendingCommandId) this.clearPendingPlaybackState();
        }
        if (isCurrent) this.clearCurrentPlaybackState();
        this.state = this.pendingAction ? "loading" : this.currentAction ? "playing" : "stopped";
        if (event.error?.message || event.reason) {
          this.reportError(new Error(`PLAYBACK_FAILED: ${event.error?.message ?? event.reason}`));
        }
        break;
      }
      case "playback.action_holding":
        if (this.matchesTerminalPlayback(event)) {
          this.state = "paused";
        }
        break;
    }
  }

  handleSnapshotUpdate(snapshot: PlaybackSnapshot): void {
    if (typeof snapshot.frameIndex === "number" && snapshot.frameIndex >= 0) {
      this.currentFrameIndex = snapshot.frameIndex;
    }
    if (snapshot.currentActionKey && this.loaded) {
      const action = this.loaded.actions.get(snapshot.currentActionKey);
      if (action && action.available) {
        this.currentAction = action;
      }
    }
  }

  private sendPlayCommand(
    action: RuntimeAction,
    commandId: string,
    context?: PlayerSwitchContext,
  ): boolean {
    if (!this.animationIpc) {
      this.clearPendingPlaybackState();
      this.state = this.currentAction ? "playing" : "stopped";
      this.reportError(new Error("ANIMATION_IPC_UNAVAILABLE"));
      return false;
    }
    const command: PlayActionCommand = {
      commandId,
      // Empty on the Main -> Renderer boundary by contract; renderer assigns it.
      playbackInstanceId: "",
      idempotencyKey: context?.idempotencyKey?.trim()
        || createHash("sha256").update(commandId).digest("hex").slice(0, 32),
      installationId: this.installationId,
      petInstanceId: this.petInstanceId,
      packageRevision: this.packageRevision,
      actionKey: action.key,
      priority: Number.isFinite(context?.priority) ? Math.floor(context?.priority ?? action.priority ?? 50) : (action.priority ?? 50),
      queuePolicy: context?.queuePolicy ?? "replace_current",
      interruptPolicy: context?.interruptPolicy ?? "respect_action",
      playbackRate: Number.isFinite(context?.playbackRate) && (context?.playbackRate ?? 0) > 0
        ? context?.playbackRate ?? 1
        : 1,
      issuedAt: new Date().toISOString(),
      requiresAuthoritativeExpiry: context?.requiresAuthoritativeExpiry === true,
      expiresAt: context?.expiresAt,
      returnOverride: context?.returnOverride ?? buildReturnTarget(action),
      minimumPlayMs: finiteNonNegative(context?.minimumPlayMs ?? action.minimumPlayMs),
      maximumPlayMs: finiteNonNegative(context?.maximumPlayMs ?? action.maximumPlayMs),
      interruptAfterMs: finiteNonNegative(context?.interruptAfterMs ?? action.interruptAfterMs),
      completionPolicy: context?.completionPolicy,
      traceId: context?.traceId,
      source: context?.source,
    };
    const result: RendererDeliveryResult = this.animationIpc.sendPlayAction(command);
    if (result.status !== "delivered" && result.status !== "queued") {
      this.clearPendingPlaybackState();
      this.state = this.currentAction ? "playing" : "stopped";
      this.reportError(
        new Error(`DELIVERY_FAILED: ${result.reason}${result.error ? ` (${result.error})` : ""}`),
      );
      return false;
    }
    return true;
  }

  private matchesTerminalPlayback(event: PlaybackEvent): boolean {
    if (!this.currentPlaybackId || !this.currentCommandId) return false;
    return event.playbackInstanceId === this.currentPlaybackId &&
      event.commandId === this.currentCommandId;
  }

  private clearPendingPlaybackState(): void {
    this.pendingAction = null;
    this.pendingPlaybackId = null;
    this.pendingCommandId = null;
  }

  private clearCurrentPlaybackState(): void {
    this.currentAction = null;
    this.currentPlaybackId = null;
    this.currentCommandId = null;
  }

  private clearAllPlaybackState(): void {
    this.clearPendingPlaybackState();
    this.clearCurrentPlaybackState();
    this.submittedPlaybacks.clear();
  }

  private reportError(error: Error): void {
    if (!this.callbacks.onError) return;
    try {
      this.callbacks.onError(error);
    } catch (err) {
      console.error("[AnimationPlayerBridge] onError callback failed", err);
    }
  }
}
