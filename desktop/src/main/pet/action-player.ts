import type { LoadedInstallation, RuntimeAction } from "./resource-loader";

export type PlayerState = "idle" | "playing" | "paused" | "stopped";

export interface PlayerCallbacks {
  onFrameChange?: (actionKey: string, frameIndex: number) => void;
  onActionComplete?: (actionKey: string, loopCount: number) => void;
  onActionSwitch?: (newActionKey: string, oldActionKey: string | null) => void;
  onError?: (error: Error) => void;
}

export interface PlayerLike {
  attachLoaded(loaded: LoadedInstallation): void;
  detachLoaded(): void;
  setSustainedActionMap(map: Record<string, string>): void;
  play(action: RuntimeAction): void;
  pause(): void;
  resume(): void;
  stop(): void;
  switchAction(action: RuntimeAction): void;
  getCurrentAction(): RuntimeAction | null;
  getCurrentFrameIndex(): number;
  getState(): PlayerState;
  getLoopCount(): number;
  getFallbackChain(action: RuntimeAction, loaded?: LoadedInstallation | null): RuntimeAction[];
}

const FRAME_INTERVAL_MS = 1000 / 60;
const MAX_FRAME_CATCHUP_PER_TICK = 64;

type GlobalWithRaf = typeof globalThis & {
  requestAnimationFrame?: (cb: (ts: number) => void) => number;
  cancelAnimationFrame?: (handle: number) => void;
};

const globalWithRaf = globalThis as GlobalWithRaf;
const nativeRaf =
  typeof globalWithRaf.requestAnimationFrame === "function"
    ? globalWithRaf.requestAnimationFrame.bind(globalWithRaf)
    : null;
const nativeCaf =
  typeof globalWithRaf.cancelAnimationFrame === "function"
    ? globalWithRaf.cancelAnimationFrame.bind(globalWithRaf)
    : null;

interface ScheduledFrameHandle {
  cancel: () => void;
}

let rafCounter = 0;
const rafHandles = new Map<number, ScheduledFrameHandle>();

function nowTimestamp(): number {
  if (
    typeof performance !== "undefined" &&
    typeof performance.now === "function"
  ) {
    return performance.now();
  }
  return Date.now();
}

function scheduleRaf(callback: (timestamp: number) => void): number {
  rafCounter += 1;
  const id = rafCounter;

  if (nativeRaf && nativeCaf) {
    const nativeId = nativeRaf((ts: number) => {
      rafHandles.delete(id);
      callback(ts);
    });
    rafHandles.set(id, {
      cancel: () => {
        try {
          nativeCaf(nativeId);
        } catch {
          void 0;
        }
      },
    });
    return id;
  }

  const timer = setTimeout(() => {
    rafHandles.delete(id);
    callback(nowTimestamp());
  }, FRAME_INTERVAL_MS);
  rafHandles.set(id, {
    cancel: () => {
      clearTimeout(timer);
    },
  });
  return id;
}

function cancelScheduledRaf(id: number | null): void {
  if (id == null) return;
  const handle = rafHandles.get(id);
  if (!handle) return;
  rafHandles.delete(id);
  handle.cancel();
}

export class ActionPlayer implements PlayerLike {
  private currentAction: RuntimeAction | null = null;
  private currentFrameIndex = 0;
  private lastTimestamp = 0;
  private accumulatedTime = 0;
  private state: PlayerState = "idle";
  private loopCount = 0;
  private rafId: number | null = null;
  private callbacks: PlayerCallbacks;
  private loaded: LoadedInstallation | null = null;
  private readonly sustainedActionMap: Map<string, string> = new Map();

  constructor(callbacks?: PlayerCallbacks) {
    this.callbacks = callbacks ?? {};
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
    if (action.frameCount <= 0 || action.frameDurationMs <= 0) {
      this.reportError(new Error(`ACTION_INVALID_FRAMES: ${action.key}`));
      return;
    }

    this.cancelScheduledFrame();
    this.currentAction = action;
    this.currentFrameIndex = 0;
    this.accumulatedTime = 0;
    this.loopCount = 0;
    this.lastTimestamp = nowTimestamp();
    this.state = "playing";
    this.emitFrameChange();
    this.startLoop();
  }

  pause(): void {
    if (this.state !== "playing") return;
    this.state = "paused";
    this.cancelScheduledFrame();
  }

  resume(): void {
    if (this.state !== "paused") return;
    if (!this.currentAction) return;
    this.state = "playing";
    this.lastTimestamp = nowTimestamp();
    this.startLoop();
  }

