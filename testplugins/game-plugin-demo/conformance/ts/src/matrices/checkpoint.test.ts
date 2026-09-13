import * as path from 'path';
import { PseudoHost } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../typescript/dist/index.js');

describe('Checkpoint', () => {
  let host: PseudoHost;

  afterEach(() => {
    if (host) {
      host.kill();
    }
  });

  it('CKPT-001: plugin responds to echo on fresh start', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.core', 'mock.core.echo', { message: 'hello' }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.message).toBe('hello');
    expect(p.count).toBe(1);
  });

  it('CKPT-002: restart increments generation', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const genBefore = host.getGeneration();
    await host.restart();
    const genAfter = host.getGeneration();
    expect(genAfter).toBeGreaterThan(genBefore);
  });

  it('CKPT-003: plugin responds correctly after restart', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    await host.restart();
    const resp = await host.callRPC('mock.core', 'mock.core.echo', { message: 'after-restart' }, 5000);
    expect(resp.ok).toBe(true);
    expect((resp.payload as Record<string, unknown>).message).toBe('after-restart');
  });

  it('CKPT-004: initial echo count starts at 1 on fresh start', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.core', 'mock.core.echo', { message: 'fresh' }, 5000);
    expect(resp.ok).toBe(true);
    expect((resp.payload as Record<string, unknown>).count).toBe(1);
  });

  it('CKPT-005: multiple rapid echoes produce sequential counter values', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const r1 = await host.callRPC('mock.core', 'mock.core.echo', { message: 'a' }, 5000);
    const r2 = await host.callRPC('mock.core', 'mock.core.echo', { message: 'b' }, 5000);
    const r3 = await host.callRPC('mock.core', 'mock.core.echo', { message: 'c' }, 5000);
    expect((r1.payload as Record<string, unknown>).count).toBe(1);
    expect((r2.payload as Record<string, unknown>).count).toBe(2);
    expect((r3.payload as Record<string, unknown>).count).toBe(3);
  });

  it('CKPT-006: get state for unknown key returns found false', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.state', 'mock.state.get', { stateId: 'never-set' }, 5000);
    expect(resp.ok).toBe(true);
    expect((resp.payload as Record<string, unknown>).found).toBe(false);
  });
});
