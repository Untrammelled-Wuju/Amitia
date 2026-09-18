import {
  PROTOCOL_VERSION,
  Client,
  StdioTransport,
  Runner,
  HandlerRegistry,
  RunnerConfig,
  SinkHelloDescriptor,
  binaryUpload,
  binaryReadAll,
  binaryStat,
  binaryRelease,
  channelPublishBinary,
  publishAgentEvent,
} from '@amitia/game-plugin-sdk';
import { FakeGameRuntime } from './state.js';
import { MockGameRPCService } from './rpc.js';
import { ControlService, ControlHandler, registerSinkDispatchHandler } from './control.js';
import { HostAPIHandler } from './hostapi.js';
import { FaultInjectionService, handleHostNotification } from './fault.js';

export class MockAmitiaXGamePlugin {
  private runtime: FakeGameRuntime;
  private rpcService: MockGameRPCService;
  private controlService: ControlService;
  private controlHandler: ControlHandler;
  private hostAPIHandler: HostAPIHandler;
  private faultService: FaultInjectionService;
  private transport: StdioTransport;
  private client: Client;
  private mainRegistry: HandlerRegistry;
  private runner: Runner;

  constructor() {
    this.runtime = new FakeGameRuntime();
    this.rpcService = new MockGameRPCService(this.runtime);
    this.controlService = new ControlService();
    this.controlHandler = new ControlHandler(this.controlService);
    this.hostAPIHandler = new HostAPIHandler();
    this.faultService = new FaultInjectionService();

    this.transport = new StdioTransport();
    this.client = new Client(this.transport, { pluginId: 'com.amitia/world-game-plugin/world-game-plugin-contribution' });

    this.mainRegistry = new HandlerRegistry();
    this.mainRegistry.registerRequest('worldgame.status', (req) => this.rpcService.handleStatus(req));
    this.mainRegistry.registerRequest('worldgame.command', (req) => this.rpcService.handleCommand(req));
    this.mainRegistry.registerRequest('worldgame.echo', (req) => this.rpcService.handleEcho(req));
    this.mainRegistry.registerRequest('worldgame.namespace.support_ping', async (req) => {
      const response = await this.client.sendRequest('worldsupport.core.ping', req.payload ?? {});
      return response.payload ?? null;
    });
    this.mainRegistry.registerRequest('worldgame.long_task', (req) => this.rpcService.handleLongTask(req));
    this.mainRegistry.registerRequest('worldgame.fail', (req) => this.rpcService.handleFail(req));

    this.mainRegistry.registerRequest('worldgame.control.authority.snapshot', (req) =>
      this.controlHandler.handleAuthoritySnapshot(req)
    );
    this.mainRegistry.registerRequest('worldgame.control.sink.register', (req) =>
      this.controlHandler.handleSinkRegister(req)
    );
    this.mainRegistry.registerRequest('worldgame.control.output', (req) =>
      this.controlHandler.handleOutput(this.client, req)
    );
    this.mainRegistry.registerRequest('worldgame.effect.status', (req) =>
      this.controlHandler.handleEffectStatus(req)
    );
    this.mainRegistry.registerRequest('worldgame.control.takeover', (req) =>
      this.controlHandler.handleTakeover(req)
    );
    this.mainRegistry.registerRequest('worldgame.control.release', (req) =>
      this.controlHandler.handleRelease(req)
    );
    this.mainRegistry.registerRequest('worldgame.control.takeover_full', (req) =>
      this.controlHandler.handleTakeoverFull(this.client, req)
    );
    this.mainRegistry.registerRequest('worldgame.control.release_full', (req) =>
      this.controlHandler.handleReleaseFull(this.client, req)
    );
    this.mainRegistry.registerRequest('worldgame.control.emergency_status', () =>
      this.controlHandler.handleEmergencyStatus()
    );

    registerSinkDispatchHandler(this.mainRegistry, (payload) =>
      this.controlHandler.handleDispatch(payload)
    );
    this.controlService.setDispatchHandlerRegistered(true);

    this.mainRegistry.registerRequest('worldgame.permission.check', (req) =>
      this.hostAPIHandler.handlePermissionCheck(this.client, req)
    );
    this.mainRegistry.registerRequest('worldgame.permission.snapshot', (req) =>
      this.hostAPIHandler.handlePermissionSnapshot(this.client, req)
    );
    this.mainRegistry.registerRequest('worldgame.permission.request', (req) =>
      this.hostAPIHandler.handlePermissionRequest(this.client, req)
    );
    this.mainRegistry.registerRequest('worldgame.hostapi.check', (req) =>
      this.hostAPIHandler.handleHostAPICheck(req)
    );
    this.mainRegistry.registerRequest('worldgame.secret.probe', () =>
      this.hostAPIHandler.handleSecretProbe()
    );
    this.mainRegistry.registerRequest('worldgame.secret.acquire', () =>
      this.hostAPIHandler.handleAcquireLease(this.client)
    );
    this.mainRegistry.registerRequest('worldgame.secret.release', () =>
      this.hostAPIHandler.handleReleaseLeaseFull(this.client)
    );
    this.mainRegistry.registerRequest('worldgame.secret.query', () =>
      this.hostAPIHandler.handleQueryLease(this.client)
    );
    this.mainRegistry.registerRequest('worldgame.hostapi.invoke', (req) =>
      this.hostAPIHandler.handleInvokeHostAPI(this.client, req)
    );

    this.mainRegistry.registerRequest('worldgame.network.restricted_probe', () =>
      this.hostAPIHandler.handleRestrictedNetworkProbe(this.client)
    );
    this.mainRegistry.registerRequest('worldgame.network.socket_probe', () =>
      this.hostAPIHandler.handleMediatedSocketProbe(this.client)
    );

    this.mainRegistry.registerNotification('host.authority.changed', async (notification) => {
      const payload = (notification.payload as { mode?: string; epoch?: number }) || {};
      this.controlService.updateAuthority(payload.mode || 'observe_only', payload.epoch || 0);
    });

    this.mainRegistry.registerNotification('host.authority.emergency', async (notification) => {
      this.controlService.setEmergencyActive(true);
      await handleHostNotification(this.faultService, notification);
    });

    this.mainRegistry.registerRequest('worldgame.binary.consume', (req) =>
      this.rpcService.handleBinaryConsume(req)
    );
    this.mainRegistry.registerRequest('worldgame.binary.roundtrip', async (req) => {
      const payload = (req.payload as { message?: string }) || {};
      const source = Buffer.from(payload.message || 'binary-e2e', 'utf8');
      const reference = await binaryUpload(this.client, {
        channelId: 'worldgame-binary',
        expectedSize: source.byteLength,
        mediaType: 'application/octet-stream',
        lifetime: 'runtime',
        metadata: { fixture: 'external-binary-e2e' },
      }, new Uint8Array(source));
      try {
        const stat = await binaryStat(this.client, reference.id);
        const read = await binaryReadAll(this.client, reference);
        await channelPublishBinary(this.client, 'mockgame-binary', reference, {
          fixture: 'external-binary-e2e',
        });
        return {
          id: reference.id,
          size: reference.size,
          statSize: stat.size,
          roundtrip: Buffer.from(read).toString('utf8'),
          checksum: reference.checksum?.value || '',
        };
      } finally {
        await binaryRelease(this.client, reference.id);
      }
    });
    this.mainRegistry.registerRequest('worldgame.agent_event.emit', async (req) => {
      const payload = (req.payload as { sessionId?: string; type?: string; value?: unknown }) || {};
      const id = `agent-event-${Date.now()}-${Math.random().toString(16).slice(2)}`;
      await publishAgentEvent(this.client, {
        id,
        sessionId: payload.sessionId || 'e2e-session',
        type: payload.type || 'worldgame.external.event',
        occurredAt: new Date().toISOString(),
        payload: { value: payload.value ?? 'external-e2e' },
        metadata: { fixture: 'external-agent-event-e2e' },
      }, { wakeAgent: true, fixture: 'external-agent-event-e2e' });
      return { published: true, id };
    });
    this.mainRegistry.registerRequest('worldgame.fault.crash', (req) =>
      this.faultService.handleCrash(req)
    );
    this.mainRegistry.registerRequest('worldgame.fault.delay', (req) =>
      this.faultService.handleDelay(req)
    );
    this.mainRegistry.registerRequest('worldgame.fault.malformed', () =>
      this.faultService.handleMalformedFrame()
    );
    this.mainRegistry.registerRequest('worldgame.fault.drop_next_response', (req) =>
      this.faultService.handleDropNextResponse(req)
    );
    this.mainRegistry.registerRequest('worldgame.fault.hang', (req) =>
      this.faultService.handleHang(req)
    );
    this.mainRegistry.registerRequest('worldgame.fault.reset', () =>
      this.faultService.handleReset()
    );
    this.mainRegistry.registerRequest('worldgame.fault.status', () =>
      this.faultService.getStatus()
    );

    const sinks: SinkHelloDescriptor[] = [
      { sinkId: 'worldgame.effect', kind: 'effect', serviceId: 'world-game-runtime' },
    ];

    const config: RunnerConfig = {
      pluginId: 'com.amitia/world-game-plugin/world-game-plugin-contribution',
      defaultServiceId: 'world-game-runtime',
      hello: {
        supportedProtocols: [PROTOCOL_VERSION],
        capabilities: [
          'realtime_control',
          'state_streaming',
          'event_streaming',
          'custom_rpc',
          'host_api',
          'shared_control',
          'multi_service',
          'binary_streaming',
        ],
        rpcNamespaces: ['worldgame'],
        channels: [{ id: 'worldgame-events' }, { id: 'worldgame-state' }, { id: 'worldgame-binary' }],
        sinks,
        sdk: {
          name: '@amitia/game-plugin-sdk',
          version: '0.1.0',
        },
        metadata: {
          mode: 'formal-plugin',
          security: 'full',
        },
      },
      onReady: async (client: Client) => {
        await client.sendNotification('worldgame.ready', {
          status: 'ready',
          services: ['world-game-runtime', 'world-game-support'],
          sinks: ['worldgame.effect'],
          plugin: 'com.amitia/world-game-plugin',
          security_ready: true,
          has_secret_lease: this.hostAPIHandler.hasActiveLease(),
        });

        this.controlService.setSinkRegistered(true);

        return async () => {
          await client.sendNotification('worldgame.shutdown', {
            status: 'shutdown',
            plugin: 'com.amitia/world-game-plugin',
          });
        };
      },
    };

    this.runner = new Runner(this.client, config);
    this.runner.addService('world-game-runtime', this.mainRegistry);
    this.runner.setDefaultRegistry(this.mainRegistry);
  }

  async run(): Promise<void> {
    await this.runner.run();
  }
}
