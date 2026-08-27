import {
  app,
  BrowserWindow,
  ipcMain,
  powerMonitor,
  screen,
  type Display,
} from "electron";
import { createHash, randomUUID } from "node:crypto";
import { readFile, stat } from "node:fs/promises";
import { isAbsolute, join } from "node:path";
import http from "node:http";
import { URL } from "node:url";
import { getAmitiaDataDir } from "../path-manager";
import { DESKTOP_PET_RUNTIME_VERSION } from "../../shared/desktop-pet-runtime-version";
import { createRuntimeBootstrapTicket } from "../backend-session-client";
import { ResourceLoader } from "./resource-loader";
import type {
  LoadedInstallation,
  RuntimeAction,
  LoadInstallationRequest,
} from "./resource-loader";
import { ResourceCache } from "./resource-cache";
import type { PlaybackSnapshot } from "../../desktop-pet/animation/contracts";
import { DesktopPetWindowAdapter } from "./window-adapter";
import { AnimationIpcAdapter } from "./animation-ipc";
import type { PetDragIpcPayload, PetHitMaskPayload, RuntimeInitFailedPayload, RuntimeReadyPayload } from "../../shared/animation-ipc";
import { AnimationPlayerBridge } from "./animation-player-bridge";
import {
  DesktopPetActionScheduler,
  ActionPriorities,
  EventSources,
} from "./action-scheduler";
import type {
  DesktopPetActionRequest,
  SchedulerEvent,
} from "./action-scheduler";
import { IdleController } from "./idle-controller";
import { DesktopPetVitalityController } from "./vitality-controller";
import { DesktopPetWorldController } from "./world-controller";
import { DesktopPetEventBridge } from "./event-bridge";
import {
  ChatStateBridge,
  CHAT_STATE_CHANGE_CHANNEL,
} from "./chat-state-bridge";
import type { ChatStateIpcPayload } from "./chat-state-bridge";
import { DragController } from "./drag-controller";
import type { DragEvent, DragState } from "./drag-controller";
import { ClickThroughController } from "./click-through-controller";
import { PetLogger } from "./logger";
import { getRuntimeId, getDeviceId } from "./runtime-identity";
import type { PetInstanceSummary } from "./runtime-bridge-client";
import {
  DesktopRuntimeHandlerV2,
  type RuntimeHandlerConfig,
  type RuntimeHandlerHooks,
  type RuntimeResumeCursor,
} from "../../desktop-pet/runtime/runtime-handler-v2";
import type {
  RuntimeEnvelope,
  HelloAckPayload,
  StateSnapshotPayload,
} from "../../desktop-pet/runtime/protocol-v2";
import type { RuntimeCommandExecutionResult } from "./runtime-v2-command-adapter";
import { getBackendSessionClient } from "../backend-session-client";
import type {
  ClickThroughMode,
  DesktopPetWindowOptions,
  Position,
} from "./types";
import {
  PET_WINDOW_SCALE_DEFAULT,
  PET_WINDOW_SCALE_MAX,
  PET_WINDOW_SCALE_MIN,
} from "./types";

const CORE_BASE_HOST = "127.0.0.1";
const CORE_BASE_PORT = 18899;
const API_BASE_PATH = "/api/desktop-pets";
const HEALTH_CHECK_PATH = "/livez";
const DEFAULT_USER_ID = "default";
const DEFAULT_ALPHA_THRESHOLD = 10;

const PET_ACTION_SWITCH_CHANNEL = "pet:action-switch";
const PET_LOAD_ERROR_CHANNEL = "pet:load-error";
const PET_STATE_CHANNEL = "pet:state";

const CLICK_THROUGH_MODE_OFF = "off";
const CLICK_THROUGH_MODE_ALPHA = "alpha";
const CLICK_THROUGH_MODE_BOUNDING_BOX = "boundingBox";

const INSTALLATION_STATUS_ENABLED = "enabled";
const INSTALLATION_STATUS_DISABLED = "disabled";
const INSTALLATION_STATUS_INVALID = "invalid";

const CORRUPTION_MANIFEST_MISSING = "CORRUPTION_MANIFEST_MISSING";
const CORRUPTION_DEFAULT_ACTION_MISSING = "CORRUPTION_DEFAULT_ACTION_MISSING";
const CORRUPTION_DEFAULT_ACTION_UNAVAILABLE =
  "CORRUPTION_DEFAULT_ACTION_UNAVAILABLE";
const CORRUPTION_FRAME_MISSING = "CORRUPTION_FRAME_MISSING";
const CORRUPTION_PACKAGE_HASH_MISMATCH = "CORRUPTION_PACKAGE_HASH_MISMATCH";
const CORRUPTION_ACTION_CONFIG_INVALID = "CORRUPTION_ACTION_CONFIG_INVALID";

const RECOVERY_DEBOUNCE_MS = 800;
const RECOVERY_RETRY_MAX_ATTEMPTS = 3;
const RECOVERY_RETRY_BASE_DELAY_MS = 1500;
const RECOVERY_REASON_RENDER_RELOAD = "render-process-gone";
const RECOVERY_REASON_WINDOW_CLOSED = "window-closed";
const RECOVERY_REASON_GPU_CRASHED = "gpu-process-crashed";
const RECOVERY_REASON_POWER_RESUME = "power-resume";
const RECOVERY_REASON_DISPLAY_CHANGED = "display-changed";

const RUNTIME_BRIDGE_WS_PATH = "/internal/desktop-pet/runtime/ws";
const BRIDGE_RECONNECT_DELAY_MS = 2000;


export type PetManagerState =
  | "uninitialized"
  | "ready"
  | "enabled"
  | "disabled"
  | "invalid"
  | "degraded";

export interface InstallationInfo {
  id: string;
  userId: string;
  characterId: string;
  packageId: string;
  packageVersion: string;
  name: string;
  status: string;
  isActive: boolean;
  installPath: string;
  manifestPath: string;
  previewPath: string;
  defaultActionKey: string;
  canvasWidth: number;
  canvasHeight: number;
  packageHash: string;
  installedAt: string;
  lastEnabledAt: string;
  lastDisabledAt: string;
  createdAt: string;
  updatedAt: string;
  petId: string;
  currentReleaseId: string;
  installedContentHash: string;
}

export interface RuntimeSettingsInfo {
  installationId: string;
  settingsRevision: number;
  alwaysOnTop: boolean;
  launchOnStartup: boolean;
  scale: number;
  positionX: number;
  positionY: number;
  screenId: string;
  idleEnabled: boolean;
  idleIntervalMinSeconds: number;
  idleIntervalMaxSeconds: number;
  clickThroughMode: ClickThroughMode;
  soundEnabled: boolean;
}

export interface PetManagerDeps {
  userId?: string;
  resourceLoader?: ResourceLoader;
  resourceCache?: ResourceCache;
  alphaThreshold?: number;
  coreHost?: string;
  corePort?: number;
  petLogger?: PetLogger;
  runtimeVersion?: string;
}

export interface CorruptionDetectionResult {
  corrupted: boolean;
  errorCode: string;
  detail: string;
}

export type RecoveryReason =
  | "render-process-gone"
  | "window-closed"
  | "gpu-process-crashed"
  | "power-resume"
  | "display-changed"
  | "manual";

export interface PetActionSwitchPayload {
  actionKey: string;
  previousActionKey: string | null;
  source: string;
}

export interface PetLoadErrorPayload {
  installationId: string;
  error: string;
  errorCode: string;
}

export interface PetStatePayload {
  state: PetManagerState;
  installationId: string | null;
  reason?: string;
}

export interface PetManagerOptions {
  userId?: string;
}

interface ApiEnvelope<T> {
  code: number;
  msg?: string;
  data?: T;
}

interface InstallationApiPayload {
  id: string;
  userId: string;
  characterId: string;
  packageId: string;
  packageVersion: string;
  name: string;
  status: string;
  isActive: number;
  installPath: string;
  manifestPath: string;
  previewPath: string;
  defaultActionKey: string;
  canvasWidth: number;
  canvasHeight: number;
  packageHash: string;
  installedAt: string;
  lastEnabledAt: string;
  lastDisabledAt: string;
  createdAt: string;
  updatedAt: string;
  petId: string;
  currentReleaseId: string;
  installedContentHash: string;
}

interface ListInstallationsApiPayload {
  items: InstallationApiPayload[];
  total: number;
}

interface RuntimeSettingsApiPayload {
  installationId: string;
  settingsRevision: number;
  alwaysOnTop: number;
  launchOnStartup: number;
  scale: number;
  positionX: number;
  positionY: number;
  screenId: string;
  idleEnabled: number;
  idleIntervalMinSeconds: number;
  idleIntervalMaxSeconds: number;
  clickThroughMode: string;
  soundEnabled: number;
}

interface RuntimeSettingsMutationApiPayload {
  operationId?: string;
  status?: string;
  stage?: string;
  settings?: RuntimeSettingsApiPayload;
}

function clampScale(scale: number): number {
  if (!Number.isFinite(scale)) return PET_WINDOW_SCALE_DEFAULT;
  return Math.min(PET_WINDOW_SCALE_MAX, Math.max(PET_WINDOW_SCALE_MIN, scale));
}

function normalizeClickThroughMode(value: string): ClickThroughMode {
  if (value === CLICK_THROUGH_MODE_ALPHA) return "alpha";
  if (value === CLICK_THROUGH_MODE_BOUNDING_BOX) return "boundingBox";
  return "none";
}

function clickThroughModeToApiValue(mode: ClickThroughMode): string {
  if (mode === "alpha") return CLICK_THROUGH_MODE_ALPHA;
  if (mode === "boundingBox") return CLICK_THROUGH_MODE_BOUNDING_BOX;
  return CLICK_THROUGH_MODE_OFF;
}

function mapInstallationPayload(payload: InstallationApiPayload): InstallationInfo {
  return {
    id: payload.id,
    userId: payload.userId,
    characterId: payload.characterId,
    packageId: payload.packageId,
    packageVersion: payload.packageVersion,
    name: payload.name,
    status: payload.status,
    isActive: payload.isActive === 1,
    installPath: payload.installPath,
    manifestPath: payload.manifestPath,
    previewPath: payload.previewPath,
    defaultActionKey: payload.defaultActionKey,
    canvasWidth: payload.canvasWidth,
    canvasHeight: payload.canvasHeight,
    packageHash: payload.packageHash,
    installedAt: payload.installedAt,
    lastEnabledAt: payload.lastEnabledAt,
    lastDisabledAt: payload.lastDisabledAt,
    createdAt: payload.createdAt,
    updatedAt: payload.updatedAt,
    petId: payload.petId ?? "",
    currentReleaseId: payload.currentReleaseId ?? "",
    installedContentHash: payload.installedContentHash ?? "",
  };
}

function mapRuntimeSettingsPayload(payload: RuntimeSettingsApiPayload): RuntimeSettingsInfo {
  return {
    installationId: payload.installationId,
    settingsRevision: payload.settingsRevision ?? 0,
    alwaysOnTop: payload.alwaysOnTop !== 0,
    launchOnStartup: payload.launchOnStartup !== 0,
    scale: payload.scale,
    positionX: payload.positionX,
    positionY: payload.positionY,
    screenId: payload.screenId,
    idleEnabled: payload.idleEnabled !== 0,
    idleIntervalMinSeconds: payload.idleIntervalMinSeconds,
    idleIntervalMaxSeconds: payload.idleIntervalMaxSeconds,
    clickThroughMode: normalizeClickThroughMode(payload.clickThroughMode),
    soundEnabled: payload.soundEnabled !== 0,
  };
}

export class DesktopPetManager {
  private readonly userId: string;
  private readonly coreHost: string;
  private readonly corePort: number;
  private readonly resourceLoader: ResourceLoader;
  private readonly resourceCache: ResourceCache;
  private readonly alphaThreshold: number;
  private readonly petLogger: PetLogger;
  private readonly runtimeVersion: string;

  private authToken: string | null = null;
  private state: PetManagerState = "uninitialized";
  private initialized = false;
  private initializing = false;
  private initializationError: Error | null = null;
  private activeInstallationId: string | null = null;
  private activeInstallation: InstallationInfo | null = null;
  private activeSettings: RuntimeSettingsInfo | null = null;
  private loadedInstallation: LoadedInstallation | null = null;
  private packageRevision = 0;

  private windowAdapter: DesktopPetWindowAdapter | null = null;
  private actionPlayer: AnimationPlayerBridge | null = null;
  private animationIpc: AnimationIpcAdapter | null = null;
  private scheduler: DesktopPetActionScheduler | null = null;
  private idleController: IdleController | null = null;
  private vitalityController: DesktopPetVitalityController | null = null;
  private worldController: DesktopPetWorldController | null = null;
  private eventBridge: DesktopPetEventBridge | null = null;
  private chatStateBridge: ChatStateBridge | null = null;
  private dragController: DragController | null = null;
  private clickThroughController: ClickThroughController | null = null;

