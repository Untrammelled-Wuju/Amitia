import { nativeImage } from "electron";
import { access } from "node:fs/promises";
import { relative } from "node:path";
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
  contentHash: string;
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
  private currentReleaseId: string | null = null;

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

  private getReleaseId(loaded: LoadedInstallation): string {
    return loaded.manifest.releaseId ?? loaded.manifest.packageId ?? "default";
  }

  private getCacheKey(releaseId: string, actionKey: string): string {
    return `${releaseId}:${actionKey}`;
  }

  private ensureReleaseId(releaseId: string): void {
    if (this.currentReleaseId !== null && this.currentReleaseId !== releaseId) {
      this.actionCache.clear();
      this.frameLru.clear();
      this.lruOrder.length = 0;
      this.preloadQueue.clear();
    }
    this.currentReleaseId = releaseId;
  }

  private buildContentHashLookup(loaded: LoadedInstallation): Map<string, string> {
    const lookup = new Map<string, string>();
    const files = loaded.manifest.integrity?.files;
    if (!files || !Array.isArray(files)) return lookup;
    for (const entry of files) {
      if (entry && typeof entry.path === "string" && typeof entry.sha256 === "string") {
        const normalizedPath = entry.path.replace(/\\/g, "/").replace(/^\/+/, "");
        lookup.set(normalizedPath, entry.sha256);
      }
    }
    return lookup;
  }

  private resolveContentHash(
    framePath: string,
    installPath: string,
    contentHashLookup: Map<string, string>,
  ): string {
    const relPath = relative(installPath, framePath).replace(/\\/g, "/");
    return contentHashLookup.get(relPath) ?? "";
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
    const releaseId = this.getReleaseId(loaded);
    this.ensureReleaseId(releaseId);
    const cacheKey = this.getCacheKey(releaseId, actionKey);

    if (this.actionCache.has(cacheKey)) {
      this.touchLRU(cacheKey);
      return;
    }
    if (this.preloadQueue.has(cacheKey)) return;

    const action = loaded.actions.get(actionKey);
    if (!action || !action.available) return;

    this.preloadQueue.add(cacheKey);
    try {
      const contentHashLookup = this.buildContentHashLookup(loaded);
      const frames = await this.loadActionFrames(
        action,
        this.maxFramesPerAction,
        contentHashLookup,
        loaded.installPath,
      );
      if (frames.length === 0) return;
      this.ensureActionCapacity(1);
      const arr: CachedFrame[] = [];
      const lru: number[] = [];
      for (const frame of frames) {
        arr[frame.frameIndex] = frame;
        lru.push(frame.frameIndex);
      }
      this.actionCache.set(cacheKey, arr);
      this.frameLru.set(cacheKey, lru);
      this.touchLRU(cacheKey);
    } finally {
      this.preloadQueue.delete(cacheKey);
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

    const releaseId = this.getReleaseId(loaded);
    this.ensureReleaseId(releaseId);
    const cacheKey = this.getCacheKey(releaseId, actionKey);

    let arr = this.actionCache.get(cacheKey);
    if (arr && arr[frameIndex]) {
      this.touchLRU(cacheKey);
      this.touchFrameLRU(cacheKey, frameIndex);
      return arr[frameIndex] ?? null;
    }

    const contentHashLookup = this.buildContentHashLookup(loaded);
    const framePath = action.frames[frameIndex];
    const contentHash = this.resolveContentHash(framePath, loaded.installPath, contentHashLookup);
    const frame = await this.loadFrame(action, actionKey, frameIndex, contentHash);
    if (!frame) return null;

    if (!arr) {
      this.ensureActionCapacity(1);
      arr = [];
      this.actionCache.set(cacheKey, arr);
      this.frameLru.set(cacheKey, []);
    }

    if (!arr[frameIndex]) {
      const count = this.countFrames(arr);
      if (count >= this.maxFramesPerAction) {
        this.evictOldestFrame(cacheKey, arr);
      }
    }

    arr[frameIndex] = frame;
    this.touchLRU(cacheKey);
    this.touchFrameLRU(cacheKey, frameIndex);
    return frame;
  }

  hasAction(actionKey: string, releaseId?: string): boolean {
    const rid = releaseId ?? this.currentReleaseId ?? "default";
    const cacheKey = this.getCacheKey(rid, actionKey);
    return this.actionCache.has(cacheKey);
  }

  evictAction(actionKey: string, releaseId?: string): void {
    if (!actionKey) return;
    const rid = releaseId ?? this.currentReleaseId ?? "default";
    const cacheKey = this.getCacheKey(rid, actionKey);
    if (!this.actionCache.delete(cacheKey)) return;
    this.frameLru.delete(cacheKey);
    const idx = this.lruOrder.indexOf(cacheKey);
    if (idx >= 0) this.lruOrder.splice(idx, 1);
  }

  release(): void {
    this.actionCache.clear();
    this.frameLru.clear();
    this.lruOrder.length = 0;
    this.preloadQueue.clear();
    this.currentReleaseId = null;
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
    contentHashLookup: Map<string, string>,
    installPath: string,
  ): Promise<CachedFrame[]> {
    const total = Math.min(action.frames.length, limit);
    const result: CachedFrame[] = [];
    for (let i = 0; i < total; i++) {
      const framePath = action.frames[i];
      const contentHash = this.resolveContentHash(framePath, installPath, contentHashLookup);
      const frame = await this.loadFrame(action, action.key, i, contentHash);
      if (frame) result.push(frame);
    }
    return result;
  }

  private async loadFrame(
    action: RuntimeAction,
    actionKey: string,
    frameIndex: number,
    contentHash: string,
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
        contentHash,
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

  private touchLRU(cacheKey: string): void {
    const idx = this.lruOrder.indexOf(cacheKey);
    if (idx >= 0) this.lruOrder.splice(idx, 1);
    this.lruOrder.push(cacheKey);
  }

  private touchFrameLRU(cacheKey: string, frameIndex: number): void {
    let lru = this.frameLru.get(cacheKey);
    if (!lru) {
      lru = [];
      this.frameLru.set(cacheKey, lru);
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

  private evictOldestFrame(cacheKey: string, arr: CachedFrame[]): void {
    const lru = this.frameLru.get(cacheKey);
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
