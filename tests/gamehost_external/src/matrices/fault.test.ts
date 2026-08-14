import * as path from 'path';
import { ExternalE2EHarness } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../testplugins/mock-amitiax-game-plugin/dist/index.js');

describe('G37 Fault Matrix', () => {
  it('F01: plugin process crash exits with expected code', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    await harness.callRPC('mockgame.echo', { message: 'before-crash' }, 5000);

    let exitCode: number | null = null;
    try {
      await harness.callRPC('mockgame.fault.crash', { exitCode: 42, delayMs: 100 }, 3000);
    } catch {
      // expected - plugin will exit
    }

    try {
      exitCode = await harness.waitExit(5000);
    } catch {
      harness.kill();
      exitCode = await harness.waitExit(3000);
    }

    expect(exitCode).toBe(42);
  }, 15000);

  it('F03: wrong protocol fixture triggers handshake reject', async () => {
    const wrongProtoPath = path.resolve(__dirname, '../../../../testplugins/mock-amitiax-game-plugin/fixtures/wrong-protocol.js');
    const harness = new ExternalE2EHarness(wrongProtoPath);
    await harness.start();

    await new Promise(resolve => setTimeout(resolve, 1000));

    expect(harness.isHandshakeDone()).toBe(false);

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('F05: malformed frame causes connection failure', async () => {
    const malformedPath = path.resolve(__dirname, '../../../../testplugins/mock-amitiax-game-plugin/fixtures/malformed-frame.js');
    const harness = new ExternalE2EHarness(malformedPath);
    await harness.start();

    await new Promise(resolve => setTimeout(resolve, 1500));

    expect(harness.isRunning()).toBe(false);

    harness.kill();
  }, 10000);

  it('F09: long task timeout is handled', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    await expect(
      harness.callRPC('mockgame.long_task', { durationMs: 10000 }, 2000)
    ).rejects.toThrow('rpc timeout');

    harness.kill();
    await harness.waitExit(3000);
  }, 15000);

  it('F10: fail rpc throws expected error', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const resp = await harness.callRPC('mockgame.fail', { errorType: 'test', message: 'intentional test failure' }, 5000);
    expect(resp.error).toBeDefined();
    expect(resp.error!.message).toContain('intentional test failure');

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('F11: disconnect while pending cleans up', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const pendingCall = harness.callRPC('mockgame.long_task', { durationMs: 5000 }, 10000);

    await new Promise(resolve => setTimeout(resolve, 200));
    harness.kill();

    try {
      await pendingCall;
    } catch {
      // expected - connection closed
    }

    await harness.waitExit(3000);
    expect(harness.isRunning()).toBe(false);
  }, 15000);

  it('F12: restart during pending cancels old pending', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    const pendingCall = harness.callRPC('mockgame.long_task', { durationMs: 5000 }, 10000).catch(() => 'cancelled');

    await new Promise(resolve => setTimeout(resolve, 200));
    harness.kill();
    await harness.waitExit(3000);

    await harness.start();
    expect(harness.isRunning()).toBe(true);
    expect(harness.isHandshakeDone()).toBe(true);

    const result = await pendingCall;
    expect(result).toBe('cancelled');

    harness.kill();
    await harness.waitExit(3000);
  }, 20000);

  it('fault reset clears fault state', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    await harness.callRPC('mockgame.fault.delay', { delayMs: 50 }, 5000);
    const resetResp = await harness.callRPC('mockgame.fault.reset', {}, 5000);
    expect((resetResp.payload as any).reset).toBe(true);

    const statusResp = await harness.callRPC('mockgame.fault.status', {}, 5000);
    expect((statusResp.payload as any).exitRequested).toBe(false);
    expect((statusResp.payload as any).delayMs).toBe(0);

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('drop next response causes rpc timeout', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    await harness.callRPC('mockgame.fault.drop_next_response', { count: 1 }, 5000);

    await expect(
      harness.callRPC('mockgame.echo', { message: 'will-be-dropped' }, 2000)
    ).rejects.toThrow('rpc timeout');

    harness.kill();
    await harness.waitExit(3000);
  }, 15000);

  it('host authority emergency notification is received', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    await harness.sendNotification('host.authority.emergency', { reason: 'test' });

    await new Promise(resolve => setTimeout(resolve, 200));

    const notifications = harness.getReceivedNotifications();
    expect(notifications.length).toBeGreaterThan(0);

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);

  it('host authority changed notification updates state', async () => {
    const harness = new ExternalE2EHarness(PLUGIN_PATH);
    await harness.start();

    await harness.sendNotification('host.authority.changed', { mode: 'plugin', epoch: 1 });

    await new Promise(resolve => setTimeout(resolve, 200));

    const resp = await harness.callRPC('mockgame.control.authority.snapshot', {}, 5000);
    expect((resp.payload as any).mode).toBe('plugin');
    expect((resp.payload as any).epoch).toBe(1);

    harness.kill();
    await harness.waitExit(3000);
  }, 10000);
});
