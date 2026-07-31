export type PlaybackMode = "loop" | "once" | "hold" | "ping_pong";

export type ReturnToRule =
  | { type: "action"; actionKey: string }
  | { type: "default" }
  | { type: "previous" }
  | { type: "current_activity" }
  | { type: "none" };

export interface RuntimeAnchor {
  x: number;
  y: number;
  coordinateSpace: "normalized_canvas";
}

export interface RuntimeFrame {
  frameId: string;
  index: number;
  file: string;
  durationMs: number;
  contentHash: string;
}

export interface RuntimeAction {
  actionKey: string;
  displayName: string;
  fps: number;
  playbackMode: PlaybackMode;
  interruptible: boolean;
  priority: number;
  cooldownMs: number;
  minimumPlayMs: number;
  maximumPlayMs: number;
  mutexGroup: string;
  returnTo: ReturnToRule;
  anchor: RuntimeAnchor;
  frames: RuntimeFrame[];
  configPath: string;
}

export interface RuntimePetPackage {
  schemaVersion: 2;
  petId: string;
  releaseId: string;
  packageRoot: string;
  displayName: string;
  defaultActionKey: string;
  compatibility: { minimumRuntimeVersion: string; renderMode: "sprite" };
  actions: Map<string, RuntimeAction>;
  integrity: { contentRootHash: string; files: Map<string, string> };
  sourceSchemaVersion: 1 | 2;
}

export const CURRENT_RUNTIME_VERSION = "1.0.0";

export interface RawManifestInput {
  schemaVersion?: number;
  packageId?: string;
  name?: string;
  characterId?: string;
  petId?: string;
  releaseId?: string;
  canvas?: { width?: number; height?: number };
  defaultAction?: string;
  preview?: string;
  actions?: Array<{ key?: string; name?: string; config?: string }>;
  binding?: { installationId?: string; petId?: string };
  compatibility?: { minRuntimeVersion?: string; minimumRuntimeVersion?: string };
  integrity?: {
    contentRootHash?: string;
    manifestHash?: string;
    files?: Record<string, string> | Array<{ path: string; hash: string }>;
  };
}

export interface RawActionInput {
  schemaVersion?: number;
  actionKey?: string;
  key?: string;
  displayName?: string;
  actionName?: string;
  name?: string;
  fps?: number;
  defaultFps?: number;
  playbackMode?: string;
  loopType?: string;
  interruptible?: boolean;
  priority?: number;
  cooldownMs?: number;
  minimumPlayMs?: number;
  maximumPlayMs?: number | null;
  mutexGroup?: string;
  returnTo?: { type?: string; actionKey?: string };
  returnAction?: string;
  anchor?: {
    x?: number;
    y?: number;
    coordinateSpace?: string;
    type?: string;
  };
  frames?: Array<
    | string
    | {
        frameId?: string;
        index?: number;
        file?: string;
        durationMs?: number;
        contentHash?: string;
      }
  >;
  frameDurationMs?: number;
  frameCount?: number;
  version?: number;
}

export interface NormalizedManifestData {
  schemaVersion: number;
  petId: string;
  releaseId: string;
  displayName: string;
  defaultActionKey: string;
  preview?: string;
  canvas: { width: number; height: number };
  actionEntries: Array<{ key: string; name: string; config: string }>;
  minimumRuntimeVersion: string;
  contentRootHash: string;
  integrityFiles: Map<string, string>;
}

export interface PackageReader {
  readManifest(raw: unknown): NormalizedManifestData;
  readAction(raw: unknown, actionKey: string, configPath: string): RuntimeAction;
}

function normalizePlaybackModeValue(
  raw: string | undefined,
  actionKey: string,
): PlaybackMode {
  if (!raw) return "loop";
  const lt = raw.toLowerCase().trim();
  if (lt === "loop" || lt === "once" || lt === "hold" || lt === "ping_pong") {
    return lt;
  }
  if (lt === "ping-pong" || lt === "pingpong") {
    return "ping_pong";
  }
  throw new Error(`UNKNOWN_PLAYBACK_MODE: ${raw} (action: ${actionKey})`);
}

