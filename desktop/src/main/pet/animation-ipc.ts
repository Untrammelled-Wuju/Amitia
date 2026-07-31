import { ipcMain, BrowserWindow, powerMonitor } from "electron";
import { join, isAbsolute, normalize, relative } from "node:path";
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
import { buildPetResourceUrl } from "./resource-resolver";
import { registerPetProtocol } from "./resource-protocol";

const MIME_MAP: Record<string, string> = {
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".gif": "image/gif",
  ".webp": "image/webp",
  ".bmp": "image/bmp",
  ".json": "application/json",
};

const PLAYBACK_EVENT_TYPE_WHITELIST = new Set<string>([
  "playback.command_accepted",
  "playback.command_rejected",
  "playback.command_queued",
  "playback.action_loading",
  "playback.action_started",
  "playback.frame_presented",
  "playback.action_holding",
  "playback.action_completed",
  "playback.action_interrupted",
  "playback.action_expired",
  "playback.action_failed",
  "playback.fallback_started",
  "playback.fallback_failed",
  "playback.default_changed",
  "playback.package_switched",
  "playback.cache_pressure",
  "playback.recovered",
]);

const MAX_COORDINATE = 1_000_000;
const MAX_STRING_LENGTH = 4096;
const MAX_PENDING_COMMANDS = 32;
const COMMAND_TTL_MS = 10_000;

export type RendererDeliveryFailureReason =
  | "window_missing"
  | "window_destroyed"
  | "renderer_not_ready"
  | "send_failed";

export type RendererDeliveryResult =
  | { delivered: true }
  | {
      delivered: false;
      reason: RendererDeliveryFailureReason;
      error?: string;
    };

interface PendingCommand {
  command: PlayActionCommand;
  queuedAt: number;
}

interface DurableState {
  packageSnapshot: PackagePlaybackSnapshot | null;
  defaultActionKey: string | null;
  windowVisible: boolean;
}

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

function isValidCoordinate(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value) && Math.abs(value) <= MAX_COORDINATE;
}

function isValidPlaybackEvent(payload: unknown): payload is PlaybackEvent {
  if (!payload || typeof payload !== "object") return false;
  const p = payload as Record<string, unknown>;
  if (typeof p.type !== "string" || !PLAYBACK_EVENT_TYPE_WHITELIST.has(p.type)) return false;
  if (typeof p.timestamp !== "number" || !Number.isFinite(p.timestamp)) return false;
  if (p.actionKey !== undefined && typeof p.actionKey !== "string") return false;
  if (p.actionKey !== undefined && (p.actionKey as string).length > MAX_STRING_LENGTH) return false;
  if (p.reason !== undefined && typeof p.reason !== "string") return false;
  if (p.reason !== undefined && (p.reason as string).length > MAX_STRING_LENGTH) return false;
  if (p.frameIndex !== undefined && typeof p.frameIndex !== "number") return false;
  if (p.packageRevision !== undefined && (typeof p.packageRevision !== "number" || p.packageRevision < 0)) return false;
  return true;
}

function isValidSnapshot(payload: unknown): payload is PlaybackSnapshot {
  if (!payload || typeof payload !== "object") return false;
  const p = payload as Record<string, unknown>;
  if (typeof p.packageRevision !== "number" || p.packageRevision < 0) return false;
  if (p.currentActionKey !== null && typeof p.currentActionKey !== "string") return false;
  return true;
}

function isValidHitMaskPayload(payload: unknown): payload is {
  width: number;
  height: number;
  threshold: number;
  data: Uint8Array;
  frameHash: string;
} {
  if (!payload || typeof payload !== "object") return false;
  const p = payload as Record<string, unknown>;
  if (typeof p.width !== "number" || !Number.isFinite(p.width) || p.width <= 0 || p.width > 256) return false;
  if (typeof p.height !== "number" || !Number.isFinite(p.height) || p.height <= 0 || p.height > 256) return false;
  if (typeof p.threshold !== "number" || !Number.isFinite(p.threshold)) return false;
  if (!(p.data instanceof Uint8Array)) return false;
  const expected = Math.floor(p.width) * Math.floor(p.height);
  if (p.data.length < expected) return false;
  if (typeof p.frameHash !== "string" || p.frameHash.length > MAX_STRING_LENGTH) return false;
  return true;
}

