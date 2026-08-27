import { DESKTOP_PET_RUNTIME_VERSION } from "../../shared/desktop-pet-runtime-version";
import { readFile } from "node:fs/promises";
import { join, dirname } from "node:path";
import {
  RuntimePackageNormalizer,
  type PlaybackMode as SharedPlaybackMode,
  type ReturnToRule as SharedReturnToRule,
  type LoadInstallationRequest,
  type RuntimeAction as SharedRuntimeAction,
  type RuntimePetPackage,
  type PackageReader,
  type ActionReadResult,
} from "../../shared/package-schema";
import type { PackageContractError } from "../../shared/package-errors";

export type { LoadInstallationRequest };

interface ActionFileFrame {
  index?: number;
  file?: string;
  durationMs?: number;
  frameId?: string;
  assetId?: string;
  contentHash?: string;
}

interface ActionFile {
  key?: string;
  actionKey?: string;
  name?: string;
  displayName?: string;
  actionName?: string;
  version?: number;
  schemaVersion?: number;
  loopType?: string;
  playbackMode?: string;
  fps?: number;
  defaultFps?: number;
  frameDurationMs?: number;
  frameCount?: number;
  frames?: Array<ActionFileFrame | string>;
  anchor?: { type?: string; x?: number; y?: number; coordinateSpace?: string };
  interruptible?: boolean;
  returnAction?: string;
  returnTo?: { type?: string; actionKey?: string };
  priority?: number;
  cooldownMs?: number;
  minimumPlayMs?: number;
  maximumPlayMs?: number | null;
  mutexGroup?: string;
  supportsDefaultIdle?: boolean;
  isStableStateCandidate?: boolean;
  isTransitionOnly?: boolean;
  interruptAfterMs?: number;
  category?: string;
}

export type ActionLoopType = "loop" | "once" | "hold" | "ping_pong";

export interface ManifestAction {
  key: string;
  name: string;
  version: string;
  loopType: ActionLoopType;
  fps: number;
  frameDurationMs: number;
  frameCount: number;
  frames: string[];
  anchor?: { x: number; y: number; coordinateSpace?: string };
  interruptible: boolean;
  returnAction?: string;
  config?: string;
}

export interface IntegrityFileEntry {
  path: string;
  sha256: string;
  bytes: number;
  mediaType: string;
  role: string;
  actionKey?: string;
  frameId?: string;
}

export interface Manifest {
  packageId: string;
  schemaVersion: number;
  name: string;
  characterId: string;
  petId?: string;
  releaseId?: string;
  canvas: { width: number; height: number };
  defaultAction: string;
  preview?: string;
  actions: ManifestAction[];
  binding?: { installationId?: string; petId?: string };
  compatibility?: { minRuntimeVersion?: string; minimumRuntimeVersion?: string };
  integrity?: { contentRootHash?: string; manifestHash?: string; files?: IntegrityFileEntry[] };
}

export interface RuntimeAction {
  key: string;
  name: string;
  version: string;
  loopType: ActionLoopType;
  fps: number;
  frameDurationMs: number;
  frameCount: number;
  frames: string[];
  anchor?: { x: number; y: number; coordinateSpace?: string };
  interruptible: boolean;
  returnAction?: string;
  config?: string;
  available: boolean;
  loadError?: string;
  playbackMode?: SharedPlaybackMode;
  priority?: number;
  cooldownMs?: number;
  mutexGroup?: string;
  minimumPlayMs?: number;
  maximumPlayMs?: number | null;
  returnTo?: SharedReturnToRule;
  supportsDefaultIdle?: boolean;
  isStableStateCandidate?: boolean;
  isTransitionOnly?: boolean;
  interruptAfterMs?: number;
  category?: string;
}

export interface LoadedInstallation {
  installationId: string;
  manifest: Manifest;
  actions: Map<string, RuntimeAction>;
  defaultAction: RuntimeAction | null;
  installPath: string;
  manifestPath: string;
  previewPath: string | null;
}

