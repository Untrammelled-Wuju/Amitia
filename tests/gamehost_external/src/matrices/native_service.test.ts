import { BackendDriver } from '../backend_driver';
import { GameCenterClient } from '../game_center_client';

const EXTENSION_ID = 'com.example/mock-game-plugin-go';
const SERVICE_ID = 'mock-go-runtime';
const V1 = process.env.NATIVE_PLUGIN_ARCHIVE_PATH;
const V2 = process.env.NATIVE_PLUGIN_ARCHIVE_PATH_V2;

function requirePath(value: string | undefined, name: string): string {
  if (!value) throw new Error(`${name} is required for native service E2E`);
  return value;
}

function nativeClient(): GameCenterClient {
  const session = process.env.GAMEHOST_NATIVE_DEVELOPER_SESSION_ID;
  if (!session) throw new Error('GAMEHOST_NATIVE_DEVELOPER_SESSION_ID is required');
  return new GameCenterClient(
    process.env.GAMEHOST_BASE_URL,
    process.env.GAMEHOST_AUTH_TOKEN,
    session,
  );
}

describe('Native service package production chain', () => {
  let client: GameCenterClient;
  let driver: BackendDriver;
  let extensionInstalled = false;
  let runtimeId: string | null = null;

  beforeEach(() => {
    client = nativeClient();
    driver = new BackendDriver({ client });
  });

  afterEach(async () => {
    if (runtimeId) {
      try { await client.stopRuntime(runtimeId); } catch { /* already stopped/crashed */ }
    }
    if (extensionInstalled) {
      try { await client.uninstallPlugin(EXTENSION_ID); } catch { /* surface original assertion */ }
    }
    runtimeId = null;
    extensionInstalled = false;
  });

  it('packages, installs, starts, reaches Ready, serves RPC, recovers crash, upgrades, and uninstalls', async () => {
    const v1 = requirePath(V1, 'NATIVE_PLUGIN_ARCHIVE_PATH');
    const v2 = requirePath(V2, 'NATIVE_PLUGIN_ARCHIVE_PATH_V2');

    await client.installPlugin(v1);
    extensionInstalled = true;
    let plugin = await driver.waitForPluginByExtension(EXTENSION_ID, 30_000);
    expect(plugin.version).toBe('0.3.0');

    await client.enablePlugin(EXTENSION_ID);
    let runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;

    await client.startRuntime(runtimeId);
    const ready = await driver.waitForRuntimeReady(runtimeId, 30_000);
    expect(ready.runtimeState).toBe('running');

    const echo = await client.invokeRuntimeRPC<{ message: string; count: number }>(
      runtimeId,
      'mock.core.echo',
      { message: 'native-service-package-e2e' },
      30_000,
      SERVICE_ID,
    );
    expect(echo.message).toBe('native-service-package-e2e');
    expect(echo.count).toBeGreaterThan(0);

    const beforeCrash = await client.getRuntime(runtimeId);
    const generationBefore = beforeCrash.process?.processGeneration ?? 0;
    expect(generationBefore).toBeGreaterThan(0);
    try {
      await client.invokeRuntimeRPC(runtimeId, 'mock.fault.crash', { exitCode: 17 }, 30_000, SERVICE_ID);
    } catch {
      // The process is intentionally terminated while the RPC is in flight.
    }

    const deadline = Date.now() + 60_000;
    let generationAfter = generationBefore;
    while (Date.now() < deadline) {
      try {
        const detail = await client.getRuntime(runtimeId);
        const handshake = await client.getHandshakeStatus(runtimeId);
        generationAfter = detail.process?.processGeneration ?? generationAfter;
        if (detail.runtimeState === 'running' && handshake.ready && generationAfter > generationBefore) break;
      } catch {
        // Transient while the recovery coordinator replaces the crashed process.
      }
      await new Promise(resolve => setTimeout(resolve, 500));
    }
    expect(generationAfter).toBeGreaterThan(generationBefore);
    await driver.assertRecoveredRuntimeUnique(runtimeId);

    await client.updatePlugin(EXTENSION_ID, v2);
    plugin = await driver.waitForPluginVersion(EXTENSION_ID, '0.4.0', { timeoutMs: 60_000 });
    expect(plugin.version).toBe('0.4.0');

    runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;
    // The upgrade coordinator owns recovery of runtimes that were running before
    // the extension update. Waiting here deliberately avoids a second StartRuntime
    // racing a legitimate starting/restarting transition and verifies auto-resume.
    const postUpgradeReady = await driver.waitForRuntimeReady(runtimeId, 60_000);
    expect(postUpgradeReady.runtimeState).toBe('running');

    const upgradedEcho = await client.invokeRuntimeRPC<{ message: string }>(
      runtimeId,
      'mock.core.echo',
      { message: 'native-service-upgraded' },
      30_000,
      SERVICE_ID,
    );
    expect(upgradedEcho.message).toBe('native-service-upgraded');

    const finalRuntimeId = runtimeId;
    const finalPluginId = plugin.pluginId;
    await client.stopRuntime(runtimeId);
    await client.uninstallPlugin(EXTENSION_ID);
    extensionInstalled = false;
    runtimeId = null;

    await driver.assertZeroResidueForExtension(EXTENSION_ID);
    await driver.assertZeroResidueForPlugin(finalPluginId);
    await driver.assertZeroResidueForRuntime(finalRuntimeId);
  }, 300_000);
});
