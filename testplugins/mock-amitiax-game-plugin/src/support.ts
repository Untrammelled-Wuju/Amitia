import {
  PROTOCOL_VERSION,
  Client,
  StdioTransport,
  Runner,
  HandlerRegistry,
  RunnerConfig,
} from '@amitia/game-plugin-sdk';

async function main(): Promise<void> {
  const transport = new StdioTransport();
  const client = new Client(transport, {
    pluginId: 'com.mock-developer/mock-amitiax-game-plugin/mock-game-plugin-contribution',
  });
  const registry = new HandlerRegistry();
  registry.registerRequest('mocksupport.core.ping', async (req) => ({
    pong: true,
    payload: req.payload ?? null,
    serviceId: 'mock-game-support',
  }));
  const config: RunnerConfig = {
    pluginId: 'com.mock-developer/mock-amitiax-game-plugin/mock-game-plugin-contribution',
    defaultServiceId: 'mock-game-support',
    hello: {
      supportedProtocols: [PROTOCOL_VERSION],
      capabilities: ['multi_service', 'custom_rpc'],
      rpcNamespaces: ['mocksupport.core'],
      sdk: {
        name: '@amitia/game-plugin-sdk',
        version: '0.1.0',
      },
      metadata: {
        mode: 'external-multi-service-support',
      },
    },
  };

  const runner = new Runner(client, config);
  runner.addService('mock-game-support', registry);
  runner.setDefaultRegistry(registry);

  const shutdown = (): void => process.exit(0);
  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);

  await runner.run();
}

main().catch((err) => {
  console.error(`support plugin error: ${err}`);
  process.exit(1);
});
