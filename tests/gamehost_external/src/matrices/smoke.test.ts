import { createDriver, BackendDriver } from '../backend_driver';

describe('G47-F15 Smoke (Backend Driver)', () => {
  let driver: BackendDriver;

  beforeEach(() => {
    driver = createDriver();
  });

  it('backend is reachable via game-center API', async () => {
    const plugins = await driver.listPlugins();
    expect(Array.isArray(plugins)).toBe(true);
  }, 10000);

  it('list runtimes returns array', async () => {
    const runtimes = await driver.listRuntimes();
    expect(Array.isArray(runtimes)).toBe(true);
  }, 10000);

  it('install plugin via API returns success', async () => {
    const archivePath = process.env.MOCK_PLUGIN_ARCHIVE_PATH;
    if (!archivePath) {
      console.log('MOCK_PLUGIN_ARCHIVE_PATH not set - skipping install test');
      return;
    }
    const result = await driver.installPlugin(archivePath);
    expect(result).toBeDefined();
  }, 30000);

  it('zero residue after fresh backend start', async () => {
    await driver.assertZeroResidue();
  }, 10000);
});
