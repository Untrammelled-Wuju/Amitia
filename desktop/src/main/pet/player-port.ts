import type { LoadedInstallation, RuntimeAction } from "./resource-loader";

export type PlayerState = "idle" | "loading" | "playing" | "paused" | "stopped";

export interface DesktopPetPlayerPort {
  play(action: RuntimeAction): void;
  pause(): void;
  resume(): void;
  stop(): void;
  switchAction(action: RuntimeAction): void;

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
