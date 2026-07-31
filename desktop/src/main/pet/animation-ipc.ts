import { ipcMain, BrowserWindow, powerMonitor } from "electron";
import { join, isAbsolute, normalize, relative } from "node:path";
import { ANIMATION_IPC_CHANNELS } from "../../shared/animation-ipc";
import type {
  PetDragIpcPayload,
  PetHitMaskPayload,
  RuntimeReadyPayload,
} from "../../shared/animation-ipc";
import type {
  PackagePlaybackSnapshot,
  PlaybackEvent,
  PlaybackSnapshot,
  PlayActionCommand,
  PlaybackRecoverySnapshot,
  LoopType,
  ReturnTarget,
} from "../../desktop-pet/animation/contracts";
import type { LoadedInstallation } from "./resource-loader";
import type { InstallationInfo } from "./manager";
import { buildPetResourceUrl } from "./resource-resolver";
import {
  PetResourceProtocolRegistry,
  buildResourceIndex,
} from "./resource-protocol";

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
const RUNTIME_READY_TIMEOUT_MS = 30_000;
const MAX_MASK_REVISION = 0x7fffffff;
const MAX_RESOURCE_BYTES = 64 * 1024 * 1024;

export type RendererDeliveryStatus = "delivered" | "queued" | "rejected";

export type RendererDeliveryFailureReason =
  | "window_missing"
  | "window_destroyed"
  | "renderer_not_ready"
  | "send_failed"
  | "queue_overflow"
  | "command_invalid"
  | "ttl_expired"
  | "renderer_reset";

export interface RendererDeliveryResult {
  status: RendererDeliveryStatus;
  reason?: RendererDeliveryFailureReason;
  error?: string;
}

interface PendingPlayCommand {
  command: PlayActionCommand;
  queuedAt: number;
  expiresAt: number;
}

