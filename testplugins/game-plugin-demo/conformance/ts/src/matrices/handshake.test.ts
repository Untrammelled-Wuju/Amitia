import * as path from 'path';
import { PseudoHost } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../typescript/dist/index.js');

describe('Handshake', () => {
  let host: PseudoHost;

  afterEach(() => {
    if (host) {
      host.kill();
    }
  });

  it('HS-001: configure handshake delay 200ms dropCount 0 returns configured state', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.handshake.delay', { delayMs: 200, dropCount: 0 }, 5000);
    expect(resp.ok).toBe(true);
    expect(resp.payload).toMatchObject({ handshakeDelayMs: 200, handshakeDropCount: 0 });
  });

  it('HS-002: configure handshake delay 0ms dropCount 3 returns configured state', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.handshake.delay', { delayMs: 0, dropCount: 3 }, 5000);
    expect(resp.ok).toBe(true);
    expect(resp.payload).toMatchObject({ handshakeDelayMs: 0, handshakeDropCount: 3 });
  });

  it('HS-003: configure handshake delay 500ms dropCount 1 echoes values', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.handshake.delay', { delayMs: 500, dropCount: 1 }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.handshakeDelayMs).toBe(500);
    expect(p.handshakeDropCount).toBe(1);
  });

  it('HS-004: configure handshake delay 0ms dropCount 0 returns zero values', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.handshake.delay', { delayMs: 0, dropCount: 0 }, 5000);
    expect(resp.ok).toBe(true);
    expect(resp.payload).toMatchObject({ handshakeDelayMs: 0, handshakeDropCount: 0 });
  });

  it('HS-005: subsequent handshake config overrides previous values', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const first = await host.callRPC('mock.fault', 'mock.fault.handshake.delay', { delayMs: 100, dropCount: 10 }, 5000);
    expect(first.ok).toBe(true);
    expect(first.payload).toMatchObject({ handshakeDelayMs: 100, handshakeDropCount: 10 });

    const second = await host.callRPC('mock.fault', 'mock.fault.handshake.delay', { delayMs: 50, dropCount: 5 }, 5000);
    expect(second.ok).toBe(true);
    expect(second.payload).toMatchObject({ handshakeDelayMs: 50, handshakeDropCount: 5 });
  });
});