function buildPackageSnapshot(
  installation: InstallationInfo,
  loaded: LoadedInstallation,
  packageRevision: number,
): PackagePlaybackSnapshot {
  const actions = loaded.manifest.actions.map((action) => {
    const configRelative = action.config || join("actions", action.key, "action.json");
    const configUrl = buildPetResourceUrl(installation.id, configRelative);

    const runtimeAction = loaded.actions.get(action.key);

    const defaultPriority = runtimeAction?.priority ?? 50;
    const cooldownMs = runtimeAction?.cooldownMs ?? 0;
    const mutexGroup = runtimeAction?.mutexGroup ?? null;
    const minimumPlayMs = runtimeAction?.minimumPlayMs ?? 0;
    const maximumPlayMs = runtimeAction?.maximumPlayMs ?? null;
    const playbackMode = runtimeAction?.playbackMode;
    const loopType = (playbackMode ?? action.loopType) as LoopType;

    const returnTarget = runtimeAction?.returnTo
      ? runtimeAction.returnTo.type === "action"
        ? { type: "action" as const, actionKey: runtimeAction.returnTo.actionKey }
        : runtimeAction.returnTo.type === "default"
          ? { type: "default" as const }
          : runtimeAction.returnTo.type === "previous"
            ? { type: "previous" as const }
            : runtimeAction.returnTo.type === "current_activity"
              ? { type: "current_activity" as const }
              : { type: "none" as const }
      : action.returnAction
        ? { type: "action" as const, actionKey: action.returnAction }
        : { type: "default" as const };

    return {
      actionKey: action.key,
      configUrl,
      specSnapshot: {
        actionKey: action.key,
        displayName: runtimeAction?.name ?? action.name,
        category: "general",
        version: parseInt(action.version || "1", 10) || 1,
        loopType,
        defaultPriority,
        interruptible: action.interruptible,
        interruptAfterMs: 0,
        minimumPlayMs,
        maximumPlayMs,
        cooldownMs,
        mutexGroup,
        returnTarget,
        supportsDefaultIdle: true,
        isStableStateCandidate: loopType === "loop",
        isTransitionOnly: false,
      },
    };
  });

  const previewUrl = loaded.manifest.preview
    ? buildPetResourceUrl(installation.id, loaded.manifest.preview)
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
  onHitMask?: (width: number, height: number, data: Uint8Array, threshold: number) => void;
  onRendererReady?: () => void;
  onRendererReadyAck?: (payload: { snapshotApplied: boolean }) => void;
  onDeliveryFailed?: (reason: RendererDeliveryFailureReason, command?: PlayActionCommand) => void;
}

export class AnimationIpcAdapter {
  private deps: AnimationIpcAdapterDeps;
  private registered = false;
  private powerMonitorListenersAttached = false;
  private rendererReady = false;
  private pendingCommands: PendingCommand[] = [];
  private durable: DurableState = {
    packageSnapshot: null,
    defaultActionKey: null,
    windowVisible: true,
  };

  constructor(deps: AnimationIpcAdapterDeps) {
    this.deps = deps;
  }

  private isCurrentPetRenderer(
    event: Electron.IpcMainEvent | Electron.IpcMainInvokeEvent,
  ): boolean {
    const win = this.deps.getPetWindow();
    return !!win &&
      !win.isDestroyed() &&
      event.sender.id === win.webContents.id;
  }

  private readonly onReportEvent = (
    event: Electron.IpcMainEvent,
    payload: PlaybackEvent,
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    if (!isValidPlaybackEvent(payload)) return;
    this.deps.onPlaybackEvent?.(payload);
  };

  private readonly onReportSnapshot = (
    event: Electron.IpcMainEvent,
    payload: PlaybackSnapshot,
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    if (!isValidSnapshot(payload)) return;
    this.deps.onSnapshotUpdate?.(payload);
  };

  private readonly onSendClick = (
    event: Electron.IpcMainEvent,
    data: { x: number; y: number },
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    if (!data || !isValidCoordinate(data.x) || !isValidCoordinate(data.y)) return;
    this.deps.onClick?.(data.x, data.y);
  };

  private readonly onSendDoubleClick = (
    event: Electron.IpcMainEvent,
    data: { x: number; y: number },
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    if (!data || !isValidCoordinate(data.x) || !isValidCoordinate(data.y)) return;
    this.deps.onDoubleClick?.(data.x, data.y);
  };

  private readonly onSendHover = (
    event: Electron.IpcMainEvent,
    data: { x: number; y: number },
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    if (!data || !isValidCoordinate(data.x) || !isValidCoordinate(data.y)) return;
    this.deps.onHover?.(data.x, data.y);
  };

  private readonly onRendererReady = (
    event: Electron.IpcMainEvent,
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    this.rendererReady = true;
    this.flushDurableState();
    this.flushPendingCommands();
    this.deps.onRendererReady?.();
  };

  private readonly onRendererReadyAck = (
    event: Electron.IpcMainEvent,
    payload: { snapshotApplied: boolean },
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    this.deps.onRendererReadyAck?.(payload);
  };

  private readonly onHitMask = (
    event: Electron.IpcMainEvent,
    payload: {
      width: number;
      height: number;
      threshold: number;
      data: Uint8Array;
      frameHash: string;
    },
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    if (!isValidHitMaskPayload(payload)) return;
    this.deps.onHitMask?.(payload.width, payload.height, payload.data, payload.threshold);
  };

