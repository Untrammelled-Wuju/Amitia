import { BrowserWindow, type MenuItemConstructorOptions } from "electron";
import type {
  DesktopBackendTopology,
  BackendEndpoint,
} from "../shared/backend-topology";
import type { DeploymentModeConfig, RuntimeStatus } from "../shared/types";
import { resolveBackendTopology } from "../shared/backend-topology";
import { getCoreBaseURL } from "./core-endpoint";
import { ConfigStore } from "./config-store";
import { DesktopRuntimeManager } from "../runtime/runtime-manager";
import {
  ensureCoreProfile,
  stopCore,
  isCoreRunning,
  BundledCoreProfile,
} from "./core-manager";
import { BusinessCoreClient } from "./business-core-client";
import { DesktopHostManager, DesktopSnapshotSync } from "./desktop-host";
import { UIHostSSE } from "./ui-host-sse";
import { DesktopPetManager } from "./pet/manager";
import { getMeshCoordinator } from "./device-mesh/coordinator";

const MAX_LOCAL_RECOVERY_ATTEMPTS = 5;
const LOCAL_RECOVERY_BASE_DELAY_MS = 1000;
const LOCAL_RECOVERY_MAX_DELAY_MS = 10000;

export interface DesktopDeploymentLifecycleDeps {
  configStore: ConfigStore;
  runtimeManager: DesktopRuntimeManager;
  getMainWindow: () => BrowserWindow | null;
  setExtensionTrayItems: (items: MenuItemConstructorOptions[]) => Promise<void>;
  desktopPetManager: DesktopPetManager;
}

export class DesktopDeploymentLifecycle {
  private readonly configStore: ConfigStore;
  private readonly runtimeManager: DesktopRuntimeManager;
  private readonly getMainWindow: () => BrowserWindow | null;
  private readonly setExtensionTrayItems: (items: MenuItemConstructorOptions[]) => Promise<void>;
  private readonly desktopPetManager: DesktopPetManager;

  private topology: DesktopBackendTopology;
  private businessCoreClient: BusinessCoreClient;
  private desktopHostManager: DesktopHostManager | null = null;
  private desktopSnapshotSync: DesktopSnapshotSync | null = null;
  private uiHostSSE: UIHostSSE | null = null;

  private reconcileChain: Promise<void> = Promise.resolve();
  private pendingReconciles = 0;
  private recoveryAttempts = 0;
  private currentConfig: DeploymentModeConfig;
  private shuttingDown = false;
  private shutdownPromise: Promise<void> | null = null;

  constructor(deps: DesktopDeploymentLifecycleDeps, initialConfig: DeploymentModeConfig) {
    this.configStore = deps.configStore;
    this.runtimeManager = deps.runtimeManager;
    this.getMainWindow = deps.getMainWindow;
    this.setExtensionTrayItems = deps.setExtensionTrayItems;
    this.desktopPetManager = deps.desktopPetManager;
    this.currentConfig = { ...initialConfig };
    this.topology = resolveBackendTopology(initialConfig, getCoreBaseURL());
    this.businessCoreClient = new BusinessCoreClient(
      this.topology.businessCore.baseURL,
      this.topology.deploymentMode === "cloud",
    );
  }

  async reconcile(config: DeploymentModeConfig): Promise<RuntimeStatus> {
    this.recoveryAttempts = 0;
    return this.enqueueReconcile(config);
  }

  async restartLocalRuntime(): Promise<RuntimeStatus> {
    if (this.shuttingDown) return this.runtimeManager.getStatus();
    this.recoveryAttempts = 0;
    this.stopBusinessIntegrations();
    const result = await this.enqueueReconcile(this.currentConfig);
    if (result.localRuntime?.state === "ready" && isCoreRunning()) this.recoveryAttempts = 0;
    return result;
  }

