import { app, BrowserWindow, ipcMain, shell } from "electron"
import { IPC_CHANNELS } from "../shared/ipc"
import type { DeploymentModeConfig } from "../shared/types"
import { ConfigStore } from "./config-store"
import type { DesktopRuntimeManager } from "../runtime/runtime-manager"

export function registerIpcHandlers(
  configStore: ConfigStore,
  runtimeManager: DesktopRuntimeManager,
  onDeploymentConfigSaved?: (config: DeploymentModeConfig) => void,
): void {
  ipcMain.handle(IPC_CHANNELS.getEnvironment, () => ({
    platform: process.platform,
    arch: process.arch,
    version: app.getVersion(),
    isPackaged: app.isPackaged,
  }))

  ipcMain.handle(IPC_CHANNELS.getDeploymentConfig, async () => {
    return configStore.getDeploymentConfig()
  })

  ipcMain.handle(IPC_CHANNELS.saveDeploymentConfig, async (_event, config: DeploymentModeConfig) => {
    const next = await configStore.saveDeploymentConfig(config)
    runtimeManager.setDeploymentConfig(next)
    onDeploymentConfigSaved?.(next)
    return next
  })

  ipcMain.handle(IPC_CHANNELS.getRuntimeStatus, () => runtimeManager.getStatus())

  ipcMain.handle(IPC_CHANNELS.openLogsDirectory, async () => {
    await shell.openPath(app.getPath("logs"))
  })

  ipcMain.handle(IPC_CHANNELS.minimizeWindow, (event) => {
    BrowserWindow.fromWebContents(event.sender)?.minimize()
  })

  ipcMain.handle(IPC_CHANNELS.toggleMaximizeWindow, (event) => {
    const win = BrowserWindow.fromWebContents(event.sender)
    if (!win) return false
    if (win.isMaximized()) {
      win.unmaximize()
      return false
    }
    win.maximize()
    return true
  })

  ipcMain.handle(IPC_CHANNELS.closeWindow, (event) => {
    BrowserWindow.fromWebContents(event.sender)?.close()
  })
}
