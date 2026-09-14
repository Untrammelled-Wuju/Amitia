import { AmitiaError, TimeoutError, CancelledError, DependencyUnavailableError, RuntimeUnavailableError } from "./errors";
import type { RuntimeScope } from "./runtime";

export interface HostToolCallRequest {
  readonly toolId: string;
  readonly input: unknown;
  readonly timeoutMs?: number;
  readonly idempotencyKey?: string;
}

export interface HostToolCallResponse<T = unknown> {
  readonly ok: boolean;
  readonly value?: T;
  readonly error?: HostCallError;
  readonly durationMs?: number;
  readonly auditId?: string;
}

export interface HostCallError {
  readonly code: string;
  readonly message: string;
  readonly retryable: boolean;
  readonly details?: Record<string, unknown>;
}

export interface HostNetworkRequest {
  readonly url: string;
  readonly method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  readonly headers?: Record<string, string>;
  readonly body?: unknown;
  readonly timeoutMs?: number;
  readonly idempotencyKey?: string;
}

export interface HostNetworkResponse {
  readonly status: number;
  readonly headers: Record<string, string>;
  readonly body: unknown;
  readonly ok: boolean;
}

export interface HostMessageRef {
  readonly messageId: string;
  readonly conversationId: string;
  readonly characterId?: string;
}

export interface HostDesktopNotification {
  readonly title: string;
  readonly body: string;
  readonly icon?: string;
  readonly priority?: "low" | "normal" | "high";
  readonly actionUrl?: string;
}

export interface HostDesktopClipboardRequest {
  readonly text?: string;
  readonly image?: string;
  readonly format?: "text" | "image";
}

export interface HostClient {
  readonly tools: HostToolClient;
  readonly network: HostNetworkClient;
  readonly messages: HostMessageClient;
  readonly desktop: HostDesktopClient;
  readonly resources?: HostResourceClient;
  readonly vectors?: HostVectorClient;
}

export interface HostToolClient {
  execute<T = unknown>(request: HostToolCallRequest): Promise<HostToolCallResponse<T>>;
  list(): Promise<HostToolDescriptor[]>;
}

export interface HostToolDescriptor {
  readonly toolId: string;
  readonly title: string;
  readonly description?: string;
  readonly deprecated?: boolean;
  readonly riskLevel?: "low" | "medium" | "high" | "critical";
}

export interface HostNetworkClient {
  request(request: HostNetworkRequest): Promise<HostNetworkResponse>;
}

export interface HostMessageClient {
  get(messageId: string): Promise<HostMessageRef | null>;
  send(conversationId: string, content: string, options?: HostMessageSendOptions): Promise<HostMessageRef>;
}

export interface HostMessageSendOptions {
  readonly characterId?: string;
  readonly idempotencyKey?: string;
  readonly forceVoice?: boolean;
  readonly metadata?: Record<string, unknown>;
  readonly parts?: HostMessagePart[];
}

export interface HostMessagePart {
  readonly type: "text" | "image" | "audio" | "video" | "file";
  readonly content?: string;
  readonly extensionType?: string;
  readonly mimeType?: string;
  readonly url?: string;
  readonly fallbackUrl?: string;
  readonly altText?: string;
  readonly width?: number;
  readonly height?: number;
  readonly isAnimated?: boolean;
  readonly metadata?: Record<string, unknown>;
}

export interface HostResourceClient {
  link(request: { scope?: "package" | "data"; path: string }): Promise<{ url: string }>;
  delete(request: { scope?: "package" | "data"; path: string }): Promise<{ deleted: boolean }>;
}

export interface HostVectorPoint {
  readonly id: string;
  readonly vector?: number[];
  readonly text?: string;
  readonly payload?: Record<string, unknown>;
}

export interface HostVectorSearchResult {
  readonly id: string;
  readonly score: number;
  readonly payload?: Record<string, unknown>;
}

export interface HostVectorClient {
  upsert(collection: string, points: HostVectorPoint[]): Promise<{ upserted: number }>;
  search(collection: string, request: { query?: string; vector?: number[]; limit?: number; filter?: Record<string, string> }): Promise<{ items: HostVectorSearchResult[] }>;
  delete(collection: string, ids: string[]): Promise<{ deleted: number }>;
}

export interface HostDesktopClient {
  readonly notifications: HostNotificationClient;
  readonly clipboard: HostClipboardClient;
}

export interface HostNotificationClient {
  show(payload: HostDesktopNotification): Promise<void>;
}

export interface HostClipboardClient {
  write(request: HostDesktopClipboardRequest): Promise<void>;
  read(): Promise<HostDesktopClipboardRequest>;
}

