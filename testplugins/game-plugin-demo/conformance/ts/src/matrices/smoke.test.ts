import * as path from 'path';
import { PseudoHost } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../typescript/dist/index.js');

describe('Smoke', () => {
  it('state.get returns not found', async () => {
    const host = new PseudoHost(PLUGIN_PATH);
    await host.start();

    const echo = await host.callRPC('mock.core', 'mock.core.echo', { message: 'before-state' }, 5000);
    console.log('ECHO RESPONSE:', JSON.stringify(echo));

    const resp = await host.callRPC('mock.state', 'mock.state.get', { stateId: 'never-set' }, 5000);
    console.log('STATE RESPONSE:', JSON.stringify(resp));
    expect(resp.ok).toBe(true);
    expect((resp.payload as any).found).toBe(false);
    host.kill();
  });
});