const DEFAULT_ACTION_NOT_FOUND_ERROR = "DEFAULT_ACTION_NOT_FOUND";
const DEFAULT_ACTION_INVALID_ERROR = "DEFAULT_ACTION_INVALID";
const UNSUPPORTED_SCHEMA_VERSION_ERROR = "UNSUPPORTED_SCHEMA_VERSION";
const MANIFEST_READ_FAILED_ERROR = "MANIFEST_READ_FAILED";
const MANIFEST_PARSE_FAILED_ERROR = "MANIFEST_PARSE_FAILED";
const ACTION_JSON_READ_FAILED = "ACTION_JSON_READ_FAILED";
const ACTION_JSON_PARSE_FAILED = "ACTION_JSON_PARSE_FAILED";

export {
  DEFAULT_ACTION_NOT_FOUND_ERROR,
  DEFAULT_ACTION_INVALID_ERROR,
  UNSUPPORTED_SCHEMA_VERSION_ERROR,
  MANIFEST_READ_FAILED_ERROR,
  MANIFEST_PARSE_FAILED_ERROR,
  ACTION_JSON_READ_FAILED,
  ACTION_JSON_PARSE_FAILED,
};

function errorMessage(err: unknown): string {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}

function isPackageContractError(err: unknown): err is PackageContractError {
  return err instanceof Error && (err as PackageContractError).code !== undefined;
}

export class ResourceLoader {
  private readonly normalizer = new RuntimePackageNormalizer();
  private readonly runtimeVersion: string;

  constructor(runtimeVersion?: string) {
    this.runtimeVersion = runtimeVersion ?? DESKTOP_PET_RUNTIME_VERSION;
  }