  async recoverUnexpectedExit(
    profile: BundledCoreProfile,
    error: Error,
    onStarting: (message: string) => void,
  ): Promise<RuntimeStatus | undefined> {
    if (this.shuttingDown || profile !== this.topology.localRuntimeProfile) return;
    if (this.pendingReconciles > 0) {
      await this.reconcileChain;
    }
    if (this.shuttingDown) return;
    const original = error.stack ?? error.message;
    this.stopBusinessIntegrations();
    if (this.recoveryAttempts >= MAX_LOCAL_RECOVERY_ATTEMPTS) {
      this.recordLocalFailure(profile, [original, `本地服务自动恢复已尝试 ${this.recoveryAttempts} 次仍未成功，请手动重启本地服务`].join("\n"));
      return this.runtimeManager.getStatus();
    }
    this.recoveryAttempts++;
    onStarting(original);
    const delayMs = Math.min(LOCAL_RECOVERY_BASE_DELAY_MS * 2 ** (this.recoveryAttempts - 1), LOCAL_RECOVERY_MAX_DELAY_MS);
    await new Promise((resolve) => setTimeout(resolve, delayMs));
    if (this.shuttingDown) return;
    const result = await this.enqueueReconcile(this.currentConfig);
    if (result.localRuntime?.state === "ready" && isCoreRunning()) {
      this.recoveryAttempts = 0;
      return this.runtimeManager.getStatus();
    }
    const retryError = result.localRuntime?.message || result.message;
    const combined = new Error([original, retryError].filter(Boolean).join("\n"));
    if (this.recoveryAttempts < MAX_LOCAL_RECOVERY_ATTEMPTS) {
      return this.recoverUnexpectedExit(profile, combined, onStarting);
    }
    this.recordLocalFailure(profile, [combined.message, `本地服务自动恢复已尝试 ${this.recoveryAttempts} 次仍未成功，请手动重启本地服务`].join("\n"));
    return this.runtimeManager.getStatus();
  }

  private recordLocalFailure(profile: BundledCoreProfile, message: string): void {
    if (profile === "local") this.runtimeManager.setBusinessCoreStatus("failed", "", message);
    this.runtimeManager.setLocalRuntimeStatus("failed", "", profile, message);
  }

  private async enqueueReconcile(config: DeploymentModeConfig): Promise<RuntimeStatus> {
    if (this.shuttingDown) {
      return this.runtimeManager.getStatus();
    }

    this.pendingReconciles++;
    this.reconcileChain = this.reconcileChain.then(async () => {
      try {
        if (this.shuttingDown) return;
        const desired = { ...config };
        this.currentConfig = desired;
        this.topology = resolveBackendTopology(desired, getCoreBaseURL());

        if (desired.mode === "local") {
          await this.reconcileLocalMode();
        } else {
          await this.reconcileCloudMode();
        }
      } catch (err) {
        console.error("[DeploymentLifecycle] reconcile失败:", err);
        const message = err instanceof Error ? err.stack ?? err.message : String(err);
        this.recordLocalFailure(this.topology.localRuntimeProfile, message);
        this.runtimeManager.setStatus("failed", message);
      } finally {
        this.pendingReconciles--;
      }
    });

    await this.reconcileChain;

    return this.runtimeManager.getStatus();
  }

  private async reconcileLocalMode(): Promise<void> {
    await this.stopLocalPetIntegrations();
    if (this.shuttingDown) return;
    this.stopBusinessIntegrations();
    getMeshCoordinator(this.getMainWindow).stop();

    this.runtimeManager.setBusinessCoreStatus(
      "starting",
      this.topology.businessCore.baseURL,
    );

    await ensureCoreProfile("local");
    this.topology = resolveBackendTopology(this.currentConfig, getCoreBaseURL());
    this.businessCoreClient = new BusinessCoreClient(
      this.topology.businessCore.baseURL,
      false,
    );
    await this.probeLocalRuntime();
    await this.probeBusinessCore(this.topology.businessCore);

    await this.rebuildBusinessIntegrations(this.topology);

    await this.startLocalPetIntegrations();
  }

  private async reconcileCloudMode(): Promise<void> {
    await this.stopLocalPetIntegrations();
    if (this.shuttingDown) return;
    this.stopBusinessIntegrations();

    const [coreResult] = await Promise.allSettled([
      ensureCoreProfile("device-agent"),
      this.probeBusinessCore(this.topology.businessCore),
    ]);
    const localRuntimeAvailable = coreResult.status === "fulfilled";

    if (coreResult.status === "rejected") {
      console.error("[DeploymentLifecycle] device-agent启动失败:", coreResult.reason);
      this.runtimeManager.setLocalRuntimeStatus(
        "failed",
        this.topology.localRuntime.baseURL,
        "device-agent",
        coreResult.reason instanceof Error ? coreResult.reason.stack ?? coreResult.reason.message : String(coreResult.reason),
      );
    } else {
      this.topology = resolveBackendTopology(this.currentConfig, getCoreBaseURL());
      await this.probeLocalRuntime();
    }

    await this.rebuildBusinessIntegrations(this.topology);

    const coordinator = getMeshCoordinator(this.getMainWindow);
    coordinator.start();

    // Cloud mode still owns a local desktop-pet body. Packages, renderer and
    // Runtime v1 remain on the device. Never construct the pet manager on top
    // of a failed device-agent profile.
    if (localRuntimeAvailable) {
      await this.startLocalPetIntegrations();
    }
  }

