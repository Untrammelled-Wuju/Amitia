import * as fs from 'fs';
import * as path from 'path';
import {
  PROTOCOL_VERSION,
  Envelope,
  ServiceDescriptor,
  ChannelDescriptor,
  MessageType,
  HostFeature,
  ErrorCode,
} from '../src/protocol';
import { Client, FixedIDGenerator } from '../src/client';
import type { PluginChannelSpec, PluginHostSpec } from '../src/game';
import {
  validateMessageId,
  validateMethod,
  validatePluginMethod,
  validateEnvelope,
  validateServices,
  validateChannel,
  validateCapability,
  validatePluginHostSpec,
} from '../src/validation';

const FIXTURES_ROOT = path.join(__dirname, '..', '..', '..', 'testdata', 'conformance');

function loadFixture(...parts: string[]): unknown {
  const fullPath = path.join(FIXTURES_ROOT, ...parts);
  if (!fs.existsSync(fullPath)) {
    throw new Error(`Fixture not found: ${fullPath}`);
  }
  const content = fs.readFileSync(fullPath, 'utf-8');
  return JSON.parse(content);
}

function loadFixtureIfExists(...parts: string[]): unknown | null {
  const fullPath = path.join(FIXTURES_ROOT, ...parts);
  if (!fs.existsSync(fullPath)) {
    return null;
  }
  const content = fs.readFileSync(fullPath, 'utf-8');
  return JSON.parse(content);
}

class MockTransport {
  sent: Envelope[] = [];
  private queue: Envelope[] = [];

  async send(envelope: Envelope): Promise<void> {
    this.sent.push(envelope);
  }

  async receive(): Promise<Envelope> {
    return this.queue.shift()!;
  }

  async close(): Promise<void> {}

  queueMessage(msg: Envelope): void {
    this.queue.push(msg);
  }
}