  private runtimeHandler: DesktopRuntimeHandlerV2 | null = null;
  private bridgeReconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private bridgeStarted = false;
  private bridgeConnecting = false;
  private currentActionKey: string | null = null;
  private currentDragId: string | null = null;
  private currentPlaybackId: string | null = null;
  private currentCommandId: string | null = null;
  private readonly playbackCommandIds = new Map<string, string>();
  private readonly playbackDecisionIds = new Map<string, string>();
  private lastAppliedDesiredRevision = 0;
  private lastAppliedSettingsRevision = 0;
  private runtimeResumeCursor: RuntimeResumeCursor = {
    lastAppliedDesiredRevision: 0,
    lastProcessedCommandSequence: 0,
    lastEventSequence: 0,
  };
  private runtimeStateSync: Promise<void> = Promise.resolve();
  private lifecycleMutationQueue: Promise<void> = Promise.resolve();
  private petInstances: PetInstanceSummary[] = [];
  private rendererHealthy = false;

  private recoveryHandlersAttached = false;
  private recoveryInProgress = false;
  private recoveryDebounceTimer: ReturnType<typeof setTimeout> | null = null;
  private recoveryRetryTimer: ReturnType<typeof setTimeout> | null = null;
  private recoveryRetryCount = 0;
  private intentionalClose = false;
  private recoveryWindowCloseListener:
    | ((...args: unknown[]) => void)
    | null = null;
  private recoveryWindowCrashedListener:
    | ((...args: unknown[]) => void)
    | null = null;

  private readonly stateListeners: Set<(payload: PetStatePayload) => void> =
    new Set();

  constructor(deps?: PetManagerDeps) {
    const opts = deps ?? {};
    this.userId = opts.userId ?? DEFAULT_USER_ID;
    this.coreHost = opts.coreHost ?? CORE_BASE_HOST;
    this.corePort = opts.corePort ?? CORE_BASE_PORT;
    this.runtimeVersion = opts.runtimeVersion ?? DESKTOP_PET_RUNTIME_VERSION;
    this.resourceLoader = opts.resourceLoader ?? new ResourceLoader(this.runtimeVersion);
    this.resourceCache = opts.resourceCache ?? new ResourceCache();
    this.alphaThreshold =
      typeof opts.alphaThreshold === "number"
        ? opts.alphaThreshold
        : DEFAULT_ALPHA_THRESHOLD;
    this.petLogger = opts.petLogger ?? PetLogger.getInstance();
  }

  setAuthToken(token: string | null): void {
    this.authToken = token && token.length > 0 ? token : null;
  }

  getUserId(): string {
    return this.userId;
  }

  getState(): PetManagerState {
    return this.state;
  }

  getActiveInstallationId(): string | null {
    return this.activeInstallationId;
  }

  getActiveInstallation(): InstallationInfo | null {
    return this.activeInstallation;
  }

  getActiveSettings(): RuntimeSettingsInfo | null {
    return this.activeSettings;
  }