  async loadInstallation(
    request: LoadInstallationRequest,
  ): Promise<LoadedInstallation> {
    let manifestRawText: string;
    try {
      manifestRawText = await readFile(request.manifestPath, "utf8");
    } catch (err) {
      throw new Error(
        `${MANIFEST_READ_FAILED_ERROR}: ${errorMessage(err)}`,
      );
    }

    let manifestRaw: unknown;
    try {
      manifestRaw = JSON.parse(manifestRawText);
    } catch (err) {
      throw new Error(
        `${MANIFEST_PARSE_FAILED_ERROR}: ${errorMessage(err)}`,
      );
    }

    if (
      manifestRaw === null ||
      typeof manifestRaw !== "object" ||
      typeof (manifestRaw as { schemaVersion?: unknown }).schemaVersion !== "number"
    ) {
      throw new Error(
        `${UNSUPPORTED_SCHEMA_VERSION_ERROR}: schemaVersion missing or invalid`,
      );
    }

    const schemaVersion = (manifestRaw as { schemaVersion: number }).schemaVersion;
    let reader: PackageReader;
    try {
      reader = this.normalizer.getReader(schemaVersion);
    } catch {
      throw new Error(
        `${UNSUPPORTED_SCHEMA_VERSION_ERROR}: ${schemaVersion}`,
      );
    }

    let manifestData;
    try {
      const result = reader.readManifest(manifestRaw);
      manifestData = result.data;
    } catch (err) {
      throw new Error(
        `${MANIFEST_PARSE_FAILED_ERROR}: ${errorMessage(err)}`,
      );
    }

    if (manifestData.petId !== request.petId) {
      throw new Error(
        `PET_IDENTITY_CONFLICT: manifest petId=${manifestData.petId}, request petId=${request.petId}`,
      );
    }
    if (manifestData.releaseId !== request.releaseId) {
      throw new Error(
        `RELEASE_VERSION_CONFLICT: manifest releaseId=${manifestData.releaseId}, request releaseId=${request.releaseId}`,
      );
    }
    if (
      request.expectedContentRootHash &&
      manifestData.integrity.contentRootHash &&
      manifestData.integrity.contentRootHash !== request.expectedContentRootHash
    ) {
      throw new Error(
        `PACKAGE_HASH_MISMATCH: manifest contentRootHash=${manifestData.integrity.contentRootHash}, expected=${request.expectedContentRootHash}`,
      );
    }

    if (!manifestData.defaultActionKey) {
      throw new Error(DEFAULT_ACTION_NOT_FOUND_ERROR);
    }

    const actionRaws = new Map<string, unknown>();
    for (const entry of manifestData.actionEntries) {
      const actionJsonPath = join(request.installPath, entry.config);
      let actionRawText: string;
      try {
        actionRawText = await readFile(actionJsonPath, "utf8");
      } catch (err) {
        throw new Error(
          `${ACTION_JSON_READ_FAILED}: ${entry.key}: ${errorMessage(err)}`,
        );
      }
      try {
        actionRaws.set(entry.key, JSON.parse(actionRawText));
      } catch (err) {
        throw new Error(
          `${ACTION_JSON_PARSE_FAILED}: ${entry.key}: ${errorMessage(err)}`,
        );
      }
    }

    let pkg: RuntimePetPackage;
    try {
      pkg = this.normalizer.normalize(
        manifestRaw,
        actionRaws,
        request.installPath,
        this.runtimeVersion,
      );
    } catch (err) {
      const msg = errorMessage(err);
      if (isPackageContractError(err)) {
        throw new Error(`${err.code}: ${msg}`);
      }
      throw new Error(msg);
    }

    const actions = new Map<string, RuntimeAction>();
    const manifestActions: ManifestAction[] = [];
    for (const [key, sharedAction] of pkg.actions) {
      const localAction = this.mapRuntimeAction(sharedAction, request.installPath);
      actions.set(key, localAction);
      manifestActions.push(this.mapManifestAction(sharedAction));
    }

    const defaultAction = actions.get(pkg.defaultActionKey) ?? null;
    if (!defaultAction) {
      throw new Error(DEFAULT_ACTION_NOT_FOUND_ERROR);
    }
    if (!defaultAction.available) {
      throw new Error(
        `${DEFAULT_ACTION_INVALID_ERROR}: ${defaultAction.loadError ?? "unknown"}`,
      );
    }

    const rawManifestObj = manifestRaw as {
      integrity?: {
        files?: IntegrityFileEntry[];
      };
    };
    const integrityFiles = rawManifestObj.integrity?.files ?? [];

    const manifest: Manifest = {
      packageId: pkg.petId,
      schemaVersion: pkg.schemaVersion,
      name: pkg.displayName,
      characterId: manifestData.binding.sourceCharacterId ?? "",
      petId: pkg.petId,
      releaseId: pkg.releaseId,
      canvas: { width: pkg.canvas.width, height: pkg.canvas.height },
      defaultAction: pkg.defaultActionKey,
      preview: pkg.preview ?? undefined,
      actions: manifestActions,
      binding: undefined,
      compatibility: { minRuntimeVersion: pkg.compatibility.minRuntimeVersion },
      integrity: {
        contentRootHash: pkg.integrity.contentRootHash,
        manifestHash: pkg.integrity.manifestHash,
        files: integrityFiles,
      },
    };

    const previewPath = pkg.preview
      ? join(request.installPath, pkg.preview)
      : null;

    return {
      installationId: request.installationId,
      manifest,
      actions,
      defaultAction,
      installPath: request.installPath,
      manifestPath: request.manifestPath,
      previewPath,
    };
  }

