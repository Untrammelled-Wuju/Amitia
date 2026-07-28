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
  startCore,
  stopCore,
  waitForCoreReady,
  isCoreRunning,
  ensureDataAndConfig,
} from "./core-manager";
import { ensureAmitiaDataDir, getAmitiaDataDir } from "./path-manager";
import { registerExtensionProtocol } from "./protocol";
import { registerUpdateManager, waitForStartupCheck } from "./update-manager";
import { ipcMain } from "electron";
import { IPC_CHANNELS } from "../shared/ipc";
import { applyBrandTheme } from "./branding";
import { DesktopPetManager } from "./pet/manager";
import { ChatStateSubscriber } from "./pet/chat-state-subscriber";
import { CharacterWatcher } from "./pet/character-watcher";
import { registerPetIpcHandlers } from "./pet-ipc";

let mainWindow: BrowserWindow | null = null;
let tray: Tray | null = null;
let currentConfig: DeploymentModeConfig = { mode: "local" };
let quitting = false;
let coreStopped = false;
let shutdownInProgress = false;
let desktopPetManager: DesktopPetManager | null = null;
let chatStateSubscriber: ChatStateSubscriber | null = null;
let characterWatcher: CharacterWatcher | null = null;

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

  registerIpcHandlers(configStore, runtimeManager, (config) => {
    if (currentConfig.mode !== config.mode) {
      if (config.mode === "local") {
        if (!isCoreRunning()) {
          notifyStatus(runtimeManager, "starting");
          try {
            startCore();
            void waitForCoreReady()
              .then(() => {
                notifyStatus(runtimeManager, "ready");
              })
              .catch((err) => {
                console.error("[AmitiaDesktop] 模式切换后核心启动失败", err);
                notifyStatus(runtimeManager, "failed", String(err));
              });
          } catch (err) {
            console.error("[AmitiaDesktop] 模式切换startCore异常:", err);
            notifyStatus(runtimeManager, "failed", String(err));
          }
        }
      } else {
        void stopCore();
        notifyStatus(runtimeManager, "ready");
      }
    }
    currentConfig = config;
  });

  const ensureResult = ensureDataAndConfig();
  if (!ensureResult.ok) {
    console.warn(
      "[AmitiaDesktop] 数据目录初始化存在缺失",
      ensureResult.errors.join(", "),
    );
  }

  if (currentConfig.mode === "local") {
    notifyStatus(runtimeManager, "starting");
    try {
      console.log("[AmitiaDesktop] 启动本地核心...");
      startCore();
      await waitForCoreReady();
      console.log("[AmitiaDesktop] 核心就绪");
      notifyStatus(runtimeManager, "ready");
    } catch (err) {
      console.error("[AmitiaDesktop] 核心启动失败:", err);
      notifyStatus(runtimeManager, "failed", String(err));
    }
  } else {
    notifyStatus(runtimeManager, "ready");
  }

  registerExtensionProtocol(join(getAmitiaDataDir(), "data", "extensions-v2"));

  mainWindow = createMainWindow();
  const trayResult = createAppTray(
    mainWindow,
    () => currentConfig,
    configStore,
  );
  tray = trayResult.tray;
  syncSystemBrandTheme();

  const autoLaunch = await configStore.getAutoLaunch();
  app.setLoginItemSettings({ openAtLogin: autoLaunch });
  registerUpdateManager(mainWindow);
  await waitForStartupCheck();

  desktopPetManager = new DesktopPetManager();
  chatStateSubscriber = new ChatStateSubscriber({
    coreHost: "127.0.0.1",
    corePort: 18899,
    onPayload: (payload) => {
      if (!desktopPetManager) return;
      try {
        desktopPetManager.handleChatStatePayload(payload);
      } catch (err) {
        console.warn("[AmitiaDesktop] 转发聊天状态失败:", err);
      }
    },
  });
  characterWatcher = new CharacterWatcher({
    coreHost: "127.0.0.1",
    corePort: 18899,
    onActiveCharacterChanged: async (characterId) => {
      if (!desktopPetManager) return;
      try {
        await desktopPetManager.handleCharacterSwitched(characterId);
      } catch (err) {
        console.warn("[AmitiaDesktop] 角色切换处理失败:", err);
      }
    },
  });
  registerPetIpcHandlers(desktopPetManager);

  void desktopPetManager.initialize().catch((err) => {
    console.warn("[AmitiaDesktop] DesktopPetManager 初始化失败:", err);
  });
  void chatStateSubscriber.start().catch((err) => {
    console.warn("[AmitiaDesktop] 聊天状态订阅启动失败:", err);
  });
  void characterWatcher.start().catch((err) => {
    console.warn("[AmitiaDesktop] 角色监听启动失败:", err);
  });

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
        void desktopPetManager?.shutdown().catch((err) => {
          console.warn("[AmitiaDesktop] DesktopPetManager shutdown 失败:", err);
        });
        chatStateSubscriber?.stop();
        characterWatcher?.stop();
        void stopCore().finally(() => {
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
