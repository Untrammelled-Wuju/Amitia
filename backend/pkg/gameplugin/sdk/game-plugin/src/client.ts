import { Envelope } from './protocol';
import { Transport } from './transport';
import { SDKError, createEncodeError, createTransportError, createValidationError } from './errors';
import { validateMessageId, validatePluginMethod, validateMethod, isReservedNamespace } from './validation';

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

export function withGeneration(generation: number): MessageOption {
  return (envelope: Envelope) => {
    envelope.generation = generation;
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
    const runtimeCrypto = (globalThis as unknown as { crypto?: {
      randomUUID?: () => string;
      getRandomValues?: (target: Uint8Array) => Uint8Array;
    } }).crypto;

    if (runtimeCrypto?.randomUUID) {
      return runtimeCrypto.randomUUID();
    }

    const bytes = new Uint8Array(16);
    if (runtimeCrypto?.getRandomValues) {
      runtimeCrypto.getRandomValues(bytes);
    } else {
      for (let i = 0; i < bytes.length; i++) {
        bytes[i] = Math.floor(Math.random() * 256);
      }
    }
    bytes[6] = (bytes[6] & 0x0f) | 0x40;
    bytes[8] = (bytes[8] & 0x3f) | 0x80;
    const hex = Array.from(bytes, value => value.toString(16).padStart(2, '0')).join('');
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
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
  generation?: number;
}

export const DEFAULT_RPC_TIMEOUT_MS = 30000;

export class Client {
  private transport: Transport;
  private idGenerator: IDGenerator;
  private pluginId: string;
  private runtimeId: string;
  private serviceId: string;
  private generation: number;
  private pending: Map<string, PendingRequest> = new Map();
  private pendingTimeoutMs: number = DEFAULT_RPC_TIMEOUT_MS;

  constructor(transport: Transport, options: ClientOptions = {}) {
    this.transport = transport;
    this.idGenerator = options.idGenerator || new UUIDGenerator();
    this.pluginId = options.pluginId || '';
    this.runtimeId = options.runtimeId || '';
    this.serviceId = options.serviceId || '';
    this.generation = options.generation || 0;
    if (!Number.isSafeInteger(this.generation) || this.generation < 0) {
      throw createValidationError('generation must be a non-negative safe integer');
    }
  }

  getTransport(): Transport {
    return this.transport;
  }

  getPendingCount(): number {
    return this.pending.size;
  }

  getGeneration(): number {
    return this.generation;
  }

  setGeneration(generation: number): void {
    if (!Number.isSafeInteger(generation) || generation < 0) {
      throw createValidationError('generation must be a non-negative safe integer');
    }
    this.generation = generation;
  }

  /**
   * Adopt the authoritative route assigned by GameHost after the bootstrap
   * handshake. Existing non-empty route fields may never be rebound.
   */
  adoptPeerRouting(envelope: Envelope): void {
    const generation = envelope.generation;
    if (!Number.isSafeInteger(generation) || !generation || generation < 1) {
      throw createValidationError('handshake response is missing a positive generation');
    }
    const adopt = (current: string, incoming: string | undefined, label: string): string => {
      if (!incoming) return current;
      if (current && current !== incoming) {
        throw createValidationError(`handshake ${label} mismatch: expected ${current}, got ${incoming}`);
      }
      return current || incoming;
    };
    if (this.generation && this.generation !== generation) {
      throw createValidationError(`handshake generation mismatch: expected ${this.generation}, got ${generation}`);
    }
    this.runtimeId = adopt(this.runtimeId, envelope.runtimeId, 'runtimeId');
    this.pluginId = adopt(this.pluginId, envelope.pluginId, 'pluginId');
    this.serviceId = adopt(this.serviceId, envelope.serviceId, 'serviceId');
    this.generation = generation;
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
    const methodErr = validateMethod(method);
    if (methodErr) throw createValidationError(methodErr);
    if (!isReservedNamespace(method)) {
      throw createValidationError(`reserved request method '${method}' is not in a reserved namespace`);
    }
    const envelope = this.newRequest(method, payload, ...opts);
    return this.sendWithPending(envelope, opts);
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

  supportsBinaryFrames(): boolean {
    return typeof this.transport.sendBinaryFrame === 'function';
  }

  async sendReservedBinaryRequest(
    method: string,
    objectId: string,
    offset: number,
    data: Uint8Array,
    ...opts: MessageOption[]
  ): Promise<Envelope> {
    const methodErr = validateMethod(method);
    if (methodErr) throw createValidationError(methodErr);
    if (!isReservedNamespace(method)) {
      throw createValidationError(`reserved request method '${method}' is not in a reserved namespace`);
    }
    const sendBinaryFrame = this.transport.sendBinaryFrame;
    if (typeof sendBinaryFrame !== 'function') {
      throw createTransportError('binary frame transport is unavailable');
    }
    const envelope = this.newRequest(method, undefined, ...opts);
    const id = envelope.id!;
    let timeout = this.pendingTimeoutMs;
    const timeoutValue = envelope.metadata?.['__timeout'];
    if (typeof timeoutValue === 'number' && timeoutValue > 0) timeout = timeoutValue;

    return new Promise<Envelope>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`request ${id} timed out after ${timeout}ms`));
      }, timeout);
      this.pending.set(id, { id, method, resolve, reject, timer, createdAt: Date.now() });
      sendBinaryFrame.call(this.transport, envelope, objectId, offset, data).catch((err) => {
        this.pending.delete(id);
        clearTimeout(timer);
        reject(err);
      });
    });
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
      generation: this.generation || undefined,
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
      generation: this.generation || undefined,
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
      generation: this.generation || undefined,
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
      generation: this.generation || undefined,
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