  async loadAction(
    actionKey: string,
    loaded: LoadedInstallation,
  ): Promise<RuntimeAction | null> {
    if (!actionKey) {
      return null;
    }

    const cached = loaded.actions.get(actionKey);
    if (cached && cached.available) {
      return cached;
    }

    const manifestAction = loaded.manifest.actions.find(
      (a) => a.key === actionKey,
    );

    if (!cached && !manifestAction) {
      return null;
    }

    const configPath =
      manifestAction?.config ?? join("actions", actionKey, "action.json");
    const actionJsonPath = join(loaded.installPath, configPath);

    let raw: string;
    try {
      raw = await readFile(actionJsonPath, "utf8");
    } catch (err) {
      throw new Error(`${ACTION_JSON_READ_FAILED}: ${errorMessage(err)}`);
    }

    let parsed: unknown;
    try {
      parsed = JSON.parse(raw);
    } catch (err) {
      throw new Error(`${ACTION_JSON_PARSE_FAILED}: ${errorMessage(err)}`);
    }

    let reader: PackageReader;
    try {
      reader = this.normalizer.getReader(loaded.manifest.schemaVersion);
    } catch {
      reader = this.normalizer.getReader(2);
    }

    let result: ActionReadResult;
    try {
      result = reader.readAction(parsed, actionKey, configPath);
    } catch (err) {
      throw new Error(errorMessage(err));
    }

    const localAction = this.mapRuntimeAction(result.action, loaded.installPath);
    loaded.actions.set(actionKey, localAction);

    if (loaded.defaultAction?.key === actionKey && localAction.available) {
      loaded.defaultAction = localAction;
    }

    return localAction;
  }

  isDefaultActionAvailable(loaded: LoadedInstallation): boolean {
    return loaded.defaultAction?.available === true;
  }

  getAvailableActions(loaded: LoadedInstallation): RuntimeAction[] {
    const result: RuntimeAction[] = [];
    for (const action of loaded.actions.values()) {
      if (action.available) {
        result.push(action);
      }
    }
    return result;
  }

  findFirstAvailableAction(
    loaded: LoadedInstallation,
    actionKeys: string[],
  ): RuntimeAction | null {
    if (!Array.isArray(actionKeys)) {
      return null;
    }
    for (const key of actionKeys) {
      if (!key) continue;
      const action = loaded.actions.get(key);
      if (action && action.available) {
        return action;
      }
    }
    return null;
  }

  private mapRuntimeAction(
    action: SharedRuntimeAction,
    installPath: string,
  ): RuntimeAction {
    const configPath = action.configPath;
    const actionConfigDir = dirname(join(installPath, configPath));
    const frames = action.frames.map((f) => join(actionConfigDir, f.file));
    const frameDurationMs = action.fps > 0 ? Math.round(1000 / action.fps) : 100;

    const returnAction =
      action.returnTo.type === "action" ? action.returnTo.actionKey : undefined;

    const extended = action as SharedRuntimeAction & {
      supportsDefaultIdle?: boolean;
      isStableStateCandidate?: boolean;
      isTransitionOnly?: boolean;
      interruptAfterMs?: number;
      category?: string;
    };

    return {
      key: action.actionKey,
      name: action.displayName,
      version: String(action.version),
      loopType: action.playbackMode,
      fps: action.fps,
      frameDurationMs,
      frameCount: action.frames.length,
      frames,
      anchor: action.anchor,
      interruptible: action.interruptible,
      returnAction,
      config: configPath,
      available: true,
      playbackMode: action.playbackMode,
      priority: action.priority,
      cooldownMs: action.cooldownMs,
      mutexGroup: action.mutexGroup || undefined,
      minimumPlayMs: action.minimumPlayMs,
      maximumPlayMs: action.maximumPlayMs || null,
      returnTo: action.returnTo,
      supportsDefaultIdle: extended.supportsDefaultIdle,
      isStableStateCandidate: extended.isStableStateCandidate,
      isTransitionOnly: extended.isTransitionOnly,
      interruptAfterMs: extended.interruptAfterMs,
      category: extended.category,
    };
  }

  private mapManifestAction(action: SharedRuntimeAction): ManifestAction {
    const frameDurationMs = action.fps > 0 ? Math.round(1000 / action.fps) : 100;
    const returnAction =
      action.returnTo.type === "action" ? action.returnTo.actionKey : undefined;

    return {
      key: action.actionKey,
      name: action.displayName,
      version: String(action.version),
      loopType: action.playbackMode,
      fps: action.fps,
      frameDurationMs,
      frameCount: action.frames.length,
      frames: action.frames.map((f) => f.file),
      anchor: action.anchor,
      interruptible: action.interruptible,
      returnAction,
      config: action.configPath,
    };
  }
}
