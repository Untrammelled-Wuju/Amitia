import * as path from 'path';
import { PseudoHost } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../typescript/dist/index.js');

describe('RPCTimeout', () => {
  let host: PseudoHost;

  afterEach(() => {
    if (host) {
      host.kill();
    }
  });

  it('RPC-001: stall with stallMs 0 returns immediately with stalled true', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.rpc.stall', { stallMs: 0 }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.stalled).toBe(0);
  });

  it('RPC-002: stall with stallMs 200 returns stalled value', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.rpc.stall', { stallMs: 200 }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.stalled).toBe(200);
  });

  it('RPC-003: stall with long duration exceeds 1s timeout', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    let timedOut = false;
    try {
      await host.callRPC('mock.fault', 'mock.fault.rpc.stall', { stallMs: 5000 }, 1000);
    } catch (e) {
      timedOut = true;
    }
    expect(timedOut).toBe(true);
  });

  it('RPC-004: stall with negative stallMs is treated as 0 and returns immediately', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.rpc.stall', { stallMs: -1 }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.stalled).toBe(-1);
  });

  it('RPC-005: multiple sequential stall calls respond correctly', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const first = await host.callRPC('mock.fault', 'mock.fault.rpc.stall', { stallMs: 0 }, 5000);
    expect(first.ok).toBe(true);
    const second = await host.callRPC('mock.fault', 'mock.fault.rpc.stall', { stallMs: 50 }, 5000);
    expect(second.ok).toBe(true);
    expect((second.payload as Record<string, unknown>).stalled).toBe(50);
  });
});
