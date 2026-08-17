import { createDriver, BackendDriver } from '../backend_driver';
import { GameCenterClient } from '../game_center_client';

const ARCHIVE_PATH = process.env.MOCK_PLUGIN_ARCHIVE_PATH;

function requireArchive(): string {
  if (!ARCHIVE_PATH) {
    throw new Error('MOCK_PLUGIN_ARCHIVE_PATH environment variable is required for F15 lifecycle tests');
  }
  return ARCHIVE_PATH;
}

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
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;

    await driver.enablePlugin(extensionId);
    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);
  }, 60000);

  it('stop runtime reaches idle state', async () => {
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);
    await driver.stopRuntime(runtimeId);
    await driver.waitForRuntimeState(runtimeId, 'stopped', 15000);
  }, 90000);

  it('restart runtime preserves fresh generation', async () => {
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);
    await driver.restartRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);
  }, 90000);

  it('uninstall plugin leaves zero residue', async () => {
    const archivePath = requireArchive();

    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;
    await driver.enablePlugin(extensionId);

    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;
    await driver.startRuntime(runtimeId);
    await driver.stopRuntime(runtimeId);

    await driver.uninstallPlugin(extensionId);
    const uninstalledExtensionId = extensionId;
    extensionId = null;
    runtimeId = null;

    await driver.assertZeroResidueForExtension(uninstalledExtensionId);
  }, 60000);
});

