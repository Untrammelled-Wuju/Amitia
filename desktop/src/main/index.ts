import { app, BrowserWindow, dialog, nativeTheme, shell, Tray } from "electron";
import { join } from "node:path";
import { initLogger, closeLogger } from "./logger";
import { ConfigStore } from "./config-store";
import { registerIpcHandlers } from "./ipc-handlers";
import { createMainWindow, revealMainWindow } from "./window";
import {
  closeSplashWindow,
  revealSplashWindow,
  showSplashWindow,
} from "./splash";
import { createAppTray } from "./tray";
import { DesktopRuntimeManager } from "../runtime/runtime-manager";
import type { DeploymentModeConfig } from "../shared/types";
import type { RuntimeStatus } from "../shared/types";
import {
  ensureDataAndConfig,
  stopCore,
  waitForCoreReadiness,
} from "./core-manager";
import { ensureAmitiaDataDir, getAmitiaDataDir, isDevMode } from "./path-manager";
import { ensureDesktopInstanceID } from "./desktop-identity";
import { registerExtensionProtocol } from "./protocol";
import { registerAppProtocol } from "./app-protocol";
import { registerUpdateManager, waitForStartupCheck } from "./update-manager";
import { ipcMain } from "electron";
import { IPC_CHANNELS } from "../shared/ipc";
import { applyBrandTheme } from "./branding";
import { DesktopPetManager } from "./pet/manager";
import { registerPetIpcHandlers } from "./pet-ipc";
import { ClipboardBridge } from "./clipboard-bridge";
import { DesktopDeploymentLifecycle } from "./deployment-lifecycle";
import { registerRealtimeMediaHandlers } from "./realtime-media";
import { registerRealtimeCallWindowHandlers } from "./realtime-call-window";

let mainWindow: BrowserWindow | null = null;
let tray: Tray | null = null;
let currentConfig: DeploymentModeConfig = { mode: "local" };
let quitting = false;
let coreStopped = false;
let shutdownInProgress = false;
let clipboardBridge: ClipboardBridge | null = null;
let deploymentLifecycle: DesktopDeploymentLifecycle | null = null;
let startupTimer: ReturnType<typeof setTimeout> | null = null;
let startupFailureHandled = false;
let mainWindowReady = false;

const STARTUP_TIMEOUT_MS = 180000;

function isDevModeForProtocol(): boolean {
  return isDevMode();
}

function syncSystemBrandTheme() {
  applyBrandTheme(
    nativeTheme.shouldUseDarkColors ? "dark" : "light",
    mainWindow,
    tray,
  );
}

function notifyStatus(
  runtimeManager: DesktopRuntimeManager,
  state: RuntimeStatus["state"],
  message?: string,
) {
  runtimeManager.setStatus(state, message);
  if (mainWindow && !mainWindow.isDestroyed()) {
    mainWindow.webContents.send(
      IPC_CHANNELS.runtimeStatusChanged,
      runtimeManager.getStatus(),
    );
  }
}

function clearStartupTimer(): void {
  if (startupTimer) {
    clearTimeout(startupTimer);
    startupTimer = null;
  }
}

async function shutdownRuntime(): Promise<void> {
  const task = deploymentLifecycle
    ? deploymentLifecycle.shutdown()
    : stopCore();
  try {
    await Promise.race([
      task.catch(async (error) => {
        console.error("[AmitiaDesktop] 停止核心失败:", error);
        await stopCore().catch((stopError) => {
          console.error("[AmitiaDesktop] 强制停止核心失败:", stopError);
        });
      }),
      new Promise<void>((resolve) => setTimeout(resolve, 15000)),
    ]);
  } catch (error) {
    console.error("[AmitiaDesktop] 关闭运行时异常:", error);
  }
}

