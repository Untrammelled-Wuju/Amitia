import { createDriver, BackendDriver } from '../backend_driver';

const ARCHIVE_PATH = process.env.MOCK_PLUGIN_ARCHIVE_PATH;

function requireArchive(): string {
  if (!ARCHIVE_PATH) {
    throw new Error('MOCK_PLUGIN_ARCHIVE_PATH environment variable is required for F15 security tests');
  }
  return ARCHIVE_PATH;
}

describe('G47-F15 Security (Backend Driver)', () => {
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

  it('install enable and acquire control', async () => {
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

    await driver.takeover(runtimeId, 'plugin');
    const detailAfterTakeover = await driver.getRuntime(runtimeId);
    expect(detailAfterTakeover.controlAuthority?.mode).toBe('plugin');

    await driver.release(runtimeId, 'observe');
    const detailAfterRelease = await driver.getRuntime(runtimeId);
    expect(detailAfterRelease.controlAuthority?.mode).toBe('observe');
  }, 90000);

  it('emergency stop and rearm flow', async () => {
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

    await driver.emergencyStop(runtimeId);
    const detailAfterEstop = await driver.getRuntime(runtimeId);
    expect(detailAfterEstop.runtimeState).toBe('stopped');

    await driver.rearm(runtimeId);
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);
  }, 90000);

  it('disable plugin prevents runtime start', async () => {
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
    await driver.stopRuntime(runtimeId);

    await driver.disablePlugin(extensionId);

    await expect(driver.startRuntime(runtimeId)).rejects.toThrow();
  }, 90000);
});