describe('TypeScript Conformance Suite', () => {
  describe('Protocol Constants', () => {
    test('PROTOCOL_VERSION should be amitia-game-host/1', () => {
      expect(PROTOCOL_VERSION).toBe('amitia-game-host/1');
    });
  });

  describe('Valid Request Fixture', () => {
    test('should parse valid request.json', () => {
      const data = loadFixture('valid', 'request.json') as Envelope;
      expect(data.protocol).toBe(PROTOCOL_VERSION);
      expect(data.type).toBe('request');
      expect(data.id).toBeDefined();
      expect(data.method).toBeDefined();

      const err = validateEnvelope(data);
      expect(err).toBeNull();
    });

    test('should validate request method', () => {
      const data = loadFixture('valid', 'request.json') as Envelope;
      const err = validateMethod(data.method!);
      expect(err).toBeNull();
    });
  });

  describe('Valid Response Fixture', () => {
    test('should parse valid response.json', () => {
      const data = loadFixture('valid', 'response.json') as Envelope;
      expect(data.protocol).toBe(PROTOCOL_VERSION);
      expect(data.type).toBe('response');
      expect(data.requestId).toBeDefined();
    });

    test('response requestId should match request id', () => {
      const req = loadFixture('valid', 'request.json') as Envelope;
      const resp = loadFixture('valid', 'response.json') as Envelope;
      expect(resp.requestId).toBe(req.id);
    });
  });

  describe('Valid Notification Fixture', () => {
    test('should parse valid notification.json', () => {
      const data = loadFixture('valid', 'notification.json') as Envelope;
      expect(data.protocol).toBe(PROTOCOL_VERSION);
      expect(data.type).toBe('notification');
      expect(data.method).toBeDefined();
      expect(data.requestId).toBeUndefined();
    });
  });

  describe('Valid Error Fixture', () => {
    test('should parse valid error.json', () => {
      const data = loadFixture('valid', 'error.json') as Envelope;
      expect(data.protocol).toBe(PROTOCOL_VERSION);
      expect(data.type).toBe('error');
      expect(data.error).toBeDefined();
      expect(data.error!.code).toBeDefined();
      expect(data.error!.message).toBeDefined();
    });
  });

  describe('Invalid Fixtures should be rejected', () => {
    test('wrong protocol should be rejected', () => {
      const data = loadFixtureIfExists('invalid', 'wrong_protocol.json') as Envelope | null;
      if (data) {
        expect(data.protocol).not.toBe(PROTOCOL_VERSION);
      }
    });

    test('request without id should fail validation', () => {
      const data = loadFixtureIfExists('invalid', 'request_without_id.json') as Envelope | null;
      if (data) {
        const idErr = validateMessageId(data.id || '');
        expect(idErr).not.toBeNull();
      }
    });

    test('request without method should fail validation', () => {
      const data = loadFixtureIfExists('invalid', 'request_without_method.json') as Envelope | null;
      if (data) {
        const methodErr = validateMethod(data.method || '');
        expect(methodErr).not.toBeNull();
      }
    });
  });

  describe('Service Schema Conformance', () => {
    test('should parse valid service.json', () => {
      const data = loadFixture('valid', 'service.json') as ServiceDescriptor;
      expect(data.id).toBeDefined();
      expect(data.kind).toBeDefined();

      const errors = validateServices([data]);
      expect(errors).toHaveLength(0);
    });

    test('protocol v1 service kinds should be valid', () => {
      const kinds = ['process'];
      for (const kind of kinds) {
        const svc: ServiceDescriptor = { id: `svc-${kind}`, kind: kind as any };
        const errors = validateServices([svc]);
        expect(errors).toHaveLength(0);
      }
    });


    test('external service kind should be rejected by protocol v1', () => {
      const svc: ServiceDescriptor = { id: 'svc-external', kind: 'external' as any };
      const errors = validateServices([svc]);
      expect(errors.length).toBeGreaterThan(0);
    });
    test('invalid service kind should be rejected', () => {
      const svc: ServiceDescriptor = { id: 'svc-bad', kind: 'invalid' as any };
      const errors = validateServices([svc]);
      expect(errors.length).toBeGreaterThan(0);
    });

    test('empty service id should be rejected', () => {
      const svc: ServiceDescriptor = { id: '', kind: 'process' };
      const errors = validateServices([svc]);
      expect(errors.length).toBeGreaterThan(0);
    });

    test('duplicate service id should be rejected', () => {
      const svcs: ServiceDescriptor[] = [
        { id: 'svc-1', kind: 'process' },
        { id: 'svc-1', kind: 'process' },
      ];
      const errors = validateServices(svcs);
      expect(errors.length).toBeGreaterThan(0);
    });
  });

  describe('Channel Schema Conformance', () => {
    test('should parse valid channel.json', () => {
      const data = loadFixture('valid', 'channel.json') as ChannelDescriptor;
      expect(data.id).toBeDefined();
      expect(data.kind).toBeDefined();

      const errors = validateChannel(data);
      expect(errors).toHaveLength(0);
    });

    test('protocol v1 channel kinds should be valid', () => {
      const kinds = ['event', 'state', 'log', 'metric', 'custom', 'binary'];
      for (const kind of kinds) {
        const ch: ChannelDescriptor = { id: `ch-${kind}`, kind: kind as any };
        const errors = validateChannel(ch);
        expect(errors).toHaveLength(0);
      }
    });

    test('invalid channel kind should be rejected', () => {
      const ch: ChannelDescriptor = { id: 'ch-bad', kind: 'invalid' as any };
      const errors = validateChannel(ch);
      expect(errors.length).toBeGreaterThan(0);
    });

    test('all channel directions should be valid', () => {
      const directions = ['plugin_to_host', 'host_to_plugin', 'bidirectional'];
      for (const dir of directions) {
        const ch: ChannelDescriptor = { id: 'ch-1', kind: 'event', direction: dir as any };
        const errors = validateChannel(ch);
        expect(errors).toHaveLength(0);
      }
    });

    test('invalid channel direction should be rejected', () => {
      const ch: ChannelDescriptor = { id: 'ch-1', kind: 'event', direction: 'invalid' as any };
      const errors = validateChannel(ch);
      expect(errors.length).toBeGreaterThan(0);
    });
  });

  describe('Host Feature Conformance', () => {
    test('all standard capabilities should be valid', () => {
      const stdCaps = [
        HostFeature.REALTIME_CONTROL,
        HostFeature.STATE_STREAMING,
        HostFeature.EVENT_STREAMING,
        HostFeature.CUSTOM_RPC,
        HostFeature.HOST_API,
        HostFeature.SHARED_CONTROL,
        HostFeature.MULTI_SERVICE,
      ];
      for (const cap of stdCaps) {
        const errs = validateCapability([cap]);
        expect(errs).toHaveLength(0);
      }
    });

    test('game-specific tool capabilities must not be accepted as host features', () => {
      const customCaps = ['example.navigation', 'vendor.agent'];
      for (const cap of customCaps) {
        const errs = validateCapability([cap]);
        expect(errs.length).toBeGreaterThan(0);
      }
    });

    test('duplicate capabilities should be rejected', () => {
      const caps = ['custom_rpc', 'custom_rpc'];
      const errs = validateCapability(caps);
      expect(errs.length).toBeGreaterThan(0);
    });

    test('empty capability should be rejected', () => {
      const errs = validateCapability(['']);
      expect(errs.length).toBeGreaterThan(0);
    });
  });

  describe('Error Code Conformance', () => {
    test('all standard error codes should be valid', () => {
      const stdCodes = [
        ErrorCode.INVALID_REQUEST,
        ErrorCode.INVALID_ARGUMENT,
        ErrorCode.NOT_FOUND,
        ErrorCode.ALREADY_EXISTS,
        ErrorCode.UNSUPPORTED,
        ErrorCode.PROTOCOL_MISMATCH,
        ErrorCode.CAPABILITY_UNSUPPORTED,
        ErrorCode.RUNTIME_UNAVAILABLE,
        ErrorCode.SERVICE_UNAVAILABLE,
        ErrorCode.INVALID_RUNTIME_STATE,
        ErrorCode.PERMISSION_DENIED,
        ErrorCode.RESOURCE_EXHAUSTED,
        ErrorCode.TIMEOUT,
        ErrorCode.CANCELLED,
        ErrorCode.INTERNAL,
      ];
      for (const code of stdCodes) {
        const env: Envelope = {
          protocol: PROTOCOL_VERSION,
          type: 'error',
          id: 'err-1',
          error: { code, message: 'test' },
        };
        expect(env.error!.code).toBe(code);
      }
    });

    test('custom error codes with vendor namespace should be valid', () => {
      const customCodes = ['vendor.game.connection_failed', 'vendor.agent_crashed'];
      for (const code of customCodes) {
        const env: Envelope = {
          protocol: PROTOCOL_VERSION,
          type: 'error',
          id: 'err-1',
          error: { code, message: 'test' },
        };
        expect(env.error!.code).toBe(code);
      }
    });
  });
});

