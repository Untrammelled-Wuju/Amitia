import type { RuntimeIntegrityFile, RuntimeResourceIndexEntry } from "../../shared/package-schema";
import { createPackageError } from "../../shared/package-errors";

export class RuntimeResourceIndex {
  private readonly entryMap: Map<string, RuntimeResourceIndexEntry>;
  private readonly _entries: RuntimeResourceIndexEntry[];

  constructor(files: RuntimeIntegrityFile[]) {
    this.entryMap = new Map<string, RuntimeResourceIndexEntry>();
    this._entries = [];
    for (const f of files) {
      const normalizedPath = normalizePathKey(f.path);
      if (this.entryMap.has(normalizedPath)) {
        continue;
      }
      const entry: RuntimeResourceIndexEntry = {
        path: f.path,
        sha256: f.sha256,
        bytes: f.bytes,
        mediaType: f.mediaType,
        role: f.role,
        actionKey: f.actionKey,
        frameId: f.frameId,
      };
      this.entryMap.set(normalizedPath, entry);
      this._entries.push(entry);
    }
  }

  lookup(path: string): RuntimeResourceIndexEntry | null {
    const key = normalizePathKey(path);
    return this.entryMap.get(key) ?? null;
  }

  has(path: string): boolean {
    return this.entryMap.has(normalizePathKey(path));
  }

  require(path: string): RuntimeResourceIndexEntry {
    const entry = this.lookup(path);
    if (!entry) {
      throw createPackageError(
        "PACKAGE_RESOURCE_NOT_DECLARED",
        `resource not declared in integrity index: ${path}`,
        { path },
      );
    }
    return entry;
  }

  size(): number {
    return this._entries.length;
  }

  list(): RuntimeResourceIndexEntry[] {
    return [...this._entries];
  }

  entries(): IterableIterator<RuntimeResourceIndexEntry> {
    return this.entryMap.values();
  }
}

function normalizePathKey(path: string): string {
  return path.replace(/\\/g, "/").replace(/^\.\/+/, "");
}