  private async probeLocalRuntime(): Promise<void> {
    const localRuntime = this.topology.localRuntime;
    this.runtimeManager.setLocalRuntimeStatus(
      "starting",
      localRuntime.baseURL,
      this.topology.localRuntimeProfile,
    );

    if (isCoreRunning()) {
      this.runtimeManager.setLocalRuntimeStatus(
        "ready",
        localRuntime.baseURL,
        this.topology.localRuntimeProfile,
        this.topology.localRuntimeProfile === "device-agent"
          ? "本机执行节点已启动"
          : undefined,
      );
    } else {
      this.runtimeManager.setLocalRuntimeStatus(
        "not-ready",
        localRuntime.baseURL,
        this.topology.localRuntimeProfile,
      );
    }
  }

  private async probeBusinessCore(businessCore: BackendEndpoint): Promise<void> {
    if (this.topology.deploymentMode === "local") {
      this.runtimeManager.setBusinessCoreStatus(
        isCoreRunning() ? "ready" : "not-ready",
        businessCore.baseURL,
      );
      return;
    }

    this.runtimeManager.setBusinessCoreStatus("starting", businessCore.baseURL);

    try {
      const client = new BusinessCoreClient(
        businessCore.baseURL,
        this.topology.deploymentMode === "cloud",
      );
      const probeResult = await client.probe();

      if (probeResult.ready) {
        this.runtimeManager.setBusinessCoreStatus(
          "ready",
          businessCore.baseURL,
        );
      } else if (probeResult.reachable) {
        this.runtimeManager.setBusinessCoreStatus(
          "not-ready",
          businessCore.baseURL,
          probeResult.error,
        );
      } else {
        this.runtimeManager.setBusinessCoreStatus(
          "not-ready",
          businessCore.baseURL,
          probeResult.error || "业务核心不可达",
        );
      }
    } catch (err) {
      this.runtimeManager.setBusinessCoreStatus(
        "not-ready",
        businessCore.baseURL,
        err instanceof Error ? err.message : String(err),
      );
    }
  }

  private async rebuildBusinessIntegrations(topology: DesktopBackendTopology): Promise<void> {
    this.stopBusinessIntegrations();

    const mainWindow = this.getMainWindow();
    if (!mainWindow) return;

    this.businessCoreClient = new BusinessCoreClient(
      topology.businessCore.baseURL,
      topology.deploymentMode === "cloud",
    );

    this.desktopHostManager = new DesktopHostManager(
      mainWindow,
      this.setExtensionTrayItems,
      this.businessCoreClient,
    );

    this.uiHostSSE = new UIHostSSE({
      businessCore: this.businessCoreClient,
      mainWindow: this.getMainWindow,
      onExtensionChange: () => {
        void this.desktopSnapshotSync?.syncNow();
      },
    });
    this.uiHostSSE.start();

    this.desktopSnapshotSync = new DesktopSnapshotSync(
      mainWindow,
      this.desktopHostManager,
      this.businessCoreClient,
    );
    this.desktopSnapshotSync.start();
  }

  private stopBusinessIntegrations(): void {
    if (this.uiHostSSE) {
      this.uiHostSSE.stop();
      this.uiHostSSE = null;
    }
    if (this.desktopSnapshotSync) {
      this.desktopSnapshotSync.stop();
      this.desktopSnapshotSync = null;
    }
    if (this.desktopHostManager) {
      this.desktopHostManager.cleanup();
      this.desktopHostManager = null;
    }
  }

  private async startLocalPetIntegrations(): Promise<void> {
    if (this.shuttingDown) {
      return;
    }

    const mainWindow = this.getMainWindow();
    if (!mainWindow) return;

    try {
      await this.desktopPetManager.initialize({ restoreActiveInstallation: true });
    } catch (err) {
      console.warn("[DeploymentLifecycle] DesktopPetManager 初始化失败:", err);
    }
  }

  private async stopLocalPetIntegrations(): Promise<void> {
    try {
      await this.desktopPetManager.shutdown();
    } catch (err) {
      console.warn("[DeploymentLifecycle] DesktopPetManager shutdown 失败:", err);
    }
  }

  getTopology(): DesktopBackendTopology {
    return this.topology;
  }

  getBusinessCoreClient(): BusinessCoreClient {
    return this.businessCoreClient;
  }

  async shutdown(strict = false): Promise<void> {
    if (!this.shutdownPromise) {
      this.shuttingDown = true;
      this.shutdownPromise = (async () => {
        // Drain the currently running reconcile before teardown. This prevents an
        // in-flight local/cloud transition from re-creating integrations after the
        // quit path has already started destroying them.
        await this.reconcileChain;

        this.stopBusinessIntegrations();
        await this.stopLocalPetIntegrations();
        getMeshCoordinator(this.getMainWindow).stop();
        await stopCore(strict);
      })();
    }
    try {
      await this.shutdownPromise;
    } catch (error) {
      this.shutdownPromise = null;
      throw error;
    }
  }
}
