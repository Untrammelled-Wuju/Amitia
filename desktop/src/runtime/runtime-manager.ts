import { DEFAULT_CLOUD_SERVER_URL, validateDeploymentConfig } from "../shared/deployment"
import type { DeploymentModeConfig, RuntimeConnection, RuntimeStatus } from "../shared/types"

export interface RuntimeManager {
  initialize(): Promise<void>
  getStatus(): RuntimeStatus
  getConnection(): Promise<RuntimeConnection>
  start(): Promise<void>
  stop(): Promise<void>
  restart(): Promise<void>
  setDeploymentConfig(config: DeploymentModeConfig): void
}

export class DesktopRuntimeManager implements RuntimeManager {
  private config: DeploymentModeConfig
  private status: RuntimeStatus

  constructor(config: DeploymentModeConfig) {
    this.config = validateDeploymentConfig(config)
    this.status = this.createStatus()
  }

  async initialize(): Promise<void> {
    this.status = this.createStatus()
  }

  getStatus(): RuntimeStatus {
    return this.status
  }

  async getConnection(): Promise<RuntimeConnection> {
    const config = validateDeploymentConfig(this.config)
    if (config.mode === "local") {
      return {
        mode: "desktop-local",
        apiBaseURL: "",
        websocketBaseURL: "",
      }
    }
    const baseURL = config.mode === "cloud" ? DEFAULT_CLOUD_SERVER_URL : config.serverURL || ""
    return {
      mode: config.mode === "cloud" ? "desktop-cloud" : "desktop-self-hosted",
      apiBaseURL: baseURL,
      websocketBaseURL: baseURL.replace(/^http:/, "ws:").replace(/^https:/, "wss:"),
    }
  }

  async start(): Promise<void> {
    if (this.config.mode === "local") {
      this.status = this.createStatus("not-installed", "本阶段尚未安装本地运行时")
      return
    }
    this.status = this.createStatus("ready")
  }

  async stop(): Promise<void> {
    this.status = this.createStatus(this.config.mode === "local" ? "not-installed" : "not-ready")
  }

  async restart(): Promise<void> {
    await this.stop()
    await this.start()
  }

  setDeploymentConfig(config: DeploymentModeConfig): void {
    this.config = validateDeploymentConfig(config)
    this.status = this.createStatus()
  }

  setStatus(state: RuntimeStatus["state"], message?: string): void {
    this.status = this.createStatus(state, message)
  }

  private createStatus(state?: RuntimeStatus["state"], message?: string): RuntimeStatus {
    const fallbackState = this.config.mode === "local" ? "not-installed" : "ready"
    return {
      state: state || fallbackState,
      mode: this.config.mode,
      message: message || (this.config.mode === "local" ? "本地运行时未安装" : undefined),
      updatedAt: new Date().toISOString(),
    }
  }
}
