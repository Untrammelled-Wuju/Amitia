import {
  CacheEntry,
  DecodedFrame,
  DEFAULT_CACHE_BUDGET_BYTES,
} from "../contracts";

export interface CacheStats {
  budgetBytes: number;
  usedBytes: number;
  entries: number;
  hitRate: number;
}

export class DecodedFrameCache {
  private entries: Map<string, CacheEntry>;
  private revisionMap: Map<string, number>;
  private budgetBytes: number;
  private usedBytes: number;
  private hits: number;
  private misses: number;

  constructor(input?: { budgetBytes?: number }) {
    this.entries = new Map();
    this.revisionMap = new Map();
    this.budgetBytes = input?.budgetBytes ?? DEFAULT_CACHE_BUDGET_BYTES;
    this.usedBytes = 0;
    this.hits = 0;
    this.misses = 0;
  }

  get(key: string): DecodedFrame | null {
    const entry = this.entries.get(key);
    if (!entry) {
      this.misses++;
      return null;
    }
    entry.lastAccessedMs = Date.now();
    this.hits++;
    return entry.frame;
  }

  put(
    key: string,
    frame: DecodedFrame,
    estimatedBytes: number,
    revision?: number,
  ): void {
    const existing = this.entries.get(key);
    if (existing) {
      this.usedBytes -= existing.estimatedBytes;
    }

    const entry: CacheEntry = {
      key,
      frame,
      estimatedBytes,
      lastAccessedMs: Date.now(),
      refCount: existing?.refCount ?? 0,
    };
    this.entries.set(key, entry);
    this.usedBytes += estimatedBytes;

    if (revision !== undefined) {
      this.revisionMap.set(key, revision);
    }

    if (this.usedBytes > this.budgetBytes) {
      this.evictTo(this.budgetBytes);
    }
  }

  retain(key: string): void {
    const entry = this.entries.get(key);
    if (entry) {
      entry.refCount++;
    }
  }

  release(key: string): void {
    const entry = this.entries.get(key);
    if (entry) {
      entry.refCount = Math.max(0, entry.refCount - 1);
    }
  }

  evictTo(targetBytes: number): void {
    const sorted = [...this.entries.entries()].sort(
      (a, b) => a[1].lastAccessedMs - b[1].lastAccessedMs,
    );
    for (const [key, entry] of sorted) {
      if (this.usedBytes <= targetBytes) break;
      if (entry.refCount > 0) continue;
      this.closeBitmap(entry.frame);
      this.usedBytes -= entry.estimatedBytes;
      this.entries.delete(key);
      this.revisionMap.delete(key);
    }
  }

  clearRevision(revision: number): void {
    const keysToRemove: string[] = [];
    for (const [key, rev] of this.revisionMap) {
      if (rev === revision) {
        keysToRemove.push(key);
      }
    }
    for (const key of keysToRemove) {
      const entry = this.entries.get(key);
      if (entry) {
        this.closeBitmap(entry.frame);
        this.usedBytes -= entry.estimatedBytes;
        this.entries.delete(key);
      }
      this.revisionMap.delete(key);
    }
  }

  clear(): void {
    for (const entry of this.entries.values()) {
      this.closeBitmap(entry.frame);
    }
    this.entries.clear();
    this.revisionMap.clear();
    this.usedBytes = 0;
  }

  getUsedBytes(): number {
    return this.usedBytes;
  }

  getEntryCount(): number {
    return this.entries.size;
  }

  setBudget(bytes: number): void {
    this.budgetBytes = bytes;
    if (this.usedBytes > this.budgetBytes) {
      this.evictTo(this.budgetBytes);
    }
  }

  getStats(): CacheStats {
    const total = this.hits + this.misses;
    return {
      budgetBytes: this.budgetBytes,
      usedBytes: this.usedBytes,
      entries: this.entries.size,
      hitRate: total > 0 ? this.hits / total : 0,
    };
  }

  private closeBitmap(frame: DecodedFrame): void {
    if (
      typeof ImageBitmap !== "undefined" &&
      frame.bitmap instanceof ImageBitmap
    ) {
      (frame.bitmap as ImageBitmap).close();
    }
  }
}
