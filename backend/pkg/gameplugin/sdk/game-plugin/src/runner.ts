import { PROTOCOL_VERSION } from './protocol';
import { Client } from './client';
import { HandlerRegistry, HelloConfiguration, RunnerConfig, ServiceHelloDescriptor, SinkHelloDescriptor, HelloResponse } from './handler';

export class Runner {
  private client: Client;
  private config: RunnerConfig;
  private services: Map<string, HandlerRegistry> = new Map();
  private defaultRegistry?: HandlerRegistry;
  private running: boolean = false;

  constructor(client: Client, config: RunnerConfig) {
    this.client = client;
    this.config = config;
  }

  addService(serviceId: string, registry: HandlerRegistry): void {
    this.services.set(serviceId, registry);
  }

  setDefaultRegistry(registry: HandlerRegistry): void {
    this.defaultRegistry = registry;
  }

  private async performHandshake(): Promise<HelloResponse> {
    const helloReq: Record<string, unknown> = {
      supportedProtocols: this.config.hello.supportedProtocols,
      capabilities: this.config.hello.capabilities,
      rpcNamespaces: this.config.hello.rpcNamespaces,
    };

    if (this.config.hello.services && this.config.hello.services.length > 0) {
      helloReq.services = this.config.hello.services;
    }
    if (this.config.hello.sinks && this.config.hello.sinks.length > 0) {
      helloReq.sinks = this.config.hello.sinks;
    }

    if (this.config.hello.sdk) {
      helloReq.sdk = this.config.hello.sdk;
    }
    if (this.config.hello.metadata) {
      helloReq.metadata = this.config.hello.metadata;
    }

    const envelope = this.client.newRequest('control.handshake.hello', helloReq);
    await this.client.getTransport().send(envelope);

    const respEnvelope = await this.client.getTransport().receive();

    if (respEnvelope.type === 'error') {
      if (respEnvelope.error) {
        throw new Error(`handshake failed: ${respEnvelope.error.code} - ${respEnvelope.error.message}`);
      }
      throw new Error('handshake failed with unknown error');
    }

    if (respEnvelope.type !== 'response') {
      throw new Error(`unexpected handshake response type: ${respEnvelope.type}`);
    }

    const respPayload = respEnvelope.payload as HelloResponse | undefined;
    if (!respPayload) {
      throw new Error('empty handshake response');
    }

    if (respPayload.protocol !== PROTOCOL_VERSION) {
      throw new Error(`protocol mismatch: got ${respPayload.protocol}, expected ${PROTOCOL_VERSION}`);
    }

    return respPayload;
  }

  private findRegistryForService(serviceId?: string): HandlerRegistry | undefined {
    if (serviceId) {
      const reg = this.services.get(serviceId);
      if (reg) return reg;
    }
    return this.defaultRegistry;
  }

  async run(defaultRegistry?: HandlerRegistry): Promise<void> {
    if (defaultRegistry) {
      this.defaultRegistry = defaultRegistry;
    }

    await this.performHandshake();
    this.running = true;

    let shutdownFn: (() => Promise<void>) | undefined;
    if (this.config.onReady) {
      shutdownFn = await this.config.onReady(this.client);
    }

    try {
      while (this.running) {
        const envelope = await this.client.getTransport().receive();

        switch (envelope.type) {
          case 'request': {
            const registry = this.findRegistryForService(envelope.serviceId);
            if (registry) {
              await registry.handleRequest(this.client, envelope);
            }
            break;
          }
          case 'notification': {
            const registry = this.findRegistryForService(envelope.serviceId);
            if (registry) {
              await registry.handleNotification(this.client, envelope);
            }
            break;
          }
          default:
            console.error(`unexpected message type: ${envelope.type}`);
        }
      }
    } finally {
      if (shutdownFn) {
        await shutdownFn();
      }
    }
  }

  stop(): void {
    this.running = false;
  }
}

export { HelloConfiguration, RunnerConfig, ServiceHelloDescriptor, SinkHelloDescriptor, HelloResponse } from './handler';
export { HandlerRegistry, RequestHandler, NotificationHandler } from './handler';