async function handleStartupFailure(error: unknown): Promise<void> {
  if (startupFailureHandled) {
    return;
  }
  startupFailureHandled = true;
  clearStartupTimer();
  const failure = error instanceof Error ? error : new Error(String(error));
  console.error("[AmitiaDesktop] 启动失败:", failure);
  closeSplashWindow();

  if (!app.isReady()) {
    app.quit();
    return;
  }

  const logDir = join(getAmitiaDataDir(), "logs");
  let response = 0;
  try {
    const result = await dialog.showMessageBox({
      type: "error",
      title: "Amitia 启动失败",
      message: "Amitia 无法完成启动",
      detail: `${failure.message}\n\n日志目录:\n${logDir}`,
      buttons: ["退出", "打开日志"],
      defaultId: 0,
      cancelId: 0,
      noLink: true,
    });
    response = result.response;
  } catch (dialogError) {
    console.error("[AmitiaDesktop] 启动失败提示展示异常:", dialogError);
  }

  if (response === 1) {
    try {
      await shell.openPath(logDir);
    } catch (openError) {
      console.error("[AmitiaDesktop] 打开日志目录失败:", openError);
    }
  }
  app.quit();
}

async function startDesktop(): Promise<void> {
  initLogger();
  console.log("[AmitiaDesktop] 应用启动, isPackaged:", app.isPackaged);

  showSplashWindow((error) => {
    void handleStartupFailure(error);
  });
  startupTimer = setTimeout(() => {
    void handleStartupFailure(new Error("启动超时，未能在限定时间内显示主界面"));
  }, STARTUP_TIMEOUT_MS);

  let dataDir: string;
  try {
    dataDir = ensureAmitiaDataDir();
    console.log("[AmitiaDesktop] 数据目录:", dataDir);
  } catch (err) {
    throw new Error(
      `数据目录创建失败: ${err instanceof Error ? err.message : String(err)}`,
    );
  }

  await enterMainApp();
  clearStartupTimer();
}

if (process.platform === "win32") {
  app.setAppUserModelId("cn.amitia.desktop");
}

const lock = app.requestSingleInstanceLock();
if (!lock) {
  app.quit();
} else {
  app.on("second-instance", () => {
    if (mainWindow && mainWindowReady && !mainWindow.isDestroyed()) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.show();
      mainWindow.focus();
      return;
    }
    if (!app.isReady() || startupFailureHandled || quitting) {
      return;
    }
    revealSplashWindow();
  });

  void app
    .whenReady()
    .then(() => startDesktop())
    .catch((error) => {
      void handleStartupFailure(error);
    });

  app.on("window-all-closed", () => {
    if (process.platform !== "darwin") {
      if (!tray) app.quit();
    }
  });

  app.on("before-quit", (event) => {
    quitting = true;
    if (coreStopped) {
      return;
    }
    event.preventDefault();
    if (shutdownInProgress) {
      return;
    }
    shutdownInProgress = true;
    void shutdownRuntime().finally(() => {
      coreStopped = true;
      closeLogger();
      app.quit();
    });
  });

  app.on("will-quit", () => {
    clearStartupTimer();
    closeSplashWindow();
  });

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      if (mainWindow) {
        mainWindow.show();
      } else {
        const win = createMainWindow();
        mainWindow = win;
        registerRealtimeMediaHandlers(() => win);
        void revealMainWindow(win);
      }
    }
  });
}

