import { app, BrowserWindow, Tray } from "electron"
import { initLogger, closeLogger } from "./logger"
import { ConfigStore } from "./config-store"
import { registerIpcHandlers } from "./ipc-handlers"
import { createMainWindow } from "./window"
import { createAppTray } from "./tray"
import { DesktopRuntimeManager } from "../runtime/runtime-manager"
import type { DeploymentModeConfig } from "../shared/types"
import type { RuntimeStatus } from "../shared/types"
import { startCore, stopCore, waitForCoreReady, isCoreRunning, ensureDataAndConfig } from "./core-manager"
import { ensureAmitiaDataDir, getAmitiaDataDir } from "./path-manager"
import { registerUpdateManager, waitForStartupCheck } from "./update-manager"
import { ipcMain } from "electron"
import { IPC_CHANNELS } from "../shared/ipc"

let mainWindow: BrowserWindow | null = null
let tray: Tray | null = null
let currentConfig: DeploymentModeConfig = { mode: "local" }
let quitting = false

function notifyStatus(runtimeManager: DesktopRuntimeManager, state: RuntimeStatus["state"], message?: string) {
  runtimeManager.setStatus(state, message)
  if (mainWindow && !mainWindow.isDestroyed()) {
    mainWindow.webContents.send(IPC_CHANNELS.runtimeStatusChanged, runtimeManager.getStatus())
  }
  }

const lock = app.requestSingleInstanceLock()
if (!lock) {
  app.quit()
} else {
  app.on("second-instance", () => {
    if (!mainWindow) return
    if (mainWindow.isMinimized()) mainWindow.restore()
    mainWindow.show()
    mainWindow.focus()
  })

  void app.whenReady().then(async () => {
    initLogger()
    console.log("[AmitiaDesktop] 应用启动, isPackaged:", app.isPackaged)

    let dataDir: string
    try {
      dataDir = ensureAmitiaDataDir()
      console.log("[AmitiaDesktop] 数据目录:", dataDir)
    } catch (err) {
      console.error("[AmitiaDesktop] 数据目录创建失败:", err)
      app.quit()
      return
    }

    await enterMainApp()
  })

  app.on("window-all-closed", () => {
    if (process.platform !== "darwin") {
      if (!tray) app.quit()
    }
  })

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      if (mainWindow) {
        mainWindow.show()
      } else {
        mainWindow = createMainWindow()
      }
    }
  })
}

async function enterMainApp(): Promise<void> {
  const configStore = new ConfigStore()
  currentConfig = await configStore.getDeploymentConfig()
  const runtimeManager = new DesktopRuntimeManager(currentConfig)
  await runtimeManager.initialize()

  registerIpcHandlers(configStore, runtimeManager, (config) => {
    if (currentConfig.mode !== config.mode) {
      if (config.mode === "local") {
        if (!isCoreRunning()) {
          notifyStatus(runtimeManager, "starting")
          try {
            startCore()
            void waitForCoreReady().then(() => {
              notifyStatus(runtimeManager, "ready")
            }).catch((err) => {
              console.error("[AmitiaDesktop] 模式切换后核心启动失败:", err)
              notifyStatus(runtimeManager, "failed", String(err))
            })
          } catch (err) {
            console.error("[AmitiaDesktop] 模式切换startCore异常:", err)
            notifyStatus(runtimeManager, "failed", String(err))
          }
        }
      } else {
        stopCore()
        notifyStatus(runtimeManager, "ready")
      }
    }
    currentConfig = config
  })

  const ensureResult = ensureDataAndConfig()
  if (!ensureResult.ok) {
    console.warn("[AmitiaDesktop] 数据目录初始化存在缺失:", ensureResult.errors.join(", "))
  }

  if (currentConfig.mode === "local") {
    notifyStatus(runtimeManager, "starting")
    try {
      console.log("[AmitiaDesktop] 启动本地核心...")
      startCore()
      await waitForCoreReady()
      console.log("[AmitiaDesktop] 核心就绪")
      notifyStatus(runtimeManager, "ready")
    } catch (err) {
      console.error("[AmitiaDesktop] 核心启动失败:", err)
      notifyStatus(runtimeManager, "failed", String(err))
    }
  } else {
    notifyStatus(runtimeManager, "ready")
  }

  mainWindow = createMainWindow()
  tray = createAppTray(mainWindow, () => currentConfig)

  const autoLaunch = await configStore.getAutoLaunch()
  app.setLoginItemSettings({ openAtLogin: autoLaunch })
  registerUpdateManager(mainWindow)
  await waitForStartupCheck()

  mainWindow.on("close", (event) => {
    if (!quitting) {
      event.preventDefault()
      mainWindow?.hide()
    }
  })

  app.on("before-quit", () => {
    quitting = true
    stopCore()
    closeLogger()
  })
}

ipcMain.handle("app:enter-main", async () => {
  await enterMainApp()
})

ipcMain.handle("app:get-data-dir", () => {
  return getAmitiaDataDir()
})

ipcMain.handle("app:get-version", () => {
  return app.getVersion()
})
