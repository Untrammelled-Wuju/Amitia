export type RuntimeMode =
  | "browser"
  | "desktop-local"
  | "desktop-cloud"
  | "desktop-self-hosted"

export interface RuntimeConnection {
  mode: RuntimeMode
  apiBaseURL: string
  websocketBaseURL: string
  accessToken?: string
}

export interface DesktopEnvironment {
  platform: string
  arch: string
  version: string
  isPackaged: boolean
}

export type DeploymentMode = "local" | "cloud" | "self-hosted"

export interface DeploymentModeConfig {
  mode: DeploymentMode
  serverURL?: string
}

export type RuntimeState = "not-installed" | "not-ready" | "ready" | "failed"

export interface RuntimeStatus {
  state: RuntimeState
  mode: DeploymentMode
  message?: string
  updatedAt: string
}

export interface AmitiaDesktopAPI {
  getEnvironment(): Promise<DesktopEnvironment>
  getDeploymentConfig(): Promise<DeploymentModeConfig>
  saveDeploymentConfig(config: DeploymentModeConfig): Promise<DeploymentModeConfig>
  getRuntimeStatus(): Promise<RuntimeStatus>
  openLogsDirectory(): Promise<void>
  onRuntimeStatusChanged(callback: (status: RuntimeStatus) => void): () => void
  getVersion(): Promise<string>
  checkUpdate(): Promise<unknown>
  startDownload(): Promise<void>
  skipVersion(): Promise<void>
  restartNow(): Promise<void>
  restartLater(): Promise<void>
}
