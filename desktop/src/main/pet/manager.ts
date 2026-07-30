import {
  app,
  BrowserWindow,
  ipcMain,
  powerMonitor,
  screen,
  type Display,
} from "electron";
import { createHash } from "node:crypto";
import { readFile, stat, readdir } from "node:fs/promises";
import { isAbsolute, join, relative, sep } from "node:path";
import http from "node:http";
import { URL } from "node:url";
import { getAmitiaDataDir } from "../path-manager";
import { ResourceLoader } from "./resource-loader";
import type { LoadedInstallation, RuntimeAction } from "./resource-loader";
import { ResourceCache } from "./resource-cache";
import type { CachedFrame } from "./resource-cache";
import { DesktopPetWindowAdapter } from "./window-adapter";
import { ActionPlayer } from "./action-player";
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
import { RuntimeBridgeClient } from "./runtime-bridge-client";
import type {
  RuntimeBridgeConfig,
  RuntimeBridgeCallbacks,
  PetInstanceSummary,
  CommandResultPayload,
  SpawnPayload,
  SyncPayload,
  RuntimeMessage,
  WelcomePayload,
} from "./runtime-bridge-client";
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
const HEALTH_CHECK_PATH = "/api/health";
const DEFAULT_USER_ID = "default";
const DEFAULT_ALPHA_THRESHOLD = 10;

const PET_FRAME_UPDATE_CHANNEL = "pet:frame-update";
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
const RECOVERY_REASON_RENDER_RELOAD = "render-process-gone";
const RECOVERY_REASON_WINDOW_CLOSED = "window-closed";
const RECOVERY_REASON_GPU_CRASHED = "gpu-process-crashed";
const RECOVERY_REASON_POWER_RESUME = "power-resume";
const RECOVERY_REASON_DISPLAY_CHANGED = "display-changed";

const RUNTIME_BRIDGE_WS_PATH = "/internal/desktop-pet/runtime/ws";
const RUNTIME_BRIDGE_TOKEN_PATH = "/api/desktop-pets/runtime/bootstrap-token";
const BRIDGE_RECONNECT_DELAY_MS = 2000;

const HASH_EXCLUDED_FILES = new Set(["metadata.json", "integrity.json"]);

export type PetManagerState =
  | "uninitialized"
  | "ready"
  | "enabled"
  | "disabled"
  | "invalid";

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
}

export interface RuntimeSettingsInfo {
  installationId: string;
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

export interface PetFrameUpdatePayload {
  actionKey: string;
  frameIndex: number;
  dataURL: string;
  width: number;
  height: number;
  loopType: string;
  fps: number;
  anchor?: { x: number; y: number };
}

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
}

interface RuntimeSettingsApiPayload {
  installationId: string;
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
  };
}

