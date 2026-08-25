import { Client, FixedIDGenerator } from './client';
import { Runner } from './runner';
import { Envelope } from './protocol';
import { Transport } from './transport';

class HandshakeErrorTransport implements Transport {
  sent: Envelope[] = [];

  async send(message: Envelope): Promise<void> {
    this.sent.push(message);
  }

  async receive(): Promise<Envelope> {
    return {
      protocol: 'amitia-game-host/1',
      type: 'response',
      id: 'hello-response',
      requestId: 'hello-request',
      runtimeId: 'runtime-1',
      pluginId: 'extension/plugin',
      serviceId: 'service-1',
      generation: 3,
      error: {
        code: 'protocol_mismatch',
        message: 'unsupported protocol',
      },
    };
  }

  async close(): Promise<void> {}
}

describe('Runner handshake', () => {
  test('surfaces errors carried by response envelopes without binding the route', async () => {
    const transport = new HandshakeErrorTransport();
    const client = new Client(transport, {
      idGenerator: new FixedIDGenerator('hello-request'),
    });
    const runner = new Runner(client, {
      pluginId: 'extension/plugin',
      hello: {
        supportedProtocols: ['amitia-game-host/1'],
        capabilities: [],
        rpcNamespaces: [],
      },
    });

    await expect(runner.run()).rejects.toThrow('unsupported protocol');
    expect(client.getGeneration()).toBe(0);
  });
});
