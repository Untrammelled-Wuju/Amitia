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
    const playbackInstanceId = context?.playbackInstanceId?.trim() || randomUUID();
    const oldKey = this.currentAction?.key ?? null;

    this.pendingAction = action;
    this.pendingPlaybackId = playbackInstanceId;
    this.pendingCommandId = commandId;
    this.loopCount = 0;
    this.currentFrameIndex = 0;
    this.state = "loading";

    // The new playback identity is created before any switch callback or IPC
    // delivery. Never associate a new command with the previous playback id.
    if (oldKey !== action.key || this.currentPlaybackId !== playbackInstanceId) {
      this.callbacks.onActionSwitch?.(action.key, oldKey, playbackInstanceId);
    }

    const delivered = this.sendPlayCommand(action, commandId, playbackInstanceId, context);
    if (!delivered) {
      return;
    }
    return { commandId, playbackInstanceId };
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

  handlePlaybackEvent(event: PlaybackEvent): void {
    const eventPlaybackId = event.playbackInstanceId ?? "";
    switch (event.type) {
      case "playback.action_started": {
        if (eventPlaybackId && this.pendingPlaybackId && eventPlaybackId !== this.pendingPlaybackId) {
          // A late start from a superseded generation must not become current.
          return;
        }
        if (event.actionKey) {
          const newAction = this.loaded?.actions.get(event.actionKey) ?? null;
          if (newAction) {
            this.currentAction = newAction;
          }
        }
        this.currentPlaybackId = eventPlaybackId || this.pendingPlaybackId;
        this.currentCommandId = event.commandId ?? this.pendingCommandId;
        this.pendingAction = null;
        this.pendingPlaybackId = null;
        this.pendingCommandId = null;
        this.state = "playing";
        break;
      }
      case "playback.action_completed": {
        const matchesCurrent = this.matchesTerminalPlayback(event);
        if (!matchesCurrent) return;
        this.loopCount += 1;
        const key = event.actionKey ?? this.currentAction?.key ?? "";
        if (key) {
          this.callbacks.onActionCompleted?.(key, this.loopCount, eventPlaybackId || this.currentPlaybackId || "");
        }
        this.state = "stopped";
        this.clearCurrentPlaybackState();
        break;
      }
      case "playback.action_interrupted": {
        const matchesCurrent = this.matchesTerminalPlayback(event);
        if (!matchesCurrent) return;
        const key = event.actionKey ?? this.currentAction?.key ?? "";
        if (key) {
          this.callbacks.onActionInterrupted?.(key, this.loopCount, eventPlaybackId || this.currentPlaybackId || "");
        }
        this.state = "stopped";
        this.clearCurrentPlaybackState();
        break;
      }
      case "playback.action_failed": {
        const isPending = !!eventPlaybackId && eventPlaybackId === this.pendingPlaybackId;
        const isCurrent = this.matchesTerminalPlayback(event);
        if (!isPending && !isCurrent && eventPlaybackId) return;
        this.state = "stopped";
        // Renderer lifecycle events are reported to Runtime v2 exclusively by
        // DesktopPetManager.handlePlaybackEvent(). Calling onActionFailed here
        // would cause the same renderer/synthetic failed event to be reported
        // twice. The callback is reserved for bridge-local failures that cannot
        // produce a renderer lifecycle event (for example sendStop rejection).
        if (isPending) this.clearPendingPlaybackState();
        if (isCurrent) this.clearCurrentPlaybackState();
        if (event.error?.message || event.reason) {
          this.reportError(new Error(`PLAYBACK_FAILED: ${event.error?.message ?? event.reason}`));
        }
        break;
      }
      case "playback.action_holding":
        if (!eventPlaybackId || eventPlaybackId === this.currentPlaybackId) {
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
    playbackInstanceId: string,
    context?: PlayerSwitchContext,
  ): boolean {
    if (!this.animationIpc) {
      this.state = "stopped";
      this.clearPendingPlaybackState();
      this.reportError(new Error("ANIMATION_IPC_UNAVAILABLE"));
      return false;
    }
    const command: PlayActionCommand = {
      commandId,
      playbackInstanceId,
      idempotencyKey: context?.idempotencyKey?.trim()
        || createHash("sha256").update(`${commandId}:${playbackInstanceId}`).digest("hex").slice(0, 32),
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
      this.state = "stopped";
      this.clearPendingPlaybackState();
      this.reportError(
        new Error(`DELIVERY_FAILED: ${result.reason}${result.error ? ` (${result.error})` : ""}`),
      );
      return false;
    }
    return true;
  }

  private matchesTerminalPlayback(event: PlaybackEvent): boolean {
    if (event.playbackInstanceId) {
      return event.playbackInstanceId === this.currentPlaybackId;
    }
    if (event.commandId && this.currentCommandId) {
      return event.commandId === this.currentCommandId;
    }
    return !!this.currentAction;
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
