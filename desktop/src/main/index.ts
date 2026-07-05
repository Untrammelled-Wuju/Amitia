import { app, BrowserWindow, Tray } from "electron"
import { initLogger, closeLogger } from "./logger"
import { ConfigStore } from "./config-store"
import { registerIpcHandlers } from "./ipc-handlers"
import { createMainWindow } from "./window"
import { createAppTray } from "./tray"
import { DesktopRuntimeManager } from "../runtime/runtime-manager"
import type { DeploymentModeConfig } from "../shared/types"
import { startCore, stopCore, waitForCoreReady, isCoreRunning, ensureDataAndConfig } from "./core-manager"
import { ensureAmitiaDataDir, getAmitiaDataDir } from "./path-manager"
import { registerUpdateManager, waitForStartupCheck } from "./update-manager"
import { ipcMain } from "electron"

let mainWindow: BrowserWindow | null = null
let tray: Tray | null = null
let currentConfig: DeploymentModeConfig = { mode: "local" }
let quitting = false

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
          runtimeManager.setStatus("starting")
          try {
            startCore()
            void waitForCoreReady().then(() => {
              runtimeManager.setStatus("ready")
            }).catch((err) => {
              console.error("[AmitiaDesktop] 模式切换后核心启动失败:", err)
              runtimeManager.setStatus("failed", String(err))
            })
          } catch (err) {
            console.error("[AmitiaDesktop] 模式切换startCore异常:", err)
            runtimeManager.setStatus("failed", String(err))
          }
        }
      } else {
        stopCore()
        runtimeManager.setStatus("ready")
      }
    }
    currentConfig = config
  })

  ensureDataAndConfig()

  mainWindow = createMainWindow()
  tray = createAppTray(mainWindow, () => currentConfig)

  registerUpdateManager(mainWindow)
  await waitForStartupCheck()

  if (currentConfig.mode === "local") {
    runtimeManager.setStatus("starting")
    try {
      console.log("[AmitiaDesktop] 启动本地核心...")
      startCore()
      await waitForCoreReady()
      console.log("[AmitiaDesktop] 核心就绪")
      runtimeManager.setStatus("ready")
    } catch (err) {
      console.error("[AmitiaDesktop] 核心启动失败:", err)
      runtimeManager.setStatus("failed", String(err))
    }
  } else {
    runtimeManager.setStatus("ready")
  }

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
