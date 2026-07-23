import { contextBridge, ipcRenderer } from "electron";
import { IPC_CHANNELS } from "../shared/ipc";
import type {
  AgentSkillDirectorySelection,
  DeploymentModeConfig,
  DesktopEnvironment,
  ExtensionPackageSelection,
  RuntimeStatus,
  SaveExtensionPackageRequest,
} from "../shared/types";

const api = {
  getEnvironment(): Promise<DesktopEnvironment> {
    return ipcRenderer.invoke(IPC_CHANNELS.getEnvironment);
  },
  getDeploymentConfig(): Promise<DeploymentModeConfig> {
    return ipcRenderer.invoke(IPC_CHANNELS.getDeploymentConfig);
  },
  saveDeploymentConfig(
    config: DeploymentModeConfig,
  ): Promise<DeploymentModeConfig> {
    return ipcRenderer.invoke(IPC_CHANNELS.saveDeploymentConfig, config);
  },
  getRuntimeStatus(): Promise<RuntimeStatus> {
    return ipcRenderer.invoke(IPC_CHANNELS.getRuntimeStatus);
  },
  openLogsDirectory(): Promise<void> {
    return ipcRenderer.invoke(IPC_CHANNELS.openLogsDirectory);
  },
  selectAgentSkillDirectory(): Promise<AgentSkillDirectorySelection | null> {
    return ipcRenderer.invoke(IPC_CHANNELS.selectAgentSkillDirectory);
  },
  selectMCPRoot(): Promise<{ path: string; name: string } | null> {
    return ipcRenderer.invoke(IPC_CHANNELS.selectMCPRoot);
  },
  selectExtensionPackage(): Promise<ExtensionPackageSelection | null> {
    return ipcRenderer.invoke(IPC_CHANNELS.selectExtensionPackage);
  },
  saveExtensionPackage(
    request: SaveExtensionPackageRequest,
  ): Promise<{ saved: boolean; fileName?: string }> {
    return ipcRenderer.invoke(IPC_CHANNELS.saveExtensionPackage, request);
  },
  minimizeWindow(): Promise<void> {
    return ipcRenderer.invoke(IPC_CHANNELS.minimizeWindow);
  },
  toggleMaximizeWindow(): Promise<boolean> {
    return ipcRenderer.invoke(IPC_CHANNELS.toggleMaximizeWindow);
  },
  getAutoLaunch(): Promise<boolean> {
    return ipcRenderer.invoke(IPC_CHANNELS.getAutoLaunch);
  },
  setAutoLaunch(enabled: boolean): Promise<boolean> {
    return ipcRenderer.invoke(IPC_CHANNELS.setAutoLaunch, enabled);
  },
  onAutoLaunchChanged(callback: (enabled: boolean) => void): () => void {
    const listener = (_event: Electron.IpcRendererEvent, enabled: boolean) =>
      callback(enabled);
    ipcRenderer.on(IPC_CHANNELS.autoLaunchChanged, listener);
    return () =>
      ipcRenderer.removeListener(IPC_CHANNELS.autoLaunchChanged, listener);
  },
  closeWindow(): Promise<void> {
    return ipcRenderer.invoke(IPC_CHANNELS.closeWindow);
  },
  onRuntimeStatusChanged(
    callback: (status: RuntimeStatus) => void,
  ): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      status: RuntimeStatus,
    ) => callback(status);
    ipcRenderer.on(IPC_CHANNELS.runtimeStatusChanged, listener);
    return () =>
      ipcRenderer.removeListener(IPC_CHANNELS.runtimeStatusChanged, listener);
  },
  getDataDir(): Promise<string> {
    return ipcRenderer.invoke("app:get-data-dir");
  },
  getVersion(): Promise<string> {
    return ipcRenderer.invoke("app:get-version");
  },
  checkUpdate(): Promise<unknown> {
    return ipcRenderer.invoke("update:check-on-startup");
  },
  checkNow(): Promise<unknown> {
    return ipcRenderer.invoke("update:check-now");
  },
  downloadUpdate(): Promise<void> {
    return ipcRenderer.invoke("update:download");
  },
  startDownload(): Promise<void> {
    return ipcRenderer.invoke("update:start-download");
  },
  skipVersion(): Promise<void> {
    return ipcRenderer.invoke("update:skip-version");
  },
  restartNow(): Promise<void> {
    return ipcRenderer.invoke("update:restart-now");
  },
  restartLater(): Promise<void> {
    return ipcRenderer.invoke("update:restart-later");
  },
  installUpdateNow(): Promise<void> {
    return ipcRenderer.invoke("update:install-now");
  },
  cancelAndEnter(): Promise<void> {
    return ipcRenderer.invoke("update:cancel-and-enter");
  },
  getCurrentVersion(): Promise<string> {
    return ipcRenderer.invoke("update:get-current-version");
  },
  openGiteeRelease(): Promise<void> {
    return ipcRenderer.invoke("update:open-gitee-release");
  },
  getReleaseNotes(): Promise<string> {
    return ipcRenderer.invoke("release-notes:get");
  },
  onUpdateChecking(callback: () => void): () => void {
    ipcRenderer.on("update:checking", callback);
    return () => ipcRenderer.removeListener("update:checking", callback);
  },
  onUpdateAvailable(
    callback: (_event: Electron.IpcRendererEvent, data: unknown) => void,
  ): () => void {
    ipcRenderer.on("update:available", callback);
    return () => ipcRenderer.removeListener("update:available", callback);
  },
  onUpdateNotAvailable(callback: () => void): () => void {
    ipcRenderer.on("update:not-available", callback);
    return () => ipcRenderer.removeListener("update:not-available", callback);
  },
  onUpdateDownloadProgress(
    callback: (_event: Electron.IpcRendererEvent, data: unknown) => void,
  ): () => void {
    ipcRenderer.on("update:download-progress", callback);
    return () =>
      ipcRenderer.removeListener("update:download-progress", callback);
  },
  onUpdateDownloaded(
    callback: (_event: Electron.IpcRendererEvent, data: unknown) => void,
  ): () => void {
    ipcRenderer.on("update:downloaded", callback);
    return () => ipcRenderer.removeListener("update:downloaded", callback);
  },
  onUpdateError(
    callback: (_event: Electron.IpcRendererEvent, data: unknown) => void,
  ): () => void {
    ipcRenderer.on("update:error", callback);
    return () => ipcRenderer.removeListener("update:error", callback);
  },
};

contextBridge.exposeInMainWorld("amitiaDesktop", api);

contextBridge.exposeInMainWorld("electronWindowApi", {
  minimize: (windowType: string = "main") => {
    if (windowType === "main") {
      return ipcRenderer.invoke("window-minimize", "main");
    }
    return ipcRenderer.invoke("window-minimize", "child");
  },
  toggleMaximize: () => ipcRenderer.invoke("window-toggle-maximize"),
  close: (windowType: string = "main") => {
    if (windowType === "main") {
      return ipcRenderer.invoke("window-close", "main");
    }
    return ipcRenderer.invoke("window-close", "child");
  },
  isMaximized: () => ipcRenderer.invoke("window-is-maximized"),
  getWindowType: () => ipcRenderer.invoke("get-window-type"),
});
