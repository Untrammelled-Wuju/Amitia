import type { LoadedInstallation, RuntimeAction } from "./resource-loader";
import type { InterruptPolicy, QueuePolicy, ReturnTarget } from "../../desktop-pet/animation/contracts";

export type PlayerState = "idle" | "loading" | "playing" | "paused" | "stopped";

/**
 * Immutable identity and execution semantics passed from the scheduler to the
 * renderer playback command. Runtime-v2 supplied values must not be replaced
 * by bridge defaults while crossing the main-process boundary.
 */
export interface PlayerSwitchContext {
  commandId?: string;
  idempotencyKey?: string;
  priority?: number;
  queuePolicy?: QueuePolicy;
  interruptPolicy?: InterruptPolicy;
  playbackRate?: number;
  requiresAuthoritativeExpiry?: boolean;
  expiresAt?: string;
  returnOverride?: ReturnTarget;
  minimumPlayMs?: number;
  maximumPlayMs?: number | null;
  interruptAfterMs?: number;
  completionPolicy?: string;
  source?: string;
  traceId?: string;
}

export interface PlayerSubmissionIdentity {
  commandId: string;
}

export interface DesktopPetPlayerPort {
  play(action: RuntimeAction, context?: PlayerSwitchContext): PlayerSubmissionIdentity | void;
  pause(): void;
  resume(): void;
  stop(reason?: string): void;
  switchAction(action: RuntimeAction, context?: PlayerSwitchContext): PlayerSubmissionIdentity | void;

  getCurrentAction(): RuntimeAction | null;
  getState(): PlayerState;
  getLoopCount(): number;

  getFallbackChain(
    action: RuntimeAction,
    loaded?: LoadedInstallation | null,
  ): RuntimeAction[];
}

export interface PlayerLifecyclePort {
  attachLoaded(loaded: LoadedInstallation): void;
  detachLoaded(): void;
  setSustainedActionMap(map: Record<string, string>): void;
}
