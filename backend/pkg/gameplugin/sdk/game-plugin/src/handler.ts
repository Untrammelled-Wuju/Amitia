import { Envelope } from './protocol';
import { Client } from './client';
import { SDKError, createValidationError } from './errors';

export type RequestHandler = (request: Envelope) => Promise<unknown>;
export type NotificationHandler = (notification: Envelope) => Promise<void>;

export interface HelloConfiguration {
  supportedProtocols: string[];
  capabilities: string[];
  rpcNamespaces: string[];
  channels?: ChannelHelloDescriptor[];
  sinks?: SinkHelloDescriptor[];
  sdk?: {
    name: string;
    version: string;
  };
  metadata?: Record<string, unknown>;
}


export interface ChannelHelloDescriptor {
  id: string;
}

export interface SinkHelloDescriptor {
  sinkId: string;
  kind: string;
  serviceId?: string;
}

export interface RunnerConfig {
  pluginId: string;
  defaultServiceId?: string;
  hello: HelloConfiguration;
  onReady?: (client: Client) => Promise<() => Promise<void>>;
}

export interface HelloResponse {
  protocol: string;
  capabilities: string[];
  rpcNamespaces?: string[];
  channels?: string[];
  metadata?: Record<string, unknown>;
}

export class HandlerRegistry {
  private requestHandlers: Map<string, RequestHandler> = new Map();
  private notificationHandlers: Map<string, NotificationHandler> = new Map();

  registerRequest(method: string, handler: RequestHandler): void {
    this.requestHandlers.set(method, handler);
  }

  registerNotification(method: string, handler: NotificationHandler): void {
    this.notificationHandlers.set(method, handler);
  }

  /** Merge another registry into this one. Duplicate request methods are rejected;
   * duplicate notification methods are chained in registration order. */
  mergeFrom(other: HandlerRegistry): void {
    if (other === this) return;
    for (const [method, handler] of other.requestHandlers) {
      if (this.requestHandlers.has(method)) {
        throw new Error(`duplicate request handler: ${method}`);
      }
      this.requestHandlers.set(method, handler);
    }
    for (const [method, handler] of other.notificationHandlers) {
      const existing = this.notificationHandlers.get(method);
      if (existing) {
        this.notificationHandlers.set(method, async (notification) => {
          await existing(notification);
          await handler(notification);
        });
      } else {
        this.notificationHandlers.set(method, handler);
      }
    }
  }

  async handleRequest(client: Client, request: Envelope): Promise<void> {
    const handler = this.requestHandlers.get(request.method!);
    if (!handler) {
      await this.sendError(client, request, 'not_found', `unknown method: ${request.method}`, false);
      return;
    }

    try {
      const response = await handler(request);
      await client.sendResponse(request, response);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      await this.sendError(client, request, 'internal', message, false);
    }
  }

  async handleNotification(client: Client, notification: Envelope): Promise<void> {
    const handler = this.notificationHandlers.get(notification.method!);
    if (!handler) {
      return;
    }

    try {
      await handler(notification);
    } catch (err) {
      console.error(`notification handler error: ${err}`);
    }
  }

  private async sendError(
    client: Client,
    request: Envelope,
    code: string,
    message: string,
    retryable: boolean
  ): Promise<void> {
    await client.sendError(request, code, message, retryable);
  }
}

export const HELLO_METHOD = 'control.handshake.hello';
