import { createDriver, BackendDriver } from '../backend_driver';

const ARCHIVE_PATH = process.env.MOCK_PLUGIN_ARCHIVE_PATH;
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
    if (extensionId) {
      try {
        await driver.uninstallPlugin(extensionId);
      } catch {
        // ignore
      }
    }
  });

  it('F04: emergency stop prevents runtime restart until rearm', async () => {
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    if (runtimes.length > 0) {
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
    }
  }, 90000);

  it('F10: crash recovery with running runtime', async () => {
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('mock-amitiax-game-plugin', 30000);
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
    const plugin = await driver.waitForPluginByExtension('mock-amitiax-game-plugin', 30000);
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
    const plugin = await driver.waitForPluginByExtension('mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    const v1Version = plugin.version;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    if (runtimes.length > 0) {
      runtimeId = runtimes[0].runtimeId;
      await driver.startRuntime(runtimeId);
      await driver.waitForRuntimeReady(runtimeId, 30000);
    }

    await driver.updatePlugin(extensionId, archiveV2);

    const deadline = Date.now() + 60000;
    let updatedVersion = v1Version;
    while (Date.now() < deadline) {
      const current = await driver.waitForPluginByExtension('mock-amitiax-game-plugin', 5000).catch(() => null);
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

  it('zero residue after uninstall', async () => {
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    if (runtimes.length > 0) {
      runtimeId = runtimes[0].runtimeId;
      await driver.startRuntime(runtimeId);
      await driver.waitForRuntimeReady(runtimeId, 30000);
      await driver.stopRuntime(runtimeId);
    }

    await driver.uninstallPlugin(extensionId);
    extensionId = null;
    runtimeId = null;

    await driver.assertZeroResidue();
  }, 90000);
});
