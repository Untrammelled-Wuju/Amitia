import { Envelope } from './protocol';
import { Transport } from './transport';
import { SDKError, createEncodeError, createTransportError, createValidationError } from './errors';
import { validateMessageId, validatePluginMethod } from './validation';
import { v4 as uuidv4 } from 'uuid';

export type MessageOption = (envelope: Envelope) => void;

export function withRuntimeID(id: string): MessageOption {
  return (envelope: Envelope) => {
    envelope.runtimeId = id;
  };
}

export function withPluginID(id: string): MessageOption {
  return (envelope: Envelope) => {
    envelope.pluginId = id;
  };
}

export function withServiceID(id: string): MessageOption {
  return (envelope: Envelope) => {
    envelope.serviceId = id;
  };
}

export function withMetadata(key: string, value: unknown): MessageOption {
  return (envelope: Envelope) => {
    if (!envelope.metadata) {
      envelope.metadata = {};
    }
    envelope.metadata[key] = value;
  };
}

export interface IDGenerator {
  newID(): string;
}

export class UUIDGenerator implements IDGenerator {
  newID(): string {
    return uuidv4();
  }
}

export class FixedIDGenerator implements IDGenerator {
  private ids: string[];
  private current: number;

  constructor(...ids: string[]) {
    this.ids = ids;
    this.current = 0;
  }

  newID(): string {
    if (this.current >= this.ids.length) {
      return 'msg-overflow';
    }
    const id = this.ids[this.current];
    this.current++;
    return id;
  }
}

export interface ClientOptions {
  idGenerator?: IDGenerator;
  pluginId?: string;
  runtimeId?: string;
  serviceId?: string;
}

export class Client {
  private transport: Transport;
  private idGenerator: IDGenerator;
  private pluginId: string;
  private runtimeId: string;
  private serviceId: string;

  constructor(transport: Transport, options: ClientOptions = {}) {
    this.transport = transport;
    this.idGenerator = options.idGenerator || new UUIDGenerator();
    this.pluginId = options.pluginId || '';
    this.runtimeId = options.runtimeId || '';
    this.serviceId = options.serviceId || '';
  }

  getTransport(): Transport {
    return this.transport;
  }

  async sendRequest(
    method: string,
    payload?: unknown,
    ...opts: MessageOption[]
  ): Promise<Envelope> {
    const err = validatePluginMethod(method);
    if (err) throw createValidationError(err);

    const envelope = this.newRequest(method, payload, ...opts);
    await this.transport.send(envelope);
    return envelope;
  }

  async sendResponse(
    request: Envelope,
    payload?: unknown,
    ...opts: MessageOption[]
  ): Promise<Envelope> {
    const envelope = this.newResponse(request, payload, ...opts);
    await this.transport.send(envelope);
    return envelope;
  }

  async sendNotification(
    method: string,
    payload?: unknown,
    ...opts: MessageOption[]
  ): Promise<Envelope> {
    const err = validatePluginMethod(method);
    if (err) throw createValidationError(err);

    const envelope = this.newNotification(method, payload, ...opts);
    await this.transport.send(envelope);
    return envelope;
  }

  async sendError(
    request: Envelope,
    code: string,
    message: string,
    retryable?: boolean,
    data?: unknown,
    ...opts: MessageOption[]
  ): Promise<Envelope> {
    const envelope = this.newError(request, code, message, retryable, data, ...opts);
    await this.transport.send(envelope);
    return envelope;
  }

  newRequest(method: string, payload?: unknown, ...opts: MessageOption[]): Envelope {
    const id = this.idGenerator.newID();
    const envelope: Envelope = {
      protocol: 'amitia-game-host/1',
      type: 'request',
      id,
      method,
      payload,
      pluginId: this.pluginId,
      runtimeId: this.runtimeId,
      serviceId: this.serviceId,
    };
    opts.forEach((opt) => opt(envelope));
    return envelope;
  }

  newResponse(request: Envelope, payload?: unknown, ...opts: MessageOption[]): Envelope {
    const id = this.idGenerator.newID();
    const envelope: Envelope = {
      protocol: 'amitia-game-host/1',
      type: 'response',
      id,
      requestId: request.id,
      payload,
      pluginId: this.pluginId,
      runtimeId: this.runtimeId,
      serviceId: this.serviceId,
    };
    opts.forEach((opt) => opt(envelope));
    return envelope;
  }

  newNotification(method: string, payload?: unknown, ...opts: MessageOption[]): Envelope {
    const id = this.idGenerator.newID();
    const envelope: Envelope = {
      protocol: 'amitia-game-host/1',
      type: 'notification',
      id,
      method,
      payload,
      pluginId: this.pluginId,
      runtimeId: this.runtimeId,
      serviceId: this.serviceId,
    };
    opts.forEach((opt) => opt(envelope));
    return envelope;
  }

  newError(
    request: Envelope,
    code: string,
    message: string,
    retryable?: boolean,
    data?: unknown,
    ...opts: MessageOption[]
  ): Envelope {
    const id = this.idGenerator.newID();
    const envelope: Envelope = {
      protocol: 'amitia-game-host/1',
      type: 'error',
      id,
      requestId: request.id,
      pluginId: this.pluginId,
      runtimeId: this.runtimeId,
      serviceId: this.serviceId,
      error: {
        code,
        message,
        retryable,
        data,
      },
    };
    opts.forEach((opt) => opt(envelope));
    return envelope;
  }

  async receive(): Promise<Envelope> {
    return this.transport.receive();
  }

  async close(): Promise<void> {
    await this.transport.close();
  }
}
