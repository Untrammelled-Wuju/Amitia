import { contextBridge, ipcRenderer } from "electron"
import { IPC_CHANNELS } from "../shared/ipc"
import type { DeploymentModeConfig, DesktopEnvironment, RuntimeStatus } from "../shared/types"

const api = {
  getEnvironment(): Promise<DesktopEnvironment> {
    return ipcRenderer.invoke(IPC_CHANNELS.getEnvironment)
  },
  getDeploymentConfig(): Promise<DeploymentModeConfig> {
    return ipcRenderer.invoke(IPC_CHANNELS.getDeploymentConfig)
  },
  saveDeploymentConfig(config: DeploymentModeConfig): Promise<DeploymentModeConfig> {
    return ipcRenderer.invoke(IPC_CHANNELS.saveDeploymentConfig, config)
  },
  getRuntimeStatus(): Promise<RuntimeStatus> {
    return ipcRenderer.invoke(IPC_CHANNELS.getRuntimeStatus)
  },
  openLogsDirectory(): Promise<void> {
    return ipcRenderer.invoke(IPC_CHANNELS.openLogsDirectory)
  },
  onRuntimeStatusChanged(callback: (status: RuntimeStatus) => void): () => void {
    const listener = (_event: Electron.IpcRendererEvent, status: RuntimeStatus) => callback(status)
    ipcRenderer.on(IPC_CHANNELS.runtimeStatusChanged, listener)
    return () => ipcRenderer.removeListener(IPC_CHANNELS.runtimeStatusChanged, listener)
  },
}

contextBridge.exposeInMainWorld("amitiaDesktop", api)