function normalizeReturnToRule(
  returnTo: unknown,
  returnAction: string | undefined,
  actionKey: string,
): ReturnToRule {
  if (returnTo && typeof returnTo === "object") {
    const rt = returnTo as { type?: string; actionKey?: string };
    const type = rt.type ?? "default";
    switch (type) {
      case "action":
        if (!rt.actionKey || typeof rt.actionKey !== "string") {
          throw new Error(`INVALID_RETURN_TO_ACTION_KEY: ${actionKey}`);
        }
        return { type: "action", actionKey: rt.actionKey };
      case "default":
        return { type: "default" };
      case "previous":
        return { type: "previous" };
      case "current_activity":
        return { type: "current_activity" };
      case "none":
        return { type: "none" };
      default:
        throw new Error(`UNKNOWN_RETURN_TO_TYPE: ${type} (action: ${actionKey})`);
    }
  }
  if (returnAction && typeof returnAction === "string" && returnAction.trim()) {
    return { type: "action", actionKey: returnAction };
  }
  return { type: "default" };
}

function normalizeAnchor(
  anchor: RawActionInput["anchor"],
  actionKey: string,
): RuntimeAnchor {
  if (!anchor) {
    return { x: 0.5, y: 1.0, coordinateSpace: "normalized_canvas" };
  }
  const x = typeof anchor.x === "number" ? anchor.x : Number(anchor.x);
  const y = typeof anchor.y === "number" ? anchor.y : Number(anchor.y);
  if (!Number.isFinite(x) || !Number.isFinite(y)) {
    throw new Error(`INVALID_ANCHOR: ${actionKey}`);
  }
  const coordinateSpace =
    anchor.coordinateSpace === "normalized_canvas"
      ? "normalized_canvas"
      : "normalized_canvas";
  return { x, y, coordinateSpace };
}

function normalizeFrames(
  frames: RawActionInput["frames"],
  defaultDurationMs: number,
  actionKey: string,
): RuntimeFrame[] {
  if (!Array.isArray(frames)) return [];
  const result: RuntimeFrame[] = [];
  for (let i = 0; i < frames.length; i++) {
    const item = frames[i];
    if (typeof item === "string") {
      if (item) {
        result.push({
          frameId: `${actionKey}_frame_${i}`,
          index: i,
          file: item,
          durationMs: defaultDurationMs,
          contentHash: "",
        });
      }
      continue;
    }
    if (item && typeof item === "object" && typeof item.file === "string" && item.file) {
      result.push({
        frameId: item.frameId ?? `${actionKey}_frame_${i}`,
        index: item.index ?? i,
        file: item.file,
        durationMs: item.durationMs ?? defaultDurationMs,
        contentHash: item.contentHash ?? "",
      });
    }
  }
  return result;
}

function computeFrameDurationMs(
  fps: number | undefined,
  frameDurationMs: number | undefined,
): number {
  if (
    frameDurationMs !== undefined &&
    Number.isFinite(frameDurationMs) &&
    frameDurationMs > 0
  ) {
    return frameDurationMs;
  }
  if (fps !== undefined && Number.isFinite(fps) && fps > 0) {
    return 1000 / fps;
  }
  return 100;
}

