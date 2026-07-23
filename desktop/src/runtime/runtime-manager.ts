import type { DeploymentModeConfig, RuntimeStatus } from "../shared/types";

export class DesktopRuntimeManager {
  private status: RuntimeStatus;
  private config: DeploymentModeConfig;

  constructor(config: DeploymentModeConfig) {
    this.config = config;
    this.status = {
      state: "not-installed",
      mode: config.mode,
      updatedAt: new Date().toISOString(),
    };
  }

  async initialize(): Promise<void> {
    this.status.updatedAt = new Date().toISOString();
  }

  setStatus(state: RuntimeStatus["state"], message?: string): void {
    this.status = {
      state,
      mode: this.config.mode,
      message,
      updatedAt: new Date().toISOString(),
    };
  }

  getStatus(): RuntimeStatus {
    return { ...this.status };
  }

  setDeploymentConfig(config: DeploymentModeConfig): void {
    this.config = config;
    this.status.mode = config.mode;
    this.status.updatedAt = new Date().toISOString();
  }
}
