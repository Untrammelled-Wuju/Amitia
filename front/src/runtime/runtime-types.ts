export type DeploymentMode = "local" | "cloud"

export interface DeploymentModeConfig {
  mode: DeploymentMode
  serverURL?: string
}

export type RuntimeState = "not-installed" | "starting" | "not-ready" | "ready" | "failed"

export interface RuntimeStatus {
  state: RuntimeState
  mode: DeploymentMode
  message?: string
  updatedAt: string
}

export interface DesktopEnvironment {
  platform: string
  arch: string
  version: string
  isPackaged: boolean
}

export interface AgentSkillDirectorySelection {
  rootName: string
  files: Array<{ path: string; name: string; base64: string }>
}

export interface ExtensionPackageSelection {
  name: string
  size: number
  base64: string
}

export interface SaveExtensionPackageRequest {
  suggestedName: string
  base64: string
}

export interface AmitiaDesktopAPI {
  getEnvironment(): Promise<DesktopEnvironment>
  getDeploymentConfig(): Promise<DeploymentModeConfig>
  saveDeploymentConfig(config: DeploymentModeConfig): Promise<DeploymentModeConfig>
  getRuntimeStatus(): Promise<RuntimeStatus>
  openLogsDirectory(): Promise<void>
  selectAgentSkillDirectory(): Promise<AgentSkillDirectorySelection | null>
  selectMCPRoot(): Promise<{ path: string; name: string } | null>
  selectExtensionPackage(): Promise<ExtensionPackageSelection | null>
  saveExtensionPackage(request: SaveExtensionPackageRequest): Promise<{ saved: boolean; fileName?: string }>
  minimizeWindow(): Promise<void>
  toggleMaximizeWindow(): Promise<boolean>
  closeWindow(): Promise<void>
  getAutoLaunch(): Promise<boolean>
  setAutoLaunch(enabled: boolean): Promise<boolean>
  onAutoLaunchChanged(callback: (enabled: boolean) => void): () => void

  onRuntimeStatusChanged(callback: (status: RuntimeStatus) => void): () => void
  getDataDir(): Promise<string>
  getVersion(): Promise<string>
  checkUpdate(): Promise<unknown>
  checkNow(): Promise<unknown>
  downloadUpdate(): Promise<void>
  startDownload(): Promise<void>
  skipVersion(): Promise<void>
  restartNow(): Promise<void>
  restartLater(): Promise<void>
  installUpdateNow(): Promise<void>
  cancelAndEnter(): Promise<void>
  getCurrentVersion(): Promise<string>
  openGiteeRelease(): Promise<void>
  onUpdateChecking(callback: () => void): () => void
  onUpdateAvailable(callback: (event: unknown, data: unknown) => void): () => void
  onUpdateNotAvailable(callback: () => void): () => void
  onUpdateDownloadProgress(callback: (event: unknown, data: unknown) => void): () => void
  onUpdateDownloaded(callback: (event: unknown, data: unknown) => void): () => void
  onUpdateError(callback: (event: unknown, data: unknown) => void): () => void
}

export interface RuntimeConnection {
  apiBaseURL: string
  websocketBaseURL: string
}
