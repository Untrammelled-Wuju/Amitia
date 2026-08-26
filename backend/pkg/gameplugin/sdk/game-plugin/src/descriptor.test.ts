import { createPluginDescriptor } from './descriptor';

describe('DescriptorBuilder', () => {
  test('should build valid descriptor', () => {
    const desc = createPluginDescriptor('example.game', 'Example Game', '1.0.0')
      .withService({
        id: 'agent',
        kind: 'process',
      })
      .withChannel({
        id: 'events',
        kind: 'event',
      })
      .withCapability('custom_rpc')
      .build();

    expect(desc.id).toBe('example.game');
    expect(desc.name).toBe('Example Game');
    expect(desc.version).toBe('1.0.0');
    expect(desc.protocolVersion).toBe('amitia-game-host/1');
    expect(desc.services).toHaveLength(1);
    expect(desc.channels).toHaveLength(1);
    expect(desc.capabilities).toHaveLength(1);
  });

  test('should reject duplicate capabilities', () => {
    expect(() => {
      createPluginDescriptor('example.game', 'Example Game', '1.0.0')
        .withCapability('custom_rpc')
        .withCapability('custom_rpc')
        .build();
    }).toThrow('duplicate capability');
  });

  test('should reject empty id', () => {
    expect(() => {
      createPluginDescriptor('', 'Example Game', '1.0.0').build();
    }).toThrow();
  });
});

describe('DescriptorBuilder validation parity', () => {
  test('rejects service names containing control characters', () => {
    expect(() => {
      createPluginDescriptor('example.game', 'Example Game', '1.0.0')
        .withService({ id: 'agent', name: 'bad\u0001name', kind: 'process' })
        .build();
    }).toThrow('control character');
  });

  test('rejects invalid dependency service ids', () => {
    expect(() => {
      createPluginDescriptor('example.game', 'Example Game', '1.0.0')
        .withService({ id: 'agent', kind: 'process', dependsOn: ['bad id'] })
        .build();
    }).toThrow('must not contain spaces');
  });

  test('rejects unknown service host features', () => {
    expect(() => {
      createPluginDescriptor('example.game', 'Example Game', '1.0.0')
        .withService({ id: 'agent', kind: 'process', capabilities: ['vendor.game.feature'] })
        .build();
    }).toThrow('unknown host feature');
  });

  test('rejects duplicate channel ids', () => {
    expect(() => {
      createPluginDescriptor('example.game', 'Example Game', '1.0.0')
        .withChannel({ id: 'events', kind: 'event' })
        .withChannel({ id: 'events', kind: 'custom' })
        .build();
    }).toThrow('duplicate channel id');
  });

  test('rejects oversized channel ids', () => {
    expect(() => {
      createPluginDescriptor('example.game', 'Example Game', '1.0.0')
        .withChannel({ id: 'a'.repeat(257), kind: 'event' })
        .build();
    }).toThrow('maximum length');
  });

  test('rejects oversized channel schema ids', () => {
    expect(() => {
      createPluginDescriptor('example.game', 'Example Game', '1.0.0')
        .withChannel({ id: 'events', kind: 'event', schemaId: 's'.repeat(1025) })
        .build();
    }).toThrow('schema id exceeds maximum length');
  });
});
