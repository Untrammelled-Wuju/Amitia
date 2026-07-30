import type {
  ActionSpecSnapshot,
  LoadedAction,
  LoopType,
  NormalizedFrame,
  PackagePlaybackSnapshot,
  RawActionConfig,
  ReturnTarget,
} from "../contracts";
import {
  FRAME_DURATION_MAX_MS,
  FRAME_DURATION_MIN_MS,
  FPS_MAX,
  FPS_MIN,
  LEGACY_FRAME_DURATION_MS,
} from "../contracts";
import { PLAYBACK_ERROR_CODES, PlaybackError } from "../errors";

interface RawFrameEntry {
  index?: number;
  file: string;
  durationMs?: number;
}

function extractRawFrames(raw: RawActionConfig): RawFrameEntry[] {
  const result: RawFrameEntry[] = [];
  if (!raw.frames || raw.frames.length === 0) return result;
  for (let i = 0; i < raw.frames.length; i++) {
    const item = raw.frames[i];
    if (typeof item === "string") {
      result.push({ file: item });
    } else if (item && typeof item.file === "string") {
      result.push({ index: item.index, file: item.file, durationMs: item.durationMs });
    }
  }
  return result;
}

function normalizeLoopType(
  rawLoopType: string,
  isNewSchema: boolean,
  actionKey: string,
  warnings: string[],
): LoopType {
  const lt = rawLoopType?.toLowerCase()?.trim();
  if (lt === "loop" || lt === "once" || lt === "hold" || lt === "ping_pong") {
    return lt;
  }
  if (lt === "pingpong") {
    warnings.push("legacy_loop_type_alias: pingpong -> ping_pong");
    return "ping_pong";
  }
  if (isNewSchema) {
    throw new PlaybackError(
      PLAYBACK_ERROR_CODES.ACTION_CONFIG_INVALID,
      `unknown loop type: ${rawLoopType}`,
      { actionKey },
    );
  }
  warnings.push(`unknown_loop_type_defaulting_to_loop: ${rawLoopType}`);
  return "loop";
}

function resolveReturnTarget(
  rawReturnAction: string | undefined,
  actionKey: string,
  specSnapshot: ActionSpecSnapshot | undefined,
  availableActionKeys: Set<string>,
  defaultActionKey: string,
  loopType: LoopType,
  warnings: string[],
): ReturnTarget {
  if (specSnapshot) {
    return specSnapshot.returnTarget;
  }

  if (!rawReturnAction || rawReturnAction.trim() === "") {
    if (loopType === "once" || loopType === "hold") {
      return { type: "default" };
    }
    return { type: "none" };
  }

  if (rawReturnAction === actionKey) {
    warnings.push("return_action_self_reference_ignored");
    return { type: "default" };
  }

  if (rawReturnAction === "idle_normal") {
    if (availableActionKeys.has("idle_normal")) {
      return { type: "action", actionKey: "idle_normal" };
    }
    warnings.push("return_action_idle_normal_not_found_using_default");
    return { type: "default" };
  }

  if (availableActionKeys.has(rawReturnAction)) {
    return { type: "action", actionKey: rawReturnAction };
  }

  warnings.push(`return_action_not_found: ${rawReturnAction}`);
  return { type: "default" };
}

function resolveAnchor(
  raw: RawActionConfig,
  canvasWidth: number,
  canvasHeight: number,
  specSnapshot: ActionSpecSnapshot | undefined,
  warnings: string[],
): { type: string; x: number; y: number } {
  if (specSnapshot) {
  }

  const ax = raw.anchor?.x;
  const ay = raw.anchor?.y;
  const atype = raw.anchor?.type ?? "bottom_center";

  if (ax !== undefined && ay !== undefined) {
    const nx = typeof ax === "number" ? ax : Number(ax);
    const ny = typeof ay === "number" ? ay : Number(ay);
    if (Number.isFinite(nx) && Number.isFinite(ny)) {
      return { type: atype, x: nx, y: ny };
    }
    warnings.push("anchor_invalid_using_default");
  }

  return {
    type: "bottom_center",
    x: canvasWidth / 2,
    y: canvasHeight,
  };
}

function computeFrameDuration(
  rawFrame: RawFrameEntry,
  raw: RawActionConfig,
  isNewSchema: boolean,
  warnings: string[],
): number {
  if (
    rawFrame.durationMs !== undefined &&
    Number.isFinite(rawFrame.durationMs) &&
    rawFrame.durationMs >= FRAME_DURATION_MIN_MS &&
    rawFrame.durationMs <= FRAME_DURATION_MAX_MS
  ) {
    return rawFrame.durationMs;
  }

  if (
    raw.frameDurationMs !== undefined &&
    Number.isFinite(raw.frameDurationMs) &&
    raw.frameDurationMs >= FRAME_DURATION_MIN_MS &&
    raw.frameDurationMs <= FRAME_DURATION_MAX_MS
  ) {
    return raw.frameDurationMs;
  }

  if (
    raw.fps !== undefined &&
    Number.isFinite(raw.fps) &&
    raw.fps >= FPS_MIN &&
    raw.fps <= FPS_MAX
  ) {
    return 1000 / raw.fps;
  }

  if (isNewSchema) {
    throw new PlaybackError(
      PLAYBACK_ERROR_CODES.ACTION_CONFIG_INVALID,
      `no valid frame duration for action: ${raw.actionKey}`,
      { actionKey: raw.actionKey },
    );
  }

  warnings.push("legacy_timing_fallback: using 100ms");
  return LEGACY_FRAME_DURATION_MS;
}

export interface NormalizeActionConfigInput {
  raw: RawActionConfig;
  packageSnapshot: PackagePlaybackSnapshot;
}

