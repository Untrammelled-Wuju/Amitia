import { BackendDriver, createDriver } from './backend_driver';
import { GameCenterClient, createClient } from './game_center_client';
import { waitUntil, sleep, TimeoutError, AbortError, WaitOptions } from './waiters';

export { waitUntil, sleep, TimeoutError, AbortError };
export type { WaitOptions };
export { BackendDriver, createDriver, GameCenterClient, createClient };

/**
 * @deprecated Use BackendDriver instead. The original ExternalPluginHarness
 * implemented a simplified fake GameHost protocol and spawned plugin processes
 * directly. For real E2E validation, the BackendDriver connects to the real
 * backend via Game Center HTTP API on port 18899, where the plugin is launched
 * exclusively by the existing trusted_service.ProcessSupervisor through the
 * canonical RuntimeManager → GameHost → ControlPlane path.
 *
 * All legacy methods below throw to prevent accidental misuse.
 */
export class ExternalPluginHarness {
  private driver: BackendDriver;

  constructor(_pluginPath?: string) {
    this.driver = createDriver();
  }

  get backendDriver(): BackendDriver {
    return this.driver;
  }

  async start(): Promise<void> {
    await this.driver.listPlugins();
  }

  async callRPC(_method: string, _payload: unknown = {}, _timeoutMs: number = 5000): Promise<any> {
    throw new Error('ExternalPluginHarness.callRPC is deprecated. Use BackendDriver with real Game Center API.');
  }

  async sendNotification(_method: string, _payload: unknown = {}): Promise<void> {
    throw new Error('ExternalPluginHarness.sendNotification is deprecated. Use BackendDriver with real Game Center API.');
  }

  kill(): void {
    throw new Error('ExternalPluginHarness.kill is deprecated. Use BackendDriver with real Game Center API.');
  }

  softKill(): void {
    throw new Error('ExternalPluginHarness.softKill is deprecated. Use BackendDriver with real Game Center API.');
  }

  async waitExit(_timeoutMs: number = 5000): Promise<number> {
    throw new Error('ExternalPluginHarness.waitExit is deprecated. Use BackendDriver with real Game Center API.');
  }

  getExitCode(): number | null {
    throw new Error('ExternalPluginHarness.getExitCode is deprecated. Use BackendDriver with real Game Center API.');
  }

  getGeneration(): number {
    throw new Error('ExternalPluginHarness.getGeneration is deprecated. Use BackendDriver with real Game Center API.');
  }

  isRunning(): boolean {
    throw new Error('ExternalPluginHarness.isRunning is deprecated. Use BackendDriver with real Game Center API.');
  }

  isHandshakeDone(): boolean {
    throw new Error('ExternalPluginHarness.isHandshakeDone is deprecated. Use BackendDriver with real Game Center API.');
  }

  getResponseCount(): number {
    return 0;
  }

  getReceivedNotifications(): unknown[] {
    return [];
  }

  async restart(): Promise<void> {
    throw new Error('ExternalPluginHarness.restart is deprecated. Use BackendDriver with real Game Center API.');
  }

  getResidue(): { processRunning: boolean; handshakeDone: boolean; responseCount: number; pendingCount: number } {
    throw new Error('ExternalPluginHarness.getResidue is deprecated. Use BackendDriver with real Game Center API.');
  }
}

export function createHarness(pluginPath?: string): ExternalPluginHarness {
  return new ExternalPluginHarness(pluginPath);
}

/**
 * @deprecated Use BackendDriver instead.
 */
export type ExternalE2EHarness = ExternalPluginHarness;
