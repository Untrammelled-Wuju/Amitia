import { createHash, randomUUID } from "node:crypto";
import type { LoadedInstallation, RuntimeAction } from "./resource-loader";
import type { PlayerLike, PlayerState } from "./action-player";
import type { AnimationIpcAdapter } from "./animation-ipc";
import type { PlayActionCommand, LoopType, ReturnTarget } from "../../desktop-pet/animation/contracts";

function toLoopType(loopType: string): LoopType {
  if (loopType === "loop" || loopType === "once" || loopType === "hold" || loopType === "ping_pong") {
    return loopType;
  }
  return "loop";
}

function buildReturnTarget(action: RuntimeAction): ReturnTarget {
  if (action.returnAction) {
    return { type: "action", actionKey: action.returnAction };
  }
  return { type: "default" };
}

export interface AnimationPlayerBridgeCallbacks {
  onActionSwitch?: (newActionKey: string, oldActionKey: string | null) => void;
  onActionComplete?: (actionKey: string, loopCount: number) => void;
  onError?: (error: Error) => void;
}

export class AnimationPlayerBridge implements PlayerLike {
  private currentAction: RuntimeAction | null = null;
  private state: PlayerState = "idle";
  private loopCount = 0;
  private loaded: LoadedInstallation | null = null;
  private readonly sustainedActionMap: Map<string, string> = new Map();
  private callbacks: AnimationPlayerBridgeCallbacks;
  private animationIpc: AnimationIpcAdapter | null = null;
  private installationId = "";
  private petInstanceId = "";
  private packageRevision = 0;

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
  }

  setSustainedActionMap(map: Record<string, string>): void {
    this.sustainedActionMap.clear();
    for (const [key, value] of Object.entries(map)) {
      if (key && value) {
        this.sustainedActionMap.set(key, value);
      }
    }
  }

  play(action: RuntimeAction): void {
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

    const oldKey = this.currentAction?.key ?? null;
    this.currentAction = action;
    this.loopCount = 0;
    this.state = "playing";

    if (oldKey !== action.key) {
      this.callbacks.onActionSwitch?.(action.key, oldKey);
    }

    this.sendPlayCommand(action);
  }

  pause(): void {
    if (this.state !== "playing") return;
    this.state = "paused";
    this.animationIpc?.sendPause();
  }

  resume(): void {
    if (this.state !== "paused") return;
    if (!this.currentAction) return;
    this.state = "playing";
    this.animationIpc?.sendResume();
  }

  stop(): void {
    this.state = "stopped";
    this.loopCount = 0;
    this.animationIpc?.sendStop();
  }

  switchAction(action: RuntimeAction): void {
    const oldKey = this.currentAction?.key ?? null;
    const newKey = action?.key ?? null;
    if (oldKey !== newKey) {
      this.callbacks.onActionSwitch?.(newKey ?? "", oldKey);
    }
    this.play(action);
  }

  getCurrentAction(): RuntimeAction | null {
    return this.currentAction;
  }

  getCurrentFrameIndex(): number {
    return 0;
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

  handlePlaybackEvent(event: { type: string; actionKey?: string; reason?: string }): void {
    switch (event.type) {
      case "playback.action_started":
        if (event.actionKey && event.actionKey !== this.currentAction?.key) {
          const oldKey = this.currentAction?.key ?? null;
          const loaded = this.loaded;
          const newAction = loaded?.actions.get(event.actionKey) ?? null;
          if (newAction) {
            this.currentAction = newAction;
            this.callbacks.onActionSwitch?.(newAction.key, oldKey);
          }
        }
        break;
      case "playback.action_completed":
        this.loopCount += 1;
        if (this.currentAction) {
          this.callbacks.onActionComplete?.(this.currentAction.key, this.loopCount);
        }
        if (this.currentAction?.loopType === "once") {
          this.state = "stopped";
        }
        break;
      case "playback.action_interrupted":
        if (this.currentAction) {
          this.callbacks.onActionComplete?.(this.currentAction.key, this.loopCount);
        }
        this.state = "stopped";
        break;
      case "playback.action_failed":
        this.state = "stopped";
        if (event.reason) {
          this.reportError(new Error(`PLAYBACK_FAILED: ${event.reason}`));
        }
        break;
      case "playback.action_holding":
        this.state = "paused";
        break;
    }
  }

  private sendPlayCommand(action: RuntimeAction): void {
    if (!this.animationIpc) return;
    const command: PlayActionCommand = {
      commandId: randomUUID(),
      idempotencyKey: createHash("sha256").update(`${action.key}-${Date.now()}`).digest("hex").slice(0, 16),
      installationId: this.installationId,
      petInstanceId: this.petInstanceId,
      packageRevision: this.packageRevision,
      actionKey: action.key,
      priority: 50,
      queuePolicy: "replace_current",
      interruptPolicy: "force_system",
      playbackRate: 1.0,
      issuedAt: new Date().toISOString(),
      returnOverride: buildReturnTarget(action),
    };
    this.animationIpc.sendPlayAction(command);
  }

  private reportError(error: Error): void {
    if (!this.callbacks.onError) return;
    try {
      this.callbacks.onError(error);
    } catch {
      void 0;
    }
  }
}