export function normalizeActionConfig(input: NormalizeActionConfigInput): LoadedAction {
  const { raw, packageSnapshot } = input;
  const warnings: string[] = [];
  const isNewSchema = packageSnapshot.schemaVersion > 1;
  const specSnapshot = packageSnapshot.actions.find(
    (a) => a.actionKey === raw.actionKey,
  )?.specSnapshot;

  const availableActionKeys = new Set(
    packageSnapshot.actions.map((a) => a.actionKey),
  );

  const rawFrames = extractRawFrames(raw);
  if (rawFrames.length === 0) {
    throw new PlaybackError(
      PLAYBACK_ERROR_CODES.ACTION_CONFIG_INVALID,
      `no frames for action: ${raw.actionKey}`,
      { actionKey: raw.actionKey },
    );
  }

  if (raw.frameCount !== undefined && raw.frameCount !== rawFrames.length) {
    if (isNewSchema) {
      throw new PlaybackError(
        PLAYBACK_ERROR_CODES.ACTION_CONFIG_INVALID,
        `frame count mismatch: declared=${raw.frameCount} actual=${rawFrames.length}`,
        { actionKey: raw.actionKey },
      );
    }
    warnings.push(`frame_count_mismatch: declared=${raw.frameCount} actual=${rawFrames.length}`);
  }

  const hasExplicitIndices = rawFrames.some((f) => f.index !== undefined);
  let normalizedFrames: NormalizedFrame[];

  if (hasExplicitIndices) {
    const seenIndices = new Set<number>();
    for (const f of rawFrames) {
      if (f.index !== undefined) {
        if (seenIndices.has(f.index)) {
          throw new PlaybackError(
            PLAYBACK_ERROR_CODES.ACTION_CONFIG_INVALID,
            `duplicate frame index: ${f.index}`,
            { actionKey: raw.actionKey },
          );
        }
        seenIndices.add(f.index);
      }
    }

    const sorted = [...rawFrames].sort((a, b) => {
      const ai = a.index ?? 0;
      const bi = b.index ?? 0;
      return ai - bi;
    });

    const isContinuous = sorted.every((f, i) => (f.index ?? i) === i);
    if (!isContinuous && !isNewSchema) {
      warnings.push("non_contiguous_frame_indices_reordered");
    }

    let cumulative = 0;
    normalizedFrames = sorted.map((f, i) => {
      const durationMs = computeFrameDuration(f, raw, isNewSchema, warnings);
      const frame: NormalizedFrame = {
        index: f.index ?? i,
        resourceUrl: f.file,
        durationMs,
        cumulativeStartMs: cumulative,
        cumulativeEndMs: cumulative + durationMs,
      };
      cumulative += durationMs;
      return frame;
    });
  } else {
    let cumulative = 0;
    normalizedFrames = rawFrames.map((f, i) => {
      const durationMs = computeFrameDuration(f, raw, isNewSchema, warnings);
      const frame: NormalizedFrame = {
        index: i,
        resourceUrl: f.file,
        durationMs,
        cumulativeStartMs: cumulative,
        cumulativeEndMs: cumulative + durationMs,
      };
      cumulative += durationMs;
      return frame;
    });
  }

  const loopType = normalizeLoopType(raw.loopType, isNewSchema, raw.actionKey, warnings);

  const anchor = resolveAnchor(
    raw,
    packageSnapshot.canvas.width,
    packageSnapshot.canvas.height,
    specSnapshot,
    warnings,
  );

  const returnTarget = resolveReturnTarget(
    raw.returnAction,
    raw.actionKey,
    specSnapshot,
    availableActionKeys,
    packageSnapshot.defaultActionKey,
    loopType,
    warnings,
  );

  const interruptible = specSnapshot?.interruptible ?? raw.interruptible ?? true;
  const interruptAfterMs = specSnapshot?.interruptAfterMs ?? raw.interruptAfterMs ?? 0;
  const minimumPlayMs = specSnapshot?.minimumPlayMs ?? raw.minimumPlayMs ?? 0;
  const maximumPlayMs = specSnapshot?.maximumPlayMs ?? raw.maximumPlayMs ?? null;
  const defaultPriority = specSnapshot?.defaultPriority ?? raw.defaultPriority ?? 50;
  const cooldownMs = specSnapshot?.cooldownMs ?? raw.cooldownMs ?? 0;
  const mutexGroup = specSnapshot?.mutexGroup ?? raw.mutexGroup ?? null;
  const isStableStateCandidate = specSnapshot?.isStableStateCandidate ?? (loopType === "loop");
  const isTransitionOnly = specSnapshot?.isTransitionOnly ?? false;
  const supportsDefaultIdle = specSnapshot?.supportsDefaultIdle ?? true;

  const cycleDurationMs = normalizedFrames.reduce(
    (sum, f) => sum + f.durationMs,
    0,
  );
  const baseDurationMs = cycleDurationMs / Math.max(1, normalizedFrames.length);

  const action: LoadedAction = {
    packageId: packageSnapshot.packageId,
    packageRevision: packageSnapshot.packageRevision,
    actionKey: raw.actionKey,
    displayName: raw.displayName || raw.actionKey,
    actionVersion: raw.version ?? 1,
    loopType,
    frames: normalizedFrames,
    baseDurationMs,
    cycleDurationMs,
    anchor,
    interruptible,
    interruptAfterMs,
    minimumPlayMs,
    maximumPlayMs,
    defaultPriority,
    cooldownMs,
    mutexGroup,
    returnTarget,
    supportsDefaultIdle,
    isStableStateCandidate,
    isTransitionOnly,
    warnings,
    specSnapshot,
  };

  return action;
}

export function createActionNormalizer() {
  return (input: { raw: RawActionConfig; packageSnapshot: PackagePlaybackSnapshot }) =>
    normalizeActionConfig(input);
}
