import { Client, FixedIDGenerator } from './client';
import { Transport } from './transport';
import { Envelope } from './protocol';

class MockTransport implements Transport {
  sent: Envelope[] = [];
  receiveQueue: Envelope[] = [];

  async send(message: Envelope): Promise<void> {
    this.sent.push(message);
  }

  async receive(): Promise<Envelope> {
    return this.receiveQueue.shift()!;
  }

  async close(): Promise<void> {}
}

describe('Client', () => {
  let transport: MockTransport;
  let client: Client;

  beforeEach(() => {
    transport = new MockTransport();
    client = new Client(transport, {
      idGenerator: new FixedIDGenerator('msg-001', 'msg-002', 'msg-003'),
    });
  });

  test('newRequest should create valid request envelope', () => {
    const payload = { command: 'perform operation' };
    const envelope = client.newRequest('example.game.operation.submit', payload);

    expect(envelope.protocol).toBe('amitia-game-host/1');
    expect(envelope.type).toBe('request');
    expect(envelope.id).toBe('msg-001');
    expect(envelope.method).toBe('example.game.operation.submit');
    expect(envelope.payload).toEqual(payload);
  });

  test('newResponse should set requestId correctly', () => {
    const request = client.newRequest('example.game.operation.submit');
    const response = client.newResponse(request, { status: 'ok' });

    expect(response.requestId).toBe(request.id);
    expect(response.type).toBe('response');
  });

  test('newNotification should not have requestId', () => {
    const notification = client.newNotification('vendor.event.state_changed', { state: 'running' });

    expect(notification.type).toBe('notification');
    expect(notification.requestId).toBeUndefined();
  });

  test('newError should set error fields', () => {
    const request = client.newRequest('example.game.operation.submit');
    const error = client.newError(request, 'permission_denied', 'permission denied', false);

    expect(error.type).toBe('error');
    expect(error.error?.code).toBe('permission_denied');
    expect(error.error?.message).toBe('permission denied');
    expect(error.requestId).toBe(request.id);
  });

  test('sendRequest should validate reserved namespace', async () => {
    await expect(client.sendRequest('host.secret.read', {})).rejects.toThrow();
  });

  test('custom methods should work', () => {
    const methods = [
      'example.game.operation.submit',
      'vendor.control.execute',
      'custom.foo.bar',
    ];

    for (const method of methods) {
      const envelope = client.newRequest(method);
      expect(envelope.method).toBe(method);
    }
  });
});