function mapRuntimeSettingsPayload(payload: RuntimeSettingsApiPayload): RuntimeSettingsInfo {
  return {
    installationId: payload.installationId,
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

  private authToken: string | null = null;
  private state: PetManagerState = "uninitialized";
  private initialized = false;
  private initializing = false;
  private activeInstallationId: string | null = null;
  private activeInstallation: InstallationInfo | null = null;
  private activeSettings: RuntimeSettingsInfo | null = null;
  private loadedInstallation: LoadedInstallation | null = null;

  private windowAdapter: DesktopPetWindowAdapter | null = null;
  private actionPlayer: ActionPlayer | null = null;
  private scheduler: DesktopPetActionScheduler | null = null;
  private idleController: IdleController | null = null;
  private eventBridge: DesktopPetEventBridge | null = null;
  private chatStateBridge: ChatStateBridge | null = null;
  private dragController: DragController | null = null;
  private clickThroughController: ClickThroughController | null = null;

  private bridgeClient: RuntimeBridgeClient | null = null;
  private bridgeToken: string | null = null;
  private bridgeReconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private bridgeStarted = false;
  private currentActionKey: string | null = null;

  private recoveryHandlersAttached = false;
  private recoveryInProgress = false;
  private recoveryDebounceTimer: ReturnType<typeof setTimeout> | null = null;
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
    this.resourceLoader = opts.resourceLoader ?? new ResourceLoader();
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
    if (this.initialized || this.initializing) {
      return;
    }
    this.initializing = true;
    try {
      await this.waitCoreReady();
      this.startBridge();
      await this.restoreActiveInstallation();
      this.initialized = true;
      if (this.state === "uninitialized") {
        this.setState("ready", null, "初始化完成但无活跃桌宠");
      }
    } catch (err) {
      console.error("[DesktopPetManager] 初始化失败:", err);
      this.setState("ready", null, `初始化失败: ${this.errorMessage(err)}`);
      this.initialized = true;
    } finally {
      this.initializing = false;
    }
  }

  async enableInstallation(installationId: string): Promise<void> {
    if (!installationId) {
      throw new Error("INSTALLATION_ID_REQUIRED");
    }
    await this.ensureInitialized();
    if (this.activeInstallationId === installationId && this.state === "enabled") {
      return;
    }
    if (this.activeInstallationId && this.activeInstallationId !== installationId) {
      await this.disableInternal(false);
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

    try {
      await this.callEnableApi(installationId);
      await this.startRuntime(installation, settings, loaded);
      this.setupRecoveryHandlers();
      this.petLogger.logEnable(installationId, installation.name);
      this.setState("enabled", installationId);
    } catch (err) {
      this.petLogger.logRuntimeCrash("enableInstallation", err);
      try {
        await this.stopRuntime();
      } catch (stopErr) {
        this.petLogger.logRuntimeCrash("enableInstallation.stopRuntime", stopErr);
      }
      this.activeInstallationId = null;
      this.activeInstallation = null;
      this.activeSettings = null;
      this.loadedInstallation = null;
      throw err;
    }
  }

  async disableInstallation(): Promise<void> {
    await this.disableInternal(true);
  }

  async switchInstallation(installationId: string): Promise<void> {
    if (!installationId) {
      throw new Error("INSTALLATION_ID_REQUIRED");
    }
    await this.ensureInitialized();
    if (
      this.activeInstallationId === installationId &&
      this.state === "enabled"
    ) {
      return;
    }
    await this.disableInternal(false);
    await this.enableInstallation(installationId);
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

  async recenter(): Promise<void> {
    if (this.state !== "enabled" || !this.windowAdapter) {
      throw new Error("PET_NOT_ENABLED");
    }
    if (this.activeInstallationId) {
      await this.callRecenterApi(this.activeInstallationId);
    }
    const primary = this.windowAdapter.listScreens().find((s) => s.isPrimary);
    const target = primary ?? this.windowAdapter.listScreens()[0];
    if (target) {
      const width = this.windowAdapter.getOptions().canvasWidth;
      const height = this.windowAdapter.getOptions().canvasHeight;
      const x = target.workArea.x + target.workArea.width - width - 40;
      const y = target.workArea.y + target.workArea.height - height - 40;
      await this.windowAdapter.setPosition(x, y, target.id);
    }
    await this.persistRuntimePosition();
  }

  async updateSettings(settings: Partial<RuntimeSettingsInfo>): Promise<void> {
    if (!this.activeInstallationId) {
      throw new Error("PET_NOT_ENABLED");
    }
    const patch = this.buildSettingsPatch(settings);
    if (Object.keys(patch).length === 0) {
      return;
    }
    await this.callUpdateSettingsApi(this.activeInstallationId, patch);
    const merged = this.mergeSettings(this.activeSettings, settings);
    this.activeSettings = merged;
    if (this.windowAdapter && merged) {
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
    if (this.idleController && merged) {
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
    const payload = await this.request<InstallationApiPayload[]>("GET", path);
    if (!Array.isArray(payload)) {
      return [];
    }
    return payload.map(mapInstallationPayload);
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
      return;
    }
    await this.initialize();
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
    const path = `${API_BASE_PATH}/installations?active=true&userId=${encodeURIComponent(this.userId)}`;
    let installations: InstallationApiPayload[];
    try {
      const data = await this.request<InstallationApiPayload[]>("GET", path);
      installations = Array.isArray(data) ? data : [];
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 查询活跃安装失败, 跳过桌宠恢复:",
        this.errorMessage(err),
      );
      return;
    }
    if (installations.length === 0) {
      return;
    }
    const active = installations[0];
    if (!active) return;
    if (active.status !== INSTALLATION_STATUS_ENABLED) {
      console.warn(
        `[DesktopPetManager] 活跃安装状态非 enabled (实际 ${active.status}), 跳过恢复`,
      );
      return;
    }
    try {
      const installation = mapInstallationPayload(active);
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
      await this.startRuntime(installation, settings, loaded);
      this.setupRecoveryHandlers();
      this.petLogger.logEnable(installation.id, installation.name);
      this.petLogger.logWindowRecovered("app-startup", installation.id);
      this.setState("enabled", installation.id, "恢复完成");
    } catch (err) {
      this.petLogger.logRuntimeCrash("restoreActiveInstallation", err);
      this.petLogger.logInstallFailed(active.id, this.errorMessage(err));
      console.error(
        "[DesktopPetManager] 恢复活跃桌宠失败:",
        this.errorMessage(err),
      );
      this.setState(
        "ready",
        active.id,
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
      const loaded = await this.resourceLoader.loadInstallation(
        installPath,
        manifestPath,
      );
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

    const windowAdapter = new DesktopPetWindowAdapter(windowOptions);
    const win = await windowAdapter.create();

    const windowCloseListener = (): void => {
      this.petLogger.logWindowRecovered(
        RECOVERY_REASON_WINDOW_CLOSED,
        this.activeInstallationId ?? undefined,
      );
      this.scheduleRecovery(RECOVERY_REASON_WINDOW_CLOSED);
    };
    const windowCrashedListener = (): void => {
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

    const actionPlayer = new ActionPlayer({
      onFrameChange: (actionKey, frameIndex) =>
        this.handleFrameChange(actionKey, frameIndex),
      onActionSwitch: (newKey, oldKey) =>
        this.handleActionSwitch(newKey, oldKey),
      onError: (err) =>
        console.error("[DesktopPetManager] ActionPlayer 错误:", err),
    });
    actionPlayer.attachLoaded(loaded);

    const scheduler = new DesktopPetActionScheduler(actionPlayer, {
      onEvent: (event, request, action) =>
        this.handleSchedulerEvent(event, request, action),
    });
    scheduler.attachLoaded(loaded);

    const idleController = new IdleController(actionPlayer, scheduler, {
      enabled: settings?.idleEnabled ?? true,
      minIntervalSeconds: settings?.idleIntervalMinSeconds ?? 30,
      maxIntervalSeconds: settings?.idleIntervalMaxSeconds ?? 120,
      maxRepeatCount: 2,
      recentActionWeight: 0.3,
    });
    idleController.attachLoaded(loaded);

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
    eventBridge.attach(win);

    const chatStateBridge = new ChatStateBridge(scheduler);
    chatStateBridge.attachLoaded(loaded);
    chatStateBridge.attachPetWindow(win);

    this.windowAdapter = windowAdapter;
    this.actionPlayer = actionPlayer;
    this.scheduler = scheduler;
    this.idleController = idleController;
    this.clickThroughController = clickThroughController;
    this.dragController = dragController;
    this.eventBridge = eventBridge;
    this.chatStateBridge = chatStateBridge;
    this.loadedInstallation = loaded;

    this.registerChatStateIpc();

    try {
      await this.resourceCache.preloadDefaultIdle(loaded);
      await this.resourceCache.preloadClickActions(loaded, [
        "clicked",
        "double_clicked",
        "hovered",
      ]);
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 预加载资源失败, 将按需加载:",
        this.errorMessage(err),
      );
    }

    idleController.start();
  }

  private async stopRuntime(): Promise<void> {
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
    this.scheduler = null;
    this.idleController = null;
    this.eventBridge = null;
    this.chatStateBridge = null;
    this.dragController = null;
    this.clickThroughController = null;
    this.loadedInstallation = null;
    this.currentActionKey = null;
  }

  private async disableInternal(notifyBackend: boolean): Promise<void> {
    if (!this.activeInstallationId) {
      return;
    }
    const installationId = this.activeInstallationId;
    this.teardownRecoveryHandlers();
    try {
      await this.persistRuntimePosition();
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 停用前持久化位置失败:",
        this.errorMessage(err),
      );
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

  private async handleFrameChange(
    actionKey: string,
    frameIndex: number,
  ): Promise<void> {
    const loaded = this.loadedInstallation;
    const win = this.windowAdapter?.getNativeWindow();
    if (!loaded || !win || win.isDestroyed()) return;

    const action = loaded.actions.get(actionKey);
    if (!action || !action.available) return;

    let frame: CachedFrame | null = null;
    try {
      frame = await this.resourceCache.getFrame(loaded, actionKey, frameIndex);
    } catch (err) {
      const message = this.errorMessage(err);
      console.warn(
        "[DesktopPetManager] 加载帧失败:",
        actionKey,
        frameIndex,
        message,
      );
      this.petLogger.logActionLoadFailed(
        actionKey,
        `frame=${frameIndex} error=${message}`,
      );
      return;
    }
    if (!frame) return;

    if (this.clickThroughController && frame.alphaData) {
      try {
        this.clickThroughController.updateFrame(
          frame.width,
          frame.height,
          frame.alphaData,
        );
      } catch (err) {
        console.warn(
          "[DesktopPetManager] 更新点击穿透帧失败:",
          this.errorMessage(err),
        );
      }
    }

    if (win.isDestroyed()) return;
    const payload: PetFrameUpdatePayload = {
      actionKey,
      frameIndex,
      dataURL: frame.dataURL,
      width: frame.width,
      height: frame.height,
      loopType: action.loopType,
      fps: action.fps,
      anchor: action.anchor,
    };
    try {
      win.webContents.send(PET_FRAME_UPDATE_CHANNEL, payload);
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 发送帧更新失败:",
        this.errorMessage(err),
      );
    }
  }

  private handleActionSwitch(newKey: string, oldKey: string | null): void {
    this.currentActionKey = newKey;
    this.petLogger.logActionSwitch(newKey, oldKey, "scheduler");
    const win = this.windowAdapter?.getNativeWindow();
    if (!win || win.isDestroyed()) return;
    const payload: PetActionSwitchPayload = {
      actionKey: newKey,
      previousActionKey: oldKey,
      source: "scheduler",
    };
    try {
      win.webContents.send(PET_ACTION_SWITCH_CHANNEL, payload);
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 发送动作切换失败:",
        this.errorMessage(err),
      );
    }
    this.sendBridgeEvent("action_switch", newKey);
  }

  private handleSchedulerEvent(
    event: SchedulerEvent,
    request: DesktopPetActionRequest,
    action: RuntimeAction | null,
  ): void {
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

  private handleDragEvent(event: DragEvent, _state: DragState): void {
    if (event === "drag-start") {
      this.petLogger.logDragStart(this.activeInstallationId ?? undefined);
      this.sendBridgeEvent("drag_start", this.activeInstallationId ?? "");
    } else if (event === "drag-end") {
      this.petLogger.logDragEnd(this.activeInstallationId ?? undefined);
      this.sendBridgeEvent("drag_end", this.activeInstallationId ?? "");
      void this.persistRuntimePosition();
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
      await this.callUpdateSettingsApi(
        this.activeInstallationId,
        this.buildSettingsPatch(patch),
      );
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
    this.syncBridgeState();
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
  ): Promise<void> {
    const path = `${API_BASE_PATH}/installations/${encodeURIComponent(installationId)}/settings`;
    await this.request("PATCH", path, patch);
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
      loaded = await this.resourceLoader.loadInstallation(installPath, manifestPath);
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

    const hashMatch = await this.verifyPackageHash(installation, installPath);
    if (!hashMatch.ok) {
      return {
        corrupted: true,
        errorCode: CORRUPTION_PACKAGE_HASH_MISMATCH,
        detail: hashMatch.detail,
      };
    }

    return { corrupted: false, errorCode: "", detail: "" };
  }

  private async verifyPackageHash(
    installation: InstallationInfo,
    installPath: string,
  ): Promise<{ ok: boolean; detail: string }> {
    if (!installation.packageHash) {
      return { ok: true, detail: "" };
    }
    try {
      const recomputed = await this.recomputePackageHash(installPath);
      if (!recomputed) {
        return { ok: true, detail: "hash 计算跳过" };
      }
      if (recomputed !== installation.packageHash) {
        return {
          ok: false,
          detail: `packageHash 不一致 expected=${installation.packageHash.slice(0, 12)}... actual=${recomputed.slice(0, 12)}...`,
        };
      }
    } catch (err) {
      return {
        ok: false,
        detail: `hash 重算失败: ${this.errorMessage(err)}`,
      };
    }
    return { ok: true, detail: "" };
  }

  private async recomputePackageHash(installPath: string): Promise<string> {
    const files = await this.listInstallationFilesForHash(installPath);
    files.sort();
    const hasher = createHash("sha256");
    for (const relPath of files) {
      hasher.update(relPath);
      hasher.update("\0");
      const absPath = join(installPath, relPath);
      const content = await readFile(absPath);
      hasher.update(content);
      hasher.update("\0");
    }
    return hasher.digest("hex");
  }

  private async listInstallationFilesForHash(
    root: string,
  ): Promise<string[]> {
    const result: string[] = [];
    const stack: string[] = [root];
    while (stack.length > 0) {
      const current = stack.pop() as string;
      let entries: import("node:fs").Dirent[];
      try {
        entries = await readdir(current, { withFileTypes: true });
      } catch {
        continue;
      }
      for (const entry of entries) {
        const entryName = entry.name;
        if (entryName.startsWith(".")) continue;
        const absPath = join(current, entryName);
        if (entry.isDirectory()) {
          stack.push(absPath);
          continue;
        }
        if (!entry.isFile()) continue;
        const rel = relative(root, absPath).split(sep).join("/");
        if (HASH_EXCLUDED_FILES.has(rel)) continue;
        result.push(rel);
      }
    }
    return result;
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
    if (this.windowAdapter) {
      if (this.recoveryWindowCloseListener) {
        this.windowAdapter.on("close", this.recoveryWindowCloseListener);
      }
      if (this.recoveryWindowCrashedListener) {
        this.windowAdapter.on("crashed", this.recoveryWindowCrashedListener);
      }
    }
  }

  private teardownRecoveryHandlers(): void {
    if (!this.recoveryHandlersAttached) return;
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
    if (this.recoveryDebounceTimer) {
      clearTimeout(this.recoveryDebounceTimer);
      this.recoveryDebounceTimer = null;
    }
    this.recoveryInProgress = false;
  }

  private scheduleRecovery(reason: RecoveryReason): void {
    if (!this.activeInstallationId || this.state !== "enabled") {
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
      this.petLogger.logWindowRecovered(reason, installationId);
      this.setState("enabled", installationId, `恢复完成 reason=${reason}`);
    } catch (err) {
      this.petLogger.logRuntimeCrash(`recoverRuntime:${reason}`, err);
      console.error(
        "[DesktopPetManager] 运行时恢复失败:",
        this.errorMessage(err),
      );
      this.setState(
        "ready",
        installationId,
        `恢复失败: ${this.errorMessage(err)}`,
      );
    } finally {
      this.recoveryInProgress = false;
    }
  }

  private buildSettingsPatch(
    settings: Partial<RuntimeSettingsInfo>,
  ): Record<string, unknown> {
    const patch: Record<string, unknown> = {};
    if (typeof settings.alwaysOnTop === "boolean") {
      patch.always_on_top = settings.alwaysOnTop ? 1 : 0;
    }
    if (typeof settings.launchOnStartup === "boolean") {
      patch.launch_on_startup = settings.launchOnStartup ? 1 : 0;
    }
    if (typeof settings.scale === "number" && Number.isFinite(settings.scale)) {
      patch.scale = settings.scale;
    }
    if (typeof settings.positionX === "number") {
      patch.position_x = Math.round(settings.positionX);
    }
    if (typeof settings.positionY === "number") {
      patch.position_y = Math.round(settings.positionY);
    }
    if (typeof settings.screenId === "string") {
      patch.screen_id = settings.screenId;
    }
    if (typeof settings.idleEnabled === "boolean") {
      patch.idle_enabled = settings.idleEnabled ? 1 : 0;
    }
    if (typeof settings.idleIntervalMinSeconds === "number") {
      patch.idle_interval_min_seconds = settings.idleIntervalMinSeconds;
    }
    if (typeof settings.idleIntervalMaxSeconds === "number") {
      patch.idle_interval_max_seconds = settings.idleIntervalMaxSeconds;
    }
    if (typeof settings.clickThroughMode !== "undefined") {
      patch.click_through_mode = clickThroughModeToApiValue(
        settings.clickThroughMode,
      );
    }
    if (typeof settings.soundEnabled === "boolean") {
      patch.sound_enabled = settings.soundEnabled ? 1 : 0;
    }
    return patch;
  }

  private mergeSettings(
    base: RuntimeSettingsInfo | null,
    updates: Partial<RuntimeSettingsInfo>,
  ): RuntimeSettingsInfo {
    const fallback: RuntimeSettingsInfo = {
      installationId: this.activeInstallationId ?? "",
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
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      Accept: "application/json",
    };
    if (this.authToken) {
      headers.Authorization = `Bearer ${this.authToken}`;
    }
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
            if (res.statusCode === undefined) {
              reject(new Error("无 HTTP 状态码"));
              return;
            }
            if (res.statusCode >= 200 && res.statusCode < 300) {
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
                  if (parsed.code !== 200) {
                    reject(
                      new Error(
                        `API 错误 ${parsed.code}: ${parsed.msg ?? "unknown"}`,
                      ),
                    );
                    return;
                  }
                  resolve(parsed.data as T);
                  return;
                }
                resolve(parsed as T);
              } catch (err) {
                reject(new Error(`JSON 解析失败: ${this.errorMessage(err)}`));
              }
              return;
            }
            reject(new Error(`HTTP ${res.statusCode}: ${data}`));
          });
        },
      );
      req.on("error", (err: Error) => reject(err));
      req.on("timeout", () => {
        req.destroy(new Error("请求超时"));
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

  private async fetchBootstrapToken(): Promise<string | null> {
    try {
      const data = await this.request<{ token: string; endpoint: string }>(
        "GET",
        RUNTIME_BRIDGE_TOKEN_PATH,
      );
      return data?.token ?? null;
    } catch (err) {
      console.warn(
        "[DesktopPetManager] 获取 runtime bridge token 失败:",
        this.errorMessage(err),
      );
      return null;
    }
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
    if (this.bridgeClient) {
      try {
        this.bridgeClient.disconnect();
      } catch {
        void 0;
      }
      this.bridgeClient = null;
    }
    this.bridgeToken = null;
  }

  private async connectBridge(): Promise<void> {
    if (!this.bridgeStarted) return;
    if (!this.bridgeToken) {
      this.bridgeToken = await this.fetchBootstrapToken();
      if (!this.bridgeToken) {
        this.scheduleBridgeReconnect();
        return;
      }
    }
    const endpoint = `ws://${this.coreHost}:${this.corePort}${RUNTIME_BRIDGE_WS_PATH}`;
    const config: RuntimeBridgeConfig = {
      endpoint,
      token: this.bridgeToken,
      appVersion: app.getVersion(),
    };
    const callbacks = this.buildBridgeCallbacks();
    this.bridgeClient = new RuntimeBridgeClient(config, callbacks);
    this.bridgeClient.connect();
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

  private buildBridgeCallbacks(): RuntimeBridgeCallbacks {
    return {
      onConnected: (welcome: WelcomePayload) => {
        console.log(
          "[DesktopPetManager] runtime bridge connected session=",
          welcome.sessionId,
        );
        this.syncBridgeState();
      },
      onDisconnected: (reason: string) => {
        console.warn(
          "[DesktopPetManager] runtime bridge disconnected:",
          reason,
        );
        if (this.bridgeStarted) {
          this.scheduleBridgeReconnect();
        }
      },
      onError: (err: Error) => {
        console.warn("[DesktopPetManager] runtime bridge error:", err.message);
      },
      onSpawn: async (
        _msg: RuntimeMessage,
        payload: SpawnPayload,
      ): Promise<CommandResultPayload> => {
        try {
          const installationId = payload.installation?.installationId;
          if (!installationId) {
            return {
              commandId: _msg.commandId || "",
              status: "rejected",
              errorCode: "MISSING_INSTALLATION_ID",
              errorMessage: "spawn payload missing installationId",
              appliedRevision: payload.desiredRevision || 0,
            };
          }
          await this.enableInstallation(installationId);
          this.bridgeClient?.setLastAppliedDesiredRevision(
            payload.desiredRevision || 0,
          );
          return {
            commandId: _msg.commandId || "",
            status: "applied",
            errorCode: "",
            errorMessage: "",
            appliedRevision: payload.desiredRevision || 0,
            actualState: this.collectPetInstanceSummary(),
          };
        } catch (err) {
          return {
            commandId: _msg.commandId || "",
            status: "failed",
            errorCode: "SPAWN_FAILED",
            errorMessage: this.errorMessage(err),
            appliedRevision: payload.desiredRevision || 0,
          };
        }
      },
      onDestroy: async (
        msg: RuntimeMessage,
        desiredRevision: number,
        _reason: string,
      ): Promise<CommandResultPayload> => {
        try {
          await this.disableInstallation();
          this.bridgeClient?.setLastAppliedDesiredRevision(desiredRevision);
          return {
            commandId: msg.commandId || "",
            status: "applied",
            errorCode: "",
            errorMessage: "",
            appliedRevision: desiredRevision,
          };
        } catch (err) {
          return {
            commandId: msg.commandId || "",
            status: "failed",
            errorCode: "DESTROY_FAILED",
            errorMessage: this.errorMessage(err),
            appliedRevision: desiredRevision,
          };
        }
      },
      onShow: async (
        msg: RuntimeMessage,
        desiredRevision: number,
      ): Promise<CommandResultPayload> => {
        try {
          const win = this.windowAdapter?.getNativeWindow();
          if (win && !win.isDestroyed()) {
            win.show();
          }
          this.bridgeClient?.setLastAppliedDesiredRevision(desiredRevision);
          return {
            commandId: msg.commandId || "",
            status: "applied",
            errorCode: "",
            errorMessage: "",
            appliedRevision: desiredRevision,
            actualState: this.collectPetInstanceSummary(),
          };
        } catch (err) {
          return {
            commandId: msg.commandId || "",
            status: "failed",
            errorCode: "SHOW_FAILED",
            errorMessage: this.errorMessage(err),
            appliedRevision: desiredRevision,
          };
        }
      },
      onHide: async (
        msg: RuntimeMessage,
        desiredRevision: number,
      ): Promise<CommandResultPayload> => {
        try {
          const win = this.windowAdapter?.getNativeWindow();
          if (win && !win.isDestroyed()) {
            win.hide();
          }
          this.bridgeClient?.setLastAppliedDesiredRevision(desiredRevision);
          return {
            commandId: msg.commandId || "",
            status: "applied",
            errorCode: "",
            errorMessage: "",
            appliedRevision: desiredRevision,
            actualState: this.collectPetInstanceSummary(),
          };
        } catch (err) {
          return {
            commandId: msg.commandId || "",
            status: "failed",
            errorCode: "HIDE_FAILED",
            errorMessage: this.errorMessage(err),
            appliedRevision: desiredRevision,
          };
        }
      },
      onPlayAction: async (
        msg: RuntimeMessage,
        actionKey: string,
        _actionSpecHash: string,
      ): Promise<CommandResultPayload> => {
        try {
          await this.playAction(actionKey);
          return {
            commandId: msg.commandId || "",
            status: "applied",
            errorCode: "",
            errorMessage: "",
            appliedRevision: this.bridgeClient?.getRuntimeId() ? 0 : 0,
            acceptedAction: actionKey,
          };
        } catch (err) {
          return {
            commandId: msg.commandId || "",
            status: "failed",
            errorCode: "PLAY_ACTION_FAILED",
            errorMessage: this.errorMessage(err),
            appliedRevision: 0,
          };
        }
      },
      onUpdateSettings: async (
        msg: RuntimeMessage,
        settingsRevision: number,
        settings: SpawnPayload["settings"],
      ): Promise<CommandResultPayload> => {
        try {
          const updates: Partial<RuntimeSettingsInfo> = {};
          if (typeof settings.alwaysOnTop === "boolean") {
            updates.alwaysOnTop = settings.alwaysOnTop;
          }
          if (typeof settings.scale === "number") {
            updates.scale = settings.scale;
          }
          if (typeof settings.positionX === "number") {
            updates.positionX = settings.positionX;
          }
          if (typeof settings.positionY === "number") {
            updates.positionY = settings.positionY;
          }
          if (typeof settings.screenId === "string") {
            updates.screenId = settings.screenId;
          }
          if (typeof settings.clickThroughMode === "string") {
            updates.clickThroughMode = normalizeClickThroughMode(
              settings.clickThroughMode,
            );
          }
          if (typeof settings.soundEnabled === "boolean") {
            updates.soundEnabled = settings.soundEnabled;
          }
          await this.updateSettings(updates);
          return {
            commandId: msg.commandId || "",
            status: "applied",
            errorCode: "",
            errorMessage: "",
            appliedRevision: settingsRevision,
            actualState: this.collectPetInstanceSummary(),
          };
        } catch (err) {
          return {
            commandId: msg.commandId || "",
            status: "failed",
            errorCode: "UPDATE_SETTINGS_FAILED",
            errorMessage: this.errorMessage(err),
            appliedRevision: settingsRevision,
          };
        }
      },
      onRecenter: async (
        msg: RuntimeMessage,
        settingsRevision: number,
        _screenId: string,
      ): Promise<CommandResultPayload> => {
        try {
          await this.recenter();
          return {
            commandId: msg.commandId || "",
            status: "applied",
            errorCode: "",
            errorMessage: "",
            appliedRevision: settingsRevision,
            actualState: this.collectPetInstanceSummary(),
          };
        } catch (err) {
          return {
            commandId: msg.commandId || "",
            status: "failed",
            errorCode: "RECENTER_FAILED",
            errorMessage: this.errorMessage(err),
            appliedRevision: settingsRevision,
          };
        }
      },
      onSync: async (
        _msg: RuntimeMessage,
        payload: SyncPayload,
      ): Promise<CommandResultPayload> => {
        try {
          if (payload.ensureAbsent && this.activeInstallationId) {
            await this.disableInstallation();
          } else if (
            payload.desiredPet &&
            payload.desiredPet.installation?.installationId
          ) {
            const installationId =
              payload.desiredPet.installation.installationId;
            if (
              this.activeInstallationId !== installationId ||
              this.state !== "enabled"
            ) {
              await this.enableInstallation(installationId);
            }
          }
          this.bridgeClient?.setLastAppliedDesiredRevision(
            payload.desiredRevision || 0,
          );
          return {
            commandId: "sync_" + (this.bridgeClient?.getSessionId() || ""),
            status: "applied",
            errorCode: "",
            errorMessage: "",
            appliedRevision: payload.desiredRevision || 0,
            actualState: this.collectPetInstanceSummary(),
          };
        } catch (err) {
          return {
            commandId: "sync_" + (this.bridgeClient?.getSessionId() || ""),
            status: "failed",
            errorCode: "SYNC_FAILED",
            errorMessage: this.errorMessage(err),
            appliedRevision: payload.desiredRevision || 0,
          };
        }
      },
      onStateProbe: (_msg: RuntimeMessage): PetInstanceSummary[] => {
        const summary = this.collectPetInstanceSummary();
        return summary ? [summary] : [];
      },
      onShutdown: (
        _msg: RuntimeMessage,
        _deadline: string,
        _reason: string,
      ): void => {
        console.log("[DesktopPetManager] runtime bridge shutdown received");
        void this.shutdown();
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

  private syncBridgeState(): void {
    if (!this.bridgeClient) return;
    const summary = this.collectPetInstanceSummary();
    this.bridgeClient.updatePetInstances(summary ? [summary] : []);
  }

  private sendBridgeEvent(eventType: string, petInstanceId: string): void {
    if (!this.bridgeClient || !this.bridgeClient.isConnected()) return;
    this.bridgeClient.sendEvent(eventType, petInstanceId, {
      actionKey: this.currentActionKey,
      state: this.state,
    });
    this.syncBridgeState();
  }
}

export {
  PET_FRAME_UPDATE_CHANNEL,
  PET_ACTION_SWITCH_CHANNEL,
  PET_LOAD_ERROR_CHANNEL,
  PET_STATE_CHANNEL,
  DEFAULT_USER_ID,
};

export type {
  LoadedInstallation,
  RuntimeAction,
};
