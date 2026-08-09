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
