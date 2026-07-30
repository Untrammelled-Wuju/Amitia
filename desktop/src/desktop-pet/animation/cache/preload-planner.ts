import {
  ActionAssetRepository,
  PackagePlaybackSnapshot,
} from "../contracts";
import { PlaybackError } from "../errors";
import { DecodedFrameCache } from "./decoded-frame-cache";

export class PreloadPlanner {
  private cache: DecodedFrameCache;
  private repository: ActionAssetRepository;

  constructor(input: {
    cache: DecodedFrameCache;
    repository: ActionAssetRepository;
  }) {
    this.cache = input.cache;
    this.repository = input.repository;
  }

  planPreload(input: {
    currentActionKey: string;
    defaultActionKey: string;
    previousStableActionKey: string | null;
    queueHeadActionKey: string | null;
    allActionKeys: string[];
    packageSnapshot: PackagePlaybackSnapshot;
  }): string[] {
    const priority: string[] = [];
    const seen = new Set<string>();

    const add = (key: string | null): void => {
      if (
        key !== null &&
        !seen.has(key) &&
        input.allActionKeys.includes(key)
      ) {
        seen.add(key);
        priority.push(key);
      }
    };

    add(input.currentActionKey);
    add(input.defaultActionKey);
    add(input.previousStableActionKey);
    add(input.queueHeadActionKey);

    for (const key of input.allActionKeys) {
      add(key);
    }

    return priority;
  }

  async preload(
    actionKeys: string[],
    packageSnapshot: PackagePlaybackSnapshot,
    signal: AbortSignal,
  ): Promise<void> {
    for (const key of actionKeys) {
      if (signal.aborted) break;

      const stats = this.cache.getStats();
      if (stats.usedBytes >= stats.budgetBytes * 0.9) {
        break;
      }

      try {
        await this.repository.loadAction({
          packageSnapshot,
          actionKey: key,
          signal,
          priority: "low",
        });
      } catch (error) {
        if (PlaybackError.isAbort(error)) throw error;
      }
    }
  }
}
