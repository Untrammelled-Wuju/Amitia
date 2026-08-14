import * as path from 'path';
import { ExternalE2EHarness } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../testplugins/mock-amitiax-game-plugin/dist/index.js');

describe('G34-G35 Lifecycle', () => {
  it('runtime start command activates game runtime', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const startResp = await harness.callRPC('mockgame.command', { action: 'start' }, 5000);
    expect((startResp.payload as any).result).toBe('started');

    const statusResp = await harness.callRPC('mockgame.status', {}, 5000);
    expect((statusResp.payload as any).state.mode).toBe('running');

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('runtime stop command deactivates game runtime', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    await harness.callRPC('mockgame.command', { action: 'start' }, 5000);
    const stopResp = await harness.callRPC('mockgame.command', { action: 'stop' }, 5000);
    expect((stopResp.payload as any).result).toBe('stopped');

    const statusResp = await harness.callRPC('mockgame.status', {}, 5000);
    expect((statusResp.payload as any).state.mode).toBe('idle');

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('damage and heal commands affect health', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    await harness.callRPC('mockgame.command', { action: 'start' }, 5000);
    const dmgResp = await harness.callRPC('mockgame.command', { action: 'damage', value: 25 }, 5000);
    expect((dmgResp.payload as any).result).toBe('damaged');
    expect((dmgResp.payload as any).health).toBe(75);

    const healResp = await harness.callRPC('mockgame.command', { action: 'heal', value: 15 }, 5000);
    expect((healResp.payload as any).result).toBe('healed');
    expect((healResp.payload as any).health).toBe(90);

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('reset command restores initial state', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    await harness.callRPC('mockgame.command', { action: 'start' }, 5000);
    await harness.callRPC('mockgame.command', { action: 'damage', value: 30 }, 5000);
    const resetResp = await harness.callRPC('mockgame.command', { action: 'reset' }, 5000);
    expect((resetResp.payload as any).result).toBe('reset');

    const statusResp = await harness.callRPC('mockgame.status', {}, 5000);
    expect((statusResp.payload as any).state.health).toBe(100);
    expect((statusResp.payload as any).state.mode).toBe('idle');

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('restart cycle preserves fresh generation', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    await harness.callRPC('mockgame.echo', { message: 'before-restart' }, 5000);
    const gen1 = harness.getGeneration();
    expect(gen1).toBe(1);

    harness.kill();
    await harness.waitExit(3000);
    await harness.start();

    const gen2 = harness.getGeneration();
    expect(gen2).toBe(2);

    harness.kill();
    await harness.waitExit(3000);
  }, 15000);

  it('binary consume rpc returns success', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.binary.consume', { binaryId: 'test-bin-001', size: 1024 }, 5000);
    expect((resp.payload as any).consumed).toBe(true);
    expect((resp.payload as any).binaryId).toBe('test-bin-001');

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);
});