export class Schema1PackageReader implements PackageReader {
  readManifest(raw: unknown): NormalizedManifestData {
    const m = (raw ?? {}) as RawManifestInput;
    const schemaVersion = m.schemaVersion ?? 1;
    const petId = m.petId ?? m.packageId ?? "";
    const releaseId = m.releaseId ?? "";
    const displayName = m.name ?? "";
    const defaultActionKey = m.defaultAction ?? "";
    const canvas = {
      width: m.canvas?.width ?? 0,
      height: m.canvas?.height ?? 0,
    };
    const actionEntries = Array.isArray(m.actions)
      ? m.actions
          .filter((a) => a && typeof a.key === "string" && a.key)
          .map((a) => ({
            key: a.key!,
            name: a.name ?? a.key!,
            config: a.config ?? `actions/${a.key}/action.json`,
          }))
      : [];
    const minimumRuntimeVersion =
      m.compatibility?.minRuntimeVersion ??
      m.compatibility?.minimumRuntimeVersion ??
      "0.0.0";
    const contentRootHash = m.integrity?.contentRootHash ?? "";
    const integrityFiles = new Map<string, string>();
    if (m.integrity?.files) {
      if (Array.isArray(m.integrity.files)) {
        for (const f of m.integrity.files) {
          if (f && typeof f.path === "string") {
            integrityFiles.set(f.path, f.hash ?? "");
          }
        }
      } else {
        for (const [path, hash] of Object.entries(m.integrity.files)) {
          integrityFiles.set(path, String(hash));
        }
      }
    }
    return {
      schemaVersion,
      petId,
      releaseId,
      displayName,
      defaultActionKey,
      preview: m.preview,
      canvas,
      actionEntries,
      minimumRuntimeVersion,
      contentRootHash,
      integrityFiles,
    };
  }

  readAction(raw: unknown, actionKey: string, configPath: string): RuntimeAction {
    const a = (raw ?? {}) as RawActionInput;
    const key = a.actionKey ?? a.key ?? actionKey;
    const displayName = a.displayName ?? a.actionName ?? a.name ?? key;
    const fps = a.fps ?? a.defaultFps ?? 0;
    const playbackMode = normalizePlaybackModeValue(
      a.playbackMode ?? a.loopType,
      key,
    );
    const interruptible = a.interruptible ?? true;
    const priority = a.priority ?? 50;
    const cooldownMs = a.cooldownMs ?? 0;
    const minimumPlayMs = a.minimumPlayMs ?? 0;
    const maximumPlayMs = a.maximumPlayMs ?? 0;
    const mutexGroup = a.mutexGroup ?? "";
    const returnTo = normalizeReturnToRule(a.returnTo, a.returnAction, key);
    const anchor = normalizeAnchor(a.anchor, key);
    const frameDurationMs = computeFrameDurationMs(fps, a.frameDurationMs);
    const frames = normalizeFrames(a.frames, frameDurationMs, key);
    return {
      actionKey: key,
      displayName,
      fps,
      playbackMode,
      interruptible,
      priority,
      cooldownMs,
      minimumPlayMs,
      maximumPlayMs,
      mutexGroup,
      returnTo,
      anchor,
      frames,
      configPath,
    };
  }
}

export class Schema2PackageReader implements PackageReader {
  readManifest(raw: unknown): NormalizedManifestData {
    const m = (raw ?? {}) as RawManifestInput;
    const schemaVersion = m.schemaVersion ?? 2;
    const petId = m.petId ?? m.packageId ?? "";
    const releaseId = m.releaseId ?? "";
    const displayName = m.name ?? "";
    const defaultActionKey = m.defaultAction ?? "";
    const canvas = {
      width: m.canvas?.width ?? 0,
      height: m.canvas?.height ?? 0,
    };
    const actionEntries = Array.isArray(m.actions)
      ? m.actions
          .filter((a) => a && typeof a.key === "string" && a.key)
          .map((a) => ({
            key: a.key!,
            name: a.name ?? a.key!,
            config: a.config ?? `actions/${a.key}/action.json`,
          }))
      : [];
    const minimumRuntimeVersion =
      m.compatibility?.minimumRuntimeVersion ??
      m.compatibility?.minRuntimeVersion ??
      "0.0.0";
    const contentRootHash = m.integrity?.contentRootHash ?? "";
    const integrityFiles = new Map<string, string>();
    if (m.integrity?.files) {
      if (Array.isArray(m.integrity.files)) {
        for (const f of m.integrity.files) {
          if (f && typeof f.path === "string") {
            integrityFiles.set(f.path, f.hash ?? "");
          }
        }
      } else {
        for (const [path, hash] of Object.entries(m.integrity.files)) {
          integrityFiles.set(path, String(hash));
        }
      }
    }
    return {
      schemaVersion,
      petId,
      releaseId,
      displayName,
      defaultActionKey,
      preview: m.preview,
      canvas,
      actionEntries,
      minimumRuntimeVersion,
      contentRootHash,
      integrityFiles,
    };
  }