  private readonly onGetPackageSnapshot = async (
    event: Electron.IpcMainInvokeEvent,
  ): Promise<PackagePlaybackSnapshot | null> => {
    if (!this.isCurrentPetRenderer(event)) return null;
    const installation = this.deps.getActiveInstallation();
    const loaded = this.deps.getLoadedInstallation();
    if (!installation || !loaded) return null;
    const revision = this.deps.getPackageRevision();
    return buildPackageSnapshot(installation, loaded, revision);
  };

  private readonly onResolveResourceUrl = async (
    event: Electron.IpcMainInvokeEvent,
    relativePath: string,
  ): Promise<{ url: string; mime: string }> => {
    if (!this.isCurrentPetRenderer(event)) {
      return { url: "", mime: "application/octet-stream" };
    }
    if (typeof relativePath !== "string" || relativePath.length > MAX_STRING_LENGTH) {
      return { url: "", mime: "application/octet-stream" };
    }
    const installation = this.deps.getActiveInstallation();
    if (!installation) {
      return { url: "", mime: "application/octet-stream" };
    }
    if (!isPathSafe(installation.installPath, relativePath)) {
      return { url: "", mime: "application/octet-stream" };
    }
    const fullPath = normalize(
      isAbsolute(relativePath) ? relativePath : join(installation.installPath, relativePath),
    );
    if (!existsSync(fullPath)) {
      return { url: "", mime: "application/octet-stream" };
    }
    const cleanRelative = relativePath.replace(/\\/g, "/").replace(/^\/+/, "");
    const url = buildPetResourceUrl(installation.id, cleanRelative);
    const mime = getMimeFromPath(fullPath);
    return { url, mime };
  };

  private readonly onGetDiagnostics = async (
    event: Electron.IpcMainInvokeEvent,
  ): Promise<unknown> => {
    if (!this.isCurrentPetRenderer(event)) return null;
    return { source: "main-process", timestamp: Date.now() };
  };

  private readonly onSuspend = (): void => {
    this.sendToRenderer(ANIMATION_IPC_CHANNELS.systemSuspend);
  };

  private readonly onResume = (): void => {
    this.sendToRenderer(ANIMATION_IPC_CHANNELS.systemResume);
  };

  register(): void {
    if (this.registered) return;
    this.registered = true;
    this.rendererReady = false;

    registerPetProtocol(() => {
      const installation = this.deps.getActiveInstallation();
      const loaded = this.deps.getLoadedInstallation();
      if (!installation || !loaded) return null;
      return {
        installationId: installation.id,
        installPath: installation.installPath,
        manifest: loaded.manifest,
      };
    });

    ipcMain.handle(ANIMATION_IPC_CHANNELS.getPackageSnapshot, this.onGetPackageSnapshot);
    ipcMain.handle(ANIMATION_IPC_CHANNELS.resolveResourceUrl, this.onResolveResourceUrl);
    ipcMain.handle(ANIMATION_IPC_CHANNELS.getDiagnostics, this.onGetDiagnostics);

    ipcMain.on(ANIMATION_IPC_CHANNELS.reportEvent, this.onReportEvent);
    ipcMain.on(ANIMATION_IPC_CHANNELS.reportSnapshot, this.onReportSnapshot);
    ipcMain.on(ANIMATION_IPC_CHANNELS.sendClick, this.onSendClick);
    ipcMain.on(ANIMATION_IPC_CHANNELS.sendDoubleClick, this.onSendDoubleClick);
    ipcMain.on(ANIMATION_IPC_CHANNELS.sendHover, this.onSendHover);
    ipcMain.on(ANIMATION_IPC_CHANNELS.rendererReady, this.onRendererReady);
    ipcMain.on(ANIMATION_IPC_CHANNELS.rendererReadyAck, this.onRendererReadyAck);
    ipcMain.on(ANIMATION_IPC_CHANNELS.hitMask, this.onHitMask);

    this.attachPowerMonitorListeners();
  }

  private attachPowerMonitorListeners(): void {
    if (this.powerMonitorListenersAttached) return;
    this.powerMonitorListenersAttached = true;
    powerMonitor.on("suspend", this.onSuspend);
    powerMonitor.on("resume", this.onResume);
  }

  private detachPowerMonitorListeners(): void {
    if (!this.powerMonitorListenersAttached) return;
    this.powerMonitorListenersAttached = false;
    powerMonitor.off("suspend", this.onSuspend);
    powerMonitor.off("resume", this.onResume);
  }

  isRendererReady(): boolean {
    return this.rendererReady;
  }

  resetRendererReady(): void {
    this.rendererReady = false;
    this.pendingCommands = [];
  }

