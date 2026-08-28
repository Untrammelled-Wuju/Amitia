import { BrowserWindow, type MenuItemConstructorOptions } from "electron";
import type {
  DesktopBackendTopology,
  BackendEndpoint,
} from "../shared/backend-topology";
import type { DeploymentModeConfig, RuntimeStatus } from "../shared/types";
import { resolveBackendTopology } from "../shared/backend-topology";
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
import { ChatStateSubscriber } from "./pet/chat-state-subscriber";
import { CharacterWatcher } from "./pet/character-watcher";
import { getMeshCoordinator } from "./device-mesh/coordinator";

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
  private chatStateSubscriber: ChatStateSubscriber | null = null;
  private characterWatcher: CharacterWatcher | null = null;

  private reconcileChain: Promise<void> = Promise.resolve();
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
    this.topology = resolveBackendTopology(initialConfig);
    this.businessCoreClient = new BusinessCoreClient(this.topology.businessCore.baseURL);
  }

  async reconcile(config: DeploymentModeConfig): Promise<RuntimeStatus> {
    if (this.shuttingDown) {
      return this.runtimeManager.getStatus();
    }

    const result: { status?: RuntimeStatus; error?: Error } = {};

    this.reconcileChain = this.reconcileChain.then(async () => {
      try {
        const desired = { ...config };
        this.currentConfig = desired;
        this.topology = resolveBackendTopology(desired);

        if (desired.mode === "local") {
          await this.reconcileLocalMode();
        } else {
          await this.reconcileCloudMode();
        }
      } catch (err) {
        console.error("[DeploymentLifecycle] reconcile失败:", err);
        result.error = err instanceof Error ? err : new Error(String(err));
      }
    });

    await this.reconcileChain;

    if (result.error) {
      this.runtimeManager.setStatus("failed", result.error.message);
    }

    return this.runtimeManager.getStatus();
  }

  private async reconcileLocalMode(): Promise<void> {
    await this.stopLegacyPetIntegrations();
    this.stopBusinessIntegrations();
    getMeshCoordinator(this.getMainWindow).stop();

    this.runtimeManager.setBusinessCoreStatus(
      "starting",
      this.topology.businessCore.baseURL,
    );

    await ensureCoreProfile("local");
    await this.probeLocalRuntime();
    await this.probeBusinessCore(this.topology.businessCore);

    await this.rebuildBusinessIntegrations(this.topology);

    await this.startLegacyPetIntegrations();
  }

  private async reconcileCloudMode(): Promise<void> {
    await this.stopLegacyPetIntegrations();
    this.stopBusinessIntegrations();

    const [coreResult] = await Promise.allSettled([
      ensureCoreProfile("device-agent"),
      this.probeBusinessCore(this.topology.businessCore),
    ]);

    if (coreResult.status === "rejected") {
      console.error("[DeploymentLifecycle] device-agent启动失败:", coreResult.reason);
      this.runtimeManager.setLocalRuntimeStatus(
        "failed",
        this.topology.localRuntime.baseURL,
        "device-agent",
        "本机执行节点启动失败",
      );
    } else {
      await this.probeLocalRuntime();
    }

    await this.rebuildBusinessIntegrations(this.topology);

    const coordinator = getMeshCoordinator(this.getMainWindow);
    coordinator.start();
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
      const client = new BusinessCoreClient(businessCore.baseURL);
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

    this.businessCoreClient = new BusinessCoreClient(topology.businessCore.baseURL);

    this.desktopHostManager = new DesktopHostManager(
      mainWindow,
      this.setExtensionTrayItems,
      this.businessCoreClient,
    );

    this.desktopSnapshotSync = new DesktopSnapshotSync(
      mainWindow,
      this.desktopHostManager,
      this.businessCoreClient,
    );
    this.desktopSnapshotSync.start();

    this.uiHostSSE = new UIHostSSE({
      businessCore: this.businessCoreClient,
      mainWindow: this.getMainWindow,
    });
    this.uiHostSSE.start();
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

  private async startLegacyPetIntegrations(): Promise<void> {
    if (this.currentConfig.mode !== "local" || this.shuttingDown) {
      return;
    }

    const mainWindow = this.getMainWindow();
    if (!mainWindow) return;

    this.chatStateSubscriber = new ChatStateSubscriber({
      coreHost: "127.0.0.1",
      corePort: 18899,
      onPayload: (payload) => {
        if (!this.desktopPetManager) return;
        try {
          this.desktopPetManager.handleChatStatePayload(payload);
        } catch (err) {
          console.warn("[DeploymentLifecycle] 转发聊天状态失败:", err);
        }
      },
    });

    this.characterWatcher = new CharacterWatcher({
      coreHost: "127.0.0.1",
      corePort: 18899,
      onActiveCharacterChanged: async (characterId) => {
        if (!this.desktopPetManager) return;
        try {
          await this.desktopPetManager.handleCharacterSwitched(characterId);
        } catch (err) {
          console.warn("[DeploymentLifecycle] 角色切换处理失败:", err);
        }
      },
    });

    try {
      await this.desktopPetManager.initialize();
    } catch (err) {
      console.warn("[DeploymentLifecycle] DesktopPetManager 初始化失败:", err);
    }

    const subscriber = this.chatStateSubscriber;
    const watcher = this.characterWatcher;
    const starts: Promise<unknown>[] = [];
    if (subscriber) {
      starts.push(
        subscriber.start().catch((err) => {
          console.warn("[DeploymentLifecycle] 聊天状态订阅启动失败:", err);
        }),
      );
    }
    if (watcher) {
      starts.push(
        watcher.start().catch((err) => {
          console.warn("[DeploymentLifecycle] 角色监听启动失败:", err);
        }),
      );
    }
    await Promise.all(starts);
  }

  private async stopLegacyPetIntegrations(): Promise<void> {
    const subscriber = this.chatStateSubscriber;
    const watcher = this.characterWatcher;
    this.chatStateSubscriber = null;
    this.characterWatcher = null;

    subscriber?.stop();
    watcher?.stop();

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

  async shutdown(): Promise<void> {
    if (!this.shutdownPromise) {
      this.shuttingDown = true;
      this.shutdownPromise = (async () => {
        // Drain the currently running reconcile before teardown. This prevents an
        // in-flight local/cloud transition from re-creating integrations after the
        // quit path has already started destroying them.
        await this.reconcileChain;

        this.stopBusinessIntegrations();
        await this.stopLegacyPetIntegrations();
        getMeshCoordinator(this.getMainWindow).stop();
        await stopCore();
      })();
    }
    await this.shutdownPromise;
  }
}
