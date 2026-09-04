import type {
  DeploymentModeConfig,
  RuntimeStatus,
  RuntimeEndpointStatus,
  RuntimeState,
} from "../shared/types";

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

  setStatus(state: RuntimeState, message?: string): void {
    this.status = {
      ...this.status,
      state,
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

  setBusinessCoreStatus(
    state: RuntimeState,
    baseURL: string,
    message?: string,
  ): void {
    this.status.businessCore = {
      state,
      baseURL,
      message,
    };
    this.recomputeTopLevelState();
  }

  setLocalRuntimeStatus(
    state: RuntimeState,
    baseURL: string,
    profile?: "local" | "device-agent",
    message?: string,
  ): void {
    this.status.localRuntime = {
      state,
      baseURL,
      profile,
      message,
    };
    this.recomputeTopLevelState();
  }

  clearBusinessCoreStatus(): void {
    this.status.businessCore = undefined;
    this.recomputeTopLevelState();
  }

  clearLocalRuntimeStatus(): void {
    this.status.localRuntime = undefined;
    this.recomputeTopLevelState();
  }

  private recomputeTopLevelState(): void {
    const businessCore = this.status.businessCore;
    const localRuntime = this.status.localRuntime;

    if (this.config.mode === "local") {
      if (businessCore) {
        this.status.state = businessCore.state;
        this.status.message = businessCore.message;
      }
      if (localRuntime && this.status.state !== "ready") {
        this.status.state = localRuntime.state;
        this.status.message = localRuntime.message;
      }
    } else {
      if (businessCore) {
        if (businessCore.state === "ready") {
          this.status.state = "ready";
          this.status.message = businessCore.message;
        } else if (businessCore.state === "failed") {
          this.status.state = "not-ready";
          this.status.message = businessCore.message || "业务核心不可用";
        } else {
          this.status.state = businessCore.state;
          this.status.message = businessCore.message;
        }
      } else {
        this.status.state = "starting";
      }

      if (localRuntime && localRuntime.state === "failed") {
        this.status.message = this.status.message || "本机执行节点暂不可用";
      }
    }

    this.status.updatedAt = new Date().toISOString();
  }
}