  private flushDurableState(): void {
    if (this.durable.packageSnapshot) {
      this.sendToRenderer(ANIMATION_IPC_CHANNELS.switchPackage, this.durable.packageSnapshot);
    }
    if (this.durable.defaultActionKey) {
      this.sendToRenderer(ANIMATION_IPC_CHANNELS.updateDefaultAction, this.durable.defaultActionKey);
    }
    if (!this.durable.windowVisible) {
      this.sendToRenderer(ANIMATION_IPC_CHANNELS.windowHidden);
    }
  }

  private flushPendingCommands(): void {
    const now = Date.now();
    const commands = this.pendingCommands;
    this.pendingCommands = [];
    for (const entry of commands) {
      if (now - entry.queuedAt > COMMAND_TTL_MS) {
        this.deps.onDeliveryFailed?.("renderer_not_ready", entry.command);
        continue;
      }
      const result = this.deliverToRenderer(ANIMATION_IPC_CHANNELS.playAction, entry.command);
      if (!result.delivered) {
        this.deps.onDeliveryFailed?.(result.reason, entry.command);
      }
    }
  }

  sendPlayAction(command: PlayActionCommand): RendererDeliveryResult {
    if (!this.rendererReady) {
      if (this.pendingCommands.length >= MAX_PENDING_COMMANDS) {
        const dropped = this.pendingCommands.shift();
        if (dropped) {
          this.deps.onDeliveryFailed?.("renderer_not_ready", dropped.command);
        }
      }
      this.pendingCommands.push({ command, queuedAt: Date.now() });
      this.deps.onPlayActionForwarded?.(command);
      return { delivered: false, reason: "renderer_not_ready" };
    }

    const result = this.deliverToRenderer(ANIMATION_IPC_CHANNELS.playAction, command);
    if (result.delivered) {
      this.deps.onPlayActionForwarded?.(command);
    } else {
      this.deps.onDeliveryFailed?.(result.reason, command);
    }
    return result;
  }

  sendPause(): RendererDeliveryResult {
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.pause);
  }

  sendResume(): RendererDeliveryResult {
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.resume);
  }

  sendStop(): RendererDeliveryResult {
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.stop);
  }

  sendSwitchPackage(snapshot: PackagePlaybackSnapshot): RendererDeliveryResult {
    this.durable.packageSnapshot = snapshot;
    if (!this.rendererReady) return { delivered: false, reason: "renderer_not_ready" };
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.switchPackage, snapshot);
  }

  sendWindowHidden(): RendererDeliveryResult {
    this.durable.windowVisible = false;
    if (!this.rendererReady) return { delivered: false, reason: "renderer_not_ready" };
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.windowHidden);
  }

  sendWindowShown(): RendererDeliveryResult {
    this.durable.windowVisible = true;
    if (!this.rendererReady) return { delivered: false, reason: "renderer_not_ready" };
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.windowShown);
  }

  sendRecovery(snapshot: PlaybackRecoverySnapshot): RendererDeliveryResult {
    if (!this.rendererReady) return { delivered: false, reason: "renderer_not_ready" };
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.recovery, snapshot);
  }

  sendUpdateDefaultAction(actionKey: string): RendererDeliveryResult {
    this.durable.defaultActionKey = actionKey;
    if (!this.rendererReady) return { delivered: false, reason: "renderer_not_ready" };
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.updateDefaultAction, actionKey);
  }

  private deliverToRenderer(channel: string, data?: unknown): RendererDeliveryResult {
    const win = this.deps.getPetWindow();
    if (!win) return { delivered: false, reason: "window_missing" };
    if (win.isDestroyed()) return { delivered: false, reason: "window_destroyed" };
    if (!this.rendererReady && channel !== ANIMATION_IPC_CHANNELS.switchPackage) {
      return { delivered: false, reason: "renderer_not_ready" };
    }
    try {
      if (data !== undefined) {
        win.webContents.send(channel, data);
      } else {
        win.webContents.send(channel);
      }
      return { delivered: true };
    } catch (err) {
      return {
        delivered: false,
        reason: "send_failed",
        error: err instanceof Error ? err.message : String(err),
      };
    }
  }

  private sendToRenderer(channel: string, data?: unknown): void {
    this.deliverToRenderer(channel, data);
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

    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.reportEvent, this.onReportEvent);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.reportSnapshot, this.onReportSnapshot);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.sendClick, this.onSendClick);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.sendDoubleClick, this.onSendDoubleClick);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.sendHover, this.onSendHover);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.rendererReady, this.onRendererReady);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.rendererReadyAck, this.onRendererReadyAck);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.hitMask, this.onHitMask);

    this.detachPowerMonitorListeners();
    this.powerMonitorListenersAttached = false;
    this.rendererReady = false;
    this.pendingCommands = [];
  }
}

export { buildPackageSnapshot };