  readAction(raw: unknown, actionKey: string, configPath: string): RuntimeAction {
    const a = (raw ?? {}) as RawActionInput;
    const key = a.actionKey ?? a.key ?? actionKey;
    const displayName = a.displayName ?? a.actionName ?? a.name ?? key;
    const fps = a.fps ?? 0;
    const playbackMode = normalizePlaybackModeValue(a.playbackMode ?? a.loopType, key);
    const interruptible = a.interruptible ?? true;
    const priority = a.priority ?? 50;
    const cooldownMs = a.cooldownMs ?? 0;
    const minimumPlayMs = a.minimumPlayMs ?? 0;
    const maximumPlayMs = a.maximumPlayMs ?? 0;
    const mutexGroup = a.mutexGroup ?? "";
    const returnTo = normalizeReturnToRule(a.returnTo, a.returnAction, key);
    const anchor = normalizeAnchor(a.anchor, key);
    const frameDurationMs = computeFrameDurationMs(fps, a.frameDurationMs);
    const frames = normalizeFrames(a.frames, frameDurationMs, key);
    return {
      actionKey: key,
      displayName,
      fps,
      playbackMode,
      interruptible,
      priority,
      cooldownMs,
      minimumPlayMs,
      maximumPlayMs,
      mutexGroup,
      returnTo,
      anchor,
      frames,
      configPath,
    };
  }
}

export class RuntimePackageNormalizer {
  private schema1Reader = new Schema1PackageReader();
  private schema2Reader = new Schema2PackageReader();

  normalize(
    manifestRaw: unknown,
    actionRaws: Map<string, unknown>,
    packageRoot: string,
  ): RuntimePetPackage {
    const m = (manifestRaw ?? {}) as RawManifestInput;
    const schemaVersion = m.schemaVersion ?? 1;
    const reader: PackageReader =
      schemaVersion >= 2 ? this.schema2Reader : this.schema1Reader;
    const manifest = reader.readManifest(manifestRaw);

    const actions = new Map<string, RuntimeAction>();
    for (const entry of manifest.actionEntries) {
      const actionRaw = actionRaws.get(entry.key);
      if (actionRaw !== undefined) {
        const action = reader.readAction(actionRaw, entry.key, entry.config);
        actions.set(entry.key, action);
      }
    }

    return {
      schemaVersion: 2,
      petId: manifest.petId,
      releaseId: manifest.releaseId,
      packageRoot,
      displayName: manifest.displayName,
      defaultActionKey: manifest.defaultActionKey,
      compatibility: {
        minimumRuntimeVersion: manifest.minimumRuntimeVersion,
        renderMode: "sprite",
      },
      actions,
      integrity: {
        contentRootHash: manifest.contentRootHash,
        files: manifest.integrityFiles,
      },
      sourceSchemaVersion: (schemaVersion >= 2 ? 2 : 1) as 1 | 2,
    };
  }

  getReader(schemaVersion: number): PackageReader {
    return schemaVersion >= 2 ? this.schema2Reader : this.schema1Reader;
  }
}

export function compareVersions(minVersion: string, currentVersion: string): boolean {
  const parse = (v: string): number[] => {
    const parts = v.split(".").map((p) => {
      const n = parseInt(p, 10);
      return Number.isNaN(n) ? 0 : n;
    });
    while (parts.length < 3) parts.push(0);
    return parts.slice(0, 3);
  };
  const min = parse(minVersion);
  const cur = parse(currentVersion);
  for (let i = 0; i < 3; i++) {
    if (cur[i] < min[i]) return false;
    if (cur[i] > min[i]) return true;
  }
  return true;
}
