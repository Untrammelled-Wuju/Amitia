import { app, BrowserWindow, nativeTheme, Tray } from "electron";
import { join } from "node:path";
import { initLogger, closeLogger } from "./logger";
import { ConfigStore } from "./config-store";
import { registerIpcHandlers } from "./ipc-handlers";
import { createMainWindow } from "./window";
import { createAppTray } from "./tray";
import { DesktopRuntimeManager } from "../runtime/runtime-manager";
import type { DeploymentModeConfig } from "../shared/types";
import type { RuntimeStatus } from "../shared/types";
import {
  ensureDataAndConfig,
} from "./core-manager";
import { ensureAmitiaDataDir, getAmitiaDataDir } from "./path-manager";
import { ensureDesktopInstanceID } from "./desktop-identity";
import { registerExtensionProtocol } from "./protocol";
import { registerUpdateManager, waitForStartupCheck } from "./update-manager";
import { ipcMain } from "electron";
import { IPC_CHANNELS } from "../shared/ipc";
import { applyBrandTheme } from "./branding";
import { DesktopPetManager } from "./pet/manager";
import { registerPetIpcHandlers } from "./pet-ipc";
import { ClipboardBridge } from "./clipboard-bridge";
import { DesktopDeploymentLifecycle } from "./deployment-lifecycle";

let mainWindow: BrowserWindow | null = null;
let tray: Tray | null = null;
let currentConfig: DeploymentModeConfig = { mode: "local" };
let quitting = false;
let coreStopped = false;
let shutdownInProgress = false;
let clipboardBridge: ClipboardBridge | null = null;
let deploymentLifecycle: DesktopDeploymentLifecycle | null = null;

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

const lock = app.requestSingleInstanceLock();
if (!lock) {
  app.quit();
} else {
  app.on("second-instance", () => {
    if (!mainWindow) return;
    if (mainWindow.isMinimized()) mainWindow.restore();
    mainWindow.show();
    mainWindow.focus();
  });

  void app.whenReady().then(async () => {
    initLogger();
    console.log("[AmitiaDesktop] 应用启动, isPackaged:", app.isPackaged);

    let dataDir: string;
    try {
      dataDir = ensureAmitiaDataDir();
      console.log("[AmitiaDesktop] 数据目录:", dataDir);
    } catch (err) {
      console.error("[AmitiaDesktop] 数据目录创建失败:", err);
      app.quit();
      return;
    }

    await enterMainApp();
  });

  app.on("window-all-closed", () => {
    if (process.platform !== "darwin") {
      if (!tray) app.quit();
    }
  });

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      if (mainWindow) {
        mainWindow.show();
      } else {
        mainWindow = createMainWindow();
      }
    }
  });
}

async function enterMainApp(): Promise<void> {
  const configStore = new ConfigStore();
  currentConfig = await configStore.getDeploymentConfig();
  const runtimeManager = new DesktopRuntimeManager(currentConfig);
  await runtimeManager.initialize();

  const desktopPetManager = new DesktopPetManager();
  registerPetIpcHandlers(desktopPetManager);

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
    console.error(
      "[AmitiaDesktop] 创建Desktop Instance失败:",
      error,
    );
    app.quit();
    return;
  }

  mainWindow = createMainWindow();
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
  });

  const autoLaunch = await configStore.getAutoLaunch();
  app.setLoginItemSettings({ openAtLogin: autoLaunch });
  registerUpdateManager(mainWindow);
  await waitForStartupCheck();

  registerExtensionProtocol(join(getAmitiaDataDir(), "data", "extensions-v2"));

  console.log(`[AmitiaDesktop] Deployment Mode: ${currentConfig.mode}`);
  if (currentConfig.mode === "cloud") {
    console.log(`[AmitiaDesktop] Business Core: ${currentConfig.serverURL}`);
    console.log("[AmitiaDesktop] Bundled Core Profile: device-agent");
    console.log("[AmitiaDesktop] Local Runtime: http://127.0.0.1:18899");
  } else {
    console.log("[AmitiaDesktop] Bundled Core Profile: local");
    console.log("[AmitiaDesktop] Local Runtime: http://127.0.0.1:18899");
  }

  notifyStatus(runtimeManager, "starting");
  try {
    await deploymentLifecycle.reconcile(currentConfig);
    const status = runtimeManager.getStatus();
    notifyStatus(runtimeManager, status.state, status.message);
  } catch (err) {
    console.error("[AmitiaDesktop] 初始reconcile失败:", err);
    notifyStatus(runtimeManager, "failed", String(err));
  }

  mainWindow.on("close", (event) => {
    if (!quitting) {
      event.preventDefault();
      mainWindow?.hide();
    }
  });

  app.on("before-quit", (event) => {
    quitting = true;
    if (!coreStopped) {
      event.preventDefault();
      if (!shutdownInProgress) {
        shutdownInProgress = true;
        void deploymentLifecycle?.shutdown().finally(() => {
          coreStopped = true;
          closeLogger();
          app.quit();
        });
      }
      return;
    }
  });
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
