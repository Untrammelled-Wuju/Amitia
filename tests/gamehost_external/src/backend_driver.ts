import { GameCenterClient, createClient, GameRuntimeDetail, GamePluginSummary } from './game_center_client';
import { waitUntil, sleep, TimeoutError } from './waiters';
import { exec, spawn } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

export { TimeoutError } from './waiters';
export type { ResidueTarget };
export { ResidueCheckError };

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
    const [plugins, runtimes] = await Promise.all([
      this.listPlugins(),
      this.listRuntimes(),
    ]);

    let targetPlugins: typeof plugins = [];
    let targetRuntimes: typeof runtimes = [];

    switch (target.type) {
      case 'extension':
        targetPlugins = plugins.filter(p => p.extensionId === target.extensionId);
        targetRuntimes = runtimes.filter(r => (r as any).extensionId === target.extensionId);
        break;
      case 'plugin':
        targetPlugins = plugins.filter(p => p.pluginId === target.pluginId);
        targetRuntimes = runtimes.filter(r => (r as any).pluginId === target.pluginId);
        break;
      case 'runtime':
        targetRuntimes = runtimes.filter(r => (r as any).runtimeId === target.runtimeId);
        break;
    }

    const parts: string[] = [];
    if (targetPlugins.length > 0) {
      parts.push(`${targetPlugins.length} plugins remain`);
    }
    if (targetRuntimes.length > 0) {
      parts.push(`${targetRuntimes.length} runtimes remain`);
    }
    if (parts.length > 0) {
      const targetDesc = target.type === 'extension'
        ? `extension ${target.extensionId}`
        : target.type === 'plugin'
          ? `plugin ${target.pluginId}`
          : `runtime ${target.runtimeId}`;
      throw new ResidueCheckError(`residue [${targetDesc}]: ${parts.join(', ')}`);
    }
  }

  async restartBackend(): Promise<void> {
    const pluginsBefore = await this.listPlugins();
    const runtimesBefore = await this.listRuntimes();

    const backendPid = await this.findBackendProcess();
    if (!backendPid) {
      throw new Error('backend process (server.exe) not found - cannot restart');
    }

    await this.killProcess(backendPid);

    const stopDeadline = Date.now() + 15000;
    while (Date.now() < stopDeadline) {
      const stillRunning = await this.isProcessAlive(backendPid);
      if (!stillRunning) {
        break;
      }
      await sleep(500);
    }

    const stillRunningAfterWait = await this.isProcessAlive(backendPid);
    if (stillRunningAfterWait) {
      throw new Error(`backend process (PID=${backendPid}) did not terminate within 15s`);
    }

    await this.startBackend();

    const recoveryDeadline = Date.now() + 60000;
    let recovered = false;
    while (Date.now() < recoveryDeadline) {
      try {
        const pluginsAfter = await this.listPlugins();
        const runtimesAfter = await this.listRuntimes();
        if (pluginsAfter.length >= pluginsBefore.length && runtimesAfter.length >= runtimesBefore.length) {
          recovered = true;
          break;
        }
      } catch {
        // transient during restart
      }
      await sleep(1000);
    }
    if (!recovered) {
      throw new Error('backend did not recover after restart within 60s');
    }
  }

  private async findBackendProcess(): Promise<number | null> {
    try {
      const { stdout } = await execAsync('tasklist /FI "IMAGENAME eq server.exe" /FO CSV /NH');
      const lines = stdout.split('\n').filter(line => line.trim() && line.includes('server.exe'));
      if (lines.length > 0) {
        const parts = lines[0].split(',');
        if (parts.length >= 2) {
          const pid = parseInt(parts[1].replace(/"/g, '').trim(), 10);
          if (!isNaN(pid) && pid > 0) {
            return pid;
          }
        }
      }
    } catch {
      // fallback to port-based detection
    }

    try {
      const { stdout } = await execAsync('netstat -ano | findstr ":18899"');
      const lines = stdout.split('\n').filter(line => line.includes('LISTENING'));
      if (lines.length > 0) {
        const parts = lines[0].trim().split(/\s+/);
        const pid = parseInt(parts[parts.length - 1], 10);
        if (!isNaN(pid) && pid > 0) {
          return pid;
        }
      }
    } catch {
      // ignore
    }

    return null;
  }

  private async killProcess(pid: number): Promise<void> {
    try {
      await execAsync(`taskkill /F /PID ${pid}`);
    } catch (err: any) {
      if (err.message && err.message.includes('not found')) {
        return;
      }
      throw err;
    }
  }

  private async isProcessAlive(pid: number): Promise<boolean> {
    try {
      const { stdout } = await execAsync(`tasklist /FI "PID eq ${pid}" /FO CSV /NH`);
      return stdout.includes(String(pid));
    } catch {
      return false;
    }
  }

  private async startBackend(): Promise<void> {
    const backendDir = process.env.BACKEND_DIR || 'D:\\桌面\\跟进项目\\U-Ai\\backend';
    const serverPath = `${backendDir}\\server.exe`;

    spawn(serverPath, [], {
      cwd: backendDir,
      detached: true,
      stdio: 'ignore',
      windowsHide: true,
    }).unref();

    const startDeadline = Date.now() + 30000;
    while (Date.now() < startDeadline) {
      try {
        await this.listPlugins();
        return;
      } catch {
        // not ready yet
      }
      await sleep(1000);
    }
    throw new Error('backend did not start within 30s');
  }
}

export function createDriver(options?: BackendDriverOptions): BackendDriver {
  return new BackendDriver(options);
}
