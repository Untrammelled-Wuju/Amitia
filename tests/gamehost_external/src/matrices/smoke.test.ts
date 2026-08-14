import * as path from 'path';
import { ExternalE2EHarness } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../testplugins/mock-amitiax-game-plugin/dist/index.js');

describe('G34 Smoke', () => {
  it('plugin performs handshake and reaches ready state', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    expect(harness.isRunning()).toBe(true);
    expect(harness.isHandshakeDone()).toBe(false);

    await new Promise(resolve => setTimeout(resolve, 500));

    expect(harness.isHandshakeDone()).toBe(true);
    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('echo rpc returns message with counter', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.echo', { message: 'hello' }, 5000);
    expect(resp.payload).toBeDefined();
    const payload = resp.payload as any;
    expect(payload.message).toBe('hello');
    expect(payload.count).toBe(1);

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('status rpc returns full state snapshot', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.status', {}, 5000);
    expect(resp.payload).toBeDefined();
    const payload = resp.payload as any;
    expect(payload.pluginVersion).toBe('1.0.0');
    expect(payload.status).toBe('ok');
    expect(payload.state).toBeDefined();

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('control authority snapshot returns valid state', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.control.authority.snapshot', {}, 5000);
    expect(resp.payload).toBeDefined();
    const payload = resp.payload as any;
    expect(payload.mode).toBe('observe');
    expect(payload.valid).toBe(true);

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);
});
