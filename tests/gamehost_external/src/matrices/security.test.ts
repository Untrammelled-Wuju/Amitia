import * as path from 'path';
import { ExternalE2EHarness } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../testplugins/mock-amitiax-game-plugin/dist/index.js');

describe('G36 Security', () => {
  it('permission check returns pending state (host decides)', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.permission.check', { permissionId: 'gamehost.control' }, 5000);
    expect((resp.payload as any).permissionId).toBe('gamehost.control');

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('permission snapshot returns valid structure', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.permission.snapshot', {}, 5000);
    expect((resp.payload as any).isValid).toBe(true);
    expect((resp.payload as any).snapshotId).toBeDefined();

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('hostapi check returns pending invoke result', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.hostapi.check', { method: 'host.runtime.health' }, 5000);
    expect((resp.payload as any).method).toBe('host.runtime.health');

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('secret probe returns lease status', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.secret.probe', {}, 5000);
    expect((resp.payload as any).hasLease).toBe(false);
    expect((resp.payload as any).ref).toBe('secret://mock_provider_token');

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('control sink register returns registered', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.control.sink.register', { sinkId: 'mockgame.effect', kind: 'effect' }, 5000);
    expect((resp.payload as any).registered).toBe(true);
    expect((resp.payload as any).sinkId).toBe('mockgame.effect');

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('control output in observe mode denied', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.control.output', { outputId: 'test-001', sinkId: 'mockgame.effect', epoch: 0 }, 5000);
    expect((resp.payload as any).allowed).toBe(false);

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('takeover returns expected structure', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.control.takeover', { targetMode: 'plugin', actor: 'test' }, 5000);
    expect((resp.payload as any).targetMode).toBe('plugin');
    expect((resp.payload as any).actor).toBe('test');

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('release returns expected structure', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.control.release', { targetMode: 'observe', actor: 'test' }, 5000);
    expect((resp.payload as any).targetMode).toBe('observe');
    expect((resp.payload as any).actor).toBe('test');

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('emergency status returns inactive by default', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.control.emergency_status', {}, 5000);
    expect((resp.payload as any).active).toBe(false);
    expect((resp.payload as any).state).toBe('inactive');

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);
});
