import { app, BrowserWindow, Tray } from "electron"
import { ConfigStore } from "./config-store"
import { registerIpcHandlers } from "./ipc-handlers"
import { createMainWindow } from "./window"
import { createAppTray } from "./tray"
import { DesktopRuntimeManager } from "../runtime/runtime-manager"
import type { DeploymentModeConfig } from "../shared/types"

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
    const configStore = new ConfigStore()
    currentConfig = await configStore.getDeploymentConfig()
    const runtimeManager = new DesktopRuntimeManager(currentConfig)
    await runtimeManager.initialize()
    registerIpcHandlers(configStore, runtimeManager, (config) => {
      currentConfig = config
    })

    mainWindow = createMainWindow()
    tray = createAppTray(mainWindow, () => currentConfig)

    mainWindow.on("close", (event) => {
      if (!quitting) {
        event.preventDefault()
        mainWindow?.hide()
      }
    })

    app.on("before-quit", () => {
      quitting = true
    })
  })

  app.on("window-all-closed", () => {
    if (process.platform !== "darwin") {
      if (!tray) app.quit()
    }
  })

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      mainWindow = createMainWindow()
    } else {
      mainWindow?.show()
    }
  })
}
