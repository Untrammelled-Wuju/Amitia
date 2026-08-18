import { GameCenterClient, createClient, GameRuntimeDetail, GamePluginSummary } from './game_center_client';
import { waitUntil, sleep, TimeoutError } from './waiters';
import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

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

export interface TargetResidueSnapshot {
  targetRuntimeId?: string;
  pluginCount: number;
  runtimeCount: number;
  connectionCount: number;
  handshakeCount: number;
  pendingRpcCount: number;
  channelCount: number;
  streamCount: number;
  binaryCount: number;
  secretLeaseCount: number;
  processCount: number;
  controlSinkCount: number;
  hostApiInflight: number;
  lifecycleIntent?: string;
  emergencyLatched: boolean;
}

export type ResidueTarget =
  | { type: 'extension'; extensionId: string }
  | { type: 'plugin'; pluginId: string }
  | { type: 'runtime'; runtimeId: string };

export class ResidueCheckError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ResidueCheckError';
  }
}

export class BackendDriver {
  private client: GameCenterClient;
  private knownRuntimeIdsByExtension = new Map<string, Set<string>>();
  private knownRuntimeIdsByPlugin = new Map<string, Set<string>>();

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

  async updatePlugin(extensionId: string, archivePath: string): Promise<unknown> {
    return this.client.updatePlugin(extensionId, archivePath);
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
    for (const runtime of resp.items) {
      if (runtime.extensionId) {
        const set = this.knownRuntimeIdsByExtension.get(runtime.extensionId) ?? new Set<string>();
        set.add(runtime.runtimeId);
        this.knownRuntimeIdsByExtension.set(runtime.extensionId, set);
      }
      if (runtime.pluginId) {
        const set = this.knownRuntimeIdsByPlugin.get(runtime.pluginId) ?? new Set<string>();
        set.add(runtime.runtimeId);
        this.knownRuntimeIdsByPlugin.set(runtime.pluginId, set);
      }
    }
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

  async waitForPluginVersion(
    extensionId: string,
    expectedVersion: string,
    options: { waitForChange?: boolean; timeoutMs?: number } = {}
  ): Promise<GamePluginSummary> {
    const timeoutMs = options.timeoutMs ?? 30000;
    return new Promise<GamePluginSummary>(async (resolve, reject) => {
      const deadline = Date.now() + timeoutMs;
      while (Date.now() < deadline) {
        try {
          const plugins = await this.listPlugins({ search: extensionId });
          const found = plugins.find(p => p.extensionId === extensionId);
          if (found) {
            if (options.waitForChange) {
              if (found.version !== expectedVersion) {
                resolve(found);
                return;
              }
            } else {
              if (found.version === expectedVersion) {
                resolve(found);
                return;
              }
            }
          }
        } catch {
          // transient
        }
        await sleep(200);
      }
      reject(new TimeoutError(`plugin ${extensionId} version wait timed out within ${timeoutMs}ms`));
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

  async assertZeroResidueForExtension(extensionId: string): Promise<void> {
    return this.assertZeroResidueFor({ type: 'extension', extensionId });
  }

  async assertZeroResidueForPlugin(pluginId: string): Promise<void> {
    return this.assertZeroResidueFor({ type: 'plugin', pluginId });
  }

  async assertZeroResidueForRuntime(runtimeId: string): Promise<void> {
    return this.assertZeroResidueFor({ type: 'runtime', runtimeId });
  }

  async assertZeroResidueFor(target: ResidueTarget): Promise<void> {
    const [plugins, runtimes] = await Promise.all([this.listPlugins(), this.listRuntimes()]);
    let targetPlugins = plugins.filter(() => false);
    let targetRuntimes = runtimes.filter(() => false);
    const runtimeIds = new Set<string>();

    switch (target.type) {
      case 'extension':
        targetPlugins = plugins.filter(p => p.extensionId === target.extensionId);
        targetRuntimes = runtimes.filter(r => (r as any).extensionId === target.extensionId);
        for (const id of this.knownRuntimeIdsByExtension.get(target.extensionId) ?? []) runtimeIds.add(id);
        break;
      case 'plugin':
        targetPlugins = plugins.filter(p => p.pluginId === target.pluginId);
        targetRuntimes = runtimes.filter(r => (r as any).pluginId === target.pluginId);
        for (const id of this.knownRuntimeIdsByPlugin.get(target.pluginId) ?? []) runtimeIds.add(id);
        break;
      case 'runtime':
        targetRuntimes = runtimes.filter(r => (r as any).runtimeId === target.runtimeId);
        runtimeIds.add(target.runtimeId);
        break;
    }
    for (const runtime of targetRuntimes) runtimeIds.add((runtime as any).runtimeId);

    const parts: string[] = [];
    if (targetPlugins.length) parts.push(`${targetPlugins.length} plugins remain`);
    if (targetRuntimes.length) parts.push(`${targetRuntimes.length} runtimes remain`);

    for (const runtimeId of runtimeIds) {
      const residue = await this.client.get<TargetResidueSnapshot>('/game-center-debug/residue', { runtimeId });
      const countFields: Array<[keyof TargetResidueSnapshot, number]> = [
        ['runtimeCount', residue.runtimeCount], ['connectionCount', residue.connectionCount],
        ['handshakeCount', residue.handshakeCount], ['pendingRpcCount', residue.pendingRpcCount],
        ['channelCount', residue.channelCount], ['streamCount', residue.streamCount],
        ['binaryCount', residue.binaryCount], ['secretLeaseCount', residue.secretLeaseCount],
        ['processCount', residue.processCount], ['controlSinkCount', residue.controlSinkCount],
        ['hostApiInflight', residue.hostApiInflight],
      ];
      for (const [name, value] of countFields) if (value > 0) parts.push(`${runtimeId}:${String(name)}=${value}`);
      if (residue.emergencyLatched) parts.push(`${runtimeId}:emergencyLatched=true`);
      if (residue.lifecycleIntent && residue.lifecycleIntent !== 'stopped' && residue.lifecycleIntent !== 'none') {
        parts.push(`${runtimeId}:lifecycleIntent=${residue.lifecycleIntent}`);
      }
    }

    if (parts.length) {
      const targetDesc = target.type === 'extension'
        ? `extension ${target.extensionId}`
        : target.type === 'plugin' ? `plugin ${target.pluginId}` : `runtime ${target.runtimeId}`;
      throw new ResidueCheckError(`residue [${targetDesc}]: ${parts.join(', ')}`);
    }
  }

  async assertRecoveredRuntimeUnique(runtimeId: string): Promise<void> {
    const residue = await this.client.get<TargetResidueSnapshot>('/game-center-debug/residue', { runtimeId });
    const problems: string[] = [];
    if (residue.runtimeCount !== 1) problems.push(`runtimeCount=${residue.runtimeCount}`);
    if (residue.processCount !== 1) problems.push(`processCount=${residue.processCount}`);
    if (residue.connectionCount !== 1) problems.push(`connectionCount=${residue.connectionCount}`);
    if (residue.handshakeCount !== 1) problems.push(`handshakeCount=${residue.handshakeCount}`);
    if (residue.pendingRpcCount !== 0) problems.push(`pendingRpcCount=${residue.pendingRpcCount}`);
    if (residue.hostApiInflight !== 0) problems.push(`hostApiInflight=${residue.hostApiInflight}`);
    if (problems.length) {
      throw new ResidueCheckError(`restart uniqueness [runtime ${runtimeId}]: ${problems.join(', ')}`);
    }
  }

  async restartBackend(target?: { extensionId?: string; runtimeId?: string }): Promise<void> {
    const command = process.env.GAMEHOST_BACKEND_RESTART_COMMAND;
    if (!command) {
      throw new Error('GAMEHOST_BACKEND_RESTART_COMMAND is required; workstation-specific process discovery is forbidden');
    }
    const targetExtensionId = target?.extensionId;
    const targetRuntimeId = target?.runtimeId;

    await execAsync(command, {
      cwd: process.env.GAMEHOST_BACKEND_CWD || process.cwd(),
      env: process.env,
      shell: process.env.ComSpec || process.env.SHELL,
      timeout: 60000,
    });

    const recoveryDeadline = Date.now() + 60000;
    while (Date.now() < recoveryDeadline) {
      try {
        await this.listPlugins();
        if (targetRuntimeId) {
          const runtimes = await this.listRuntimes();
          const matching = runtimes.filter(r => (r as any).runtimeId === targetRuntimeId);
          if (matching.length !== 1) throw new Error(`restart recovery: expected exactly one target runtime, got ${matching.length}`);
          await this.waitForRuntimeReady(targetRuntimeId, 5000);
          await this.assertRecoveredRuntimeUnique(targetRuntimeId);
        }
        if (targetExtensionId) {
          const plugins = await this.listPlugins();
          const matching = plugins.filter(p => p.extensionId === targetExtensionId);
          if (matching.length !== 1) throw new Error(`restart recovery: expected exactly one target plugin, got ${matching.length}`);
        }
        return;
      } catch {
        await sleep(1000);
      }
    }
    throw new Error('backend did not recover after restart within 60s');
  }

}

export function createDriver(options?: BackendDriverOptions): BackendDriver {
  return new BackendDriver(options);
}
