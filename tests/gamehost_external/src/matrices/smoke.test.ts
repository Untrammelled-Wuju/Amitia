import { createDriver, BackendDriver } from '../backend_driver';

const ARCHIVE_PATH = process.env.MOCK_PLUGIN_ARCHIVE_PATH;

function requireArchive(): string {
  if (!ARCHIVE_PATH) {
    throw new Error('MOCK_PLUGIN_ARCHIVE_PATH environment variable is required for F15 smoke tests');
  }
  return ARCHIVE_PATH;
}

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
    const archivePath = requireArchive();
    const result = await driver.installPlugin(archivePath);
    expect(result).toBeDefined();
  }, 30000);

  it('zero residue after fresh backend start', async () => {
    await driver.assertZeroResidue();
  }, 10000);
});
