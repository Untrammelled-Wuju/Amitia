import { ipcMain, BrowserWindow, powerMonitor } from "electron";
import { join, isAbsolute, normalize, relative } from "node:path";
import { pathToFileURL } from "node:url";
import { existsSync } from "node:fs";
import { ANIMATION_IPC_CHANNELS } from "../../shared/animation-ipc";
import type {
  PackagePlaybackSnapshot,
  PlaybackEvent,
  PlaybackSnapshot,
  PlayActionCommand,
  PlaybackRecoverySnapshot,
  LoopType,
} from "../../desktop-pet/animation/contracts";
import type { LoadedInstallation } from "./resource-loader";
import type { InstallationInfo } from "./manager";

const MIME_MAP: Record<string, string> = {
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".gif": "image/gif",
  ".webp": "image/webp",
  ".bmp": "image/bmp",
  ".json": "application/json",
};

function getMimeFromPath(filePath: string): string {
  const lower = filePath.toLowerCase();
  for (const [ext, mime] of Object.entries(MIME_MAP)) {
    if (lower.endsWith(ext)) return mime;
  }
  return "application/octet-stream";
}

function isPathSafe(basePath: string, targetPath: string): boolean {
  const normalized = normalize(isAbsolute(targetPath) ? targetPath : join(basePath, targetPath));
  const rel = relative(basePath, normalized);
  return !rel.startsWith("..") && !isAbsolute(rel);
}

function buildPackageSnapshot(
  installation: InstallationInfo,
  loaded: LoadedInstallation,
  packageRevision: number,
): PackagePlaybackSnapshot {
  const actions = loaded.manifest.actions.map((action) => {
    const configUrl = pathToFileURL(
      join(installation.installPath, "actions", action.key, "config.json"),
    ).href;

    return {
      actionKey: action.key,
      configUrl,
      specSnapshot: {
        actionKey: action.key,
        displayName: action.name,
        category: "general",
        version: parseInt(action.version || "1", 10) || 1,
        loopType: action.loopType as LoopType,
        defaultPriority: 50,
        interruptible: action.interruptible,
        interruptAfterMs: 0,
        minimumPlayMs: 0,
        maximumPlayMs: null,
        cooldownMs: 0,
        mutexGroup: null,
        returnTarget: action.returnAction
          ? { type: "action" as const, actionKey: action.returnAction }
          : { type: "default" as const },
        supportsDefaultIdle: true,
        isStableStateCandidate: action.loopType === "loop",
        isTransitionOnly: false,
      },
    };
  });

  const previewUrl = loaded.manifest.preview
    ? pathToFileURL(join(installation.installPath, loaded.manifest.preview)).href
    : undefined;

  return {
    packageId: loaded.manifest.packageId,
    packageRevision,
    schemaVersion: loaded.manifest.schemaVersion,
    canvas: loaded.manifest.canvas,
    defaultActionKey: loaded.manifest.defaultAction,
    actions,
    previewUrl,
    interpolationMode: "nearest",
  };
}

export interface AnimationIpcAdapterDeps {
  getActiveInstallation: () => InstallationInfo | null;
  getLoadedInstallation: () => LoadedInstallation | null;
  getPackageRevision: () => number;
  getPetWindow: () => BrowserWindow | null;
  onPlaybackEvent?: (event: PlaybackEvent) => void;
  onSnapshotUpdate?: (snapshot: PlaybackSnapshot) => void;
  onClick?: (x: number, y: number) => void;
  onDoubleClick?: (x: number, y: number) => void;
  onHover?: (x: number, y: number) => void;
  onPlayActionForwarded?: (command: PlayActionCommand) => void;
}

export class AnimationIpcAdapter {
  private deps: AnimationIpcAdapterDeps;
  private registered = false;
  private powerMonitorListenersAttached = false;

  constructor(deps: AnimationIpcAdapterDeps) {
    this.deps = deps;
  }

