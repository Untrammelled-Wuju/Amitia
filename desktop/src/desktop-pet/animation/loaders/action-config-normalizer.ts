import type {
  ActionSpecSnapshot,
  LoadedAction,
  LoopType,
  NormalizedFrame,
  PackagePlaybackSnapshot,
  RawActionConfig,
  ReturnTarget,
} from "../contracts";
import { FRAME_DURATION_MAX_MS, FRAME_DURATION_MIN_MS } from "../contracts";
import { PLAYBACK_ERROR_CODES, PlaybackError } from "../errors";

interface CanonicalRawFrame {
  index?: number;
  file: string;
  durationMs?: number;
  frameId?: string;
  assetId?: string;
  contentHash?: string;
}

function invalid(message: string, actionKey: string): never {
  throw new PlaybackError(
    PLAYBACK_ERROR_CODES.ACTION_CONFIG_INVALID,
    message,
    { actionKey },
  );
}

function extractCanonicalFrames(raw: RawActionConfig): CanonicalRawFrame[] {
  if (!Array.isArray(raw.frames) || raw.frames.length === 0) {
    return invalid(`no frames for action: ${raw.actionKey}`, raw.actionKey);
  }
  return raw.frames.map((item, index) => {
    if (typeof item === "string" || item === null || typeof item !== "object") {
      return invalid(`frame ${index} must be an object: ${raw.actionKey}`, raw.actionKey);
    }
    if (typeof item.file !== "string" || item.file.length === 0) {
      return invalid(`frame ${index} is missing file: ${raw.actionKey}`, raw.actionKey);
    }
    return {
      index: item.index,
      file: item.file,
      durationMs: item.durationMs,
      frameId: item.frameId,
      assetId: item.assetId,
      contentHash: item.contentHash,
    };
  });
}

function normalizeLoopType(
  rawPlaybackMode: string | undefined,
  actionKey: string,
): LoopType {
  if (
    rawPlaybackMode === "loop" ||
    rawPlaybackMode === "once" ||
    rawPlaybackMode === "hold" ||
    rawPlaybackMode === "ping_pong"
  ) {
    return rawPlaybackMode;
  }
  return invalid(
    `unknown playback mode: ${rawPlaybackMode ?? ""}`,
    actionKey,
  );
}

function resolveReturnTarget(
  rawReturnTo: { type?: string; actionKey?: string } | undefined,
  actionKey: string,
  availableActionKeys: Set<string>,
): ReturnTarget {
  if (!rawReturnTo || typeof rawReturnTo.type !== "string") {
    return invalid(`returnTo is required: ${actionKey}`, actionKey);
  }
  switch (rawReturnTo.type) {
    case "default":
      return { type: "default" };
    case "previous":
      return { type: "previous" };
    case "current_activity":
      return { type: "current_activity" };
    case "none":
      return { type: "none" };
    case "action": {
      const targetKey = rawReturnTo.actionKey;
      if (!targetKey || targetKey === actionKey || !availableActionKeys.has(targetKey)) {
        return invalid(
          `invalid returnTo action target: ${targetKey ?? ""}`,
          actionKey,
        );
      }
      return { type: "action", actionKey: targetKey };
    }
    default:
      return invalid(`unknown returnTo type: ${rawReturnTo.type}`, actionKey);
  }
}

function resolveAnchor(
  raw: RawActionConfig,
): { type: string; x: number; y: number } {
  const anchor = raw.anchor;
  if (anchor?.coordinateSpace !== "normalized_canvas") {
    return invalid(`anchor coordinateSpace must be normalized_canvas: ${raw.actionKey}`, raw.actionKey);
  }
  if (
    typeof anchor.x !== "number" ||
    typeof anchor.y !== "number" ||
    !Number.isFinite(anchor.x) ||
    !Number.isFinite(anchor.y) ||
    anchor.x < 0 ||
    anchor.x > 1 ||
    anchor.y < 0 ||
    anchor.y > 1
  ) {
    return invalid(`anchor x/y must be normalized to [0, 1]: ${raw.actionKey}`, raw.actionKey);
  }
  return {
    type: anchor.type ?? "normalized_canvas",
    x: anchor.x,
    y: anchor.y,
  };
}

function resolveFrameDuration(
  rawFrame: CanonicalRawFrame,
  actionKey: string,
): number {
  const durationMs = rawFrame.durationMs;
  if (
    typeof durationMs !== "number" ||
    !Number.isFinite(durationMs) ||
    durationMs < FRAME_DURATION_MIN_MS ||
    durationMs > FRAME_DURATION_MAX_MS
  ) {
    return invalid(`invalid frame durationMs: ${actionKey}`, actionKey);
  }
  return durationMs;
}