interface DurableState {
  packageSnapshot: PackagePlaybackSnapshot | null;
  defaultActionKey: string | null;
  windowVisible: boolean;
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

function isValidHitMaskPayload(payload: unknown): payload is PetHitMaskPayload {
  if (!payload || typeof payload !== "object") return false;
  const p = payload as Record<string, unknown>;
  if (typeof p.width !== "number" || !Number.isFinite(p.width) || p.width <= 0 || p.width > 256) return false;
  if (typeof p.height !== "number" || !Number.isFinite(p.height) || p.height <= 0 || p.height > 256) return false;
  if (typeof p.threshold !== "number" || !Number.isFinite(p.threshold) || p.threshold < 0 || p.threshold > 255) return false;
  if (!(p.data instanceof Uint8Array)) return false;
  const expected = Math.floor(p.width) * Math.floor(p.height);
  if (p.data.length < expected) return false;
  if (typeof p.packageRevision !== "number" || p.packageRevision < 0) return false;
  if (typeof p.actionKey !== "string" || p.actionKey.length > MAX_STRING_LENGTH) return false;
  if (typeof p.frameIndex !== "number" || p.frameIndex < 0) return false;
  if (typeof p.playbackInstanceId !== "string" || p.playbackInstanceId.length > MAX_STRING_LENGTH) return false;
  if (typeof p.maskRevision !== "number" || p.maskRevision < 0 || p.maskRevision > MAX_MASK_REVISION) return false;
  return true;
}

function isValidDragPayload(payload: unknown): payload is PetDragIpcPayload {
  if (!payload || typeof payload !== "object") return false;
  const p = payload as Record<string, unknown>;
  if (typeof p.pointerId !== "number" || !Number.isFinite(p.pointerId)) return false;
  if (typeof p.screenX !== "number" || !Number.isFinite(p.screenX) || Math.abs(p.screenX) > MAX_COORDINATE) return false;
  if (typeof p.screenY !== "number" || !Number.isFinite(p.screenY) || Math.abs(p.screenY) > MAX_COORDINATE) return false;
  if (typeof p.canvasX !== "number" || !Number.isFinite(p.canvasX)) return false;
  if (typeof p.canvasY !== "number" || !Number.isFinite(p.canvasY)) return false;
  if (typeof p.occurredAt !== "number" || !Number.isFinite(p.occurredAt)) return false;
  return true;
}

function isValidRuntimeReadyPayload(payload: unknown): payload is RuntimeReadyPayload {
  if (!payload || typeof payload !== "object") return false;
  const p = payload as Record<string, unknown>;
  if (p.snapshotApplied !== true) return false;
  if (typeof p.packageId !== "string" || p.packageId.length > MAX_STRING_LENGTH) return false;
  if (typeof p.packageRevision !== "number" || p.packageRevision < 0) return false;
  if (typeof p.defaultActionKey !== "string" || p.defaultActionKey.length > MAX_STRING_LENGTH) return false;
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
        category: runtimeAction?.category,
        version: parseInt(action.version || "1", 10) || 1,
        loopType,
        defaultPriority: runtimeAction?.priority,
        interruptible: action.interruptible,
        interruptAfterMs: runtimeAction?.interruptAfterMs,
        minimumPlayMs: runtimeAction?.minimumPlayMs,
        maximumPlayMs: runtimeAction?.maximumPlayMs ?? null,
        cooldownMs: runtimeAction?.cooldownMs,
        mutexGroup: runtimeAction?.mutexGroup ?? null,
        returnTarget,
        supportsDefaultIdle: runtimeAction?.supportsDefaultIdle,
        isStableStateCandidate: runtimeAction?.isStableStateCandidate,
        isTransitionOnly: runtimeAction?.isTransitionOnly,
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
  onHitMask?: (payload: PetHitMaskPayload) => void;
  onRendererBootstrapped?: () => void;
  onRuntimeReady?: (payload: RuntimeReadyPayload) => void;
  onDeliveryFailed?: (reason: RendererDeliveryFailureReason, command?: PlayActionCommand) => void;
  onDragStart?: (payload: PetDragIpcPayload) => void;
  onDragMove?: (payload: PetDragIpcPayload) => void;
  onDragEnd?: (payload: PetDragIpcPayload) => void;
  onDragCancel?: (payload: PetDragIpcPayload) => void;
}

export class AnimationIpcAdapter {
  private deps: AnimationIpcAdapterDeps;
  private registered = false;
  private powerMonitorListenersAttached = false;
  private bootstrapped = false;
  private runtimeReady = false;
  private pendingCommands: PendingPlayCommand[] = [];
  private durable: DurableState = {
    packageSnapshot: null,
    defaultActionKey: null,
    windowVisible: true,
  };
  private runtimeReadyPromise: Promise<RuntimeReadyPayload | null> | null = null;
  private runtimeReadyResolve: ((payload: RuntimeReadyPayload | null) => void) | null = null;
  private runtimeReadyTimeout: ReturnType<typeof setTimeout> | null = null;

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

  private readonly onRendererBootstrapped = (
    event: Electron.IpcMainEvent,
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    this.bootstrapped = true;
    this.flushDurableState();
    this.deps.onRendererBootstrapped?.();
  };

  private readonly onRuntimeReady = (
    event: Electron.IpcMainEvent,
    payload: RuntimeReadyPayload,
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    if (!isValidRuntimeReadyPayload(payload)) return;
    this.runtimeReady = true;
    this.flushPendingCommands();
    this.deps.onRuntimeReady?.(payload);
    if (this.runtimeReadyResolve) {
      this.runtimeReadyResolve(payload);
      this.runtimeReadyResolve = null;
    }
    if (this.runtimeReadyTimeout) {
      clearTimeout(this.runtimeReadyTimeout);
      this.runtimeReadyTimeout = null;
    }
  };

  private readonly onHitMask = (
    event: Electron.IpcMainEvent,
    payload: PetHitMaskPayload,
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    if (!isValidHitMaskPayload(payload)) return;
    this.deps.onHitMask?.(payload);
  };

  private readonly onDragStart = (
    event: Electron.IpcMainEvent,
    payload: PetDragIpcPayload,
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    if (!isValidDragPayload(payload)) return;
    this.deps.onDragStart?.(payload);
  };

  private readonly onDragMove = (
    event: Electron.IpcMainEvent,
    payload: PetDragIpcPayload,
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    if (!isValidDragPayload(payload)) return;
    this.deps.onDragMove?.(payload);
  };

  private readonly onDragEnd = (
    event: Electron.IpcMainEvent,
    payload: PetDragIpcPayload,
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    if (!isValidDragPayload(payload)) return;
    this.deps.onDragEnd?.(payload);
  };