export interface HostBridgeLike {
  invokeTool(request: HostToolCallRequest): Promise<HostToolCallResponse>;
  listTools?(): Promise<HostToolDescriptor[]>;
  networkRequest(request: HostNetworkRequest): Promise<HostNetworkResponse>;
  getMessage(messageId: string): Promise<HostMessageRef | null>;
  sendMessage(conversationId: string, content: string, options?: HostMessageSendOptions): Promise<HostMessageRef>;
  showNotification(payload: HostDesktopNotification): Promise<void>;
  clipboardWrite(request: HostDesktopClipboardRequest): Promise<void>;
  clipboardRead(): Promise<HostDesktopClipboardRequest>;
  resourceLink?(request: { scope?: "package" | "data"; path: string }): Promise<{ url: string }>;
  resourceDelete?(request: { scope?: "package" | "data"; path: string }): Promise<{ deleted: boolean }>;
  vectorUpsert?(collection: string, points: HostVectorPoint[]): Promise<{ upserted: number }>;
  vectorSearch?(collection: string, request: { query?: string; vector?: number[]; limit?: number; filter?: Record<string, string> }): Promise<{ items: HostVectorSearchResult[] }>;
  vectorDelete?(collection: string, ids: string[]): Promise<{ deleted: number }>;
}

export class DefaultHostClient implements HostClient {
  readonly resources?: HostResourceClient;
  readonly vectors?: HostVectorClient;

  readonly tools: HostToolClient = {
    execute: async <T = unknown>(request: HostToolCallRequest): Promise<HostToolCallResponse<T>> => {
      return this.bridge.invokeTool(request) as Promise<HostToolCallResponse<T>>;
    },
    list: async (): Promise<HostToolDescriptor[]> => {
      if (this.bridge.listTools) return this.bridge.listTools();
      return [];
    },
  };

  readonly network: HostNetworkClient = {
    request: async (request: HostNetworkRequest): Promise<HostNetworkResponse> => {
      return this.bridge.networkRequest(request);
    },
  };

  readonly messages: HostMessageClient = {
    get: async (messageId: string): Promise<HostMessageRef | null> => {
      return this.bridge.getMessage(messageId);
    },
    send: async (conversationId: string, content: string, options?: HostMessageSendOptions): Promise<HostMessageRef> => {
      return this.bridge.sendMessage(conversationId, content, options);
    },
  };

  readonly desktop: HostDesktopClient = {
    notifications: {
      show: async (payload: HostDesktopNotification): Promise<void> => {
        return this.bridge.showNotification(payload);
      },
    },
    clipboard: {
      write: async (request: HostDesktopClipboardRequest): Promise<void> => {
        return this.bridge.clipboardWrite(request);
      },
      read: async (): Promise<HostDesktopClipboardRequest> => {
        return this.bridge.clipboardRead();
      },
    },
  };

  constructor(
    private readonly bridge: HostBridgeLike,
    _scope: RuntimeScope,
    _traceId: string,
  ) {
    if (this.bridge.resourceLink) {
      this.resources = {
        link: async (request) => this.bridge.resourceLink!(request),
        delete: async (request) => {
          if (!this.bridge.resourceDelete) throw new Error("host resource delete is unavailable");
          return this.bridge.resourceDelete(request);
        },
      };
    }
    if (this.bridge.vectorUpsert && this.bridge.vectorSearch && this.bridge.vectorDelete) {
      this.vectors = {
        upsert: async (collection, points) => this.bridge.vectorUpsert!(collection, points),
        search: async (collection, request) => this.bridge.vectorSearch!(collection, request),
        delete: async (collection, ids) => this.bridge.vectorDelete!(collection, ids),
      };
    }
  }
}

export function mapHostError(cause: unknown, defaultCode = "host_call_failed"): AmitiaError {
  if (cause instanceof AmitiaError) return cause;
  const message = cause instanceof Error ? cause.message : String(cause);
  if (/timeout|deadline/i.test(message)) {
    return new TimeoutError(message);
  }
  if (/cancel|abort/i.test(message)) {
    return new CancelledError(message);
  }
  if (/unavailable|not running|offline/i.test(message)) {
    return new DependencyUnavailableError(message);
  }
  if (/runtime|crash/i.test(message)) {
    return new RuntimeUnavailableError(message);
  }
  return new AmitiaError(message, {
    code: defaultCode,
    category: "internal",
    retryable: false,
  });
}

export function unwrapHostCall<T>(response: HostToolCallResponse<T>): T {
  if (!response.ok || response.error) {
    const err = response.error ?? { code: "host_call_failed", message: "unknown error", retryable: false };
    throw new AmitiaError(err.message, {
      code: err.code,
      category: "internal",
      retryable: err.retryable,
      details: err.details,
    });
  }
  return response.value as T;
}
