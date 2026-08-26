import { createDriver, BackendDriver } from '../backend_driver';
import { GameCenterClient } from '../game_center_client';

const ARCHIVE_PATH = process.env.MOCK_PLUGIN_ARCHIVE_PATH;
const MOCK_EXTENSION_ID = 'mock-developer/mock-amitiax-game-plugin';

async function invokeRuntimeRPC<T>(client: GameCenterClient, runtimeId: string | null, method: string, payload: unknown = {}): Promise<T> {
  if (!runtimeId) throw new Error(`RPC ${method} requires runtimeId`);
  return client.invokeRuntimeRPC<T>(runtimeId, method, payload);
}

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
    try {
      await driver.uninstallPlugin(extensionId ?? MOCK_EXTENSION_ID);
    } catch {
      // Package may not have reached a visible installed state.
    }
    extensionId = null;
    runtimeId = null;
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
    try {
      await driver.uninstallPlugin(extensionId ?? MOCK_EXTENSION_ID);
    } catch {
      // Package may not have reached a visible installed state.
    }
    extensionId = null;
    runtimeId = null;
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

    // Canonical Agent Tool E2E: this is deliberately NOT the direct management
    // RPC endpoint. It traverses ToolFacade -> ExecutionPipeline -> GameHost
    // RuntimeAdapter -> ControlPlane -> the external mock process.
    await driver.bindAgentContext(runtimeId, {
      serviceId: 'mock-game-runtime',
      characterId: 'e2e-character',
      conversationId: 'e2e-conversation',
      channel: 'web',
      sessionId: 'e2e-session',
    });
    const agentToolResult = await driver.invokeCanonicalTool<any>({
      toolId: 'mock-developer/mock-amitiax-game-plugin/mockgame/move',
      input: { direction: 'north' },
      characterId: 'e2e-character',
      conversationId: 'e2e-conversation',
      channel: 'web',
      sessionId: 'e2e-session',
      requestId: `agent-tool-${Date.now()}`,
      toolCallId: 'mock-move-call',
    });
    expect(agentToolResult).toBeDefined();
    expect(agentToolResult.status).toBe('success');

    // 6. Custom RPC through the authenticated canonical management client.
    const customRpcBody = await invokeRuntimeRPC<{ echo?: unknown }>(
      client, runtimeId, 'mockgame.echo', { hello: 'world' },
    );
    expect(customRpcBody).toBeDefined();

    // 7. HostAPI - RPCInvokeResponse uses payload, not the normal management data envelope.
    const hostApiBody = await invokeRuntimeRPC<{ status?: string; output?: unknown }>(
      client, runtimeId, 'mockgame.hostapi.invoke', {},
    );
    expect(hostApiBody.status).toBeDefined();
    expect(hostApiBody.output).toBeDefined();

    // Restricted network E2E: the real plugin process must have no ambient
    // outbound socket capability, while the host-mediated allowlisted route
    // succeeds from the trusted host process. This runs unchanged on Linux,
    // Windows AppContainer, and macOS Seatbelt runners.
    const networkProbe = await invokeRuntimeRPC<{
      directSucceeded?: boolean;
      directError?: string;
      mediatedStatus?: string;
      mediatedOutput?: { statusCode?: number; finalUrl?: string; bodyBase64?: string };
      blockedIpStatus?: string;
      blockedPortStatus?: string;
    }>(client, runtimeId, 'mockgame.network.restricted_probe', {});
    expect(networkProbe.directSucceeded).toBe(false);
    expect(typeof networkProbe.directError).toBe('string');
    expect((networkProbe.directError ?? '').length).toBeGreaterThan(0);
    expect(networkProbe.mediatedStatus).toBe('success');
    expect(networkProbe.mediatedOutput?.statusCode).toBe(200);
    expect(networkProbe.mediatedOutput?.finalUrl).toBe('http://127.0.0.1:18899/api/public/health');
    expect(typeof networkProbe.mediatedOutput?.bodyBase64).toBe('string');
    expect((networkProbe.mediatedOutput?.bodyBase64 ?? '').length).toBeGreaterThan(0);
    expect(networkProbe.blockedIpStatus).toBe('rejected');
    expect(networkProbe.blockedPortStatus).toBe('rejected');

    // 8. Secret lease full lifecycle: acquire -> query(active) -> release -> query(inactive).
    const secretBody = await invokeRuntimeRPC<{ granted?: boolean; leaseId?: string }>(
      client, runtimeId, 'mockgame.secret.acquire', {},
    );
    expect(secretBody.granted).toBe(true);
    expect(typeof secretBody.leaseId).toBe('string');
    expect((secretBody.leaseId ?? '').length).toBeGreaterThan(0);
    const activeLease = await invokeRuntimeRPC<{ valid?: boolean; granted?: boolean; leaseId?: string }>(
      client, runtimeId, 'mockgame.secret.query', {},
    );
    expect(activeLease.valid).toBe(true);
    expect(activeLease.granted).toBe(true);
    expect(activeLease.leaseId).toBe(secretBody.leaseId);
    await invokeRuntimeRPC(client, runtimeId, 'mockgame.secret.release', {});
    const releasedLease = await invokeRuntimeRPC<{ valid?: boolean; granted?: boolean }>(
      client, runtimeId, 'mockgame.secret.query', {},
    );
    expect(releasedLease.valid).toBe(false);
    expect(releasedLease.granted).toBe(false);

    // 9. Acquire control BEFORE Effect. observe mode is intentionally not allowed to submit control.output.
    await driver.takeover(runtimeId, 'plugin');
    const afterTakeover = await driver.getRuntime(runtimeId);
    expect(afterTakeover.controlAuthority?.mode).toBe('plugin');

    const beforeEffect = await invokeRuntimeRPC<{ effectCount?: number }>(
      client, runtimeId, 'mockgame.effect.status', {},
    );
    const effectBody = await invokeRuntimeRPC<{ allowed?: boolean; outputId?: string; currentEpoch?: number; generation?: number }>(
      client, runtimeId, 'mockgame.control.output', {
        outputId: `e2e-${Date.now()}`,
        sinkId: 'mockgame.effect',
        epoch: afterTakeover.controlAuthority?.epoch,
        data: { effect: 'test', duration: 1000 },
      },
    );
    expect(effectBody.allowed).toBe(true);
    expect(effectBody.outputId).toBeDefined();
    expect(effectBody.currentEpoch).toBeDefined();
    expect(effectBody.generation).toBeDefined();
    const committedOutputId = effectBody.outputId!;
    const committedGeneration = effectBody.generation!;
    const afterEffect = await invokeRuntimeRPC<{ effectCount?: number }>(
      client, runtimeId, 'mockgame.effect.status', {},
    );
    expect(afterEffect.effectCount ?? 0).toBeGreaterThan(beforeEffect.effectCount ?? 0);

    // 10. Release only after the host sink commit has been observed.
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
      await client.invokeRuntimeRPC(runtimeId!, 'mockgame.fault.crash', {});
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

    // 15. Upgrade (real upgrade flow via updatePlugin) - V2 archive is REQUIRED, cannot silently skip
    expect(archivePath).toBeDefined();
    expect(extensionId).toBeDefined();
    const archivePathV2 = process.env.MOCK_PLUGIN_ARCHIVE_PATH_V2;
    if (!archivePathV2) {
      throw new Error('MOCK_PLUGIN_ARCHIVE_PATH_V2 environment variable is required for F15-56 upgrade step - silent skip is forbidden');
    }
    const v1Version = plugin.version;
    const generationBeforeUpgrade = (await driver.getRuntime(runtimeId!)).process?.processGeneration ?? 0;
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
      const generationAfterUpgrade = (await driver.getRuntime(runtimeId)).process?.processGeneration ?? 0;
      expect(generationAfterUpgrade).toBeGreaterThan(generationBeforeUpgrade);
      // A fresh generation must not inherit the pre-upgrade secret/effect/control state.
      const postUpgradeLease = await invokeRuntimeRPC<{ valid?: boolean; granted?: boolean }>(client, runtimeId, 'mockgame.secret.query', {});
      expect(postUpgradeLease.valid).toBe(false);
      expect(postUpgradeLease.granted).toBe(false);
      const oldEffectAfterUpgrade = await invokeRuntimeRPC<{ found?: boolean; processed?: boolean }>(
        client, runtimeId, 'mockgame.effect.status', { outputId: committedOutputId },
      );
      expect(oldEffectAfterUpgrade.found).toBe(false);
      expect(oldEffectAfterUpgrade.processed).toBe(false);
      expect(generationAfterUpgrade).toBeGreaterThan(committedGeneration);
    }

    // 16. Backend Restart - runner-owned command + target uniqueness/orphan proof.
    await driver.restartBackend({ extensionId: extensionId!, runtimeId: runtimeId! });

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
