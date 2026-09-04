import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { BackendDriver } from '../backend_driver';
import { GameCenterClient } from '../game_center_client';

const EXTENSION_ID = 'com.mock-developer/mock-amitiax-game-plugin';
const ARCHIVE_PATH = process.env.MOCK_PLUGIN_REQUIRED_ARTIFACT_ARCHIVE_PATH;
const ARCHIVE_PATH_V2 = process.env.MOCK_PLUGIN_REQUIRED_ARTIFACT_ARCHIVE_PATH_V2;

function requiredArchive(): string {
  if (!ARCHIVE_PATH) throw new Error('MOCK_PLUGIN_REQUIRED_ARTIFACT_ARCHIVE_PATH is required');
  return ARCHIVE_PATH;
}

function requiredArchiveV2(): string {
  if (!ARCHIVE_PATH_V2) throw new Error('MOCK_PLUGIN_REQUIRED_ARTIFACT_ARCHIVE_PATH_V2 is required');
  return ARCHIVE_PATH_V2;
}


describe('Required artifact runtime-start lifecycle gate', () => {
  let client: GameCenterClient;
  let driver: BackendDriver;
  let runtimeId: string | null = null;
  let targetRoot: string | null = null;
  let installed = false;

  beforeEach(() => {
    client = new GameCenterClient();
    driver = new BackendDriver({ client });
  });

  afterEach(async () => {
    if (runtimeId) {
      await client.stopRuntime(runtimeId).catch(() => undefined);
    }
    if (targetRoot) {
      await client.removeArtifact(EXTENSION_ID, 'mock-companion-file', targetRoot).catch(() => undefined);
      await client.revokeArtifactRoot(EXTENSION_ID, targetRoot).catch(() => undefined);
    }
    if (installed) {
      await client.uninstallPlugin(EXTENSION_ID).catch(() => undefined);
    }
    if (targetRoot) await rm(targetRoot, { recursive: true, force: true });
    runtimeId = null;
    targetRoot = null;
    installed = false;
  });

  it('fails closed without a grant, then auto-deploys the required artifact before any service starts', async () => {
    await client.installPlugin(requiredArchive());
    installed = true;
    const plugin = await driver.waitForPluginByExtension(EXTENSION_ID, 30_000);
    await client.enablePlugin(EXTENSION_ID);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;

    await expect(client.startRuntime(runtimeId)).rejects.toThrow();
    const blocked = await client.getRuntime(runtimeId);
    expect(['created', 'stopped']).toContain(blocked.runtimeState);
    expect(blocked.process?.running ?? false).toBe(false);

    targetRoot = await mkdtemp(join(tmpdir(), 'amitia-required-artifact-e2e-'));
    const grant = await client.authorizeArtifactRoot(EXTENSION_ID, targetRoot);
    expect(grant.extensionId).toBe(EXTENSION_ID);
    expect((grant.generation ?? '').length).toBeGreaterThan(0);

    await client.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30_000);

    // No deploy endpoint is called here. A healthy record proves Runtime start
    // invoked the host-managed required-artifact deployment prerequisite.
    const artifact = await client.verifyArtifact(EXTENSION_ID, 'mock-companion-file', targetRoot);
    expect(artifact.installed).toBe(true);
    expect(artifact.healthy).toBe(true);
    expect((artifact.installedHash ?? '').length).toBeGreaterThan(0);

    // Upgrade while the runtime is active. The upgrade coordinator must carry
    // the exact management-authorized root to the new package generation before
    // it auto-resumes the runtime, and the runtime-start prerequisite must deploy
    // required artifacts without any plugin-side authorization shortcut.
    await client.updatePlugin(EXTENSION_ID, requiredArchiveV2());
    const upgraded = await driver.waitForPluginVersion(EXTENSION_ID, '1.1.0', { timeoutMs: 60_000 });
    const upgradedRuntimes = await driver.listRuntimes({ pluginId: upgraded.pluginId });
    expect(upgradedRuntimes.length).toBeGreaterThan(0);
    runtimeId = upgradedRuntimes[0].runtimeId;
    await driver.waitForRuntimeReady(runtimeId, 60_000);
    const upgradedArtifact = await client.verifyArtifact(EXTENSION_ID, 'mock-companion-file', targetRoot);
    expect(upgradedArtifact.installed).toBe(true);
    expect(upgradedArtifact.healthy).toBe(true);

    await client.stopRuntime(runtimeId);
    await client.removeArtifact(EXTENSION_ID, 'mock-companion-file', targetRoot);
    await client.revokeArtifactRoot(EXTENSION_ID, targetRoot);
    await client.uninstallPlugin(EXTENSION_ID);
    installed = false;
    runtimeId = null;

    await driver.assertZeroResidueForExtension(EXTENSION_ID);
  }, 120_000);
});
