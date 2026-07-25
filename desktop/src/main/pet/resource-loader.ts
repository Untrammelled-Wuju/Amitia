import { access, readFile } from "node:fs/promises";
import { join } from "node:path";

const MANIFEST_SCHEMA_VERSION = 1;

const DEFAULT_ACTION_NOT_FOUND_ERROR = "DEFAULT_ACTION_NOT_FOUND";
const DEFAULT_ACTION_INVALID_ERROR = "DEFAULT_ACTION_INVALID";
const UNSUPPORTED_SCHEMA_VERSION_ERROR = "UNSUPPORTED_SCHEMA_VERSION";
const MANIFEST_READ_FAILED_ERROR = "MANIFEST_READ_FAILED";
const MANIFEST_PARSE_FAILED_ERROR = "MANIFEST_PARSE_FAILED";

export type ActionLoopType = "loop" | "once" | "hold";

export interface ManifestAction {
  key: string;
  name: string;
  version: string;
  loopType: ActionLoopType;
  fps: number;
  frameDurationMs: number;
  frameCount: number;
  frames: string[];
  anchor?: { x: number; y: number };
  interruptible: boolean;
  returnAction?: string;
}

export interface Manifest {
  packageId: string;
  schemaVersion: number;
  name: string;
  characterId: string;
  canvas: { width: number; height: number };
  defaultAction: string;
  preview?: string;
  actions: ManifestAction[];
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
  anchor?: { x: number; y: number };
  interruptible: boolean;
  returnAction?: string;
  available: boolean;
  loadError?: string;
}

export interface LoadedInstallation {
  installationId: string;
  manifest: Manifest;
  actions: Map<string, RuntimeAction>;
  defaultAction: RuntimeAction | null;
  installPath: string;
  manifestPath: string;
  previewPath: string;
}

export {
  DEFAULT_ACTION_NOT_FOUND_ERROR,
  DEFAULT_ACTION_INVALID_ERROR,
  UNSUPPORTED_SCHEMA_VERSION_ERROR,
  MANIFEST_READ_FAILED_ERROR,
  MANIFEST_PARSE_FAILED_ERROR,
};

interface ManifestFile {
  schemaVersion?: number;
  packageId?: string;
  name?: string;
  characterId?: string;
  canvas?: { width?: number; height?: number };
  defaultAction?: string;
  preview?: string;
  actions?: Array<{ key?: string; name?: string; config?: string }>;
}

interface ActionFileFrame {
  index?: number;
  file?: string;
  durationMs?: number;
}

interface ActionFile {
  key?: string;
  name?: string;
  version?: number;
  loopType?: string;
  fps?: number;
  frameDurationMs?: number;
  frameCount?: number;
  frames?: Array<ActionFileFrame | string>;
  anchor?: { type?: string; x?: number; y?: number };
  interruptible?: boolean;
  returnAction?: string;
}

interface ActionLoadResult {
  manifestAction: ManifestAction | null;
  runtimeAction: RuntimeAction;
}

interface FrameCheckResult {
  ok: boolean;
  error?: string;
}

export class ResourceLoader {
  constructor() {}