  stop(): void {
    this.cancelScheduledFrame();
    this.state = "stopped";
    this.currentFrameIndex = 0;
    this.loopCount = 0;
    this.accumulatedTime = 0;
  }

  switchAction(action: RuntimeAction): void {
    const oldKey = this.currentAction?.key ?? null;
    const newKey = action?.key ?? null;
    if (oldKey !== newKey && this.callbacks.onActionSwitch) {
      try {
        this.callbacks.onActionSwitch(newKey ?? "", oldKey);
      } catch (err) {
        this.reportError(this.wrapError("onActionSwitch", err));
      }
    }
    this.play(action);
  }

  getCurrentAction(): RuntimeAction | null {
    return this.currentAction;
  }

  getCurrentFrameIndex(): number {
    return this.currentFrameIndex;
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

  private startLoop(): void {
    this.cancelScheduledFrame();
    this.rafId = scheduleRaf((ts) => this.tick(ts));
  }

  private tick(timestamp: number): void {
    if (this.state !== "playing") return;
    const action = this.currentAction;
    if (!action) return;

    const delta = timestamp - this.lastTimestamp;
    this.lastTimestamp = timestamp;
    if (delta > 0) {
      this.accumulatedTime += delta;
    }

    const frameDurationMs = action.frameDurationMs;
    if (frameDurationMs <= 0) {
      this.reportError(
        new Error(`ACTION_INVALID_FRAME_DURATION: ${action.key}`),
      );
      this.stop();
      return;
    }

    let advanced = false;
    let catchup = 0;
    const rafIdBefore = this.rafId;
    while (
      this.accumulatedTime >= frameDurationMs &&
      catchup < MAX_FRAME_CATCHUP_PER_TICK
    ) {
      this.accumulatedTime -= frameDurationMs;
      advanced = true;
      catchup += 1;
      this.advanceFrame(action);
      if (this.state !== "playing") break;
    }

    if (this.accumulatedTime >= frameDurationMs) {
      this.accumulatedTime = this.accumulatedTime % frameDurationMs;
    }

    if (advanced && this.state === "playing") {
      this.emitFrameChange();
    }

    if (this.state === "playing" && this.rafId === rafIdBefore) {
      this.rafId = scheduleRaf((ts) => this.tick(ts));
    }
  }

  private advanceFrame(action: RuntimeAction): void {
    const frameCount = action.frameCount;
    if (frameCount <= 0) return;
    const next = this.currentFrameIndex + 1;

    if (action.loopType === "loop") {
      if (next >= frameCount) {
        this.currentFrameIndex = 0;
        this.loopCount += 1;
      } else {
        this.currentFrameIndex = next;
      }
      return;
    }

    if (action.loopType === "once") {
      if (next >= frameCount) {
        this.loopCount += 1;
        const actionBefore = this.currentAction;
        this.emitActionComplete();
        if (this.currentAction === actionBefore) {
          this.state = "stopped";
          this.cancelScheduledFrame();
        }
        return;
      }
      this.currentFrameIndex = next;
      return;
    }

    if (next >= frameCount) {
      this.currentFrameIndex = frameCount - 1;
      this.loopCount += 1;
      const actionBefore = this.currentAction;
      this.emitActionComplete();
      if (this.currentAction === actionBefore) {
        this.state = "paused";
        this.cancelScheduledFrame();
      }
      return;
    }
    this.currentFrameIndex = next;
  }

  private cancelScheduledFrame(): void {
    if (this.rafId != null) {
      cancelScheduledRaf(this.rafId);
      this.rafId = null;
    }
  }

  private emitFrameChange(): void {
    if (!this.callbacks.onFrameChange) return;
    const action = this.currentAction;
    if (!action) return;
    try {
      this.callbacks.onFrameChange(action.key, this.currentFrameIndex);
    } catch (err) {
      this.reportError(this.wrapError("onFrameChange", err));
    }
  }

  private emitActionComplete(): void {
    if (!this.callbacks.onActionComplete) return;
    const action = this.currentAction;
    if (!action) return;
    try {
      this.callbacks.onActionComplete(action.key, this.loopCount);
    } catch (err) {
      this.reportError(this.wrapError("onActionComplete", err));
    }
  }

  private reportError(error: Error): void {
    if (!this.callbacks.onError) return;
    try {
      this.callbacks.onError(error);
    } catch {
      void 0;
    }
  }

  private wrapError(label: string, err: unknown): Error {
    if (err instanceof Error) return err;
    return new Error(`${label}: ${String(err)}`);
  }
}