  register(): void {
    if (this.registered) return;
    this.registered = true;

    ipcMain.handle(ANIMATION_IPC_CHANNELS.getPackageSnapshot, async () => {
      const installation = this.deps.getActiveInstallation();
      const loaded = this.deps.getLoadedInstallation();
      if (!installation || !loaded) return null;
      const revision = this.deps.getPackageRevision();
      return buildPackageSnapshot(installation, loaded, revision);
    });

    ipcMain.handle(
      ANIMATION_IPC_CHANNELS.resolveResourceUrl,
      async (_event, relativePath: string) => {
        const installation = this.deps.getActiveInstallation();
        if (!installation) {
          return { url: "", mime: "application/octet-stream" };
        }
        const basePath = installation.installPath;
        if (!isPathSafe(basePath, relativePath)) {
          return { url: "", mime: "application/octet-stream" };
        }
        const fullPath = normalize(
          isAbsolute(relativePath) ? relativePath : join(basePath, relativePath),
        );
        if (!existsSync(fullPath)) {
          return { url: "", mime: "application/octet-stream" };
        }
        const url = pathToFileURL(fullPath).href;
        const mime = getMimeFromPath(fullPath);
        return { url, mime };
      },
    );

    ipcMain.handle(ANIMATION_IPC_CHANNELS.getDiagnostics, async () => {
      return { source: "main-process", timestamp: Date.now() };
    });

    ipcMain.on(ANIMATION_IPC_CHANNELS.reportEvent, (_event, payload: PlaybackEvent) => {
      this.deps.onPlaybackEvent?.(payload);
    });

    ipcMain.on(
      ANIMATION_IPC_CHANNELS.reportSnapshot,
      (_event, payload: PlaybackSnapshot) => {
        this.deps.onSnapshotUpdate?.(payload);
      },
    );

    ipcMain.on(
      ANIMATION_IPC_CHANNELS.sendClick,
      (_event, data: { x: number; y: number }) => {
        this.deps.onClick?.(data.x, data.y);
      },
    );

    ipcMain.on(
      ANIMATION_IPC_CHANNELS.sendDoubleClick,
      (_event, data: { x: number; y: number }) => {
        this.deps.onDoubleClick?.(data.x, data.y);
      },
    );

    ipcMain.on(
      ANIMATION_IPC_CHANNELS.sendHover,
      (_event, data: { x: number; y: number }) => {
        this.deps.onHover?.(data.x, data.y);
      },
    );

    this.attachPowerMonitorListeners();
  }

  private attachPowerMonitorListeners(): void {
    if (this.powerMonitorListenersAttached) return;
    this.powerMonitorListenersAttached = true;

    powerMonitor.on("suspend", () => {
      this.sendToRenderer(ANIMATION_IPC_CHANNELS.systemSuspend);
    });

    powerMonitor.on("resume", () => {
      this.sendToRenderer(ANIMATION_IPC_CHANNELS.systemResume);
    });
  }

  sendPlayAction(command: PlayActionCommand): void {
    this.sendToRenderer(ANIMATION_IPC_CHANNELS.playAction, command);
    this.deps.onPlayActionForwarded?.(command);
  }

  sendPause(): void {
    this.sendToRenderer(ANIMATION_IPC_CHANNELS.pause);
  }

  sendResume(): void {
    this.sendToRenderer(ANIMATION_IPC_CHANNELS.resume);
  }

  sendStop(): void {
    this.sendToRenderer(ANIMATION_IPC_CHANNELS.stop);
  }

  sendSwitchPackage(snapshot: PackagePlaybackSnapshot): void {
    this.sendToRenderer(ANIMATION_IPC_CHANNELS.switchPackage, snapshot);
  }

  sendWindowHidden(): void {
    this.sendToRenderer(ANIMATION_IPC_CHANNELS.windowHidden);
  }

  sendWindowShown(): void {
    this.sendToRenderer(ANIMATION_IPC_CHANNELS.windowShown);
  }

  sendRecovery(snapshot: PlaybackRecoverySnapshot): void {
    this.sendToRenderer(ANIMATION_IPC_CHANNELS.recovery, snapshot);
  }

  sendUpdateDefaultAction(actionKey: string): void {
    this.sendToRenderer(ANIMATION_IPC_CHANNELS.updateDefaultAction, actionKey);
  }

  private sendToRenderer(channel: string, data?: unknown): void {
    const win = this.deps.getPetWindow();
    if (!win || win.isDestroyed()) return;
    try {
      if (data !== undefined) {
        win.webContents.send(channel, data);
      } else {
        win.webContents.send(channel);
      }
    } catch {
      void 0;
    }
  }

  buildSnapshot(
    installation: InstallationInfo,
    loaded: LoadedInstallation,
    revision: number,
  ): PackagePlaybackSnapshot {
    return buildPackageSnapshot(installation, loaded, revision);
  }

  unregister(): void {
    if (!this.registered) return;
    this.registered = false;

    ipcMain.removeHandler(ANIMATION_IPC_CHANNELS.getPackageSnapshot);
    ipcMain.removeHandler(ANIMATION_IPC_CHANNELS.resolveResourceUrl);
    ipcMain.removeHandler(ANIMATION_IPC_CHANNELS.getDiagnostics);
    ipcMain.removeAllListeners(ANIMATION_IPC_CHANNELS.reportEvent);
    ipcMain.removeAllListeners(ANIMATION_IPC_CHANNELS.reportSnapshot);
    ipcMain.removeAllListeners(ANIMATION_IPC_CHANNELS.sendClick);
    ipcMain.removeAllListeners(ANIMATION_IPC_CHANNELS.sendDoubleClick);
    ipcMain.removeAllListeners(ANIMATION_IPC_CHANNELS.sendHover);
  }
}

export { buildPackageSnapshot };
