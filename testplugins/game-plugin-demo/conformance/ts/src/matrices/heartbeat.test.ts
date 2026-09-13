import * as path from 'path';
import { PseudoHost } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../typescript/dist/index.js');

describe('Heartbeat', () => {
  let host: PseudoHost;

  afterEach(() => {
    if (host) {
      host.kill();
    }
  });

  it('HB-001: pause heartbeat with pauseMs 500 returns paused true', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.heartbeat.pause', { pauseMs: 500 }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.paused).toBe(true);
    expect(p.pauseMs).toBe(500);
  });

  it('HB-002: resume heartbeat after pause returns paused false', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const pauseResp = await host.callRPC('mock.fault', 'mock.fault.heartbeat.pause', { pauseMs: 2000 }, 5000);
    expect(pauseResp.ok).toBe(true);
    expect((pauseResp.payload as Record<string, unknown>).paused).toBe(true);

    const resumeResp = await host.callRPC('mock.fault', 'mock.fault.heartbeat.resume', {}, 5000);
    expect(resumeResp.ok).toBe(true);
    const p = resumeResp.payload as Record<string, unknown>;
    expect(p.paused).toBe(false);
  });

  it('HB-003: drop heartbeat with count 5 returns dropped count and remaining', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.heartbeat.drop', { count: 5 }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.dropped).toBe(5);
    expect(p.remaining).toBe(5);
  });

  it('HB-004: pause heartbeat with pauseMs 0 returns paused true indefinite', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.fault', 'mock.fault.heartbeat.pause', { pauseMs: 0 }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.paused).toBe(true);
    expect(p.pauseMs).toBe(0);
  });

  it('HB-005: drop heartbeat counts accumulate across multiple calls', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const first = await host.callRPC('mock.fault', 'mock.fault.heartbeat.drop', { count: 3 }, 5000);
    expect(first.ok).toBe(true);
    expect((first.payload as Record<string, unknown>).remaining).toBe(3);

    const second = await host.callRPC('mock.fault', 'mock.fault.heartbeat.drop', { count: 4 }, 5000);
    expect(second.ok).toBe(true);
    expect((second.payload as Record<string, unknown>).remaining).toBe(7);
  });

  it('HB-006: pause then resume then pause again works correctly', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const firstPause = await host.callRPC('mock.fault', 'mock.fault.heartbeat.pause', { pauseMs: 1000 }, 5000);
    expect(firstPause.ok).toBe(true);
    expect((firstPause.payload as Record<string, unknown>).paused).toBe(true);

    const resume = await host.callRPC('mock.fault', 'mock.fault.heartbeat.resume', {}, 5000);
    expect(resume.ok).toBe(true);
    expect((resume.payload as Record<string, unknown>).paused).toBe(false);

    const secondPause = await host.callRPC('mock.fault', 'mock.fault.heartbeat.pause', { pauseMs: 2000 }, 5000);
    expect(secondPause.ok).toBe(true);
    const p = secondPause.payload as Record<string, unknown>;
    expect(p.paused).toBe(true);
    expect(p.pauseMs).toBe(2000);
  });
});
