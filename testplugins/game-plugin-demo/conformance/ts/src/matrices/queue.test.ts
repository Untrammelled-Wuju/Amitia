import * as path from 'path';
import { PseudoHost } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../typescript/dist/index.js');

describe('Queue', () => {
  let host: PseudoHost;

  afterEach(() => {
    if (host) {
      host.kill();
    }
  });

  it('QUEUE-001: queue fill with count 5 returns sent equal to requested', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.queue.fill', { count: 5 }, 10000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.sent).toBe(5);
    expect(p.requested).toBe(5);
  });

  it('QUEUE-002: queue fill with count 1 returns correct values', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.queue.fill', { count: 1 }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.sent).toBe(1);
    expect(p.requested).toBe(1);
  });

  it('QUEUE-003: queue fill with count 0 returns 0 sent', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.queue.fill', { count: 0 }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.sent).toBe(0);
    expect(p.requested).toBe(0);
  });

  it('QUEUE-004: queue fill with count 10 returns all sent', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.queue.fill', { count: 10 }, 15000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.sent).toBe(10);
    expect(p.requested).toBe(10);
  });

  it('QUEUE-005: multiple queue fill calls accumulate correctly', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const first = await host.callRPC('mock.fault', 'mock.fault.queue.fill', { count: 3 }, 10000);
    expect(first.ok).toBe(true);
    expect((first.payload as Record<string, unknown>).sent).toBe(3);

    const second = await host.callRPC('mock.fault', 'mock.fault.queue.fill', { count: 2 }, 10000);
    expect(second.ok).toBe(true);
    expect((second.payload as Record<string, unknown>).sent).toBe(2);
  });
});
