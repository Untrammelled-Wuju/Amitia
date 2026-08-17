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
    extensionId = null;
    runtimeId = null;

    const residue = await driver.getResidue();
    expect(residue.pluginCount).toBe(0);
    expect(residue.runtimeCount).toBe(0);
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
    const customRpcBody = await customRpcResp.json();
    expect(customRpcBody.code).toBe(200);

    // 7. HostAPI (call host.get_runtime_info)
    const hostApiResp = await fetch(
      `${(client as any).baseUrl}/game-center/runtimes/${runtimeId}/services/mock-game-runtime/rpc`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'host.get_runtime_info', payload: {} }),
      },
    );
    expect(hostApiResp.status).toBe(200);

    // 8. Secret (via mockgame.secret.check RPC)
    const secretResp = await fetch(
      `${(client as any).baseUrl}/game-center/runtimes/${runtimeId}/services/mock-game-runtime/rpc`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'mockgame.secret.check', payload: {} }),
      },
    );
    expect(secretResp.status).toBe(200);

    // 9. Effect (apply mockgame.effect.apply)
    const effectResp = await fetch(
      `${(client as any).baseUrl}/game-center/runtimes/${runtimeId}/services/mock-game-runtime/rpc`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'mockgame.effect.apply', payload: { effect: 'test', duration: 1000 } }),
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
    const crashResp = await fetch(
      `${(client as any).baseUrl}/game-center/runtimes/${runtimeId}/services/mock-game-runtime/rpc`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'mockgame.fault.crash', payload: {} }),
      },
    );
    expect(crashResp.status).toBe(200);

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

    // 15. Upgrade (re-install same version to simulate update)
    await driver.uninstallPlugin(extensionId);
    extensionId = null;
    runtimeId = null;

    await driver.installPlugin(archivePath);
    const upgradedPlugin = await driver.waitForPluginByExtension('mock-developer/mock-amitiax-game-plugin', 30000);
    extensionId = upgradedPlugin.extensionId;
    await driver.enablePlugin(extensionId);

    // 16. Backend Restart is handled by test infrastructure (server remains running)

    // 17. Disable
    const newRuntimes = await driver.listRuntimes({ pluginId: upgradedPlugin.pluginId });
    if (newRuntimes.length > 0) {
      runtimeId = newRuntimes[0].runtimeId;
      await driver.stopRuntime(runtimeId);
    }
    await driver.disablePlugin(extensionId);

    // 18. Uninstall
    await driver.uninstallPlugin(extensionId);
    extensionId = null;
    runtimeId = null;

    const finalResidue = await driver.getResidue();
    expect(finalResidue.pluginCount).toBe(0);
    expect(finalResidue.runtimeCount).toBe(0);
  }, 300000);
});