  async loadInstallation(
    installPath: string,
    manifestPath: string,
  ): Promise<LoadedInstallation> {
    const file = await this.readManifestFile(manifestPath);

    if (file.schemaVersion !== MANIFEST_SCHEMA_VERSION) {
      throw new Error(
        `${UNSUPPORTED_SCHEMA_VERSION_ERROR}: ${file.schemaVersion ?? "unknown"}`,
      );
    }

    const manifestActions: ManifestAction[] = [];
    const runtimeActions = new Map<string, RuntimeAction>();

    const entries = Array.isArray(file.actions) ? file.actions : [];
    for (const entry of entries) {
      const actionKey = entry?.key;
      if (!actionKey) {
        continue;
      }

      const result = await this.loadActionEntry(
        installPath,
        actionKey,
        entry.name ?? actionKey,
      );

      if (result.manifestAction) {
        manifestActions.push(result.manifestAction);
      }
      runtimeActions.set(actionKey, result.runtimeAction);
    }

    const defaultAction = file.defaultAction ?? "";
    const manifest: Manifest = {
      packageId: file.packageId ?? "",
      schemaVersion: MANIFEST_SCHEMA_VERSION,
      name: file.name ?? "",
      characterId: file.characterId ?? "",
      canvas: {
        width: file.canvas?.width ?? 0,
        height: file.canvas?.height ?? 0,
      },
      defaultAction,
      preview: file.preview,
      actions: manifestActions,
    };

    if (!defaultAction) {
      throw new Error(DEFAULT_ACTION_NOT_FOUND_ERROR);
    }

    const defaultRuntime = runtimeActions.get(defaultAction);
    if (!defaultRuntime) {
      throw new Error(DEFAULT_ACTION_NOT_FOUND_ERROR);
    }
    if (!defaultRuntime.available) {
      const detail = defaultRuntime.loadError ?? "";
      throw new Error(
        detail
          ? `${DEFAULT_ACTION_INVALID_ERROR}: ${detail}`
          : DEFAULT_ACTION_INVALID_ERROR,
      );
    }

    const previewPath = manifest.preview
      ? join(installPath, manifest.preview)
      : join(installPath, "preview.png");

    const installationId = manifest.packageId || installPath;

    return {
      installationId,
      manifest,
      actions: runtimeActions,
      defaultAction: defaultRuntime,
      installPath,
      manifestPath,
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

    const result = await this.loadActionEntry(
      loaded.installPath,
      actionKey,
      manifestAction?.name ?? cached?.name ?? actionKey,
    );

    if (result.manifestAction) {
      if (manifestAction) {
        Object.assign(manifestAction, result.manifestAction);
      } else {
        loaded.manifest.actions.push(result.manifestAction);
      }
    }

    loaded.actions.set(actionKey, result.runtimeAction);

    if (
      loaded.defaultAction?.key === actionKey &&
      result.runtimeAction.available
    ) {
      loaded.defaultAction = result.runtimeAction;
    }

    return result.runtimeAction;
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

  private async readManifestFile(
    manifestPath: string,
  ): Promise<ManifestFile> {
    let raw: string;
    try {
      raw = await readFile(manifestPath, "utf8");
    } catch (err) {
      throw new Error(
        `${MANIFEST_READ_FAILED_ERROR}: ${this.errorMessage(err)}`,
      );
    }

    try {
      return JSON.parse(raw) as ManifestFile;
    } catch (err) {
      throw new Error(
        `${MANIFEST_PARSE_FAILED_ERROR}: ${this.errorMessage(err)}`,
      );
    }
  }

  private async loadActionEntry(
    installPath: string,
    actionKey: string,
    fallbackName: string,
  ): Promise<ActionLoadResult> {
    const actionJsonPath = join(
      installPath,
      "actions",
      actionKey,
      "action.json",
    );

    const buildFailed = (error: string): ActionLoadResult => ({
      manifestAction: null,
      runtimeAction: this.buildFailedRuntimeAction(
        actionKey,
        fallbackName,
        error,
      ),
    });

    let raw: string;
    try {
      raw = await readFile(actionJsonPath, "utf8");
    } catch (err) {
      return buildFailed(`ACTION_JSON_READ_FAILED: ${this.errorMessage(err)}`);
    }

    let parsed: ActionFile;
    try {
      parsed = JSON.parse(raw) as ActionFile;
    } catch (err) {
      return buildFailed(`ACTION_JSON_PARSE_FAILED: ${this.errorMessage(err)}`);
    }

    const loopType = this.normalizeLoopType(parsed.loopType);
    const relFrames = this.extractFrameFiles(parsed);
    const fps = this.normalizeNumber(parsed.fps, 0);
    const frameDurationMs = this.normalizeNumber(parsed.frameDurationMs, 0);
    const frameCount = this.normalizeNumber(
      parsed.frameCount,
      relFrames.length,
    );
    const anchor = this.normalizeAnchor(parsed.anchor);
    const interruptible = parsed.interruptible === true;
    const returnAction = parsed.returnAction || undefined;
    const name = parsed.name || fallbackName;
    const key = parsed.key || actionKey;
    const version = String(parsed.version ?? 1);

    const manifestAction: ManifestAction = {
      key,
      name,
      version,
      loopType,
      fps,
      frameDurationMs,
      frameCount,
      frames: relFrames,
      anchor,
      interruptible,
      returnAction,
    };

    const absoluteFrames = relFrames.map((rel) => join(installPath, rel));
    const frameCheck = await this.checkFramesExist(absoluteFrames);
    if (!frameCheck.ok) {
      return {
        manifestAction,
        runtimeAction: {
          key,
          name,
          version,
          loopType,
          fps,
          frameDurationMs,
          frameCount,
          frames: absoluteFrames,
          anchor,
          interruptible,
          returnAction,
          available: false,
          loadError: frameCheck.error,
        },
      };
    }

    return {
      manifestAction,
      runtimeAction: {
        key,
        name,
        version,
        loopType,
        fps,
        frameDurationMs,
        frameCount,
        frames: absoluteFrames,
        anchor,
        interruptible,
        returnAction,
        available: true,
      },
    };
  }

  private buildFailedRuntimeAction(
    actionKey: string,
    fallbackName: string,
    error: string,
  ): RuntimeAction {
    return {
      key: actionKey,
      name: fallbackName,
      version: "1",
      loopType: "loop",
      fps: 0,
      frameDurationMs: 0,
      frameCount: 0,
      frames: [],
      interruptible: false,
      available: false,
      loadError: error,
    };
  }

  private async checkFramesExist(
    absoluteFrames: string[],
  ): Promise<FrameCheckResult> {
    for (const framePath of absoluteFrames) {
      try {
        await access(framePath);
      } catch (err) {
        return {
          ok: false,
          error: `FRAME_MISSING: ${framePath} (${this.errorMessage(err)})`,
        };
      }
    }
    return { ok: true };
  }

  private normalizeLoopType(value: string | undefined): ActionLoopType {
    if (value === "loop" || value === "once" || value === "hold") {
      return value;
    }
    return "loop";
  }

  private extractFrameFiles(parsed: ActionFile): string[] {
    if (!Array.isArray(parsed.frames)) {
      return [];
    }
    const result: string[] = [];
    for (const item of parsed.frames) {
      if (typeof item === "string") {
        if (item) {
          result.push(item);
        }
        continue;
      }
      const file = item?.file;
      if (typeof file === "string" && file) {
        result.push(file);
      }
    }
    return result;
  }

  private normalizeAnchor(
    anchor: ActionFile["anchor"],
  ): { x: number; y: number } | undefined {
    if (!anchor) {
      return undefined;
    }
    const x =
      typeof anchor.x === "number" ? anchor.x : Number(anchor.x);
    const y =
      typeof anchor.y === "number" ? anchor.y : Number(anchor.y);
    if (!Number.isFinite(x) || !Number.isFinite(y)) {
      return undefined;
    }
    return { x, y };
  }

  private normalizeNumber(
    value: number | undefined,
    fallback: number,
  ): number {
    if (typeof value !== "number" || !Number.isFinite(value)) {
      return fallback;
    }
    return value;
  }

  private errorMessage(err: unknown): string {
    if (err instanceof Error) {
      return err.message;
    }
    return String(err);
  }
}