  handleChatStatePayload(payload: ChatStateIpcPayload): void {
    if (!this.chatStateBridge) return;
    try {
      this.chatStateBridge.handleIpcPayload(payload);
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 处理聊天状态失败:",
        this.errorMessage(err),
      );
    }
  }

  async handleCharacterSwitched(characterId: string): Promise<void> {
    if (!characterId) return;
    await this.ensureInitialized();
    let installations: InstallationInfo[];
    try {
      installations = await this.listInstallations();
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 角色切换时查询安装列表失败:",
        this.errorMessage(err),
      );
      return;
    }
    const candidate = installations.find(
      (inst) =>
        inst.characterId === characterId &&
        inst.status === INSTALLATION_STATUS_ENABLED,
    );
    if (!candidate) return;
    if (
      this.activeInstallationId === candidate.id &&
      this.state === "enabled"
    ) {
      return;
    }
    try {
      await this.switchInstallation(candidate.id);
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 角色切换后切换桌宠失败:",
        this.errorMessage(err),
      );
    }
  }

  onStateChange(listener: (payload: PetStatePayload) => void): () => void {
    this.stateListeners.add(listener);
    return () => {
      this.stateListeners.delete(listener);
    };
  }

  async initialize(): Promise<void> {
    if (this.initialized) {
      return;
    }
    if (this.initializing) {
      await this.ensureInitialized();
      return;
    }
    this.initializing = true;
    const retryingAfterInitializationFailure =
      this.state === "degraded" &&
      this.initializationError !== null &&
      !this.activeInstallationId;
    this.initializationError = null;
    if (retryingAfterInitializationFailure) {
      this.setState("uninitialized", null, "重新初始化");
    }
    try {
      await this.waitCoreReady();
      await this.registerDeviceIdentity();
      await this.restoreActiveInstallation();
      this.startBridge();
      this.initialized = true;
      this.initializationError = null;
      if (this.state === "uninitialized") {
        this.setState("ready", null, "初始化完成但无活跃桌宠");
      }
    } catch (err) {
      const failure = err instanceof Error ? err : new Error(this.errorMessage(err));
      console.error("[DesktopPetManager] 初始化失败:", failure);
      this.initialized = false;
      this.initializationError = failure;
      this.setState("degraded", null, `初始化失败: ${failure.message}`);
      throw failure;
    } finally {
      this.initializing = false;
    }
  }

  async enableInstallation(installationId: string): Promise<void> {
    await this.runLifecycleMutation(() =>
      this.enableInstallationInternal(installationId, true, false),
    );
  }

  private async enableInstallationInternal(
    installationId: string,
    notifyBackend: boolean,
    forceReload: boolean,
  ): Promise<void> {
    if (!installationId) {
      throw new Error("INSTALLATION_ID_REQUIRED");
    }
    await this.ensureInitialized();
    if (
      this.activeInstallationId === installationId &&
      this.state === "enabled"
    ) {
      if (!forceReload) {
        return;
      }
      // A release reload must tear down the current player/window before
      // loading the replacement assets. Re-entering startRuntime on top of an
      // enabled instance leaks the old runtime and defeats the reload command.
      await this.disableInternal(false, false);
    } else if (
      this.activeInstallationId &&
      this.activeInstallationId !== installationId
    ) {
      await this.disableInternal(false, notifyBackend);
    }

    const installation = await this.fetchInstallation(installationId);
    if (!installation) {
      this.petLogger.logInstallFailed(installationId, "INSTALLATION_NOT_FOUND");
      throw new Error("INSTALLATION_NOT_FOUND");
    }
    if (
      installation.status === INSTALLATION_STATUS_INVALID ||
      installation.status === "uninstalling" ||
      installation.status === "uninstalled"
    ) {
      this.petLogger.logInstallFailed(
        installationId,
        `INSTALLATION_STATUS_INVALID: ${installation.status}`,
      );
      throw new Error(`INSTALLATION_STATUS_INVALID: ${installation.status}`);
    }

    const settings = await this.fetchRuntimeSettings(installationId);
    const detection = await this.detectCorruption(installation);
    if (detection.corrupted) {
      await this.handleCorruption(installation, detection);
      throw new Error(`CORRUPTION_DETECTED: ${detection.errorCode}`);
    }
    const loaded = await this.loadAndValidateInstallation(installation);
    if (!loaded) {
      await this.handleCorruption(installation, {
        corrupted: true,
        errorCode: "RESOURCE_VALIDATION_FAILED",
        detail: "loadAndValidateInstallation 返回空结果",
      });
      throw new Error("RESOURCE_VALIDATION_FAILED");
    }

    this.activeInstallationId = installationId;
    this.activeInstallation = installation;
    this.activeSettings = settings;
    this.loadedInstallation = loaded;
    this.packageRevision += 1;

    let backendEnableCommitted = false;
    try {
      if (notifyBackend) {
        await this.callEnableApi(installationId);
        backendEnableCommitted = true;
      }
      this.markSettingsRevisionApplied(settings?.settingsRevision ?? 0);
      await this.startRuntime(installation, settings, loaded);
      this.setupRecoveryHandlers();
      this.petLogger.logEnable(installationId, installation.name);
      this.setState("enabled", installationId);
    } catch (err) {
      this.petLogger.logRuntimeCrash("enableInstallation", err);
      this.teardownRecoveryHandlers();
      try {
        await this.stopRuntime();
      } catch (stopErr) {
        this.petLogger.logRuntimeCrash("enableInstallation.stopRuntime", stopErr);
      }
      if (backendEnableCommitted) {
        try {
          await this.callDisableApi(installationId);
        } catch (rollbackErr) {
          this.petLogger.logRuntimeCrash(
            "enableInstallation.rollbackBackendEnable",
            rollbackErr,
          );
        }
      }
      this.activeInstallationId = null;
      this.activeInstallation = null;
      this.activeSettings = null;
      this.loadedInstallation = null;
      throw err;
    }
  }

  async disableInstallation(): Promise<void> {
    await this.runLifecycleMutation(() => this.disableInternal(true));
  }

  async switchInstallation(installationId: string): Promise<void> {
    if (!installationId) {
      throw new Error("INSTALLATION_ID_REQUIRED");
    }
    await this.runLifecycleMutation(async () => {
      await this.ensureInitialized();
      if (
        this.activeInstallationId === installationId &&
        this.state === "enabled"
      ) {
        return;
      }
      await this.disableInternal(false);
      await this.enableInstallationInternal(installationId, true, false);
    });
  }

  async playAction(actionKey: string): Promise<void> {
    if (!actionKey) {
      throw new Error("ACTION_KEY_REQUIRED");
    }
    if (this.state !== "enabled" || !this.scheduler || !this.loadedInstallation) {
      throw new Error("PET_NOT_ENABLED");
    }
    if (this.dragController?.isDragging()) {
      throw new Error("PET_IS_DRAGGING");
    }
    const request: DesktopPetActionRequest = {
      actionKey,
      source: EventSources.MANUAL,
      priority: ActionPriorities.CLICK,
      interrupt: true,
    };
    const result = this.scheduler.submit(request);
    if (result === "rejected") {
      throw new Error(`ACTION_REJECTED: ${actionKey}`);
    }
    await this.callPlayActionApi(actionKey);
  }

  stopAction(): void {
    if (this.scheduler) {
      this.scheduler.forceInterrupt("resource_invalid");
    }
    this.actionPlayer?.stop();
  }

  pauseAction(): void {
    this.actionPlayer?.pause?.();
  }

  resumeAction(): void {
    this.actionPlayer?.resume?.();
  }

  async recenter(): Promise<void> {
    if (this.state !== "enabled" || !this.windowAdapter) {
      throw new Error("PET_NOT_ENABLED");
    }
    if (this.activeInstallationId) {
      await this.callRecenterApi(this.activeInstallationId);
    }
    await this.applyRecenterLocal();
    await this.persistRuntimePosition();
  }

  private async applyRecenterLocal(): Promise<void> {
    if (this.state !== "enabled" || !this.windowAdapter) {
      throw new Error("PET_NOT_ENABLED");
    }
    const primary = this.windowAdapter.listScreens().find((screen) => screen.isPrimary);
    const target = primary ?? this.windowAdapter.listScreens()[0];
    if (!target) return;
    const width = this.windowAdapter.getOptions().canvasWidth;
    const height = this.windowAdapter.getOptions().canvasHeight;
    const x = (target.workArea.x - target.bounds.x) + target.workArea.width - width - 40;
    const y = (target.workArea.y - target.bounds.y) + target.workArea.height - height - 40;
    await this.windowAdapter.setPosition(x, y, target.id);
  }

  async updateSettings(settings: Partial<RuntimeSettingsInfo>): Promise<void> {
    if (!this.activeInstallationId) {
      throw new Error("PET_NOT_ENABLED");
    }
    const patch = this.buildSettingsPatch(settings);
    if (Object.keys(patch).length === 0) {
      return;
    }
    const updated = await this.callUpdateSettingsApi(this.activeInstallationId, patch);
    await this.applyRuntimeSettingsLocal(updated ?? settings, updated?.settingsRevision ?? 0);
  }

  private async applyRuntimeSettingsLocal(
    settings: Partial<RuntimeSettingsInfo>,
    settingsRevision = 0,
  ): Promise<void> {
    const revision = Math.max(settingsRevision, settings.settingsRevision ?? 0);
    const merged = this.mergeSettings(this.activeSettings, {
      ...settings,
      settingsRevision: revision > 0
        ? revision
        : this.activeSettings?.settingsRevision ?? 0,
    });
    this.activeSettings = merged;
    this.markSettingsRevisionApplied(merged.settingsRevision);
    if (this.windowAdapter) {
      if (typeof settings.scale === "number") {
        await this.windowAdapter.setScale(clampScale(merged.scale));
      }
      if (typeof settings.alwaysOnTop === "boolean") {
        await this.windowAdapter.setAlwaysOnTop(merged.alwaysOnTop);
      }
      if (typeof settings.clickThroughMode !== "undefined") {
        await this.windowAdapter.setClickThroughMode(merged.clickThroughMode);
        if (this.clickThroughController) {
          const win = this.windowAdapter.getNativeWindow();
          if (win) {
            this.clickThroughController.attach(win);
          }
        }
      }
    }
    if (this.idleController) {
      this.idleController.reset();
    }
  }

  async updateDefaultAction(actionKey: string): Promise<void> {
    if (!actionKey) {
      throw new Error("ACTION_KEY_REQUIRED");
    }
    if (!this.activeInstallationId) {
      throw new Error("PET_NOT_ENABLED");
    }
    await this.callUpdateDefaultActionApi(this.activeInstallationId, actionKey);
    if (this.loadedInstallation) {
      const runtime = this.loadedInstallation.actions.get(actionKey);
      if (runtime && runtime.available) {
        this.loadedInstallation.defaultAction = runtime;
        if (this.idleController) {
          this.idleController.playDefaultIdle();
        }
      }
    }
    if (this.activeInstallation) {
      this.activeInstallation.defaultActionKey = actionKey;
    }
  }

  async shutdown(): Promise<void> {
    this.teardownRecoveryHandlers();
    this.stopBridge();
    await this.stopRuntime();
    this.activeInstallationId = null;
    this.activeInstallation = null;
    this.activeSettings = null;
    this.loadedInstallation = null;
    this.initialized = false;
    this.setState("uninitialized", null, "shutdown");
  }

  async listInstallations(): Promise<InstallationInfo[]> {
    const path = `${API_BASE_PATH}/installations`;
    const payload = await this.request<ListInstallationsApiPayload>("GET", path);
    const items = Array.isArray(payload?.items) ? payload.items : [];
    return items.map(mapInstallationPayload);
  }

  async getInstallation(installationId: string): Promise<InstallationInfo | null> {
    if (!installationId) return null;
    const path = `${API_BASE_PATH}/installations/${encodeURIComponent(installationId)}`;
    const payload = await this.request<InstallationApiPayload>("GET", path);
    return payload ? mapInstallationPayload(payload) : null;
  }

  async getRuntimeSettings(installationId: string): Promise<RuntimeSettingsInfo | null> {
    if (!installationId) return null;
    const path = `${API_BASE_PATH}/installations/${encodeURIComponent(installationId)}/settings`;
    const payload = await this.request<RuntimeSettingsApiPayload>("GET", path);
    return payload ? mapRuntimeSettingsPayload(payload) : null;
  }

  private async ensureInitialized(): Promise<void> {
    if (this.initialized) return;
    if (this.initializing) {
      while (this.initializing) {
        await new Promise((r) => setTimeout(r, 50));
      }
      if (this.initialized) return;
      throw this.initializationError ?? new Error("DESKTOP_PET_INITIALIZATION_FAILED");
    }
    await this.initialize();
    if (!this.initialized) {
      throw this.initializationError ?? new Error("DESKTOP_PET_INITIALIZATION_FAILED");
    }
  }

  private async waitCoreReady(timeoutMs = 60000): Promise<void> {
    const startedAt = Date.now();
    while (Date.now() - startedAt < timeoutMs) {
      const ok = await this.healthCheck();
      if (ok) return;
      await new Promise((r) => setTimeout(r, 500));
    }
    console.warn(
      "[DesktopPetManager] 等待 AmitiaCore 就绪超时, 继续后续流程",
    );
  }

  private async healthCheck(): Promise<boolean> {
    return new Promise((resolve) => {
      const req = http.request(
        {
          hostname: this.coreHost,
          port: this.corePort,
          path: HEALTH_CHECK_PATH,
          method: "GET",
          timeout: 2000,
        },
        (res) => {
          res.resume();
          resolve(res.statusCode === 200);
        },
      );
      req.on("error", () => resolve(false));
      req.on("timeout", () => {
        req.destroy();
        resolve(false);
      });
      req.end();
    });
  }

  private async restoreActiveInstallation(): Promise<void> {
    let installations: InstallationInfo[];
    try {
      installations = await this.listInstallations();
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 查询活跃安装失败, 跳过桌宠恢复:",
        this.errorMessage(err),
      );
      return;
    }
    const installation =
      installations.find(
        (item) => item.isActive && item.status === INSTALLATION_STATUS_ENABLED,
      ) ?? installations.find((item) => item.status === INSTALLATION_STATUS_ENABLED);
    if (!installation) {
      return;
    }
    try {
      const settings = await this.fetchRuntimeSettings(installation.id);
      const detection = await this.detectCorruption(installation);
      if (detection.corrupted) {
        await this.handleCorruption(installation, detection);
        this.setState("invalid", installation.id, detection.detail);
        return;
      }
      const loaded = await this.loadAndValidateInstallation(installation);
      if (!loaded) {
        await this.handleCorruption(installation, {
          corrupted: true,
          errorCode: "RESOURCE_VALIDATION_FAILED",
          detail: "loadAndValidateInstallation 返回空结果",
        });
        this.setState("invalid", installation.id, "资源校验失败");
        return;
      }
      this.activeInstallationId = installation.id;
      this.activeInstallation = installation;
      this.activeSettings = settings;
      this.loadedInstallation = loaded;
      this.packageRevision += 1;
      this.markSettingsRevisionApplied(settings?.settingsRevision ?? 0);
      await this.startRuntime(installation, settings, loaded);
      this.setupRecoveryHandlers();
      this.petLogger.logEnable(installation.id, installation.name);
      this.petLogger.logWindowRecovered("app-startup", installation.id);
      this.setState("enabled", installation.id, "恢复完成");
    } catch (err) {
      this.petLogger.logRuntimeCrash("restoreActiveInstallation", err);
      this.petLogger.logInstallFailed(installation.id, this.errorMessage(err));
      console.error(
        "[DesktopPetManager] 恢复活跃桌宠失败:",
        this.errorMessage(err),
      );
      try {
        await this.stopRuntime();
      } catch (stopErr) {
        this.petLogger.logRuntimeCrash("restoreActiveInstallation.stopRuntime", stopErr);
      }
      this.activeInstallationId = null;
      this.activeInstallation = null;
      this.activeSettings = null;
      this.loadedInstallation = null;
      this.setState(
        "degraded",
        installation.id,
        `恢复失败: ${this.errorMessage(err)}`,
      );
    }
  }

  private async loadAndValidateInstallation(
    installation: InstallationInfo,
  ): Promise<LoadedInstallation | null> {
    const installPath = this.resolveAbsolutePath(installation.installPath);
    const manifestPath = this.resolveAbsolutePath(installation.manifestPath);
    if (!installPath || !manifestPath) {
      console.error(
        "[DesktopPetManager] 安装路径无效:",
        installation.installPath,
        installation.manifestPath,
      );
      return null;
    }
    try {
      const request: LoadInstallationRequest = {
        installationId: installation.id,
        petId: installation.petId,
        releaseId: installation.currentReleaseId,
        installPath,
        manifestPath,
        expectedContentRootHash: installation.installedContentHash || installation.packageHash,
      };
      const loaded = await this.resourceLoader.loadInstallation(request);
      if (!loaded.defaultAction || !loaded.defaultAction.available) {
        console.error(
          "[DesktopPetManager] 默认动作不可用:",
          installation.id,
          loaded.defaultAction?.loadError ?? "unknown",
        );
        return null;
      }
      return loaded;
    } catch (err) {
      console.error(
        "[DesktopPetManager] 加载安装资源失败:",
        installation.id,
        this.errorMessage(err),
      );
      return null;
    }
  }

  private async startRuntime(
    installation: InstallationInfo,
    settings: RuntimeSettingsInfo | null,
    loaded: LoadedInstallation,
  ): Promise<void> {
    if (this.windowAdapter) {
      await this.stopRuntime();
    }

    const canvasWidth = installation.canvasWidth || loaded.manifest.canvas.width;
    const canvasHeight =
      installation.canvasHeight || loaded.manifest.canvas.height;
    const scale = clampScale(settings?.scale ?? PET_WINDOW_SCALE_DEFAULT);
    const clickThroughMode = settings?.clickThroughMode ?? "none";
    const alwaysOnTop = settings?.alwaysOnTop ?? true;

    const position: Position | undefined = settings
      ? {
          x: settings.positionX,
          y: settings.positionY,
          screenId: settings.screenId || undefined,
        }
      : undefined;

    const windowOptions: DesktopPetWindowOptions = {
      canvasWidth,
      canvasHeight,
      scale,
      alwaysOnTop,
      clickThroughMode,
      position,
    };

    this.rendererHealthy = false;
    const windowAdapter = new DesktopPetWindowAdapter(windowOptions);
    // Build and wire the entire main-process runtime before navigation. The
    // renderer invokes animation IPC during its bootstrap, so loading it first
    // creates a race where the initial snapshot request can arrive before the
    // handlers exist.
    const win = windowAdapter.createWindow();

    const windowCloseListener = (): void => {
      if (this.intentionalClose) return;
      this.petLogger.logWindowRecovered(
        RECOVERY_REASON_WINDOW_CLOSED,
        this.activeInstallationId ?? undefined,
      );
      this.scheduleRecovery(RECOVERY_REASON_WINDOW_CLOSED);
    };
    const windowCrashedListener = (): void => {
      this.rendererHealthy = false;
      this.syncRuntimeState();
      this.petLogger.logWindowRecovered(
        RECOVERY_REASON_RENDER_RELOAD,
        this.activeInstallationId ?? undefined,
      );
      this.scheduleRecovery(RECOVERY_REASON_RENDER_RELOAD);
    };
    windowAdapter.on("close", windowCloseListener);
    windowAdapter.on("crashed", windowCrashedListener);
    this.recoveryWindowCloseListener = windowCloseListener;
    this.recoveryWindowCrashedListener = windowCrashedListener;

    const animationIpc = new AnimationIpcAdapter({
      getActiveInstallation: () => this.activeInstallation,
      getLoadedInstallation: () => this.loadedInstallation,
      getPackageRevision: () => this.packageRevision,
      getPetWindow: () => windowAdapter.getNativeWindow(),
      onPlaybackEvent: (event) => this.handlePlaybackEvent(event),
      onSnapshotUpdate: (snapshot) => this.handleSnapshotUpdate(snapshot),
      onClick: (x, y) => {
        this.vitalityController?.notifyInteraction("click");
        this.handleClick(x, y);
        this.eventBridge?.handleClick(x, y);
      },
      onDoubleClick: (x, y) => {
        this.handleDoubleClick(x, y);
        this.eventBridge?.handleDoubleClick(x, y);
      },
      onHover: (x, y) => {
        this.vitalityController?.notifyInteraction("hover");
        this.handleHover(x, y);
        this.eventBridge?.handleHover(x, y);
      },
      onHitMask: (payload) => this.handleHitMask(payload),
      onRendererBootstrapped: () => this.handleRendererBootstrapped(),
      onRuntimeReady: (payload) => this.handleRuntimeReady(payload),
      onRuntimeInitFailed: (payload) => this.handleRuntimeInitFailed(payload),
      onDeliveryFailed: (reason, command) =>
        this.handleDeliveryFailed(reason, command),
      onDragStart: (payload) => { this.vitalityController?.notifyInteraction("drag"); this.dragController?.handleDragStart(payload); },
      onDragMove: (payload) => this.dragController?.handleDragMove(payload),
      onDragEnd: (payload) => this.dragController?.handleDragEnd(payload),
      onDragCancel: (payload) => this.dragController?.handleDragCancel(payload),
    });

    const actionPlayer = new AnimationPlayerBridge({
      onActionSwitch: (newKey, oldKey, playbackId) =>
        this.handleActionSwitch(newKey, oldKey, playbackId),
      onActionCompleted: (actionKey, loopCount) => {
        this.handleActionCompleted(actionKey, loopCount);
        this.scheduler?.notifyActionCompleted(actionKey);
      },
      onActionInterrupted: (actionKey, loopCount) => {
        this.handleActionInterrupted(actionKey, loopCount);
        this.scheduler?.notifyActionInterrupted(actionKey);
      },
      onActionFailed: (actionKey, reason) => {
        console.warn(
          "[DesktopPetManager] 动作播放失败:",
          actionKey,
          reason,
        );
      },
      onError: (err) =>
        console.error("[DesktopPetManager] AnimationPlayerBridge 错误:", err),
    });
    actionPlayer.setAnimationIpc(animationIpc);
    actionPlayer.setInstallationContext(
      loaded.installationId,
      this.activeInstallationId ?? loaded.installationId,
      this.packageRevision,
    );
    actionPlayer.attachLoaded(loaded);

    const scheduler = new DesktopPetActionScheduler(actionPlayer, {
      onEvent: (event, request, action) =>
        this.handleSchedulerEvent(event, request, action),
    });
    scheduler.attachLoaded(loaded);

    const idleController = new IdleController(scheduler, {
      enabled: settings?.idleEnabled ?? true,
      minIntervalSeconds: settings?.idleIntervalMinSeconds ?? 30,
      maxIntervalSeconds: settings?.idleIntervalMaxSeconds ?? 120,
      maxRepeatCount: 2,
      recentActionWeight: 0.3,
    });
    idleController.attachLoaded(loaded);

    const vitalityController = new DesktopPetVitalityController(scheduler, windowAdapter);
    const worldController = new DesktopPetWorldController(scheduler, windowAdapter);

    const clickThroughController = new ClickThroughController(
      windowAdapter,
      this.alphaThreshold,
    );
    clickThroughController.attach(win);

    const dragController = new DragController(
      windowAdapter,
      clickThroughController,
      (event, state) => this.handleDragEvent(event, state),
    );
    dragController.attach(win);

    const eventBridge = new DesktopPetEventBridge(scheduler, dragController);

    const chatStateBridge = new ChatStateBridge(scheduler);
    chatStateBridge.attachLoaded(loaded);
    chatStateBridge.attachPetWindow(win);

    this.windowAdapter = windowAdapter;
    this.actionPlayer = actionPlayer;
    this.animationIpc = animationIpc;
    this.scheduler = scheduler;
    this.idleController = idleController;
    this.vitalityController = vitalityController;
    this.worldController = worldController;
    this.clickThroughController = clickThroughController;
    this.dragController = dragController;
    this.eventBridge = eventBridge;
    this.chatStateBridge = chatStateBridge;
    this.loadedInstallation = loaded;

    this.registerChatStateIpc();

    // The IPC adapter must be live before the renderer is loaded. The initial
    // package snapshot is durable inside the adapter and will be flushed after
    // renderer bootstrap.
    animationIpc.register();
    this.sendInitialPackageSnapshot();
    await windowAdapter.loadRenderer();

    const readyPayload = await animationIpc.waitForRuntimeReady();
    if (
      readyPayload.packageRevision !== this.packageRevision ||
      readyPayload.packageId !== loaded.manifest.packageId
    ) {
      throw new Error(
        `RUNTIME_READY_PACKAGE_MISMATCH: expected package=${loaded.manifest.packageId} revision=${this.packageRevision}, got package=${readyPayload.packageId} revision=${readyPayload.packageRevision}`,
      );
    }

    idleController.start();
    vitalityController.start();
    worldController.start();
    windowAdapter.showWhenRuntimeReady();
  }

  private async stopRuntime(): Promise<void> {
    this.intentionalClose = true;
    this.unregisterChatStateIpc();
    try {
      if (this.windowAdapter) {
        if (this.recoveryWindowCloseListener) {
          this.windowAdapter.off("close", this.recoveryWindowCloseListener);
        }
        if (this.recoveryWindowCrashedListener) {
          this.windowAdapter.off("crashed", this.recoveryWindowCrashedListener);
        }
      }
    } catch (err) {
      console.warn("[DesktopPetManager] 移除窗口恢复监听失败:", err);
    }
    this.recoveryWindowCloseListener = null;
    this.recoveryWindowCrashedListener = null;
    try {
      if (this.dragController?.isDragging()) {
        this.dragController.detach();
      }
    } catch (err) {
      console.warn("[DesktopPetManager] 停止拖动控制器失败:", err);
    }

    try {
      this.worldController?.stop();
    } catch (err) {
      console.warn("[DesktopPetManager] 停止桌面世界控制器失败:", err);
    }
    this.worldController = null;

    try {
      this.vitalityController?.stop();
    } catch (err) {
      console.warn("[DesktopPetManager] 停止桌宠活力控制器失败:", err);
    }
    this.vitalityController = null;

    try {
      this.idleController?.stop();
    } catch (err) {
      console.warn("[DesktopPetManager] 停止待机控制器失败:", err);
    }
    try {
      this.scheduler?.forceInterrupt("app_exit");
    } catch (err) {
      console.warn("[DesktopPetManager] 停止调度器失败:", err);
    }
    try {
      this.actionPlayer?.stop();
    } catch (err) {
      console.warn("[DesktopPetManager] 停止播放器失败:", err);
    }
    try {
      this.animationIpc?.unregister();
    } catch (err) {
      console.warn("[DesktopPetManager] 注销动画IPC失败:", err);
    }
    try {
      this.eventBridge?.dispose();
    } catch (err) {
      console.warn("[DesktopPetManager] 销毁事件桥失败:", err);
    }
    try {
      this.chatStateBridge?.reset();
      this.chatStateBridge?.attachPetWindow(null);
    } catch (err) {
      console.warn("[DesktopPetManager] 重置聊天状态桥失败:", err);
    }
    try {
      this.dragController?.detach();
    } catch (err) {
      console.warn("[DesktopPetManager] 销毁拖动控制器失败:", err);
    }
    try {
      this.clickThroughController?.detach();
    } catch (err) {
      console.warn("[DesktopPetManager] 销毁点击穿透控制器失败:", err);
    }
    try {
      if (this.scheduler) {
        this.scheduler.detachLoaded();
        this.scheduler.dispose();
      }
    } catch (err) {
      console.warn("[DesktopPetManager] 销毁调度器失败:", err);
    }
    try {
      if (this.windowAdapter) {
        await this.windowAdapter.destroy();
      }
    } catch (err) {
      console.warn("[DesktopPetManager] 销毁窗口适配器失败:", err);
    }

    try {
      this.resourceCache.release();
    } catch (err) {
      console.warn("[DesktopPetManager] 释放资源缓存失败:", err);
    }

    this.windowAdapter = null;
    this.actionPlayer = null;
    this.animationIpc = null;
    this.scheduler = null;
    this.idleController = null;
    this.eventBridge = null;
    this.chatStateBridge = null;
    this.dragController = null;
    this.clickThroughController = null;
    this.loadedInstallation = null;
    this.currentActionKey = null;
    this.currentPlaybackId = null;
    this.currentCommandId = null;
    this.playbackCommandIds.clear();
    this.playbackDecisionIds.clear();
    this.rendererHealthy = false;
    this.intentionalClose = false;
  }

  private runLifecycleMutation<T>(operation: () => Promise<T>): Promise<T> {
    const run = this.lifecycleMutationQueue.then(operation, operation);
    this.lifecycleMutationQueue = run.then(
      () => undefined,
      () => undefined,
    );
    return run;
  }

  private async disableInternal(
    notifyBackend: boolean,
    persistPosition = true,
  ): Promise<void> {
    if (!this.activeInstallationId) {
      return;
    }
    const installationId = this.activeInstallationId;
    this.teardownRecoveryHandlers();
    if (persistPosition) {
      try {
        await this.persistRuntimePosition();
      } catch (err) {
        console.warn(
          "[DesktopPetManager] 停用前持久化位置失败:",
          this.errorMessage(err),
        );
      }
    }
    await this.stopRuntime();
    if (notifyBackend) {
      try {
        await this.callDisableApi(installationId);
      } catch (err) {
        console.warn(
          "[DesktopPetManager] 调用后端 disable 接口失败:",
          this.errorMessage(err),
        );
      }
    }
    this.petLogger.logDisable(installationId, notifyBackend ? "user" : "switch");
    this.activeInstallationId = null;
    this.activeInstallation = null;
    this.activeSettings = null;
    this.setState("disabled", installationId, "已停用");
  }

  private sendInitialPackageSnapshot(): void {
    if (!this.animationIpc || !this.activeInstallation || !this.loadedInstallation) {
      return;
    }
    try {
      const snapshot = this.animationIpc.buildSnapshot(
        this.activeInstallation,
        this.loadedInstallation,
        this.packageRevision,
      );
      this.animationIpc.sendSwitchPackage(snapshot);
      if (this.loadedInstallation.defaultAction) {
        this.animationIpc.sendUpdateDefaultAction(this.loadedInstallation.defaultAction.key);
      }
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 发送初始包快照失败:",
        this.errorMessage(err),
      );
    }
  }

  private runtimeEventContext(decisionId = ""): {
    installationId: string;
    characterId: string;
    petInstanceId: string;
    decisionId?: string;
  } {
    const context: {
      installationId: string;
      characterId: string;
      petInstanceId: string;
      decisionId?: string;
    } = {
      installationId: this.activeInstallationId ?? this.activeInstallation?.id ?? "",
      characterId: this.activeInstallation?.characterId ?? this.loadedInstallation?.manifest.characterId ?? "",
      petInstanceId: getRuntimeId(),
    };
    if (decisionId) {
      context.decisionId = decisionId;
    }
    return context;
  }

  private handlePlaybackEvent(event: { type: string; actionKey?: string; reason?: string; playbackInstanceId?: string; frameIndex?: number }): void {
    if (this.actionPlayer && this.actionPlayer instanceof AnimationPlayerBridge) {
      this.actionPlayer.handlePlaybackEvent(event);
    }

    const playbackId = event.playbackInstanceId ?? this.currentPlaybackId ?? "";
    const commandId = playbackId
      ? (this.playbackCommandIds.get(playbackId) ?? "")
      : (this.currentCommandId ?? "");
    const decisionId = (playbackId ? this.playbackDecisionIds.get(playbackId) : undefined)
      ?? this.scheduler?.getCurrent()?.metadata?.runtimeDecisionId
      ?? "";
    const context = this.runtimeEventContext(decisionId);

    switch (event.type) {
      case "playback.action_started":
        if (playbackId && commandId) {
          void this.runtimeHandler?.sendPlaybackStarted(
            playbackId,
            commandId,
            event.actionKey ?? "",
            context,
          );
        }
        break;
      case "playback.action_completed":
        if (commandId) {
          const runtime = this.runtimeHandler;
          if (runtime) {
            void runtime.sendPlaybackEnded(
              playbackId,
              commandId,
              event.actionKey ?? "",
              0,
              "natural_end",
              context,
            ).then(() => this.syncRuntimeState()).catch((err) => {
              console.warn("[DesktopPetManager] 上报播放完成失败:", this.errorMessage(err));
            });
          }
        }
        if (playbackId) {
          this.playbackCommandIds.delete(playbackId);
          this.playbackDecisionIds.delete(playbackId);
          if (this.currentPlaybackId === playbackId) {
            this.currentPlaybackId = null;
          }
        }
        if (this.currentCommandId === commandId) {
          this.currentCommandId = null;
        }
        break;
      case "playback.action_interrupted":
        if (commandId) {
          const runtime = this.runtimeHandler;
          if (runtime) {
            void runtime.sendPlaybackInterrupted(
              playbackId,
              commandId,
              event.actionKey ?? "",
              0,
              event.reason ?? "higher_priority_action",
              context,
            ).then(() => this.syncRuntimeState()).catch((err) => {
              console.warn("[DesktopPetManager] 上报播放中断失败:", this.errorMessage(err));
            });
          }
        }
        if (playbackId) {
          this.playbackCommandIds.delete(playbackId);
          this.playbackDecisionIds.delete(playbackId);
          if (this.currentPlaybackId === playbackId) {
            this.currentPlaybackId = null;
          }
        }
        if (this.currentCommandId === commandId) {
          this.currentCommandId = null;
        }
        break;
      case "playback.action_failed":
        console.warn(
          "[DesktopPetManager] 动画失败:",
          event.actionKey,
          event.reason,
        );
        if (commandId) {
          const runtime = this.runtimeHandler;
          if (runtime) {
            void runtime.sendPlaybackFailed(
              playbackId,
              commandId,
              event.actionKey ?? "",
              event.reason ?? "playback_failed",
              "playback execution failed",
              context,
            ).then(() => this.syncRuntimeState()).catch((err) => {
              console.warn("[DesktopPetManager] 上报播放失败失败:", this.errorMessage(err));
            });
          }
        }
        if (playbackId) {
          this.playbackCommandIds.delete(playbackId);
          this.playbackDecisionIds.delete(playbackId);
          if (this.currentPlaybackId === playbackId) {
            this.currentPlaybackId = null;
          }
        }
        if (this.currentCommandId === commandId) {
          this.currentCommandId = null;
        }
        break;
    }
  }

  private handleSnapshotUpdate(snapshot: PlaybackSnapshot): void {
    if (this.actionPlayer) {
      this.actionPlayer.handleSnapshotUpdate(snapshot);
    }
  }

  private handleActionCompleted(actionKey: string, _loopCount: number): void {
    void actionKey;
  }

  private handleActionInterrupted(actionKey: string, _loopCount: number): void {
    void actionKey;
  }

  private handleHitMask(payload: PetHitMaskPayload): void {
    this.clickThroughController?.updateHitMask(
      payload.width,
      payload.height,
      payload.data,
      payload.threshold,
    );
  }

  private handleRendererBootstrapped(): void {
    this.rendererHealthy = false;
    this.syncRuntimeState();
    this.petLogger.logWindowRecovered(
      "renderer-bootstrapped",
      this.activeInstallationId ?? undefined,
    );
  }

  private handleRuntimeReady(payload: RuntimeReadyPayload): void {
    if (!payload.snapshotApplied) {
      console.warn(
        "[DesktopPetManager] 渲染进程 RuntimeReady: snapshot 未应用",
      );
      return;
    }
    this.rendererHealthy = true;
    this.syncRuntimeState();
    this.petLogger.logWindowRecovered(
      "runtime-ready",
      this.activeInstallationId ?? undefined,
    );
  }

  private handleRuntimeInitFailed(payload: RuntimeInitFailedPayload): void {
    this.rendererHealthy = false;
    this.syncRuntimeState();
    this.petLogger.logRuntimeCrash(
      "renderer-runtime-init",
      new Error(payload.reason),
    );
  }

  private handleDeliveryFailed(
    reason: string,
    command: { actionKey?: string } | undefined,
  ): void {
    console.warn(
      "[DesktopPetManager] 指令投递失败:",
      reason,
      command?.actionKey ?? "",
    );
  }

  private handleClick(x: number, y: number): void {
    void this.runtimeHandler?.sendRuntimeEvent("runtime.pointer.clicked", {
      ...this.runtimeEventContext(),
      gestureId: randomUUID(),
      sequence: Date.now(),
      button: "left",
      clickCount: 1,
      canvasX: x,
      canvasY: y,
      screenX: x,
      screenY: y,
      frameIndex: this.actionPlayer?.getCurrentFrameIndex() ?? 0,
      actionKey: this.currentActionKey ?? "",
      occurredAt: new Date().toISOString(),
    });
  }

  private handleDoubleClick(x: number, y: number): void {
    void this.runtimeHandler?.sendRuntimeEvent("runtime.pointer.double_clicked", {
      ...this.runtimeEventContext(),
      gestureId: randomUUID(),
      sequence: Date.now(),
      button: "left",
      clickCount: 2,
      canvasX: x,
      canvasY: y,
      screenX: x,
      screenY: y,
      frameIndex: this.actionPlayer?.getCurrentFrameIndex() ?? 0,
      actionKey: this.currentActionKey ?? "",
      occurredAt: new Date().toISOString(),
    });
  }

  private handleHover(x: number, y: number): void {
    void this.runtimeHandler?.sendRuntimeEvent("runtime.pointer.hovered", {
      ...this.runtimeEventContext(),
      gestureId: randomUUID(),
      sequence: Date.now(),
      x,
      y,
      actionKey: this.currentActionKey ?? "",
      frameIndex: this.actionPlayer?.getCurrentFrameIndex() ?? 0,
      occurredAt: new Date().toISOString(),
    });
  }

  private handleActionSwitch(newKey: string, oldKey: string | null, playbackId?: string): void {
    this.currentActionKey = newKey;
    if (playbackId) {
      this.currentPlaybackId = playbackId;
      const request = this.scheduler?.getCurrent();
      const commandId = request?.metadata?.runtimeCommandId ?? "";
      const decisionId = request?.metadata?.runtimeDecisionId ?? "";
      if (commandId) {
        this.playbackCommandIds.set(playbackId, commandId);
      }
      if (decisionId) {
        this.playbackDecisionIds.set(playbackId, decisionId);
      }
    }
    this.petLogger.logActionSwitch(newKey, oldKey, "scheduler");
  }

  private handleSchedulerEvent(
    event: SchedulerEvent,
    request: DesktopPetActionRequest,
    action: RuntimeAction | null,
  ): void {
    if (event === "action-started") {
      this.currentCommandId = request.metadata?.runtimeCommandId ?? null;
      return;
    }
    if (event === "action-rejected") {
      console.warn(
        "[DesktopPetManager] 动作请求被拒绝:",
        request.actionKey,
        request.source,
      );
      return;
    }
    if (event === "action-fallback") {
      console.log(
        "[DesktopPetManager] 动作回退:",
        request.actionKey,
        "->",
        action?.key ?? "null",
      );
      this.petLogger.logActionFallback(
        request.actionKey,
        action?.key ?? null,
        request.source,
      );
      return;
    }
    if (event === "action-interrupted") {
      console.log(
        "[DesktopPetManager] 动作被打断:",
        request.actionKey,
        request.source,
      );
      return;
    }
  }

  private handleDragEvent(event: DragEvent, state: DragState): void {
    if (event === "drag-start") {
      this.worldController?.setDragging(true);
      this.currentDragId = randomUUID();
      this.petLogger.logDragStart(this.activeInstallationId ?? undefined);
      void this.runtimeHandler?.sendRuntimeEvent("runtime.drag.started", {
        ...this.runtimeEventContext(),
        gestureId: this.currentDragId,
        sequence: Date.now(),
        dragId: this.currentDragId,
        startX: state.startX,
        startY: state.startY,
        currentX: state.currentX,
        currentY: state.currentY,
        displayId: state.startScreenId,
        occurredAt: new Date().toISOString(),
      });
    } else if (event === "drag-end") {
      this.worldController?.setDragging(false);
      this.worldController?.onDrop();
      this.petLogger.logDragEnd(this.activeInstallationId ?? undefined);
      void this.runtimeHandler?.sendRuntimeEvent("runtime.drag.completed", {
        ...this.runtimeEventContext(),
        gestureId: this.currentDragId ?? "",
        sequence: Date.now(),
        dragId: this.currentDragId ?? "",
        startX: state.startX,
        startY: state.startY,
        currentX: state.currentX,
        currentY: state.currentY,
        displayId: state.currentScreenId,
        occurredAt: new Date().toISOString(),
      });
      this.currentDragId = null;
      void this.persistRuntimePosition();
    }
  }

  private resolveBehaviorEventSource(semantic: string): DesktopPetActionRequest["source"] {
    if (semantic.includes("speaking")) return EventSources.CHAT_SPEAKING;
    if (semantic.includes("listening")) return EventSources.CHAT_LISTENING;
    if (semantic.includes("thinking") || semantic.includes("processing")) {
      return EventSources.CHAT_THINKING;
    }
    if (semantic.startsWith("tool_") || semantic.includes("work")) {
      return EventSources.TOOL_WORKING;
    }
    if (semantic.startsWith("affect_") || semantic.startsWith("emotion_")) {
      return EventSources.EMOTION;
    }
    if (semantic.startsWith("proactive_")) return EventSources.PROACTIVE;
    if (semantic.startsWith("gesture_")) return EventSources.SYSTEM;
    return EventSources.SYSTEM;
  }

  private resolveBehaviorPriority(semantic: string, backendPriority?: number): number {
    if (semantic.includes("drag")) return ActionPriorities.DRAG;
    if (semantic.includes("drop") || semantic.includes("fall")) return ActionPriorities.FALL;
    if (semantic.includes("speaking")) return ActionPriorities.SPEAKING;
    if (semantic.includes("listening") || semantic.includes("thinking") || semantic.startsWith("tool_")) {
      return ActionPriorities.THINKING;
    }
    if (semantic.startsWith("affect_") || semantic.startsWith("emotion_")) {
      return ActionPriorities.EMOTION;
    }
    if (semantic.startsWith("proactive_")) return ActionPriorities.GREETING;
    if (semantic.startsWith("activity_") || semantic.startsWith("life_")) {
      return ActionPriorities.LIFE;
    }
    if (typeof backendPriority === "number" && Number.isFinite(backendPriority)) {
      if (backendPriority >= 800) return ActionPriorities.SPEAKING;
      if (backendPriority >= 600) return ActionPriorities.THINKING;
      if (backendPriority >= 400) return ActionPriorities.EMOTION;
      return ActionPriorities.LIFE;
    }
    return ActionPriorities.EMOTION;
  }

  private applyVitalityForBehaviorSemantic(semantic: string): void {
    if (!this.vitalityController) return;
    if (semantic.includes("speaking") || semantic.includes("listening")) {
      this.vitalityController.notifyInteraction("voice");
      this.vitalityController.setActivity("attentive");
      return;
    }
    if (semantic.includes("thinking") || semantic.startsWith("tool_") || semantic.includes("work")) {
      this.vitalityController.notifyInteraction("tool");
      this.vitalityController.setActivity("working");
      return;
    }
    if (semantic.startsWith("dialogue_") || semantic.startsWith("proactive_")) {
      this.vitalityController.notifyInteraction("chat");
      this.vitalityController.setActivity("attentive");
    }
  }

  private async persistRuntimePosition(): Promise<void> {
    if (!this.windowAdapter || !this.activeInstallationId) return;
    const snapshot = this.windowAdapter.snapshotRuntimePosition();
    const patch: Partial<RuntimeSettingsInfo> = {
      positionX: Math.round(snapshot.x),
      positionY: Math.round(snapshot.y),
      screenId: snapshot.screenId,
      scale: snapshot.scale,
    };
    try {
      const updated = await this.callUpdateSettingsApi(
        this.activeInstallationId,
        this.buildSettingsPatch(patch),
      );
      if (updated) {
        this.activeSettings = updated;
        this.markSettingsRevisionApplied(updated.settingsRevision);
      }
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 持久化运行时位置失败:",
        this.errorMessage(err),
      );
    }
  }

  private registerChatStateIpc(): void {
    ipcMain.on(CHAT_STATE_CHANGE_CHANNEL, this.boundChatStateChange);
  }

  private unregisterChatStateIpc(): void {
    ipcMain.removeListener(CHAT_STATE_CHANGE_CHANNEL, this.boundChatStateChange);
  }

  private readonly boundChatStateChange = (
    _event: Electron.IpcMainEvent,
    payload: ChatStateIpcPayload,
  ): void => {
    if (!this.chatStateBridge) return;
    try {
      this.chatStateBridge.handleIpcPayload(payload);
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 处理聊天状态失败:",
        this.errorMessage(err),
      );
    }
  };

  private setState(
    state: PetManagerState,
    installationId: string | null,
    reason?: string,
  ): void {
    this.state = state;
    const payload: PetStatePayload = {
      state,
      installationId,
      reason,
    };
    for (const listener of this.stateListeners) {
      try {
        listener(payload);
      } catch (err) {
        console.warn("[DesktopPetManager] 状态监听器异常:", err);
      }
    }
    const mainWin = BrowserWindow.getAllWindows().find(
      (w) => !w.isDestroyed() && w.webContents.getURL().includes("renderer"),
    );
    if (mainWin && !mainWin.isDestroyed()) {
      try {
        mainWin.webContents.send(PET_STATE_CHANNEL, payload);
      } catch {
        void 0;
      }
    }
    this.syncRuntimeState();
  }

  private resolveAbsolutePath(relPath: string): string {
    if (!relPath) return "";
    const dataDir = getAmitiaDataDir();
    if (isAbsolute(relPath)) {
      return relPath;
    }
    return join(dataDir, relPath);
  }

  private async fetchInstallation(
    installationId: string,
  ): Promise<InstallationInfo | null> {
    return this.getInstallation(installationId);
  }

  private async fetchRuntimeSettings(
    installationId: string,
  ): Promise<RuntimeSettingsInfo | null> {
    try {
      return await this.getRuntimeSettings(installationId);
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 查询运行时设置失败, 使用默认设置:",
        this.errorMessage(err),
      );
      return null;
    }
  }

  private async callEnableApi(installationId: string): Promise<void> {
    const path = `${API_BASE_PATH}/installations/${encodeURIComponent(installationId)}/enable`;
    await this.request("POST", path, {});
  }

  private async callDisableApi(installationId: string): Promise<void> {
    const path = `${API_BASE_PATH}/installations/${encodeURIComponent(installationId)}/disable`;
    await this.request("POST", path, {});
  }

  private async callPlayActionApi(actionKey: string): Promise<void> {
    if (!this.activeInstallationId) return;
    const path = `${API_BASE_PATH}/installations/${encodeURIComponent(this.activeInstallationId)}/actions/${encodeURIComponent(actionKey)}/play`;
    await this.request("POST", path, {});
  }

  private async callRecenterApi(installationId: string): Promise<void> {
    const path = `${API_BASE_PATH}/installations/${encodeURIComponent(installationId)}/recenter`;
    await this.request("POST", path, {});
  }

  private async callUpdateDefaultActionApi(
    installationId: string,
    actionKey: string,
  ): Promise<void> {
    const path = `${API_BASE_PATH}/installations/${encodeURIComponent(installationId)}/default-action`;
    await this.request("PATCH", path, { actionKey });
  }

  private async callUpdateSettingsApi(
    installationId: string,
    patch: Record<string, unknown>,
  ): Promise<RuntimeSettingsInfo | null> {
    const path = `${API_BASE_PATH}/installations/${encodeURIComponent(installationId)}/settings`;
    const response = await this.request<RuntimeSettingsMutationApiPayload>(
      "PATCH",
      path,
      patch,
    );
    return response?.settings ? mapRuntimeSettingsPayload(response.settings) : null;
  }

  private async markInstallationInvalid(
    installationId: string,
    errorCode: string,
  ): Promise<void> {
    this.petLogger.logCorruption(installationId, errorCode, errorCode);
    const path = `${API_BASE_PATH}/installations/${encodeURIComponent(installationId)}/disable`;
    try {
      await this.request("POST", path, { reason: errorCode, invalid: true });
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 调用后端标记 invalid 失败:",
        installationId,
        this.errorMessage(err),
      );
    }
    const win = BrowserWindow.getAllWindows().find(
      (w) => !w.isDestroyed() && w.webContents.getURL().includes("renderer"),
    );
    if (win) {
      const payload: PetLoadErrorPayload = {
        installationId,
        error: "安装资源校验失败, 已标记为 invalid",
        errorCode,
      };
      try {
        win.webContents.send(PET_LOAD_ERROR_CHANNEL, payload);
      } catch {
        void 0;
      }
    }
  }

  private async detectCorruption(
    installation: InstallationInfo,
  ): Promise<CorruptionDetectionResult> {
    const installPath = this.resolveAbsolutePath(installation.installPath);
    const manifestPath = this.resolveAbsolutePath(installation.manifestPath);
    if (!installPath || !manifestPath) {
      return {
        corrupted: true,
        errorCode: CORRUPTION_MANIFEST_MISSING,
        detail: `安装路径无效: installPath=${installation.installPath} manifestPath=${installation.manifestPath}`,
      };
    }
    try {
      await stat(installPath);
    } catch (err) {
      return {
        corrupted: true,
        errorCode: CORRUPTION_MANIFEST_MISSING,
        detail: `安装目录不可访问: ${installPath} (${this.errorMessage(err)})`,
      };
    }
    try {
      await readFile(manifestPath, "utf8");
    } catch (err) {
      return {
        corrupted: true,
        errorCode: CORRUPTION_MANIFEST_MISSING,
        detail: `manifest 不可读取: ${manifestPath} (${this.errorMessage(err)})`,
      };
    }

    let loaded: LoadedInstallation | null = null;
    try {
      const request: LoadInstallationRequest = {
        installationId: installation.id,
        petId: installation.petId,
        releaseId: installation.currentReleaseId,
        installPath,
        manifestPath,
        expectedContentRootHash: installation.installedContentHash || installation.packageHash,
      };
      loaded = await this.resourceLoader.loadInstallation(request);
    } catch (err) {
      const message = this.errorMessage(err);
      if (message.includes("FRAME_MISSING")) {
        return {
          corrupted: true,
          errorCode: CORRUPTION_FRAME_MISSING,
          detail: `帧文件丢失: ${message}`,
        };
      }
      if (
        message.includes("ACTION_JSON_PARSE_FAILED") ||
        message.includes("ACTION_JSON_READ_FAILED")
      ) {
        return {
          corrupted: true,
          errorCode: CORRUPTION_ACTION_CONFIG_INVALID,
          detail: `动作配置无法解析: ${message}`,
        };
      }
      if (message.includes("DEFAULT_ACTION_INVALID")) {
        return {
          corrupted: true,
          errorCode: CORRUPTION_DEFAULT_ACTION_UNAVAILABLE,
          detail: `默认动作不可用: ${message}`,
        };
      }
      if (message.includes("DEFAULT_ACTION_NOT_FOUND")) {
        return {
          corrupted: true,
          errorCode: CORRUPTION_DEFAULT_ACTION_MISSING,
          detail: `默认动作丢失: ${message}`,
        };
      }
      if (
        message.includes("PACKAGE_HASH_MISMATCH") ||
        message.includes("PACKAGE_MANIFEST_HASH_MISMATCH") ||
        message.includes("PACKAGE_FILE_UNDECLARED") ||
        message.includes("PACKAGE_RESOURCE_HASH_MISMATCH") ||
        message.includes("FRAME_HASH_MISMATCH")
      ) {
        return {
          corrupted: true,
          errorCode: CORRUPTION_PACKAGE_HASH_MISMATCH,
          detail: `Package v2 完整性校验失败: ${message}`,
        };
      }
      return {
        corrupted: true,
        errorCode: CORRUPTION_ACTION_CONFIG_INVALID,
        detail: `资源加载异常: ${message}`,
      };
    }
    if (!loaded.defaultAction || !loaded.defaultAction.available) {
      const detail = loaded.defaultAction?.loadError ?? "defaultAction null";
      return {
        corrupted: true,
        errorCode: CORRUPTION_DEFAULT_ACTION_UNAVAILABLE,
        detail: `默认动作不可用: ${detail}`,
      };
    }

    if (
      installation.packageHash &&
      installation.installedContentHash &&
      installation.packageHash !== installation.installedContentHash
    ) {
      return {
        corrupted: true,
        errorCode: CORRUPTION_PACKAGE_HASH_MISMATCH,
        detail: `安装记录哈希语义不一致 packageHash=${installation.packageHash.slice(0, 12)}... installedContentHash=${installation.installedContentHash.slice(0, 12)}...`,
      };
    }

    return { corrupted: false, errorCode: "", detail: "" };
  }

  private async handleCorruption(
    installation: InstallationInfo,
    detection: CorruptionDetectionResult,
  ): Promise<void> {
    this.petLogger.logCorruption(
      installation.id,
      detection.errorCode,
      detection.detail,
    );
    this.teardownRecoveryHandlers();
    try {
      await this.stopRuntime();
    } catch (err) {
      this.petLogger.logRuntimeCrash("handleCorruption.stopRuntime", err);
    }
    await this.markInstallationInvalid(installation.id, detection.errorCode);
    this.activeInstallationId = null;
    this.activeInstallation = null;
    this.activeSettings = null;
    this.loadedInstallation = null;
    this.setState("invalid", installation.id, detection.detail);
  }

  private setupRecoveryHandlers(): void {
    if (this.recoveryHandlersAttached) return;
    this.recoveryHandlersAttached = true;
    powerMonitor.on("resume", this.boundPowerResume);
    screen.on("display-added", this.boundDisplayAdded);
    screen.on("display-removed", this.boundDisplayRemoved);
    screen.on("display-metrics-changed", this.boundDisplayMetricsChanged);
    app.on("child-process-gone", this.boundChildProcessGone);
  }

  private teardownRecoveryHandlers(): void {
    if (!this.recoveryHandlersAttached) {
      this.clearRecoveryTimers();
      return;
    }
    this.recoveryHandlersAttached = false;
    try {
      powerMonitor.off("resume", this.boundPowerResume);
    } catch {
      void 0;
    }
    try {
      screen.off("display-added", this.boundDisplayAdded);
    } catch {
      void 0;
    }
    try {
      screen.off("display-removed", this.boundDisplayRemoved);
    } catch {
      void 0;
    }
    try {
      screen.off("display-metrics-changed", this.boundDisplayMetricsChanged);
    } catch {
      void 0;
    }
    try {
      app.off("child-process-gone", this.boundChildProcessGone);
    } catch {
      void 0;
    }
    this.clearRecoveryTimers();
    this.recoveryInProgress = false;
  }

  private clearRecoveryTimers(): void {
    if (this.recoveryDebounceTimer) {
      clearTimeout(this.recoveryDebounceTimer);
      this.recoveryDebounceTimer = null;
    }
    if (this.recoveryRetryTimer) {
      clearTimeout(this.recoveryRetryTimer);
      this.recoveryRetryTimer = null;
    }
    this.recoveryRetryCount = 0;
  }

  private scheduleRecovery(reason: RecoveryReason): void {
    if (
      !this.activeInstallationId ||
      (this.state !== "enabled" && this.state !== "degraded")
    ) {
      return;
    }
    if (this.recoveryDebounceTimer) {
      clearTimeout(this.recoveryDebounceTimer);
    }
    this.recoveryDebounceTimer = setTimeout(() => {
      this.recoveryDebounceTimer = null;
      void this.recoverRuntime(reason);
    }, RECOVERY_DEBOUNCE_MS);
    if (typeof this.recoveryDebounceTimer.unref === "function") {
      this.recoveryDebounceTimer.unref();
    }
  }

  private scheduleRecoveryRetry(reason: RecoveryReason): void {
    if (
      !this.activeInstallationId ||
      this.recoveryRetryCount >= RECOVERY_RETRY_MAX_ATTEMPTS
    ) {
      return;
    }
    this.recoveryRetryCount += 1;
    const delay = RECOVERY_RETRY_BASE_DELAY_MS * (2 ** (this.recoveryRetryCount - 1));
    if (this.recoveryRetryTimer) {
      clearTimeout(this.recoveryRetryTimer);
    }
    this.recoveryRetryTimer = setTimeout(() => {
      this.recoveryRetryTimer = null;
      void this.recoverRuntime(reason);
    }, delay);
    if (typeof this.recoveryRetryTimer.unref === "function") {
      this.recoveryRetryTimer.unref();
    }
  }

  private readonly boundPowerResume = (): void => {
    this.petLogger.logWindowRecovered(
      RECOVERY_REASON_POWER_RESUME,
      this.activeInstallationId ?? undefined,
    );
    this.scheduleRecovery(RECOVERY_REASON_POWER_RESUME);
  };

  private readonly boundDisplayAdded = (_event: unknown, _display: Display): void => {
    this.scheduleRecovery(RECOVERY_REASON_DISPLAY_CHANGED);
  };

  private readonly boundDisplayRemoved = (
    _event: unknown,
    _display: Display,
  ): void => {
    this.scheduleRecovery(RECOVERY_REASON_DISPLAY_CHANGED);
  };

  private readonly boundDisplayMetricsChanged = (
    _event: unknown,
    _display: Display,
    changedMetrics: string[],
  ): void => {
    if (
      Array.isArray(changedMetrics) &&
      (changedMetrics.includes("bounds") ||
        changedMetrics.includes("workArea") ||
        changedMetrics.includes("scaleFactor"))
    ) {
      this.scheduleRecovery(RECOVERY_REASON_DISPLAY_CHANGED);
    }
  };

  private readonly boundChildProcessGone = (
    _event: unknown,
    details: { type?: string; reason?: string },
  ): void => {
    if (details && details.type !== "GPU") {
      return;
    }
    this.petLogger.logWindowRecovered(
      RECOVERY_REASON_GPU_CRASHED,
      this.activeInstallationId ?? undefined,
    );
    this.scheduleRecovery(RECOVERY_REASON_GPU_CRASHED);
  };

  private async recoverRuntime(reason: RecoveryReason): Promise<void> {
    if (this.recoveryInProgress) {
      return;
    }
    const installationId = this.activeInstallationId;
    const installation = this.activeInstallation;
    if (!installationId || !installation) {
      return;
    }
    this.recoveryInProgress = true;
    try {
      const detection = await this.detectCorruption(installation);
      if (detection.corrupted) {
        await this.handleCorruption(installation, detection);
        return;
      }
      const settings = await this.fetchRuntimeSettings(installationId);
      const loaded = await this.loadAndValidateInstallation(installation);
      if (!loaded) {
        await this.handleCorruption(installation, {
          corrupted: true,
          errorCode: "RESOURCE_VALIDATION_FAILED",
          detail: "恢复期间 loadAndValidateInstallation 返回空",
        });
        return;
      }
      this.scheduler?.clearQueue();
      await this.stopRuntime();
      this.activeSettings = settings;
      this.loadedInstallation = loaded;
      await this.startRuntime(installation, settings, loaded);
      this.setupRecoveryHandlers();
      this.recoveryRetryCount = 0;
      if (this.recoveryRetryTimer) {
        clearTimeout(this.recoveryRetryTimer);
        this.recoveryRetryTimer = null;
      }
      this.petLogger.logWindowRecovered(reason, installationId);
      this.setState("enabled", installationId, `恢复完成 reason=${reason}`);
    } catch (err) {
      this.petLogger.logRuntimeCrash(`recoverRuntime:${reason}`, err);
      console.error(
        "[DesktopPetManager] 运行时恢复失败:",
        this.errorMessage(err),
      );
      try {
        await this.stopRuntime();
      } catch (stopErr) {
        this.petLogger.logRuntimeCrash("recoverRuntime.stopRuntime", stopErr);
      }
      this.activeInstallationId = installationId;
      this.activeInstallation = installation;
      this.setState(
        "degraded",
        installationId,
        `恢复失败: ${this.errorMessage(err)}`,
      );
      this.scheduleRecoveryRetry(reason);
    } finally {
      this.recoveryInProgress = false;
    }
  }

  private buildSettingsPatch(
    settings: Partial<RuntimeSettingsInfo>,
  ): Record<string, unknown> {
    const patch: Record<string, unknown> = {};
    if (typeof settings.alwaysOnTop === "boolean") {
      patch.alwaysOnTop = settings.alwaysOnTop ? 1 : 0;
    }
    if (typeof settings.launchOnStartup === "boolean") {
      patch.launchOnStartup = settings.launchOnStartup ? 1 : 0;
    }
    if (typeof settings.scale === "number" && Number.isFinite(settings.scale)) {
      patch.scale = settings.scale;
    }
    if (typeof settings.positionX === "number") {
      patch.positionX = Math.round(settings.positionX);
    }
    if (typeof settings.positionY === "number") {
      patch.positionY = Math.round(settings.positionY);
    }
    if (typeof settings.screenId === "string") {
      patch.screenId = settings.screenId;
    }
    if (typeof settings.idleEnabled === "boolean") {
      patch.idleEnabled = settings.idleEnabled ? 1 : 0;
    }
    if (typeof settings.idleIntervalMinSeconds === "number") {
      patch.idleIntervalMinSeconds = settings.idleIntervalMinSeconds;
    }
    if (typeof settings.idleIntervalMaxSeconds === "number") {
      patch.idleIntervalMaxSeconds = settings.idleIntervalMaxSeconds;
    }
    if (typeof settings.clickThroughMode !== "undefined") {
      patch.clickThroughMode = clickThroughModeToApiValue(
        settings.clickThroughMode,
      );
    }
    if (typeof settings.soundEnabled === "boolean") {
      patch.soundEnabled = settings.soundEnabled ? 1 : 0;
    }
    return patch;
  }

  private mergeSettings(
    base: RuntimeSettingsInfo | null,
    updates: Partial<RuntimeSettingsInfo>,
  ): RuntimeSettingsInfo {
    const fallback: RuntimeSettingsInfo = {
      installationId: this.activeInstallationId ?? "",
      settingsRevision: 0,
      alwaysOnTop: true,
      launchOnStartup: false,
      scale: PET_WINDOW_SCALE_DEFAULT,
      positionX: 0,
      positionY: 0,
      screenId: "",
      idleEnabled: true,
      idleIntervalMinSeconds: 30,
      idleIntervalMaxSeconds: 120,
      clickThroughMode: "none",
      soundEnabled: false,
    };
    const current = base ?? fallback;
    return {
      installationId: current.installationId,
      settingsRevision:
        typeof updates.settingsRevision === "number"
          ? updates.settingsRevision
          : current.settingsRevision,
      alwaysOnTop:
        typeof updates.alwaysOnTop === "boolean"
          ? updates.alwaysOnTop
          : current.alwaysOnTop,
      launchOnStartup:
        typeof updates.launchOnStartup === "boolean"
          ? updates.launchOnStartup
          : current.launchOnStartup,
      scale:
        typeof updates.scale === "number" ? updates.scale : current.scale,
      positionX:
        typeof updates.positionX === "number"
          ? updates.positionX
          : current.positionX,
      positionY:
        typeof updates.positionY === "number"
          ? updates.positionY
          : current.positionY,
      screenId:
        typeof updates.screenId === "string"
          ? updates.screenId
          : current.screenId,
      idleEnabled:
        typeof updates.idleEnabled === "boolean"
          ? updates.idleEnabled
          : current.idleEnabled,
      idleIntervalMinSeconds:
        typeof updates.idleIntervalMinSeconds === "number"
          ? updates.idleIntervalMinSeconds
          : current.idleIntervalMinSeconds,
      idleIntervalMaxSeconds:
        typeof updates.idleIntervalMaxSeconds === "number"
          ? updates.idleIntervalMaxSeconds
          : current.idleIntervalMaxSeconds,
      clickThroughMode:
        typeof updates.clickThroughMode !== "undefined"
          ? updates.clickThroughMode
          : current.clickThroughMode,
      soundEnabled:
        typeof updates.soundEnabled === "boolean"
          ? updates.soundEnabled
          : current.soundEnabled,
    };
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
  ): Promise<T> {
    const url = new URL(`http://${this.coreHost}:${this.corePort}${path}`);

    const sessionClient = getBackendSessionClient();
    const authHeaders = await sessionClient.getMainProcessAuthHeaders();

    const headers: Record<string, string> = {
      Accept: "application/json",
      "Content-Type": "application/json",
      "X-Amitia-Device-ID": getDeviceId(),
      ...authHeaders,
    };

    const bodyStr = body !== undefined ? JSON.stringify(body) : undefined;
    if (bodyStr !== undefined) {
      headers["Content-Length"] = Buffer.byteLength(bodyStr).toString();
    }

    return new Promise<T>((resolve, reject) => {
      const req = http.request(
        {
          hostname: url.hostname,
          port: url.port,
          path: url.pathname + url.search,
          method,
          headers,
          timeout: 10000,
        },
        (res) => {
          let data = "";
          res.setEncoding("utf8");
          res.on("data", (chunk: string) => {
            data += chunk;
          });
          res.on("end", () => {
            const statusCode = res.statusCode ?? 500;
            if (statusCode < 200 || statusCode >= 300) {
              if (statusCode === 401) {
                sessionClient.invalidateSession();
              }
              reject(
                new Error(`desktop pet api failed: ${statusCode} ${data}`),
              );
              return;
            }
            if (!data) {
              resolve(undefined as T);
              return;
            }
            try {
              const parsed = JSON.parse(data) as ApiEnvelope<T>;
              if (
                parsed &&
                typeof parsed === "object" &&
                "code" in parsed &&
                typeof parsed.code === "number"
              ) {
                if (parsed.code < 200 || parsed.code >= 300) {
                  reject(
                    new Error(
                      `desktop pet api failed: ${parsed.code} ${parsed.msg ?? ""}`,
                    ),
                  );
                  return;
                }
                resolve(parsed.data as T);
                return;
              }
              resolve(parsed as unknown as T);
            } catch (error) {
              reject(
                new Error(
                  `desktop pet api parse failed: ${
                    error instanceof Error ? error.message : String(error)
                  }`,
                ),
              );
            }
          });
        },
      );
      req.on("error", reject);
      req.on("timeout", () => {
        req.destroy(new Error("desktop pet api timeout"));
      });
      if (bodyStr !== undefined) {
        req.write(bodyStr);
      }
      req.end();
    });
  }

  private errorMessage(err: unknown): string {
    if (err instanceof Error) return err.message;
    return String(err);
  }

  private startBridge(): void {
    if (this.bridgeStarted) return;
    this.bridgeStarted = true;
    void this.connectBridge();
  }

  private stopBridge(): void {
    this.bridgeStarted = false;
    if (this.bridgeReconnectTimer) {
      clearTimeout(this.bridgeReconnectTimer);
      this.bridgeReconnectTimer = null;
    }
    if (this.runtimeHandler) {
      this.captureRuntimeCursor(this.runtimeHandler);
      try {
        this.runtimeHandler.disconnect();
      } catch {
        void 0;
      }
      this.runtimeHandler = null;
    }
  }

  private async connectBridge(): Promise<void> {
    if (!this.bridgeStarted || this.bridgeConnecting) return;
    this.bridgeConnecting = true;

    const runtimeId = getRuntimeId();
    const deviceId = getDeviceId();

    try {
      if (this.runtimeHandler) {
        const previousHandler = this.runtimeHandler;
        this.captureRuntimeCursor(previousHandler);
        this.runtimeHandler = null;
        try {
          previousHandler.disconnect();
        } catch {
          void 0;
        }
      }

      const issued = await createRuntimeBootstrapTicket(deviceId, runtimeId);
      const wsUrl = this.buildRuntimeV2URL(issued.ticket, runtimeId, deviceId);
      const resumeCursor: RuntimeResumeCursor = {
        ...this.runtimeResumeCursor,
        lastAppliedDesiredRevision: Math.max(
          this.runtimeResumeCursor.lastAppliedDesiredRevision,
          this.lastAppliedDesiredRevision,
        ),
      };
      const handlerConfig: RuntimeHandlerConfig = {
        url: wsUrl,
        userId: issued.userId,
        deviceId,
        runtimeId,
        runtimeVersion: this.runtimeVersion,
        autoReconnect: false,
        heartbeatIntervalMs: 15000,
        maxReconnectAttempts: 0,
        resumeCursor,
      };

      const hooks = this.buildRuntimeHooks();
      const handler = new DesktopRuntimeHandlerV2(handlerConfig, hooks);
      this.runtimeHandler = handler;
      const reconnectReason = resumeCursor.lastEventSequence > 0
        ? "transport_lost"
        : "initial";
      await handler.connect(reconnectReason);
      this.captureRuntimeCursor(handler);
    } catch (error) {
      console.warn(
        "[DesktopPetManager] runtime ticket/连接失败:",
        this.errorMessage(error),
      );
      this.scheduleBridgeReconnect();
    } finally {
      this.bridgeConnecting = false;
    }
  }

  private buildRuntimeV2URL(ticket: string, runtimeId: string, deviceId: string): string {
    const wsBase = `ws://${this.coreHost}:${this.corePort}${RUNTIME_BRIDGE_WS_PATH}`;
    const url = new URL(wsBase);
    url.searchParams.set("ticket", ticket);
    url.searchParams.set("deviceId", deviceId);
    url.searchParams.set("runtimeId", runtimeId);
    return url.toString();
  }

  private scheduleBridgeReconnect(): void {
    if (!this.bridgeStarted) return;
    if (this.bridgeReconnectTimer) {
      clearTimeout(this.bridgeReconnectTimer);
    }
    this.bridgeReconnectTimer = setTimeout(() => {
      this.bridgeReconnectTimer = null;
      void this.connectBridge();
    }, BRIDGE_RECONNECT_DELAY_MS);
    if (typeof this.bridgeReconnectTimer.unref === "function") {
      this.bridgeReconnectTimer.unref();
    }
  }

  private async registerDeviceIdentity(): Promise<void> {
    await this.request("POST", "/api/local/devices/register", {
      deviceId: getDeviceId(),
      desktopInstanceId: getBackendSessionClient().getDesktopInstanceID(),
      platform: process.platform,
      appVersion: app.getVersion(),
    });
  }

  private buildRuntimeHooks(): RuntimeHandlerHooks {
    return {
      onState: (state) => {
        if (state === "disconnected") {
          console.warn("[DesktopPetManager] runtime disconnected");
          const activeHandler = this.runtimeHandler;
          if (
            this.bridgeStarted &&
            activeHandler &&
            activeHandler.getState() === "disconnected"
          ) {
            this.scheduleBridgeReconnect();
          }
        }
      },
      onHelloAck: (ack: HelloAckPayload) => {
        console.log(
          "[DesktopPetManager] runtime connected session=",
          ack.sessionId,
        );
        void ack.currentDesiredRevision;
        this.syncRuntimeState();
      },
      onError: (err: Error) => {
        console.warn("[DesktopPetManager] runtime error:", err.message);
      },
      onEvent: (_envelope: RuntimeEnvelope) => {
      },
      onDesiredSync: (revision: number) => {
        this.markDesiredRevisionApplied(revision);
      },
      onCommandSettled: (result, envelope) => {
        const command = envelope.payload as { settingsRevision?: number } | undefined;
        if (result.status === "applied" || result.status === "duplicate") {
          this.markSettingsRevisionApplied(command?.settingsRevision ?? 0);
        }
        this.captureRuntimeCursor(this.runtimeHandler);
        this.syncRuntimeState();
      },
      onCommand: async (
        command: unknown,
        _envelope: RuntimeEnvelope,
      ): Promise<RuntimeCommandExecutionResult> => {
        const cmd = command as {
          commandId?: string;
          commandType?: string;
          desiredRevision?: number;
          installationId?: string;
          settingsRevision?: number;
          actionKey?: string;
          payload?: {
            installation?: { installationId?: string };
            desiredPet?: { installation?: { installationId?: string } };
            ensureAbsent?: boolean;
            actionKey?: string;
            priority?: number;
            queuePolicy?: string;
            interruptible?: boolean;
            minimumPlayMs?: number;
            maximumPlayMs?: number;
            returnTo?: string;
            decisionId?: string;
            semantic?: string;
            reasonCode?: string;
            characterId?: string;
            petInstanceId?: string;
            installationId?: string;
            settings?: {
              alwaysOnTop?: boolean;
              scale?: number;
              positionX?: number;
              positionY?: number;
              screenId?: string;
              clickThroughMode?: string;
              soundEnabled?: boolean;
            };
          };
        };

        const commandId = cmd.commandId ?? "";
        const commandType = cmd.commandType ?? "";
        const desiredRevision = cmd.desiredRevision ?? 0;

        try {
          switch (commandType) {
            case "spawn":
            case "runtime.command.sync_desired_state": {
              const installationId = cmd.payload?.installation?.installationId
                ?? cmd.installationId
                ?? "";
              if (!installationId) {
                return {
                  commandId,
                  status: "rejected",
                  errorCode: "MISSING_INSTALLATION_ID",
                  errorMessage: "spawn payload missing installationId",
                  appliedRevision: desiredRevision,
                };
              }
              await this.runLifecycleMutation(() =>
                this.enableInstallationInternal(installationId, false, false),
              );
              this.markDesiredRevisionApplied(desiredRevision);
              return {
                commandId,
                status: "applied",
                errorCode: "",
                errorMessage: "",
                appliedRevision: desiredRevision,
                actualState: this.collectPetInstanceSummary(),
              };
            }
            case "destroy":
            case "runtime.command.ensure_absent": {
              await this.runLifecycleMutation(() =>
                this.disableInternal(false, false),
              );
              this.markDesiredRevisionApplied(desiredRevision);
              return {
                commandId,
                status: "applied",
                errorCode: "",
                errorMessage: "",
                appliedRevision: desiredRevision,
              };
            }
            case "show": {
              const win = this.windowAdapter?.getNativeWindow();
              if (win && !win.isDestroyed()) {
                win.show();
              }
              this.markDesiredRevisionApplied(desiredRevision);
              return {
                commandId,
                status: "applied",
                errorCode: "",
                errorMessage: "",
                appliedRevision: desiredRevision,
                actualState: this.collectPetInstanceSummary(),
              };
            }
            case "hide": {
              const win = this.windowAdapter?.getNativeWindow();
              if (win && !win.isDestroyed()) {
                win.hide();
              }
              this.markDesiredRevisionApplied(desiredRevision);
              return {
                commandId,
                status: "applied",
                errorCode: "",
                errorMessage: "",
                appliedRevision: desiredRevision,
                actualState: this.collectPetInstanceSummary(),
              };
            }
            case "play_action":
            case "runtime.command.play_action": {
              const actionKey = cmd.payload?.actionKey ?? cmd.actionKey ?? "";
              if (!actionKey || !this.scheduler || !this.loadedInstallation) {
                return {
                  commandId,
                  status: "rejected",
                  errorCode: !actionKey ? "MISSING_ACTION_KEY" : "PET_NOT_READY",
                  errorMessage: !actionKey
                    ? "play_action payload missing actionKey"
                    : "desktop pet scheduler is not ready",
                  appliedRevision: desiredRevision,
                };
              }

              const semantic = cmd.payload?.semantic ?? "";
              const source = this.resolveBehaviorEventSource(semantic);
              const request: DesktopPetActionRequest = {
                actionKey,
                source,
                priority: this.resolveBehaviorPriority(semantic, cmd.payload?.priority),
                interrupt: cmd.payload?.queuePolicy === "replace_current",
                dedupeKey: cmd.payload?.decisionId || `runtime_${commandId}`,
                metadata: {
                  semantic,
                  reasonCode: cmd.payload?.reasonCode ?? "",
                  minimumPlayMs: String(cmd.payload?.minimumPlayMs ?? 0),
                  maximumPlayMs: String(cmd.payload?.maximumPlayMs ?? 0),
                  returnTo: cmd.payload?.returnTo ?? "default",
                  runtimeCommandId: commandId,
                  runtimeDecisionId: cmd.payload?.decisionId ?? "",
                  runtimeInstallationId: cmd.payload?.installationId ?? this.activeInstallationId ?? "",
                  runtimeCharacterId: cmd.payload?.characterId ?? this.activeInstallation?.characterId ?? "",
                  runtimePetInstanceId: cmd.payload?.petInstanceId ?? getRuntimeId(),
                },
              };
              this.applyVitalityForBehaviorSemantic(semantic);
              const scheduleResult = this.scheduler.submit(request);
              if (scheduleResult === "rejected") {
                return {
                  commandId,
                  status: "rejected",
                  errorCode: "ACTION_REJECTED",
                  errorMessage: `scheduler rejected action ${actionKey}`,
                  appliedRevision: desiredRevision,
                };
              }
              return {
                commandId,
                status: "accepted",
                errorCode: "",
                errorMessage: "",
                appliedRevision: desiredRevision,
                acceptedAction: actionKey,
                playbackRequestId: cmd.payload?.decisionId ?? commandId,
              };
            }
            case "update_settings": {
              const settings = cmd.payload?.settings;
              const updates: Partial<RuntimeSettingsInfo> = {};
              if (typeof settings?.alwaysOnTop === "boolean") {
                updates.alwaysOnTop = settings.alwaysOnTop;
              }
              if (typeof settings?.scale === "number") {
                updates.scale = settings.scale;
              }
              if (typeof settings?.positionX === "number") {
                updates.positionX = settings.positionX;
              }
              if (typeof settings?.positionY === "number") {
                updates.positionY = settings.positionY;
              }
              if (typeof settings?.screenId === "string") {
                updates.screenId = settings.screenId;
              }
              if (typeof settings?.clickThroughMode === "string") {
                updates.clickThroughMode = normalizeClickThroughMode(
                  settings.clickThroughMode,
                );
              }
              if (typeof settings?.soundEnabled === "boolean") {
                updates.soundEnabled = settings.soundEnabled;
              }
              await this.applyRuntimeSettingsLocal(updates, cmd.settingsRevision ?? 0);
              return {
                commandId,
                status: "applied",
                errorCode: "",
                errorMessage: "",
                appliedRevision: desiredRevision,
                actualState: this.collectPetInstanceSummary(),
              };
            }
            case "recenter":
            case "runtime.command.recenter_once": {
              await this.applyRecenterLocal();
              return {
                commandId,
                status: "applied",
                errorCode: "",
                errorMessage: "",
                appliedRevision: desiredRevision,
                actualState: this.collectPetInstanceSummary(),
              };
            }
            case "runtime.command.reload_release": {
              const reloadInstallationId = cmd.payload?.installation?.installationId
                ?? cmd.installationId
                ?? "";
              if (!reloadInstallationId) {
                return {
                  commandId,
                  status: "rejected",
                  errorCode: "MISSING_INSTALLATION_ID",
                  errorMessage: "reload_release payload missing installationId",
                  appliedRevision: desiredRevision,
                };
              }
              await this.runLifecycleMutation(() =>
                this.enableInstallationInternal(reloadInstallationId, false, true),
              );
              return {
                commandId,
                status: "applied",
                errorCode: "",
                errorMessage: "",
                appliedRevision: desiredRevision,
                actualState: this.collectPetInstanceSummary(),
              };
            }
            case "runtime.command.stop_action": {
              this.stopAction();
              return {
                commandId,
                status: "applied",
                errorCode: "",
                errorMessage: "",
                appliedRevision: desiredRevision,
              };
            }
            case "runtime.command.pause_action": {
              this.pauseAction();
              return {
                commandId,
                status: "applied",
                errorCode: "",
                errorMessage: "",
                appliedRevision: desiredRevision,
              };
            }
            case "runtime.command.resume_action": {
              this.resumeAction();
              return {
                commandId,
                status: "applied",
                errorCode: "",
                errorMessage: "",
                appliedRevision: desiredRevision,
              };
            }
            case "sync": {
              const payload = cmd.payload;
              if (payload?.ensureAbsent && this.activeInstallationId) {
                await this.runLifecycleMutation(() =>
                  this.disableInternal(false, false),
                );
              } else if (
                payload?.desiredPet?.installation?.installationId
              ) {
                const installationId = payload.desiredPet.installation.installationId;
                if (
                  this.activeInstallationId !== installationId ||
                  this.state !== "enabled"
                ) {
                  await this.runLifecycleMutation(() =>
                    this.enableInstallationInternal(installationId, false, false),
                  );
                }
              }
              this.markDesiredRevisionApplied(desiredRevision);
              return {
                commandId,
                status: "applied",
                errorCode: "",
                errorMessage: "",
                appliedRevision: desiredRevision,
                actualState: this.collectPetInstanceSummary(),
              };
            }
            default: {
              return {
                commandId,
                status: "rejected",
                errorCode: "UNKNOWN_COMMAND_TYPE",
                errorMessage: `unknown command type: ${commandType}`,
                appliedRevision: desiredRevision,
              };
            }
          }
        } catch (err) {
          return {
            commandId,
            status: "failed",
            errorCode: "COMMAND_EXECUTION_FAILED",
            errorMessage: this.errorMessage(err),
            appliedRevision: desiredRevision,
          };
        }
      },
    };
  }

  private collectPetInstanceSummary(): PetInstanceSummary | undefined {
    if (!this.activeInstallationId) return undefined;
    const pos = this.windowAdapter?.getPosition();
    const options = this.windowAdapter?.getOptions();
    const win = this.windowAdapter?.getNativeWindow();
    const visible = win ? win.isVisible() : false;
    return {
      petInstanceId: this.activeInstallationId,
      installationId: this.activeInstallationId,
      visible,
      currentActionKey: this.currentActionKey ?? "",
      positionX: pos?.x ?? 0,
      positionY: pos?.y ?? 0,
      screenId: pos?.screenId ?? "",
      scale: options?.scale ?? PET_WINDOW_SCALE_DEFAULT,
    };
  }

  private markDesiredRevisionApplied(revision: number): void {
    if (!Number.isFinite(revision) || revision <= 0) return;
    this.lastAppliedDesiredRevision = Math.max(
      this.lastAppliedDesiredRevision,
      Math.floor(revision),
    );
  }

  private markSettingsRevisionApplied(revision: number): void {
    if (!Number.isFinite(revision) || revision <= 0) return;
    this.lastAppliedSettingsRevision = Math.max(
      this.lastAppliedSettingsRevision,
      Math.floor(revision),
    );
  }

  private captureRuntimeCursor(handler: DesktopRuntimeHandlerV2 | null): void {
    if (!handler) return;
    const cursor = handler.getResumeCursor();
    this.runtimeResumeCursor = {
      lastAppliedDesiredRevision: Math.max(
        cursor.lastAppliedDesiredRevision,
        this.lastAppliedDesiredRevision,
      ),
      lastProcessedCommandSequence: Math.max(
        cursor.lastProcessedCommandSequence,
        this.runtimeResumeCursor.lastProcessedCommandSequence,
      ),
      lastEventSequence: Math.max(
        cursor.lastEventSequence,
        this.runtimeResumeCursor.lastEventSequence,
      ),
      actualStateHash: cursor.actualStateHash ?? this.runtimeResumeCursor.actualStateHash,
    };
  }

  private buildRuntimeStateSnapshot(
    handler: DesktopRuntimeHandlerV2,
    summary: PetInstanceSummary | undefined,
  ): StateSnapshotPayload {
    const playerState = this.actionPlayer?.getState() ?? "idle";
    const playbackStatus = playerState === "loading"
      ? "loading"
      : playerState === "playing"
        ? "playing"
        : playerState === "paused"
          ? "paused"
          : playerState === "stopped"
            ? "stopped"
            : "idle";
    const stableActionKey = this.activeInstallation?.defaultActionKey
      ?? this.loadedInstallation?.defaultAction?.key
      ?? "";
    const currentActionKey = this.currentActionKey ?? stableActionKey;
    const rendererBootstrapped = this.animationIpc?.isBootstrapped() ?? false;
    const rendererRuntimeReady = this.animationIpc?.isRuntimeReady() ?? false;
    const instanceStatus = !this.activeInstallationId
      ? "absent"
      : this.state === "invalid" || this.state === "degraded"
        ? "failed"
        : this.state === "enabled"
          ? rendererRuntimeReady
            ? "ready"
            : "renderer_initializing"
          : this.state === "uninitialized"
            ? "starting"
            : "stopped";
    const windowStatus = summary?.visible
      ? "visible"
      : this.activeInstallationId
        ? "hidden"
        : "absent";
    const rendererStatus = !this.activeInstallationId
      ? "absent"
      : !this.rendererHealthy
        ? "failed"
        : rendererRuntimeReady
          ? "runtime_ready"
          : rendererBootstrapped
            ? "bootstrapped"
            : "absent";

    // Hash the same canonical runtime facts that are projected to the server.
    // Renderer bootstrap/runtime-ready transitions must change the hash even
    // when the high-level PetManager state remains `enabled`.
    const actualState = {
      installationId: this.activeInstallationId ?? "",
      petId: this.activeInstallation?.petId ?? "",
      releaseId: this.activeInstallation?.currentReleaseId ?? "",
      instanceStatus,
      windowStatus,
      rendererStatus,
      playbackStatus,
      stableActionKey,
      currentActionKey,
      playbackInstanceId: this.currentPlaybackId ?? "",
      currentCommandId: this.currentCommandId ?? "",
      visible: summary?.visible ?? false,
      positionX: summary?.positionX ?? 0,
      positionY: summary?.positionY ?? 0,
      screenId: summary?.screenId ?? "",
      scale: summary?.scale ?? PET_WINDOW_SCALE_DEFAULT,
    };
    const actualStateHash = "sha256:" + createHash("sha256")
      .update(JSON.stringify(actualState), "utf8")
      .digest("hex");

    return {
      connectionGeneration: Math.max(1, handler.getConnectionGeneration()),
      eventSequence: handler.getEventSequence() + 1,
      actualStateHash,
      instanceStatus,
      windowStatus,
      rendererStatus,
      playbackStatus,
      appliedDesiredRevision: this.lastAppliedDesiredRevision,
      appliedSettingsRevision: this.lastAppliedSettingsRevision,
      installationId: this.activeInstallationId ?? "",
      petId: this.activeInstallation?.petId ?? "",
      releaseId: this.activeInstallation?.currentReleaseId ?? "",
      stableActionKey,
      currentActionKey,
      playbackInstanceId: this.currentPlaybackId ?? undefined,
      currentCommandId: this.currentCommandId ?? undefined,
      lastProcessedCommandSequence: handler.getLastProcessedCommandSequence(),
      capturedAt: new Date().toISOString(),
    };
  }

  private syncRuntimeState(): void {
    const summary = this.collectPetInstanceSummary();
    this.petInstances = summary ? [summary] : [];
    const handler = this.runtimeHandler;
    if (!handler || !handler.isConnected()) return;

    this.runtimeStateSync = this.runtimeStateSync
      .catch(() => undefined)
      .then(async () => {
        if (this.runtimeHandler !== handler || !handler.isConnected()) return;
        await handler.sendRendererHealth(this.rendererHealthy);
        if (this.runtimeHandler !== handler || !handler.isConnected()) return;
        await handler.sendRendererState(this.buildRuntimeStateSnapshot(handler, summary));
        this.captureRuntimeCursor(handler);
      })
      .catch((err) => {
        console.warn(
          "[DesktopPetManager] 同步 Runtime v2 权威状态失败:",
          this.errorMessage(err),
        );
      });
  }
}

export {
  PET_ACTION_SWITCH_CHANNEL,
  PET_LOAD_ERROR_CHANNEL,
  PET_STATE_CHANNEL,
  DEFAULT_USER_ID,
};

export type {
  LoadedInstallation,
  RuntimeAction,
};
