import type { PlaybackClock } from "./contracts";

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

const FALLBACK_INTERVAL_MS = 1000 / 60;

interface ScheduledHandle {
  cancel: () => void;
}

export class MonotonicPlaybackClock implements PlaybackClock {
  private counter = 0;
  private handles = new Map<number, ScheduledHandle>();

  now(): number {
    if (typeof performance !== "undefined" && typeof performance.now === "function") {
      return performance.now();
    }
    return Date.now();
  }

  requestTick(callback: (now: number) => void): number {
    this.counter += 1;
    const id = this.counter;

    if (nativeRaf && nativeCaf) {
      const nativeId = nativeRaf((ts: number) => {
        this.handles.delete(id);
        callback(ts);
      });
      this.handles.set(id, {
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
      this.handles.delete(id);
      callback(this.now());
    }, FALLBACK_INTERVAL_MS);
    this.handles.set(id, {
      cancel: () => {
        clearTimeout(timer);
      },
    });
    return id;
  }

  cancelTick(handle: number): void {
    const h = this.handles.get(handle);
    if (!h) return;
    this.handles.delete(handle);
    h.cancel();
  }

  cancelAll(): void {
    for (const h of this.handles.values()) {
      h.cancel();
    }
    this.handles.clear();
  }
}

interface FakeTick {
  callback: (now: number) => void;
  handle: number;
}

export class FakePlaybackClock implements PlaybackClock {
  private currentTime = 0;
  private counter = 0;
  private pendingTicks: FakeTick[] = [];

  now(): number {
    return this.currentTime;
  }

  requestTick(callback: (now: number) => void): number {
    this.counter += 1;
    const handle = this.counter;
    this.pendingTicks.push({ callback, handle });
    return handle;
  }

  cancelTick(handle: number): void {
    this.pendingTicks = this.pendingTicks.filter((t) => t.handle !== handle);
  }

  advance(ms: number): void {
    if (ms < 0) return;
    this.currentTime += ms;
    const ticksToFire = [...this.pendingTicks];
    this.pendingTicks = [];
    for (const tick of ticksToFire) {
      tick.callback(this.currentTime);
    }
  }

  advanceTo(time: number): void {
    if (time <= this.currentTime) return;
    this.advance(time - this.currentTime);
  }

  reset(): void {
    this.currentTime = 0;
    this.counter = 0;
    this.pendingTicks = [];
  }

  getPendingCount(): number {
    return this.pendingTicks.length;
  }
}