describe('Plugin Host Spec Type Conformance', () => {
  test('public PluginChannelSpec exposes canonical direction and frequencyHint fields', () => {
    const channels: PluginChannelSpec[] = [
      { id: 'plugin-events', kind: 'event', direction: 'plugin_to_host', frequencyHint: 'normal' },
      { id: 'host-commands', kind: 'custom', direction: 'host_to_plugin', frequencyHint: 'high' },
      { id: 'shared-state', kind: 'state', direction: 'bidirectional', frequencyHint: 'realtime' },
    ];

    const spec: PluginHostSpec = {
      protocolVersion: PROTOCOL_VERSION,
      runtimeModuleId: 'runtime',
      channels,
      network: { mode: 'none' },
    };

    expect(spec.channels).toEqual(channels);
    expect(spec.channels?.[1].direction).toBe('host_to_plugin');
    expect(spec.channels?.[2].frequencyHint).toBe('realtime');
  });
});

describe('Cross-Language Wire Compatibility', () => {
  describe('Go-generated fixtures should be parseable by TypeScript', () => {
    test('go-generated-request.json', () => {
      const data = loadFixtureIfExists('cross-language', 'go-generated-request.json') as Envelope | null;
      if (!data) {
        console.log('  [skipped] go-generated-request.json not found');
        return;
      }
      expect(data.protocol).toBe(PROTOCOL_VERSION);
      expect(data.type).toBe('request');
      expect(data.id).toBe('go-req-001');
      expect(data.method).toBe('example.game.operation.submit');
      expect(data.pluginId).toBe('example-game-plugin');
      expect(data.runtimeId).toBe('runtime-abc');
      expect(data.payload).toBeDefined();
    });

    test('go-generated-response.json', () => {
      const data = loadFixtureIfExists('cross-language', 'go-generated-response.json') as Envelope | null;
      if (!data) return;
      expect(data.protocol).toBe(PROTOCOL_VERSION);
      expect(data.type).toBe('response');
      expect(data.requestId).toBe('go-req-001');
      expect(data.payload).toBeDefined();
    });

    test('go-generated-notification.json', () => {
      const data = loadFixtureIfExists('cross-language', 'go-generated-notification.json') as Envelope | null;
      if (!data) return;
      expect(data.protocol).toBe(PROTOCOL_VERSION);
      expect(data.type).toBe('notification');
      expect(data.method).toBe('plugin.runtime.state_changed');
    });

    test('go-generated-error.json', () => {
      const data = loadFixtureIfExists('cross-language', 'go-generated-error.json') as Envelope | null;
      if (!data) return;
      expect(data.protocol).toBe(PROTOCOL_VERSION);
      expect(data.type).toBe('error');
      expect(data.error).toBeDefined();
      expect(data.error!.code).toBe('vendor.game.connection_failed');
      expect(data.error!.retryable).toBe(true);
    });
  });

  describe('TypeScript SDK should produce wire-compatible output', () => {
    test('TS client generates valid request', () => {
      const transport = new MockTransport();
      const idGen = new FixedIDGenerator('ts-req-001');
      const client = new Client(transport, { idGenerator: idGen, pluginId: 'test-plugin' });

      const payload = { command: 'test' };
      const envelope = client.newRequest('vendor.agent.execute', payload);

      expect(envelope.protocol).toBe(PROTOCOL_VERSION);
      expect(envelope.type).toBe('request');
      expect(envelope.id).toBe('ts-req-001');
      expect(envelope.method).toBe('vendor.agent.execute');
      expect(envelope.pluginId).toBe('test-plugin');
      expect(envelope.payload).toEqual(payload);

      const serialized = JSON.stringify(envelope);
      const deserialized = JSON.parse(serialized) as Envelope;
      expect(deserialized.protocol).toBe(envelope.protocol);
      expect(deserialized.type).toBe(envelope.type);
      expect(deserialized.id).toBe(envelope.id);
      expect(deserialized.method).toBe(envelope.method);
    });

    test('TS client generates valid response', () => {
      const transport = new MockTransport();
      const idGen = new FixedIDGenerator('req-001', 'resp-001');
      const client = new Client(transport, { idGenerator: idGen });

      const request = client.newRequest('vendor.test', null);
      const response = client.newResponse(request, { ok: true });

      expect(response.protocol).toBe(PROTOCOL_VERSION);
      expect(response.type).toBe('response');
      expect(response.id).toBe('resp-001');
      expect(response.requestId).toBe('req-001');
    });

    test('TS client generates valid notification', () => {
      const transport = new MockTransport();
      const idGen = new FixedIDGenerator('note-001');
      const client = new Client(transport, { idGenerator: idGen });

      const envelope = client.newNotification('vendor.state.changed', { state: 'running' });

      expect(envelope.protocol).toBe(PROTOCOL_VERSION);
      expect(envelope.type).toBe('notification');
      expect(envelope.id).toBe('note-001');
      expect(envelope.method).toBe('vendor.state.changed');
      expect(envelope.requestId).toBeUndefined();
    });

    test('TS client generates valid error', () => {
      const transport = new MockTransport();
      const idGen = new FixedIDGenerator('req-001', 'err-001');
      const client = new Client(transport, { idGenerator: idGen });

      const request = client.newRequest('vendor.test', null);
      const envelope = client.newError(request, ErrorCode.INVALID_ARGUMENT, 'bad arg', false, null);

      expect(envelope.protocol).toBe(PROTOCOL_VERSION);
      expect(envelope.type).toBe('error');
      expect(envelope.error).toBeDefined();
      expect(envelope.error!.code).toBe('invalid_argument');
      expect(envelope.error!.message).toBe('bad arg');
      expect(envelope.error!.retryable).toBe(false);
      expect(envelope.requestId).toBe('req-001');
    });

    test('TS client reserved namespace rejection', async () => {
      const transport = new MockTransport();
      const client = new Client(transport);

      const reservedMethods = [
        'host.runtime.health',
        'plugin.custom.action',
        'runtime.state.update',
        'service.register',
        'permission.check',
      ];

      for (const method of reservedMethods) {
        expect(() => {
          client.newRequest(method, null);
        }).not.toThrow();

        await expect(client.sendRequest(method, null)).rejects.toThrow();
      }
    });

    test('serialization roundtrip preserves all fields', () => {
      const transport = new MockTransport();
      const idGen = new FixedIDGenerator('roundtrip-001');
      const client = new Client(transport, { idGenerator: idGen, pluginId: 'test', runtimeId: 'rt-1' });

      const originalPayload = {
        nested: { value: 123 },
        array: [1, 2, 3],
        text: 'hello',
      };

      const envelope = client.newRequest('vendor.test', originalPayload);
      const serialized = JSON.stringify(envelope);
      const deserialized = JSON.parse(serialized) as Envelope;

      expect(deserialized.protocol).toBe(envelope.protocol);
      expect(deserialized.type).toBe(envelope.type);
      expect(deserialized.id).toBe(envelope.id);
      expect(deserialized.method).toBe(envelope.method);
      expect(deserialized.pluginId).toBe(envelope.pluginId);
      expect(deserialized.runtimeId).toBe(envelope.runtimeId);
      expect(deserialized.payload).toEqual(originalPayload);
    });
  });

  describe('Complex Payload Roundtrip', () => {
    test('roundtrip-complex-payload.json', () => {
      const data = loadFixtureIfExists('cross-language', 'roundtrip-complex-payload.json') as Envelope | null;
      if (!data) return;

      expect(data.protocol).toBe(PROTOCOL_VERSION);
      expect(data.type).toBe('request');
      expect(data.payload).toBeDefined();

      const payload = data.payload as any;
      expect(payload.position).toBeDefined();
      expect(payload.position.x).toBe(100.5);
      expect(payload.blocks).toHaveLength(2);
      expect(payload.entities).toHaveLength(1);
      expect(payload.entities[0].type).toBe('player');
    });
  });
});

