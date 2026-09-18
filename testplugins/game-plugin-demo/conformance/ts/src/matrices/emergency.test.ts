import * as path from 'path';
import { PseudoHost } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../typescript/dist/index.js');

describe('Emergency', () => {
  let host: PseudoHost;

  afterEach(() => {
    if (host) {
      host.kill();
    }
  });

  it('EMRG-001: emergency notification is received and acknowledged', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    await host.sendNotification('mock.fault', 'host.authority.emergency', { reason: 'fire' });
    const resp = await host.callRPC('mock.fault', 'mock.fault.residue.counts', {}, 5000);
    expect(resp.ok).toBe(true);
  });

  it('EMRG-002: emergency notification does not prevent subsequent RPCs', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    await host.sendNotification('mock.fault', 'host.authority.emergency', { reason: 'intrusion' });
    const echo = await host.callRPC('mock.core', 'mock.core.echo', { message: 'post-emergency' }, 5000);
    expect(echo.ok).toBe(true);
    expect((echo.payload as Record<string, unknown>).message).toBe('post-emergency');
  });

  it('EMRG-003: multiple emergency notifications are handled', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    await host.sendNotification('mock.fault', 'host.authority.emergency', { reason: 'first' });
    await host.sendNotification('mock.fault', 'host.authority.emergency', { reason: 'second' });
    const resp = await host.callRPC('mock.core', 'mock.core.echo', { message: 'multi-emergency' }, 5000);
    expect(resp.ok).toBe(true);
    expect((resp.payload as Record<string, unknown>).message).toBe('multi-emergency');
  });

  it('EMRG-004: emergency notification with empty payload is handled', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    await host.sendNotification('mock.fault', 'host.authority.emergency', {});
    const resp = await host.callRPC('mock.core', 'mock.core.echo', { message: 'empty-emergency' }, 5000);
    expect(resp.ok).toBe(true);
  });

  it('EMRG-005: secret probe after emergency notification returns valid result', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    await host.sendNotification('mock.fault', 'host.authority.emergency', { reason: 'test' });
    const resp = await host.callRPC('mock.fault', 'mock.fault.secret.probe', {}, 5000);
    expect(resp.ok).toBe(true);
  });
});