  private readonly onDragCancel = (
    event: Electron.IpcMainEvent,
    payload: PetDragIpcPayload,
  ): void => {
    if (!this.isCurrentPetRenderer(event)) return;
    if (!isValidDragPayload(payload)) return;
    this.deps.onDragCancel?.(payload);
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
    const loaded = this.deps.getLoadedInstallation();
    if (!installation || !loaded) {
      return { url: "", mime: "application/octet-stream" };
    }
    if (!isPathSafe(installation.installPath, relativePath)) {
      return { url: "", mime: "application/octet-stream" };
    }
    const cleanRelative = relativePath.replace(/\\/g, "/").replace(/^\/+/, "");
    const resourceIndex = buildResourceIndex(loaded.manifest);
    const indexEntry = resourceIndex.get(cleanRelative);
    if (!indexEntry) {
      return { url: "", mime: "application/octet-stream" };
    }
    const url = buildPetResourceUrl(installation.id, cleanRelative);
    const mime = indexEntry.mediaType || "application/octet-stream";
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
    this.bootstrapped = false;
    this.runtimeReady = false;

    const registry = PetResourceProtocolRegistry.getInstance();
    registry.setActiveInstallationResolver(() => {
      const installation = this.deps.getActiveInstallation();
      const loaded = this.deps.getLoadedInstallation();
      if (!installation || !loaded) return null;
      return {
        installationId: installation.id,
        installPath: installation.installPath,
        manifest: loaded.manifest,
        resourceIndex: buildResourceIndex(loaded.manifest),
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
    ipcMain.on(ANIMATION_IPC_CHANNELS.rendererBootstrapped, this.onRendererBootstrapped);
    ipcMain.on(ANIMATION_IPC_CHANNELS.runtimeReady, this.onRuntimeReady);
    ipcMain.on(ANIMATION_IPC_CHANNELS.hitMask, this.onHitMask);
    ipcMain.on(ANIMATION_IPC_CHANNELS.dragStart, this.onDragStart);
    ipcMain.on(ANIMATION_IPC_CHANNELS.dragMove, this.onDragMove);
    ipcMain.on(ANIMATION_IPC_CHANNELS.dragEnd, this.onDragEnd);
    ipcMain.on(ANIMATION_IPC_CHANNELS.dragCancel, this.onDragCancel);

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

  isBootstrapped(): boolean {
    return this.bootstrapped;
  }

  isRuntimeReady(): boolean {
    return this.runtimeReady;
  }

  waitForRuntimeReady(timeoutMs: number = RUNTIME_READY_TIMEOUT_MS): Promise<RuntimeReadyPayload | null> {
    if (this.runtimeReady) {
      return Promise.resolve(null);
    }
    if (!this.runtimeReadyPromise) {
      this.runtimeReadyPromise = new Promise<RuntimeReadyPayload | null>((resolve) => {
        this.runtimeReadyResolve = resolve;
        this.runtimeReadyTimeout = setTimeout(() => {
          this.runtimeReadyResolve = null;
          this.runtimeReadyTimeout = null;
          resolve(null);
        }, timeoutMs);
      });
    }
    return this.runtimeReadyPromise;
  }

  resetRendererReady(): void {
    this.bootstrapped = false;
    this.runtimeReady = false;
    this.cancelAllPendingCommands("renderer_reset");
    if (this.runtimeReadyTimeout) {
      clearTimeout(this.runtimeReadyTimeout);
      this.runtimeReadyTimeout = null;
    }
    this.runtimeReadyResolve = null;
    this.runtimeReadyPromise = null;
  }

  private cancelAllPendingCommands(reason: RendererDeliveryFailureReason): void {
    const commands = this.pendingCommands;
    this.pendingCommands = [];
    for (const entry of commands) {
      this.deps.onDeliveryFailed?.(reason, entry.command);
    }
  }

  private flushDurableState(): void {
    if (!this.bootstrapped) return;
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
      if (now > entry.expiresAt) {
        this.deps.onDeliveryFailed?.("ttl_expired", entry.command);
        continue;
      }
      const result = this.deliverToRenderer(ANIMATION_IPC_CHANNELS.playAction, entry.command);
      if (result.status !== "delivered") {
        this.deps.onDeliveryFailed?.(result.reason ?? "send_failed", entry.command);
      }
    }
  }

  sendPlayAction(command: PlayActionCommand): RendererDeliveryResult {
    if (!this.runtimeReady) {
      const existing = this.pendingCommands.find(
        (c) => c.command.commandId === command.commandId,
      );
      if (existing) {
        return { status: "queued" };
      }
      if (this.pendingCommands.length >= MAX_PENDING_COMMANDS) {
        const dropped = this.pendingCommands.shift();
        if (dropped) {
          this.deps.onDeliveryFailed?.("queue_overflow", dropped.command);
        }
      }
      const now = Date.now();
      this.pendingCommands.push({
        command,
        queuedAt: now,
        expiresAt: now + COMMAND_TTL_MS,
      });
      return { status: "queued" };
    }

    const result = this.deliverToRenderer(ANIMATION_IPC_CHANNELS.playAction, command);
    if (result.status !== "delivered") {
      this.deps.onDeliveryFailed?.(result.reason ?? "send_failed", command);
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
    if (!this.bootstrapped) return { status: "queued" };
    if (!this.runtimeReady) return { status: "queued" };
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.switchPackage, snapshot);
  }

  sendWindowHidden(): RendererDeliveryResult {
    this.durable.windowVisible = false;
    if (!this.bootstrapped) return { status: "queued" };
    if (!this.runtimeReady) return { status: "queued" };
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.windowHidden);
  }

  sendWindowShown(): RendererDeliveryResult {
    this.durable.windowVisible = true;
    if (!this.bootstrapped) return { status: "queued" };
    if (!this.runtimeReady) return { status: "queued" };
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.windowShown);
  }

  sendRecovery(snapshot: PlaybackRecoverySnapshot): RendererDeliveryResult {
    if (!this.runtimeReady) return { status: "rejected", reason: "renderer_not_ready" };
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.recovery, snapshot);
  }

