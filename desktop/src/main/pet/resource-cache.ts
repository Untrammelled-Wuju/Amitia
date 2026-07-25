import { nativeImage } from "electron";
import { access } from "node:fs/promises";
import type { LoadedInstallation, RuntimeAction } from "./resource-loader";

const DEFAULT_MAX_ACTIONS = 5;
const DEFAULT_MAX_FRAMES_PER_ACTION = 60;

export interface CachedFrame {
  actionKey: string;
  frameIndex: number;
  dataURL: string;
  width: number;
  height: number;
  alphaData?: Uint8Array;
}

export interface ResourceCacheStats {
  cachedActions: number;
  totalFrames: number;
  maxActions: number;
}

export class ResourceCache {
  private readonly maxActions: number;
  private readonly maxFramesPerAction: number;
  private readonly actionCache: Map<string, CachedFrame[]> = new Map();
  private readonly lruOrder: string[] = [];
  private readonly preloadQueue: Set<string> = new Set();
  private readonly frameLru: Map<string, number[]> = new Map();

  constructor(maxActions?: number, maxFramesPerAction?: number) {
    this.maxActions = this.normalizePositiveNumber(
      maxActions,
      DEFAULT_MAX_ACTIONS,
    );
    this.maxFramesPerAction = this.normalizePositiveNumber(
      maxFramesPerAction,
      DEFAULT_MAX_FRAMES_PER_ACTION,
    );
  }

  async preloadDefaultIdle(loaded: LoadedInstallation): Promise<void> {
    const action = loaded.defaultAction;
    if (!action || !action.available) return;
    await this.preloadAction(loaded, action.key);
  }

  async preloadClickActions(
    loaded: LoadedInstallation,
    actionKeys: string[],
  ): Promise<void> {
    if (!Array.isArray(actionKeys)) return;
    for (const key of actionKeys) {
      if (!key) continue;
      const action = loaded.actions.get(key);
      if (!action || !action.available) continue;
      await this.preloadAction(loaded, key);
    }
  }

  async preloadAction(
    loaded: LoadedInstallation,
    actionKey: string,
  ): Promise<void> {
    if (!actionKey) return;
    if (this.actionCache.has(actionKey)) {
      this.touchLRU(actionKey);
      return;
    }
    if (this.preloadQueue.has(actionKey)) return;

    const action = loaded.actions.get(actionKey);
    if (!action || !action.available) return;

    this.preloadQueue.add(actionKey);
    try {
      const frames = await this.loadActionFrames(
        action,
        this.maxFramesPerAction,
      );
      if (frames.length === 0) return;
      this.ensureActionCapacity(1);
      const arr: CachedFrame[] = [];
      const lru: number[] = [];
      for (const frame of frames) {
        arr[frame.frameIndex] = frame;
        lru.push(frame.frameIndex);
      }
      this.actionCache.set(actionKey, arr);
      this.frameLru.set(actionKey, lru);
      this.touchLRU(actionKey);
    } finally {
      this.preloadQueue.delete(actionKey);
    }
  }

  async getFrame(
    loaded: LoadedInstallation,
    actionKey: string,
    frameIndex: number,
  ): Promise<CachedFrame | null> {
    if (!actionKey || frameIndex < 0) return null;

    const action = loaded.actions.get(actionKey);
    if (!action || !action.available) return null;
    if (frameIndex >= action.frames.length) return null;

    let arr = this.actionCache.get(actionKey);
    if (arr && arr[frameIndex]) {
      this.touchLRU(actionKey);
      this.touchFrameLRU(actionKey, frameIndex);
      return arr[frameIndex] ?? null;
    }

    const frame = await this.loadFrame(action, actionKey, frameIndex);
    if (!frame) return null;

    if (!arr) {
      this.ensureActionCapacity(1);
      arr = [];
      this.actionCache.set(actionKey, arr);
      this.frameLru.set(actionKey, []);
    }

    if (!arr[frameIndex]) {
      const count = this.countFrames(arr);
      if (count >= this.maxFramesPerAction) {
        this.evictOldestFrame(actionKey, arr);
      }
    }

    arr[frameIndex] = frame;
    this.touchLRU(actionKey);
    this.touchFrameLRU(actionKey, frameIndex);
    return frame;
  }