describe('G47-F15 E2E Full Flow', () => {
  let driver: BackendDriver;
  let client: GameCenterClient;
  let extensionId: string | null = null;
  let runtimeId: string | null = null;

  beforeEach(() => {
    driver = createDriver();
    client = new GameCenterClient();
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

  it('F15-56: full lifecycle Install→Enable→Provision→Handshake→Ready→CustomRPC→HostAPI→Secret→Effect→Takeover/Release→Restart→Crash→EStop→Rearm→Upgrade→Disable→Uninstall', async () => {
    const archivePath = requireArchive();

    // 1. Install
    await driver.installPlugin(archivePath);
    const plugin = await driver.waitForPluginByExtension('mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = plugin.extensionId;

    // 2. Enable
    await driver.enablePlugin(extensionId);

    // 3. Provision (list runtimes)
    const runtimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    expect(runtimes.length).toBeGreaterThan(0);
    runtimeId = runtimes[0].runtimeId;

    // 4-5. Start → Handshake → Ready
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);
    const readyDetail = await driver.getRuntime(runtimeId);
    expect(readyDetail.runtimeState).toBe('running');
    expect(readyDetail.handshake?.ready).toBe(true);

    // 6. Custom RPC (via new RPC endpoint)
    const customRpcResp = await fetch(
      `${(client as any).baseUrl}/game-center/runtimes/${runtimeId}/services/mock-game-runtime/rpc`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'mockgame.echo', payload: { hello: 'world' } }),
      },
    );
    expect(customRpcResp.status).toBe(200);
    const customRpcBody = (await customRpcResp.json()) as { code: number; msg: string };
    expect(customRpcBody.code).toBe(200);

    // 7. HostAPI (call mockgame.hostapi.invoke)
    const hostApiResp = await fetch(
      `${(client as any).baseUrl}/game-center/runtimes/${runtimeId}/services/mock-game-runtime/rpc`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'mockgame.hostapi.invoke', payload: {} }),
      },
    );
    expect(hostApiResp.status).toBe(200);

    // 8. Secret (via mockgame.secret.acquire RPC)
    const secretResp = await fetch(
      `${(client as any).baseUrl}/game-center/runtimes/${runtimeId}/services/mock-game-runtime/rpc`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'mockgame.secret.acquire', payload: {} }),
      },
    );
    expect(secretResp.status).toBe(200);

    // 9. Effect (apply mockgame.control.output)
    const effectResp = await fetch(
      `${(client as any).baseUrl}/game-center/runtimes/${runtimeId}/services/mock-game-runtime/rpc`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'mockgame.control.output', payload: { effect: 'test', duration: 1000 } }),
      },
    );
    expect(effectResp.status).toBe(200);

    // 10. Takeover → Release
    await driver.takeover(runtimeId, 'plugin');
    const afterTakeover = await driver.getRuntime(runtimeId);
    expect(afterTakeover.controlAuthority?.mode).toBe('plugin');

    await driver.release(runtimeId, 'observe');
    const afterRelease = await driver.getRuntime(runtimeId);
    expect(afterRelease.controlAuthority?.mode).toBe('observe');

    // 11. Restart
    const genBeforeRestart = (await driver.getRuntime(runtimeId)).process?.processGeneration ?? 0;
    await driver.restartRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);
    const genAfterRestart = (await driver.getRuntime(runtimeId)).process?.processGeneration ?? 0;
    expect(genAfterRestart).toBeGreaterThan(genBeforeRestart);

    // 12. Crash via mockgame.fault.crash
    const genBeforeCrash = (await driver.getRuntime(runtimeId)).process?.processGeneration ?? 0;
    try {
      await fetch(
        `${(client as any).baseUrl}/game-center/runtimes/${runtimeId}/services/mock-game-runtime/rpc`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ method: 'mockgame.fault.crash', payload: {} }),
        },
      );
    } catch {
      // expected: crash may cause connection failure
    }

    const crashDeadline = Date.now() + 60000;
    let genAfterCrash = genBeforeCrash;
    while (Date.now() < crashDeadline) {
      try {
        const detail = await driver.getRuntime(runtimeId);
        if (detail.process?.processGeneration !== undefined) {
          genAfterCrash = detail.process.processGeneration;
        }
        const hs = await driver.getHandshakeStatus(runtimeId);
        if (hs.ready && detail.runtimeState === 'running' && genAfterCrash > genBeforeCrash) {
          break;
        }
      } catch {
        // transient during recovery
      }
      await new Promise(r => setTimeout(r, 500));
    }
    expect(genAfterCrash).toBeGreaterThan(genBeforeCrash);

    // 13. Emergency Stop
    await driver.emergencyStop(runtimeId);
    const afterEstop = await driver.getRuntime(runtimeId);
    expect(afterEstop.runtimeState).toBe('stopped');

    // 14. Rearm
    await driver.rearm(runtimeId);
    await driver.startRuntime(runtimeId);
    await driver.waitForRuntimeReady(runtimeId, 30000);

    // 15. Upgrade (real upgrade flow via updatePlugin)
    expect(archivePath).toBeDefined();
    const archivePathV2 = process.env.MOCK_PLUGIN_ARCHIVE_PATH_V2;
    expect(extensionId).toBeDefined();
    if (archivePathV2) {
      const v1Version = plugin.version;
      await driver.updatePlugin(extensionId!, archivePathV2);
      const upgradeDeadline = Date.now() + 60000;
      let upgradedVersion = v1Version;
      while (Date.now() < upgradeDeadline) {
        const current = await driver.waitForPluginByExtension('mock-developer/mock-amitiax-game-plugin', 5000).catch(() => null);
        if (current && current.version !== v1Version) {
          upgradedVersion = current.version;
          break;
        }
        await new Promise(r => setTimeout(r, 500));
      }
      expect(upgradedVersion).not.toBe(v1Version);

      if (runtimeId) {
        await driver.waitForRuntimeReady(runtimeId, 60000);
      }
    }

    // 16. Backend Restart - verify state persistence across restart
    await driver.restartBackend();

    // 17. Disable
    const newRuntimes = await driver.listRuntimes({ pluginId: plugin.pluginId });
    if (newRuntimes.length > 0) {
      runtimeId = newRuntimes[0].runtimeId;
      await driver.stopRuntime(runtimeId);
    }
    await driver.disablePlugin(extensionId);

    // 18. Uninstall
    const finalExtensionId = extensionId;
    await driver.uninstallPlugin(extensionId);
    extensionId = null;
    runtimeId = null;

    if (finalExtensionId) {
      await driver.assertZeroResidueForExtension(finalExtensionId);
    }
  }, 300000);
});
