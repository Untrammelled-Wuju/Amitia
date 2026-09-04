import {
  ActionAssetRepository,
  DecodedFrame,
  DecoderRegistry,
  LoadPriority,
  LoadedAction,
  LoadedActionAssets,
  NormalizedFrame,
  PackagePlaybackSnapshot,
  RawActionConfig,
} from "../contracts";
import { PLAYBACK_ERROR_CODES, PlaybackError } from "../errors";
import { DecodedFrameCache } from "../cache/decoded-frame-cache";
import { FrameSequenceLoader } from "./frame-sequence-loader";

export type ActionNormalizer = (input: {
  raw: RawActionConfig;
  packageSnapshot: PackagePlaybackSnapshot;
}) => LoadedAction;

export class ActionAssetRepositoryImpl implements ActionAssetRepository {
  private decoderRegistry: DecoderRegistry;
  private frameLoader: FrameSequenceLoader;
  private cache: DecodedFrameCache;
  private normalizer: ActionNormalizer;
  private resolveResourceUrl?: (relativePath: string, configUrl: string) => string;

  constructor(input: {
    decoderRegistry: DecoderRegistry;
    frameLoader: FrameSequenceLoader;
    cache: DecodedFrameCache;
    normalizer: ActionNormalizer;
    resolveResourceUrl?: (relativePath: string, configUrl: string) => string;
  }) {
    this.decoderRegistry = input.decoderRegistry;
    this.frameLoader = input.frameLoader;
    this.cache = input.cache;
    this.normalizer = input.normalizer;
    this.resolveResourceUrl = input.resolveResourceUrl;
  }

  async loadAction(input: {
    packageSnapshot: PackagePlaybackSnapshot;
    actionKey: string;
    signal: AbortSignal;
    priority: LoadPriority;
  }): Promise<LoadedActionAssets> {
    const actionEntry = input.packageSnapshot.actions.find(
      (a) => a.actionKey === input.actionKey,
    );
    if (!actionEntry) {
      throw new PlaybackError(
        PLAYBACK_ERROR_CODES.ACTION_NOT_FOUND,
        `action not found: ${input.actionKey}`,
        { actionKey: input.actionKey },
      );
    }

    let response: Response;
    try {
      response = await fetch(actionEntry.configUrl, {
        signal: input.signal,
      });
    } catch (error) {
      if (PlaybackError.isAbort(error)) throw error;
      throw new PlaybackError(
        PLAYBACK_ERROR_CODES.FRAME_FETCH_FAILED,
        `fetch action config failed: ${actionEntry.configUrl}`,
        { actionKey: input.actionKey, resourceUrl: actionEntry.configUrl },
      );
    }

    if (!response.ok) {
      throw new PlaybackError(
        PLAYBACK_ERROR_CODES.FRAME_FETCH_FAILED,
        `fetch action config failed: ${response.status} ${response.statusText}`,
        { actionKey: input.actionKey, resourceUrl: actionEntry.configUrl },
      );
    }

    let raw: RawActionConfig;
    try {
      raw = (await response.json()) as RawActionConfig;
    } catch (error) {
      if (PlaybackError.isAbort(error)) throw error;
      throw new PlaybackError(
        PLAYBACK_ERROR_CODES.ACTION_CONFIG_INVALID,
        `parse action config failed: ${String(error)}`,
        { actionKey: input.actionKey },
      );
    }

    const action = this.normalizer({
      raw,
      packageSnapshot: input.packageSnapshot,
    });

    if (this.resolveResourceUrl) {
      const configUrl = actionEntry.configUrl;
      for (let i = 0; i < action.frames.length; i++) {
        const frame = action.frames[i];
        const resolved = this.resolveResourceUrl(frame.resourceUrl, configUrl);
        (action.frames as unknown as NormalizedFrame[])[i] = {
          ...frame,
          resourceUrl: resolved,
        };
      }
    }

    const cacheKeyPrefix = `${action.packageId}:${action.actionKey}`;
    const cachedFrames: DecodedFrame[] = [];
    let allCached = true;

    for (const frame of action.frames) {
      const frameKey = `${cacheKeyPrefix}:${frame.index}`;
      const cached = this.cache.get(frameKey);
      if (cached) {
        cachedFrames.push(cached);
      } else {
        allCached = false;
        break;
      }
    }

    if (
      allCached &&
      cachedFrames.length === action.frames.length &&
      action.frames.length > 0
    ) {
      const totalEstimatedBytes = cachedFrames.reduce(
        (sum, f) => sum + f.estimatedBytes,
        0,
      );
      return {
        action,
        decodedFrames: cachedFrames,
        totalEstimatedBytes,
      };
    }

    const decodedFrames = await this.frameLoader.loadFrames({
      frames: action.frames,
      signal: input.signal,
    });

    for (const frame of decodedFrames) {
      const frameKey = `${cacheKeyPrefix}:${frame.frameIndex}`;
      this.cache.put(
        frameKey,
        frame,
        frame.estimatedBytes,
        action.packageRevision,
      );
    }

    const totalEstimatedBytes = decodedFrames.reduce(
      (sum, f) => sum + f.estimatedBytes,
      0,
    );

    return {
      action,
      decodedFrames,
      totalEstimatedBytes,
    };
  }
}
