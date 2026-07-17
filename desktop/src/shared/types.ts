export type DeploymentMode = "local" | "cloud" | "self-hosted"

export interface DeploymentModeConfig {
  mode: DeploymentMode
  serverURL?: string
}

export type RuntimeMode = "browser" | "desktop-local" | "desktop-cloud" | "desktop-self-hosted"

export interface RuntimeConnection {
  mode: RuntimeMode
  apiBaseURL: string
  websocketBaseURL: string
  accessToken?: string
}

export type RuntimeState = "not-installed" | "starting" | "not-ready" | "ready" | "failed"

export interface RuntimeStatus {
  state: RuntimeState
  mode: DeploymentMode
  message?: string
  updatedAt: string
}

export interface RuntimeError {
  code: string
  message: string
}

export interface RuntimeManifest {
  version: string
  platform: string
  architecture: string
  files: Array<{ path: string; sha256: string; size: number }>
}

export interface DesktopEnvironment {
  platform: NodeJS.Platform
  arch: string
  version: string
  isPackaged: boolean
}

export interface ManagedProcessDefinition {
  id: string
  executable: string
  args: string[]
  cwd: string
  env: NodeJS.ProcessEnv
  logFile: string
}

export type ManagedProcessState = "stopped" | "starting" | "running" | "stopping" | "failed"

export interface ManagedProcessStatus {
  id: string
  state: ManagedProcessState
  pid?: number
  exitCode?: number
  error?: string
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
