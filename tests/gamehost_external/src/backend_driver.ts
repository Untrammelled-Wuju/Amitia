import { GameCenterClient, createClient, GameRuntimeDetail, GamePluginSummary } from './game_center_client';
import { waitUntil, sleep, TimeoutError } from './waiters';

export { TimeoutError } from './waiters';

export interface BackendDriverOptions {
  baseUrl?: string;
  client?: GameCenterClient;
}

export interface ResidueSnapshot {
  pluginCount: number;
  runtimeCount: number;
  runningRuntimes: number;
  readyRuntimes: number;
}

export class BackendDriver {
  private client: GameCenterClient;

  constructor(options: BackendDriverOptions = {}) {
    this.client = options.client ?? createClient(options.baseUrl);
  }

  get rawClient(): GameCenterClient {
    return this.client;
  }

  async installPlugin(archivePath: string): Promise<unknown> {
    return this.client.installPlugin(archivePath);
  }

  async enablePlugin(extensionId: string): Promise<unknown> {
    return this.client.enablePlugin(extensionId);
  }

  async disablePlugin(extensionId: string): Promise<unknown> {
    return this.client.disablePlugin(extensionId);
  }

  async uninstallPlugin(extensionId: string): Promise<unknown> {
    return this.client.uninstallPlugin(extensionId);
  }

  async startRuntime(runtimeId: string): Promise<unknown> {
    return this.client.startRuntime(runtimeId);
  }

  async stopRuntime(runtimeId: string): Promise<unknown> {
    return this.client.stopRuntime(runtimeId);
  }

  async restartRuntime(runtimeId: string): Promise<unknown> {
    return this.client.restartRuntime(runtimeId);
  }

  async takeover(runtimeId: string, targetMode: string = 'plugin', expectedEpoch?: number): Promise<unknown> {
    return this.client.takeover(runtimeId, { targetMode, expectedEpoch });
  }

  async release(runtimeId: string, targetMode: string = 'observe', expectedEpoch?: number): Promise<unknown> {
    return this.client.release(runtimeId, { targetMode, expectedEpoch });
  }

  async emergencyStop(runtimeId: string): Promise<unknown> {
    return this.client.emergencyStop(runtimeId);
  }

  async rearm(runtimeId: string): Promise<unknown> {
    return this.client.rearm(runtimeId);
  }

  async getRuntime(runtimeId: string, pluginId?: string): Promise<GameRuntimeDetail> {
    return this.client.getRuntime(runtimeId, pluginId);
  }

  async getHandshakeStatus(runtimeId: string): Promise<{ handshakeState: string; ready: boolean }> {
    return this.client.getHandshakeStatus(runtimeId);
  }

  async listPlugins(filter?: { search?: string; status?: string }): Promise<GamePluginSummary[]> {
    const resp = await this.client.listPlugins(filter);
    return resp.items;
  }

  async listRuntimes(filter?: { pluginId?: string; status?: string }): Promise<GameRuntimeDetail[]> {
    const resp = await this.client.listRuntimes(filter);
    return resp.items as unknown as GameRuntimeDetail[];
  }

  async waitForRuntimeReady(runtimeId: string, timeoutMs: number = 30000): Promise<GameRuntimeDetail> {
    return new Promise<GameRuntimeDetail>(async (resolve, reject) => {
      const deadline = Date.now() + timeoutMs;
      while (Date.now() < deadline) {
        try {
          const detail = await this.client.getRuntime(runtimeId);
          const hs = await this.client.getHandshakeStatus(runtimeId);
          if (hs.ready && detail.runtimeState === 'running') {
            resolve(detail);
            return;
          }
        } catch {
          // transient
        }
        await sleep(200);
      }
      reject(new TimeoutError(`runtime ${runtimeId} not ready within ${timeoutMs}ms`));
    });
  }

  async waitForRuntimeState(
    runtimeId: string,
    desiredState: string,
    timeoutMs: number = 15000
  ): Promise<GameRuntimeDetail> {
    return new Promise<GameRuntimeDetail>(async (resolve, reject) => {
      const deadline = Date.now() + timeoutMs;
      while (Date.now() < deadline) {
        try {
          const detail = await this.client.getRuntime(runtimeId);
          if (detail.runtimeState === desiredState) {
            resolve(detail);
            return;
          }
        } catch {
          // transient
        }
        await sleep(200);
      }
      reject(new TimeoutError(`runtime ${runtimeId} did not reach state ${desiredState} within ${timeoutMs}ms`));
    });
  }

  async waitForPluginByExtension(extensionId: string, timeoutMs: number = 15000): Promise<GamePluginSummary> {
    return new Promise<GamePluginSummary>(async (resolve, reject) => {
      const deadline = Date.now() + timeoutMs;
      while (Date.now() < deadline) {
        try {
          const plugins = await this.listPlugins({ search: extensionId });
          const found = plugins.find(p => p.extensionId === extensionId);
          if (found) {
            resolve(found);
            return;
          }
        } catch {
          // transient
        }
        await sleep(200);
      }
      reject(new TimeoutError(`plugin ${extensionId} not found within ${timeoutMs}ms`));
    });
  }

  async getResidue(): Promise<ResidueSnapshot> {
    const [plugins, runtimes] = await Promise.all([
      this.listPlugins(),
      this.listRuntimes(),
    ]);
    return {
      pluginCount: plugins.length,
      runtimeCount: runtimes.length,
      runningRuntimes: runtimes.filter(r => (r as any).runtimeState === 'running').length,
      readyRuntimes: runtimes.filter(r => (r as any).ready === true).length,
    };
  }

  async assertZeroResidue(): Promise<void> {
    const residue = await this.getResidue();
    if (residue.pluginCount > 0) {
      throw new Error(`residue: ${residue.pluginCount} plugins remain`);
    }
    if (residue.runtimeCount > 0) {
      throw new Error(`residue: ${residue.runtimeCount} runtimes remain`);
    }
    if (residue.runningRuntimes > 0) {
      throw new Error(`residue: ${residue.runningRuntimes} runtimes still running`);
    }
  }
}

export function createDriver(options?: BackendDriverOptions): BackendDriver {
  return new BackendDriver(options);
}