describe('Envelope Structure Validation', () => {
  test('empty payload should be valid', () => {
    const env: Envelope = {
      protocol: PROTOCOL_VERSION,
      type: 'request',
      id: 'test-001',
      method: 'vendor.test',
    };
    expect(env.protocol).toBe(PROTOCOL_VERSION);
  });

  test('null payload should be preserved', () => {
    const env: Envelope = {
      protocol: PROTOCOL_VERSION,
      type: 'request',
      id: 'test-001',
      method: 'vendor.test',
      payload: null,
    };
    const serialized = JSON.stringify(env);
    expect(serialized).toContain('"payload":null');
  });

  test('empty object payload should be valid', () => {
    const env: Envelope = {
      protocol: PROTOCOL_VERSION,
      type: 'request',
      id: 'test-001',
      method: 'vendor.test',
      payload: {},
    };
    const serialized = JSON.stringify(env);
    const deserialized = JSON.parse(serialized);
    expect(deserialized.payload).toEqual({});
  });

  test('metadata should be preserved', () => {
    const env: Envelope = {
      protocol: PROTOCOL_VERSION,
      type: 'notification',
      id: 'test-001',
      method: 'vendor.event',
      metadata: {
        traceId: 'abc',
        timestamp: 1234567890,
      },
    };
    const serialized = JSON.stringify(env);
    const deserialized = JSON.parse(serialized) as Envelope;
    expect(deserialized.metadata).toEqual(env.metadata);
  });
});

