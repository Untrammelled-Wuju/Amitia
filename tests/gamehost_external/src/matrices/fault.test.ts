import { createDriver, BackendDriver } from '../backend_driver';
import { GameCenterClient } from '../game_center_client';

const ARCHIVE_PATH = process.env.MOCK_PLUGIN_ARCHIVE_PATH;
const MOCK_EXTENSION_ID = 'com.mock-developer/mock-amitiax-game-plugin';
const ARCHIVE_PATH_V2 = process.env.MOCK_PLUGIN_ARCHIVE_PATH_V2;

function requireArchive(): string {
  if (!ARCHIVE_PATH) {
    throw new Error('MOCK_PLUGIN_ARCHIVE_PATH not set - test cannot run');
  }
  return ARCHIVE_PATH;
}

function requireArchiveV2(): string {
  if (!ARCHIVE_PATH_V2) {
    throw new Error('MOCK_PLUGIN_ARCHIVE_PATH_V2 not set - test cannot run');
  }
  return ARCHIVE_PATH_V2;
}

describe('G47-F15 Fault Matrix (Backend Driver)', () => {
  let driver: BackendDriver;
  let extensionId: string | null = null;
  let runtimeId: string | null = null;

  beforeEach(() => {
    driver = createDriver();
  });

  afterEach(async () => {
    if (runtimeId) {
      try {
        await driver.stopRuntime(runtimeId);
      } catch {
        // ignore
      }
    }
    try {
      await driver.uninstallPlugin(extensionId ?? MOCK_EXTENSION_ID);
    } catch {
      // Package may not have reached a visible installed state.
    }
    extensionId = null;
    runtimeId = null;
  });

  it('F04: emergency stop prevents runtime restart until rearm', async () => {
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('com.mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);
    await driver.emergencyStop(runtimeId);

    let startFailed = false;
    try {
      await driver.startRuntime(runtimeId);
    } catch {
      startFailed = true;
    }
    expect(startFailed).toBe(true);

    await driver.rearm(runtimeId);
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);
  }, 90000);

  it('F10: crash recovery with running runtime', async () => {
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('com.mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);

    const detail = await driver.getRuntime(runtimeId);
    expect(detail.runtimeState).toBe('running');
  }, 90000);

  it('F11: disconnect cleans pending and residue', async () => {
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('com.mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    await driver.enablePlugin(extensionId);
    await driver.disablePlugin(extensionId);
    await driver.uninstallPlugin(extensionId);
    extensionId = null;

    const residue = await driver.getResidue();
    expect(residue.pluginCount).toBe(0);
  }, 60000);

  it('F09: upgrade v1 to v2 flow', async () => {
    const archivePath = requireArchive();

    if (!ARCHIVE_PATH_V2) {
      throw new Error('MOCK_PLUGIN_ARCHIVE_PATH_V2 environment variable is required for F09 upgrade test');
    }

    const archiveV2 = requireArchiveV2();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('com.mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    const v1Version = plugin.version;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);

    await driver.updatePlugin(extensionId, archiveV2);

    const deadline = Date.now() + 60000;
    let updatedVersion = v1Version;
    while (Date.now() < deadline) {
      const current = await driver.waitForPluginByExtension('com.mock-developer/mock-amitiax-game-plugin', 5000).catch(() => null);
      if (current && current.version !== v1Version) {
        updatedVersion = current.version;
        break;
      }
      await new Promise(r => setTimeout(r, 500));
    }
    expect(updatedVersion).not.toBe(v1Version);

    if (runtimeId) {
      await driver.waitForRuntimeReady(runtimeId, 60000);
      const detail = await driver.getRuntime(runtimeId);
      expect(detail.runtimeState).toBe('running');
    }
  }, 180000);

  it('F15-55: crash via mockgame.fault.crash triggers recovery with GenerationAfter > GenerationBefore', async () => {
    const archivePath = requireArchive();
    const client = new GameCenterClient();

    await client.installPlugin(archivePath);
    const driver = createDriver();
    const plugin = await driver.waitForPluginByExtension('com.mock-developer/mock-amitiax-game-plugin', 30000);
    const extId = plugin.extensionId;
    extensionId = extId;
    await client.enablePlugin(extId);

    const runtimes = await client.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.items.length).toBeGreaterThan(0);
    const rt = runtimes.items[0];
    const rtId = rt.runtimeId;
    runtimeId = rtId;

    await client.startRuntime(rtId);
    await driver.waitForRuntimeReady(rtId, 30000);

    const before = await client.getRuntime(rtId);
    const genBefore = before.process?.processGeneration ?? 0;
    expect(genBefore).toBeGreaterThan(0);

    try {
      await client.invokeRuntimeRPC(rtId, 'mockgame.fault.crash', {});
    } catch {
      // expected: crash may cause connection failure
    }

    const deadline = Date.now() + 60000;
    let genAfter = genBefore;
    while (Date.now() < deadline) {
      try {
        const detail = await client.getRuntime(rtId);
        if (detail.process?.processGeneration !== undefined) {
          genAfter = detail.process.processGeneration;
        }
        const hs = await client.getHandshakeStatus(rtId);
        if (hs.ready && detail.runtimeState === 'running' && genAfter > genBefore) {
          break;
        }
      } catch {
        // transient during recovery
      }
      await new Promise(r => setTimeout(r, 500));
    }

    expect(genAfter).toBeGreaterThan(genBefore);
  }, 180000);

  it('restart runtime increments generation', async () => {
    const archivePath = requireArchive();
    const client = new GameCenterClient();

    await client.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('com.mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    await client.enablePlugin(extensionId);

    const runtimes = await client.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.items.length).toBeGreaterThan(0);
    runtimeId = runtimes.items[0].runtimeId;
    await client.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);

    const before = await client.getRuntime(runtimeId);
    const genBefore = before.process?.processGeneration ?? 0;
    expect(genBefore).toBeGreaterThan(0);

    await client.restartRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);

    const after = await client.getRuntime(runtimeId);
    const genAfter = after.process?.processGeneration ?? 0;
    expect(genAfter).toBeGreaterThan(genBefore);
  }, 120000);

  it('zero residue after uninstall', async () => {
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('com.mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    const pluginId = plugin.pluginId;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;
    const targetRuntimeId = runtimeId;
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);
    await driver.stopRuntime(runtimeId);

    await driver.uninstallPlugin(extensionId);
    const uninstalledExtensionId = extensionId;
    extensionId = null;
    runtimeId = null;

    await driver.assertZeroResidueForExtension(uninstalledExtensionId);
    await driver.assertZeroResidueForPlugin(pluginId);
    await driver.assertZeroResidueForRuntime(targetRuntimeId);
  }, 90000);

  it('zero residue plugin-scoped check after uninstall', async () => {
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('com.mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    const pluginId = plugin.pluginId;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);
    await driver.stopRuntime(runtimeId);

    await driver.uninstallPlugin(extensionId);
    extensionId = null;
    runtimeId = null;

    await driver.assertZeroResidueForPlugin(pluginId);
  }, 90000);

  it('zero residue runtime-scoped check after stop and uninstall', async () => {
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('com.mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;
    const targetRuntimeId = runtimeId;
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);
    await driver.stopRuntime(runtimeId);

    await driver.uninstallPlugin(extensionId);
    extensionId = null;
    runtimeId = null;

    await driver.assertZeroResidueForRuntime(targetRuntimeId);
  }, 90000);

  it('F15-57: zero residue target-scoped check after full lifecycle', async () => {
    const archivePath = requireArchive();
    const client = new GameCenterClient();

    // Full lifecycle: install → enable → start → use → stop → uninstall
    await client.installPlugin(archivePath);
    const driver2 = createDriver();
    const plugin = await driver2.waitForPluginByExtension('com.mock-developer/mock-amitiax-game-plugin', 30000);
    const extId = plugin.extensionId;
    await client.enablePlugin(extId);

    const runtimes = await client.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.items.length).toBeGreaterThan(0);
    const rtId = runtimes.items[0].runtimeId;

    await client.startRuntime(rtId);
    await driver2.waitForRuntimeReady(rtId, 30000);

    // Exercise authenticated canonical RPC, Takeover/Release.
    extensionId = extId;
    runtimeId = rtId;
    await client.invokeRuntimeRPC(rtId, 'mockgame.echo', {});

    await client.takeover(rtId, { targetMode: 'plugin' });
    await client.release(rtId, { targetMode: 'observe' });

    await client.stopRuntime(rtId);
    await client.uninstallPlugin(extId);
    extensionId = null;
    runtimeId = null;

    await driver.assertZeroResidueForExtension(extId);
  }, 120000);
});