function resolveFrameIdentity(
  frame: CanonicalRawFrame,
  actionKey: string,
): { frameId: string; assetId: string; contentHash: string } {
  if (!frame.frameId || !frame.assetId || !frame.contentHash) {
    return invalid(`frame identity is incomplete: ${actionKey}`, actionKey);
  }
  return {
    frameId: frame.frameId,
    assetId: frame.assetId,
    contentHash: frame.contentHash,
  };
}

export interface NormalizeActionConfigInput {
  raw: RawActionConfig;
  packageSnapshot: PackagePlaybackSnapshot;
}

export function normalizeActionConfig(input: NormalizeActionConfigInput): LoadedAction {
  const { raw, packageSnapshot } = input;
  if (packageSnapshot.schemaVersion !== 1) {
    return invalid(
      `unsupported package schemaVersion: ${packageSnapshot.schemaVersion}`,
      raw.actionKey,
    );
  }

  const rawFrames = extractCanonicalFrames(raw);
  if (raw.frameCount !== rawFrames.length) {
    return invalid(
      `frame count mismatch: declared=${raw.frameCount} actual=${rawFrames.length}`,
      raw.actionKey,
    );
  }

  const hasExplicitIndices = rawFrames.some((frame) => frame.index !== undefined);
  const orderedFrames = hasExplicitIndices
    ? [...rawFrames].sort((left, right) => (left.index ?? 0) - (right.index ?? 0))
    : rawFrames;

  const seenIndices = new Set<number>();
  let cumulative = 0;
  const normalizedFrames: NormalizedFrame[] = orderedFrames.map((frame, position) => {
    const index = frame.index ?? position;
    if (!Number.isInteger(index) || index < 0 || seenIndices.has(index)) {
      return invalid(`invalid or duplicate frame index: ${index}`, raw.actionKey);
    }
    seenIndices.add(index);
    const durationMs = resolveFrameDuration(frame, raw.actionKey);
    const identity = resolveFrameIdentity(frame, raw.actionKey);
    const normalized: NormalizedFrame = {
      index,
      resourceUrl: frame.file,
      durationMs,
      cumulativeStartMs: cumulative,
      cumulativeEndMs: cumulative + durationMs,
      frameId: identity.frameId,
      assetId: identity.assetId,
      contentHash: identity.contentHash,
    };
    cumulative += durationMs;
    return normalized;
  });

  const loopType = normalizeLoopType(raw.playbackMode, raw.actionKey);
  const specSnapshot = packageSnapshot.actions.find(
    (action) => action.actionKey === raw.actionKey,
  )?.specSnapshot;
  const availableActionKeys = new Set(
    packageSnapshot.actions.map((action) => action.actionKey),
  );
  const anchor = resolveAnchor(raw);
  const returnTarget = resolveReturnTarget(
    raw.returnTo,
    raw.actionKey,
    availableActionKeys,
  );
  const cycleDurationMs = normalizedFrames.reduce(
    (sum, frame) => sum + frame.durationMs,
    0,
  );
  const baseDurationMs = cycleDurationMs / Math.max(1, normalizedFrames.length);

  return {
    packageId: packageSnapshot.packageId,
    packageRevision: packageSnapshot.packageRevision,
    actionKey: raw.actionKey,
    displayName: raw.displayName,
    actionVersion: raw.version,
    loopType,
    frames: normalizedFrames,
    baseDurationMs,
    cycleDurationMs,
    anchor,
    interruptible: specSnapshot?.interruptible ?? raw.interruptible ?? true,
    interruptAfterMs: specSnapshot?.interruptAfterMs ?? raw.interruptAfterMs ?? 0,
    minimumPlayMs: specSnapshot?.minimumPlayMs ?? raw.minimumPlayMs ?? 0,
    maximumPlayMs: specSnapshot?.maximumPlayMs ?? raw.maximumPlayMs ?? null,
    defaultPriority: specSnapshot?.defaultPriority ?? raw.defaultPriority ?? raw.priority ?? 50,
    cooldownMs: specSnapshot?.cooldownMs ?? raw.cooldownMs ?? 0,
    mutexGroup: specSnapshot?.mutexGroup ?? raw.mutexGroup ?? null,
    returnTarget,
    supportsDefaultIdle: specSnapshot?.supportsDefaultIdle ?? raw.supportsDefaultIdle ?? true,
    isStableStateCandidate: specSnapshot?.isStableStateCandidate ?? raw.isStableStateCandidate ?? (loopType === "loop"),
    isTransitionOnly: specSnapshot?.isTransitionOnly ?? raw.isTransitionOnly ?? false,
    warnings: [],
    specSnapshot,
  };
}

export function createActionNormalizer() {
  return (input: {
    raw: RawActionConfig;
    packageSnapshot: PackagePlaybackSnapshot;
  }) => normalizeActionConfig(input);
}