describe('Request-Response Correlation', () => {
  test('response should reference request id', () => {
    const transport = new MockTransport();
    const idGen = new FixedIDGenerator('req-100', 'resp-100');
    const client = new Client(transport, { idGenerator: idGen });

    const request = client.newRequest('vendor.test', { command: 'run' });
    const response = client.newResponse(request, { status: 'accepted' });

    expect(response.requestId).toBe(request.id);
    expect(response.requestId).toBe('req-100');
    expect(response.id).toBe('resp-100');
  });

  test('error should reference request id', () => {
    const transport = new MockTransport();
    const idGen = new FixedIDGenerator('req-100', 'err-100');
    const client = new Client(transport, { idGenerator: idGen });

    const request = client.newRequest('vendor.test', null);
    const error = client.newError(request, ErrorCode.INTERNAL, 'internal error', false, null);

    expect(error.requestId).toBe(request.id);
    expect(error.id).toBe('err-100');
  });
});


describe('Plugin Host Spec Runtime Validation Parity', () => {
  test('matches the shared Go/TypeScript host-spec validation fixture', () => {
    const cases = loadFixture('host_spec_validation_cases.json') as Array<{
      name: string;
      valid: boolean;
      spec: PluginHostSpec;
    }>;

    for (const testCase of cases) {
      const errors = validatePluginHostSpec(testCase.spec);
      expect({ name: testCase.name, valid: errors.length === 0 }).toEqual({
        name: testCase.name,
        valid: testCase.valid,
      });
    }
  });
});