  hasAction(actionKey: string): boolean {
    return this.actionCache.has(actionKey);
  }

  evictAction(actionKey: string): void {
    if (!actionKey) return;
    if (!this.actionCache.delete(actionKey)) return;
    this.frameLru.delete(actionKey);
    const idx = this.lruOrder.indexOf(actionKey);
    if (idx >= 0) this.lruOrder.splice(idx, 1);
  }

  release(): void {
    this.actionCache.clear();
    this.frameLru.clear();
    this.lruOrder.length = 0;
    this.preloadQueue.clear();
  }

  getStats(): ResourceCacheStats {
    let total = 0;
    for (const arr of this.actionCache.values()) {
      total += this.countFrames(arr);
    }
    return {
      cachedActions: this.actionCache.size,
      totalFrames: total,
      maxActions: this.maxActions,
    };
  }

  private async loadActionFrames(
    action: RuntimeAction,
    limit: number,
  ): Promise<CachedFrame[]> {
    const total = Math.min(action.frames.length, limit);
    const result: CachedFrame[] = [];
    for (let i = 0; i < total; i++) {
      const frame = await this.loadFrame(action, action.key, i);
      if (frame) result.push(frame);
    }
    return result;
  }

  private async loadFrame(
    action: RuntimeAction,
    actionKey: string,
    frameIndex: number,
  ): Promise<CachedFrame | null> {
    const framePath = action.frames[frameIndex];
    if (!framePath) return null;

    try {
      await access(framePath);
    } catch {
      return null;
    }

    try {
      const image = nativeImage.createFromPath(framePath);
      if (image.isEmpty()) return null;
      const size = image.getSize();
      if (size.width <= 0 || size.height <= 0) return null;
      const dataURL = image.toDataURL();
      const bitmap = image.toBitmap();
      const alphaData = this.extractAlpha(bitmap, size.width, size.height);
      return {
        actionKey,
        frameIndex,
        dataURL,
        width: size.width,
        height: size.height,
        alphaData,
      };
    } catch {
      return null;
    }
  }

  private extractAlpha(
    bitmap: Buffer,
    width: number,
    height: number,
  ): Uint8Array {
    const pixelCount = width * height;
    const alpha = new Uint8Array(pixelCount);
    const byteLength = Math.min(bitmap.length, pixelCount * 4);
    for (let i = 0, p = 0; i + 3 < byteLength; i += 4, p++) {
      alpha[p] = bitmap[i + 3];
    }
    return alpha;
  }

  private touchLRU(actionKey: string): void {
    const idx = this.lruOrder.indexOf(actionKey);
    if (idx >= 0) this.lruOrder.splice(idx, 1);
    this.lruOrder.push(actionKey);
  }

  private touchFrameLRU(actionKey: string, frameIndex: number): void {
    let lru = this.frameLru.get(actionKey);
    if (!lru) {
      lru = [];
      this.frameLru.set(actionKey, lru);
    }
    const idx = lru.indexOf(frameIndex);
    if (idx >= 0) lru.splice(idx, 1);
    lru.push(frameIndex);
  }

  private ensureActionCapacity(needed: number): void {
    while (
      this.actionCache.size + needed > this.maxActions &&
      this.lruOrder.length > 0
    ) {
      const oldest = this.lruOrder[0];
      if (oldest === undefined) break;
      this.lruOrder.shift();
      this.actionCache.delete(oldest);
      this.frameLru.delete(oldest);
    }
  }

  private evictOldestFrame(actionKey: string, arr: CachedFrame[]): void {
    const lru = this.frameLru.get(actionKey);
    if (!lru || lru.length === 0) return;
    const oldest = lru.shift();
    if (typeof oldest === "number") {
      delete arr[oldest];
    }
  }

  private countFrames(arr: CachedFrame[]): number {
    let count = 0;
    for (const frame of arr) {
      if (frame) count++;
    }
    return count;
  }

  private normalizePositiveNumber(
    value: number | undefined,
    fallback: number,
  ): number {
    if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
      return fallback;
    }
    return Math.floor(value);
  }
}