  sendUpdateDefaultAction(actionKey: string): RendererDeliveryResult {
    this.durable.defaultActionKey = actionKey;
    if (!this.bootstrapped) return { status: "queued" };
    if (!this.runtimeReady) return { status: "queued" };
    return this.deliverToRenderer(ANIMATION_IPC_CHANNELS.updateDefaultAction, actionKey);
  }

  private deliverToRenderer(channel: string, data?: unknown): RendererDeliveryResult {
    const win = this.deps.getPetWindow();
    if (!win) return { status: "rejected", reason: "window_missing" };
    if (win.isDestroyed()) return { status: "rejected", reason: "window_destroyed" };
    if (!this.runtimeReady && channel !== ANIMATION_IPC_CHANNELS.switchPackage) {
      return { status: "rejected", reason: "renderer_not_ready" };
    }
    try {
      if (data !== undefined) {
        win.webContents.send(channel, data);
      } else {
        win.webContents.send(channel);
      }
      return { status: "delivered" };
    } catch (err) {
      return {
        status: "rejected",
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

    const registry = PetResourceProtocolRegistry.getInstance();
    registry.clearActiveInstallationResolver();

    ipcMain.removeHandler(ANIMATION_IPC_CHANNELS.getPackageSnapshot);
    ipcMain.removeHandler(ANIMATION_IPC_CHANNELS.resolveResourceUrl);
    ipcMain.removeHandler(ANIMATION_IPC_CHANNELS.getDiagnostics);

    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.reportEvent, this.onReportEvent);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.reportSnapshot, this.onReportSnapshot);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.sendClick, this.onSendClick);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.sendDoubleClick, this.onSendDoubleClick);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.sendHover, this.onSendHover);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.rendererBootstrapped, this.onRendererBootstrapped);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.runtimeReady, this.onRuntimeReady);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.hitMask, this.onHitMask);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.dragStart, this.onDragStart);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.dragMove, this.onDragMove);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.dragEnd, this.onDragEnd);
    ipcMain.removeListener(ANIMATION_IPC_CHANNELS.dragCancel, this.onDragCancel);

    this.detachPowerMonitorListeners();
    this.powerMonitorListenersAttached = false;
    this.bootstrapped = false;
    this.runtimeReady = false;
    this.cancelAllPendingCommands("renderer_reset");
    if (this.runtimeReadyTimeout) {
      clearTimeout(this.runtimeReadyTimeout);
      this.runtimeReadyTimeout = null;
    }
    this.runtimeReadyResolve = null;
    this.runtimeReadyPromise = null;
  }
}

export { buildPackageSnapshot };
