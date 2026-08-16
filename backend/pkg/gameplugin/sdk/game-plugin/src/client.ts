import { Envelope } from './protocol';
import { Transport } from './transport';
import { SDKError, createEncodeError, createTransportError, createValidationError } from './errors';
import { validateMessageId, validatePluginMethod, validateMethod } from './validation';
import { v4 as uuidv4 } from 'uuid';

export interface PendingRequest {
  id: string;
  method: string;
  resolve: (envelope: Envelope) => void;
  reject: (error: Error) => void;
  timer: ReturnType<typeof setTimeout>;
  createdAt: number;
}

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

export function withTimeout(timeoutMs: number): MessageOption {
  return (envelope: Envelope) => {
    if (!envelope.metadata) {
      envelope.metadata = {};
    }
    envelope.metadata['__timeout'] = timeoutMs;
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

export const DEFAULT_RPC_TIMEOUT_MS = 30000;

export class Client {
  private transport: Transport;
  private idGenerator: IDGenerator;
  private pluginId: string;
  private runtimeId: string;
  private serviceId: string;
  private pending: Map<string, PendingRequest> = new Map();
  private pendingTimeoutMs: number = DEFAULT_RPC_TIMEOUT_MS;

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

  getPendingCount(): number {
    return this.pending.size;
  }

  cancelPendingRequests(reason: string = 'client cancelled'): void {
    for (const [id, pending] of this.pending) {
      clearTimeout(pending.timer);
      pending.reject(new Error(`request ${id} cancelled: ${reason}`));
    }
    this.pending.clear();
  }

  async sendRequest(
    method: string,
    payload?: unknown,
    ...opts: MessageOption[]
  ): Promise<Envelope> {
    const err = validatePluginMethod(method);
    if (err) throw createValidationError(err);

    const envelope = this.newRequest(method, payload, ...opts)
    return this.sendWithPending(envelope, opts)
  }

  async sendReservedRequest(
    method: string,
    payload?: unknown,
    ...opts: MessageOption[]
  ): Promise<Envelope> {
    const envelope = this.newRequest(method, payload, ...opts)
    return this.sendWithPending(envelope, opts)
  }

  private async sendWithPending(
    envelope: Envelope,
    opts: MessageOption[]
  ): Promise<Envelope> {
    const id = envelope.id!
    let timeout = this.pendingTimeoutMs
    if (envelope.metadata && envelope.metadata['__timeout']) {
      const t = envelope.metadata['__timeout']
      if (typeof t === 'number') {
        timeout = t
      }
    }

    return new Promise<Envelope>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id)
        reject(new Error(`request ${id} timed out after ${timeout}ms`))
      }, timeout)

      this.pending.set(id, {
        id,
        method: envelope.method || '',
        resolve,
        reject,
        timer,
        createdAt: Date.now(),
      })

      this.transport.send(envelope).catch((err) => {
        this.pending.delete(id)
        clearTimeout(timer)
        reject(err)
      })
    })
  }

  handleIncomingResponse(envelope: Envelope): boolean {
    if (envelope.type !== 'response' && envelope.type !== 'error') {
      return false;
    }
    const requestId = envelope.requestId;
    if (!requestId) return false;
    const pending = this.pending.get(requestId);
    if (!pending) return false;

    clearTimeout(pending.timer);
    this.pending.delete(requestId);

    if (envelope.type === 'error') {
      const errMsg = envelope.error ? `${envelope.error.code}: ${envelope.error.message}` : 'unknown error';
      pending.reject(new Error(errMsg));
    } else {
      pending.resolve(envelope);
    }
    return true;
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

  async sendReservedNotification(
    method: string,
    payload?: unknown,
    ...opts: MessageOption[]
  ): Promise<Envelope> {
    const err = validateMethod(method);
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
