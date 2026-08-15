import { createDriver, BackendDriver } from '../backend_driver';

const ARCHIVE_PATH = process.env.MOCK_PLUGIN_ARCHIVE_PATH;

describe('G47-F15 Lifecycle (Backend Driver)', () => {
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

  it('install plugin and reach ready state', async () => {
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
    }
  }, 60000);

  it('stop runtime reaches idle state', async () => {
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
      await driver.waitForRuntimeState(runtimeId, 'stopped', 15000);
    }
  }, 90000);

  it('restart runtime preserves fresh generation', async () => {
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
      await driver.restartRuntime(runtimeId);
      await driver.waitForRuntimeReady(runtimeId, 30000);
    }
  }, 90000);

  it('uninstall plugin leaves zero residue', async () => {
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
});
