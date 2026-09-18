import {
  PROTOCOL_VERSION,
  Client,
  StdioTransport,
  Runner,
  HandlerRegistry,
  RunnerConfig,
} from '@amitia/game-plugin-sdk';

interface SinkHelloDescriptor {
  sinkId: string;
  kind: string;
  serviceId?: string;
}
import { CoreService, StateService, DataService } from './mockService.js';
import { ControlService, SecurityService, TakeoverPayload, ReleasePayload, CheckPayload } from './securityService.js';
import { FaultInjectionService, handleNotification } from './faultService.js';

class MockPlugin {
  private coreService: CoreService;
  private stateService: StateService;
  private dataService: DataService;
  private controlService: ControlService;
  private securityService: SecurityService;
  private faultService: FaultInjectionService;
  private transport: StdioTransport;
  private client: Client;
  private coreRegistry: HandlerRegistry;
  private stateRegistry: HandlerRegistry;
  private dataRegistry: HandlerRegistry;
  private controlRegistry: HandlerRegistry;
  private faultRegistry: HandlerRegistry;
  private runner: Runner;

  constructor() {
    this.coreService = new CoreService();
    this.stateService = new StateService();
    this.dataService = new DataService();
    this.controlService = new ControlService();
    this.securityService = new SecurityService();

    this.transport = new StdioTransport();
    this.client = new Client(this.transport);

    this.faultService = new FaultInjectionService(this.client);

    this.coreRegistry = new HandlerRegistry();
    this.coreRegistry.registerRequest('mock.core.echo', this.coreService.handleEcho);
    this.coreRegistry.registerRequest('mock.core.increment', this.coreService.handleIncrement);
    this.coreRegistry.registerRequest('mock.core.generate', this.coreService.handleGenerate);

    this.stateRegistry = this.coreRegistry;
    this.stateRegistry.registerRequest('mock.state.set', this.stateService.handleSetState);
    this.stateRegistry.registerRequest('mock.state.get', this.stateService.handleGetState);

    this.dataRegistry = this.coreRegistry;
    this.dataRegistry.registerRequest('mock.data.store', this.dataService.handleStoreBinary);
    this.dataRegistry.registerRequest('mock.data.stream.append', this.dataService.handleStreamAppend);
    this.dataRegistry.registerRequest('mock.data.channel.send', this.dataService.handleChannelSend);

    this.controlRegistry = this.coreRegistry;
    this.controlRegistry.registerRequest(
      'mock.control.probe',
      async (req) => this.controlService.handleControlProbe(req)
    );
    this.controlRegistry.registerRequest(
      'mock.control.authority.snapshot',
      async () => ({
        mode: this.controlService.getAuthorityMode(),
        epoch: this.controlService.getAuthorityEpoch(),
      })
    );
    this.controlRegistry.registerRequest(
      'mock.control.authority.takeover',
      async (req) => {
        const payload = (req.payload as TakeoverPayload) || {};
        return {
          targetMode: payload.targetMode || 'user_control',
          actor: payload.actor || 'user',
          mode: this.controlService.getAuthorityMode(),
          epoch: this.controlService.getAuthorityEpoch(),
        };
      }
    );
    this.controlRegistry.registerRequest(
      'mock.control.authority.release',
      async (req) => {
        const payload = (req.payload as ReleasePayload) || {};
        return {
          targetMode: payload.targetMode || 'observe_only',
          actor: payload.actor || 'host',
          mode: this.controlService.getAuthorityMode(),
          epoch: this.controlService.getAuthorityEpoch(),
        };
      }
    );
    this.controlRegistry.registerRequest(
      'mock.security.secret_probe',
      async (req) => this.securityService.handleSecretProbe(req)
    );
    this.controlRegistry.registerRequest(
      'mock.security.release_lease',
      async () => this.securityService.handleReleaseLease()
    );
    this.controlRegistry.registerRequest(
      'mock.security.hostapi_check',
      async (req) => {
        const payload = (req.payload as CheckPayload) || {};
        const permissionId = payload.permissionId || 'gamehost.control';
        return {
          permission: permissionId,
          decision: this.controlService.isControlAllowed() ? 'allowed' : 'denied',
          reason: this.controlService.isControlAllowed() ? 'ok' : 'authority_mode',
        };
      }
    );

    this.controlRegistry.registerNotification('host.authority.changed', async (notification) => {
      const payload = (notification.payload as { mode?: string; epoch?: number }) || {};
      const mode = payload.mode || 'observe_only';
      const epoch = payload.epoch || 0;
      this.controlService.updateAuthority(mode, epoch);
      await Promise.resolve();
    });

    this.controlRegistry.registerNotification('host.authority.emergency', async () => {
      this.controlService.setEmergencyActive(true);
      await Promise.resolve();
    });

    this.faultRegistry = this.coreRegistry;
    this.faultRegistry.registerRequest(
      'mock.fault.crash',
      async (req) => this.faultService.handleCrash(req)
    );
    this.faultRegistry.registerRequest(
      'mock.fault.handshake.delay',
      async (req) => this.faultService.handleHandshakeDelay(req)
    );
    this.faultRegistry.registerRequest(
      'mock.fault.heartbeat.pause',
      async (req) => this.faultService.handleHeartbeatPause(req)
    );
    this.faultRegistry.registerRequest(
      'mock.fault.heartbeat.resume',
      async () => this.faultService.handleHeartbeatResume()
    );
    this.faultRegistry.registerRequest(
      'mock.fault.heartbeat.drop',
      async (req) => this.faultService.handleHeartbeatDrop(req)
    );
    this.faultRegistry.registerRequest(
      'mock.fault.rpc.stall',
      async (req) => this.faultService.handleRPCStall(req)
    );
    this.faultRegistry.registerRequest(
      'mock.fault.queue.fill',
      async (req) => this.faultService.handleQueueFill(req)
    );
    this.faultRegistry.registerRequest(
      'mock.fault.secret.probe',
      async (req) => this.faultService.handleSecretProbe(req)
    );
    this.faultRegistry.registerRequest(
      'mock.fault.control.ignore_shutdown',
      async (req) => this.faultService.handleControlIgnoreShutdown(req)
    );
    this.faultRegistry.registerRequest(
      'mock.fault.residue.counts',
      async () => this.faultService.handleResidueCounts()
    );
    this.faultRegistry.registerNotification('host.authority.emergency', async (notification) => {
      this.controlService.setEmergencyActive(true);
      await handleNotification(this.faultService, notification);
    });

    const sinks: SinkHelloDescriptor[] = [
      { sinkId: 'mock.control.effect', kind: 'effect', serviceId: 'mock-ts-runtime' },
    ];

    const config: RunnerConfig = {
      pluginId: 'mock-game-plugin-ts',
      defaultServiceId: 'mock-ts-runtime',
      hello: {
        supportedProtocols: [PROTOCOL_VERSION],
        capabilities: [
          'realtime_control',
          'state_streaming',
          'event_streaming',
          'custom_rpc',
          'host_api',
          'shared_control',
        ],
        rpcNamespaces: ['mock.core', 'mock.state', 'mock.data', 'mock.control', 'mock.security', 'mock.fault'],
        channels: [{ id: 'mock-events' }, { id: 'mock-state' }],
        sinks,
        sdk: {
          name: '@amitia/game-plugin-sdk',
          version: '0.1.0',
        },
        metadata: {
          mode: 'g36-mock',
          security: 'full',
        },
      },
      onReady: async (client: Client) => {
        await client.sendNotification('mock.ready', {
          status: 'ready',
          services: ['mock-ts-runtime'],
          sinks: ['mock.control.effect'],
          plugin: 'mock-game-plugin-ts',
          security_ready: true,
          has_secret_lease: this.securityService.hasActiveLease(),
        });

        this.controlService.setSinkRegistered(true);

        return async () => {
          this.securityService.handleReleaseLease();
          await client.sendNotification('mock.shutdown', {
            status: 'shutdown',
            plugin: 'mock-game-plugin-ts',
          });
        };
      },
    };

    this.runner = new Runner(this.client, config);
    this.runner.addService('mock-ts-runtime', this.coreRegistry);
    this.runner.setDefaultRegistry(this.coreRegistry);
  }

  async run(): Promise<void> {
    await this.runner.run();
  }
}

async function main(): Promise<void> {
  const plugin = new MockPlugin();

  const shutdown = (): void => {
    process.exit(0);
  };

  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);

  try {
    await plugin.run();
  } catch (err) {
    console.error(`plugin error: ${err}`);
    process.exit(1);
  }
}

main().catch((err) => {
  console.error(`fatal error: ${err}`);
  process.exit(1);
});
