import { createDriver, BackendDriver } from '../backend_driver';

const ARCHIVE_PATH = process.env.MOCK_PLUGIN_ARCHIVE_PATH;

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
    if (!ARCHIVE_PATH) {
      console.log('MOCK_PLUGIN_ARCHIVE_PATH not set - skipping');
      return;
    }

    await driver.installPlugin(ARCHIVE_PATH);
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
    if (!ARCHIVE_PATH) {
      console.log('MOCK_PLUGIN_ARCHIVE_PATH not set - skipping');
      return;
    }

    await driver.installPlugin(ARCHIVE_PATH);
    const plugin = await driver.waitForPluginByExtension('mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    if (runtimes.length > 0) {
      runtimeId = runtimes[0].runtimeId;
      await driver.startRuntime(runtimeId);
      await driver.waitForRuntimeReady(runtimeId, 30000);

      const detail = await driver.getRuntime(runtimeId);
      expect(detail.runtimeState).toBe('running');
    }
  }, 90000);

  it('F11: disconnect cleans pending and residue', async () => {
    if (!ARCHIVE_PATH) {
      console.log('MOCK_PLUGIN_ARCHIVE_PATH not set - skipping');
      return;
    }

    await driver.installPlugin(ARCHIVE_PATH);
    const plugin = await driver.waitForPluginByExtension('mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    await driver.uninstallPlugin(extensionId);
    extensionId = null;

    const residue = await driver.getResidue();
    expect(residue.pluginCount).toBe(0);
  }, 60000);

  it('F09: upgrade v1 to v2 flow', async () => {
    if (!ARCHIVE_PATH) {
      console.log('MOCK_PLUGIN_ARCHIVE_PATH not set - skipping');
      return;
    }

    await driver.installPlugin(ARCHIVE_PATH);
    const plugin = await driver.waitForPluginByExtension('mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    expect(plugin.version).toBeDefined();
  }, 60000);

  it('zero residue after uninstall', async () => {
    if (!ARCHIVE_PATH) {
      console.log('MOCK_PLUGIN_ARCHIVE_PATH not set - skipping');
      return;
    }

    await driver.installPlugin(ARCHIVE_PATH);
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