async function enterMainApp(): Promise<void> {
  console.log("[AmitiaDesktop] 启动阶段: 读取部署配置");
  const configStore = new ConfigStore();
  currentConfig = await configStore.getDeploymentConfig();
  const runtimeManager = new DesktopRuntimeManager(currentConfig);
  await runtimeManager.initialize();

  const desktopPetManager = new DesktopPetManager({
    getChatStateWindow: () => mainWindow,
  });
  registerPetIpcHandlers(desktopPetManager);

  console.log("[AmitiaDesktop] 启动阶段: 初始化数据与运行时");
  const ensureResult = ensureDataAndConfig();
  if (!ensureResult.ok) {
    console.warn(
      "[AmitiaDesktop] 数据目录初始化存在缺失",
      ensureResult.errors.join(", "),
    );
  }

  try {
    ensureDesktopInstanceID();
  } catch (error) {
    throw new Error(
      `创建 Desktop Instance 失败: ${
        error instanceof Error ? error.message : String(error)
      }`,
    );
  }

  if (!isDevModeForProtocol()) {
    registerAppProtocol(join(app.getAppPath(), "dist", "renderer"));
  }

  console.log("[AmitiaDesktop] 启动阶段: 创建主窗口");
  const mainWin = createMainWindow();
  mainWindow = mainWin;
  registerRealtimeMediaHandlers(() => mainWin);
  registerRealtimeCallWindowHandlers(() => mainWindow);
  const trayResult = createAppTray(
    mainWindow,
    () => currentConfig,
    configStore,
  );
  tray = trayResult.tray;
  clipboardBridge = new ClipboardBridge(mainWindow);
  clipboardBridge.start();
  syncSystemBrandTheme();

  deploymentLifecycle = new DesktopDeploymentLifecycle(
    {
      configStore,
      runtimeManager,
      getMainWindow: () => mainWindow,
      setExtensionTrayItems: trayResult.setExtensionItems,
      desktopPetManager,
    },
    currentConfig,
  );

  registerIpcHandlers(configStore, runtimeManager, async (config) => {
    currentConfig = config;
    notifyStatus(runtimeManager, "starting");
    try {
      await deploymentLifecycle?.reconcile(config);
      const status = runtimeManager.getStatus();
      if (status.state === "failed") {
        notifyStatus(runtimeManager, "failed", status.message);
      } else {
        notifyStatus(runtimeManager, status.state, status.message);
      }
    } catch (err) {
      console.error("[AmitiaDesktop] reconcile失败:", err);
      notifyStatus(runtimeManager, "failed", String(err));
    }
  }, () => mainWindow);

  const autoLaunch = await configStore.getAutoLaunch();
  app.setLoginItemSettings({ openAtLogin: autoLaunch });
  registerUpdateManager(mainWindow);
  void waitForStartupCheck();

  registerExtensionProtocol(join(getAmitiaDataDir(), "data", "extensions-v2"));

  console.log(`[AmitiaDesktop] Deployment Mode: ${currentConfig.mode}`);
  if (currentConfig.mode === "cloud") {
    console.log(`[AmitiaDesktop] Business Core: ${currentConfig.serverURL}`);
    console.log("[AmitiaDesktop] Bundled Core Profile: device-agent");
    console.log("[AmitiaDesktop] Local Runtime: 动态分配");
  } else {
    console.log("[AmitiaDesktop] Bundled Core Profile: local");
    console.log("[AmitiaDesktop] Local Runtime: 动态分配");
  }

  notifyStatus(runtimeManager, "starting");
  console.log("[AmitiaDesktop] 启动阶段: 启动核心与运行时");
  try {
    await deploymentLifecycle.reconcile(currentConfig);
    const status = runtimeManager.getStatus();
    notifyStatus(runtimeManager, status.state, status.message);
  } catch (err) {
    console.error("[AmitiaDesktop] 初始reconcile失败:", err);
    notifyStatus(runtimeManager, "failed", String(err));
  }

  console.log("[AmitiaDesktop] 启动阶段: 等待核心就绪");
  const coreReady = await waitForCoreReadiness();
  if (!coreReady) {
    console.warn("[AmitiaDesktop] 核心未就绪, 直接显示主界面");
  }

  mainWindow.on("close", (event) => {
    if (!quitting) {
      event.preventDefault();
      mainWindow?.hide();
    }
  });

  if (quitting || startupFailureHandled) {
    return;
  }

  console.log("[AmitiaDesktop] 启动阶段: 显示主窗口");
  try {
    await revealMainWindow(mainWin);
    mainWindowReady = true;
  } finally {
    closeSplashWindow();
    clearStartupTimer();
  }
}

ipcMain.handle("app:enter-main", async () => {
  await enterMainApp();
});

ipcMain.handle("app:get-data-dir", () => {
  return getAmitiaDataDir();
});

ipcMain.handle("app:get-version", () => {
  return app.getVersion();
});

nativeTheme.on("updated", syncSystemBrandTheme);
