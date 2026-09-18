import * as path from 'path';
import { PseudoHost } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../typescript/dist/index.js');

describe('CrossRuntime', () => {
  let host: PseudoHost;

  afterEach(() => {
    if (host) {
      host.kill();
    }
  });

  it('CROSS-001: mock.core.echo returns incremented counter across sequential calls', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const first = await host.callRPC('mock.core', 'mock.core.echo', { message: 'first' }, 5000);
    expect(first.ok).toBe(true);
    expect((first.payload as Record<string, unknown>).count).toBe(1);

    const second = await host.callRPC('mock.core', 'mock.core.echo', { message: 'second' }, 5000);
    expect(second.ok).toBe(true);
    expect((second.payload as Record<string, unknown>).count).toBe(2);
  });

  it('CROSS-002: mock.core.increment adjusts counter by delta', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const first = await host.callRPC('mock.core', 'mock.core.increment', { delta: 5 }, 5000);
    expect(first.ok).toBe(true);
    expect((first.payload as Record<string, unknown>).counter).toBe(5);

    const second = await host.callRPC('mock.core', 'mock.core.increment', { delta: 3 }, 5000);
    expect(second.ok).toBe(true);
    expect((second.payload as Record<string, unknown>).counter).toBe(8);
  });

  it('CROSS-003: mock.state.set then mock.state.get returns stored value', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const setResp = await host.callRPC('mock.state', 'mock.state.set', { stateId: 'cross-key', value: 'cross-value' }, 5000);
    expect(setResp.ok).toBe(true);
    expect((setResp.payload as Record<string, unknown>).acked).toBe(true);

    const getResp = await host.callRPC('mock.state', 'mock.state.get', { stateId: 'cross-key' }, 5000);
    expect(getResp.ok).toBe(true);
    const p = getResp.payload as Record<string, unknown>;
    expect(p.found).toBe(true);
    expect(p.value).toBe('cross-value');
  });

  it('CROSS-004: mock.data.store returns stored true with size', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.data', 'mock.data.store', { binaryId: 'bin-1', data: 'hello world' }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.stored).toBe(true);
    expect(p.size).toBe(11);
  });

  it('CROSS-005: mock.data.stream.append returns appended true with streamId', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.data', 'mock.data.stream.append', { streamId: 'stream-1', data: 'chunk' }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.appended).toBe(true);
    expect(p.streamId).toBe('stream-1');
    expect(typeof p.sequence).toBe('number');
  });

  it('CROSS-006: mock.data.channel.send returns sent true with channelId', async () => {
    host = new PseudoHost(PLUGIN_PATH);
    await host.start();
    const resp = await host.callRPC('mock.data', 'mock.data.channel.send', { channelId: 'chan-1', message: 'ping' }, 5000);
    expect(resp.ok).toBe(true);
    const p = resp.payload as Record<string, unknown>;
    expect(p.sent).toBe(true);
    expect(p.channelId).toBe('chan-1');
  });
});
