import type { ExtensionID, ModuleID } from "./types";

export interface RuntimeContext {
  extensionId: ExtensionID;
  moduleId: ModuleID;
  generation: number;
  scope: RuntimeScope;
  platform: string;
  locale: string;
  traceId: string;
  deadline?: number;
}

export interface RuntimeScope {
  scope: "global" | "character" | "conversation";
  characterId?: string;
  conversationId?: string;
  userId?: string;
}

export interface RuntimeCallResult<T = unknown> {
  ok: boolean;
  value?: T;
  error?: RuntimeError;
}

export interface RuntimeError {
  code: string;
  message: string;
  details?: unknown;
  recoverable: boolean;
}

export interface ToolCallRequest {
  toolId: string;
  input: unknown;
  timeout?: number;
  idempotencyKey?: string;
}

export interface ToolCallResponse {
  result?: unknown;
  error?: RuntimeError;
  duration?: number;
}

export interface StorageEntry {
  key: string;
  value: unknown;
  version: number;
  updatedAt: string;
}

export interface EventBusSubscription {
  subscriptionId: string;
  eventTypes: string[];
}

export interface EventBusEvent {
  type: string;
  payload: unknown;
  timestamp: string;
  source: string;
}

export interface HostBridge {
  invokeTool(request: ToolCallRequest): Promise<ToolCallResponse>;
  storageGet(key: string): Promise<StorageEntry | null>;
  storageSet(key: string, value: unknown): Promise<void>;
  storageDelete(key: string): Promise<void>;
  storageList(prefix?: string): Promise<StorageEntry[]>;
  emitEvent(type: string, payload: unknown): Promise<void>;
  subscribeEvents(eventTypes: string[], callback: (event: EventBusEvent) => void): Promise<EventBusSubscription>;
  unsubscribe(subscriptionId: string): Promise<void>;
  log(level: "debug" | "info" | "warn" | "error", message: string, fields?: Record<string, unknown>): void;
  fetchFromHost(url: string, init?: RequestInit): Promise<Response>;
  requestPermission(permission: string, reason?: string): Promise<boolean>;
}

export abstract class ExtensionRuntime {
  protected bridge: HostBridge;
  protected context: RuntimeContext;

  constructor(bridge: HostBridge, context: RuntimeContext) {
    this.bridge = bridge;
    this.context = context;
  }

  abstract onActivate(): Promise<void>;
  abstract onDeactivate(): Promise<void>;

  getExtensionId(): ExtensionID {
    return this.context.extensionId;
  }

  getModuleId(): ModuleID {
    return this.context.moduleId;
  }

  getGeneration(): number {
    return this.context.generation;
  }

  getScope(): RuntimeScope {
    return this.context.scope;
  }

  protected async invokeTool(toolId: string, input: unknown, timeout?: number): Promise<unknown> {
    const response = await this.bridge.invokeTool({ toolId, input, timeout });
    if (response.error) {
      throw new ExtensionRuntimeError(response.error);
    }
    return response.result;
  }

  protected async storageGet<T = unknown>(key: string): Promise<T | null> {
    const entry = await this.bridge.storageGet(key);
    if (!entry) return null;
    return entry.value as T;
  }

  protected async storageSet(key: string, value: unknown): Promise<void> {
    await this.bridge.storageSet(key, value);
  }

  protected async storageDelete(key: string): Promise<void> {
    await this.bridge.storageDelete(key);
  }

  protected async emitEvent(type: string, payload: unknown): Promise<void> {
    await this.bridge.emitEvent(type, payload);
  }

  protected log(level: "debug" | "info" | "warn" | "error", message: string, fields?: Record<string, unknown>): void {
    this.bridge.log(level, message, fields);
  }
}

export class ExtensionRuntimeError extends Error {
  readonly code: string;
  readonly recoverable: boolean;
  readonly details?: unknown;

  constructor(error: RuntimeError) {
    super(error.message);
    this.name = "ExtensionRuntimeError";
    this.code = error.code;
    this.recoverable = error.recoverable;
    this.details = error.details;
  }
}

export type RuntimeFactory<T extends ExtensionRuntime = ExtensionRuntime> = (
  bridge: HostBridge,
  context: RuntimeContext
) => T;

export interface RuntimeRegistration {
  moduleId: string;
  factory: RuntimeFactory;
}

const registrations: RuntimeRegistration[] = [];

export function registerRuntime<T extends ExtensionRuntime>(
  moduleId: string,
  factory: RuntimeFactory<T>
): void {
  registrations.push({ moduleId, factory });
}

export function getRegistrations(): RuntimeRegistration[] {
  return [...registrations];
}

export function clearRegistrations(): void {
  registrations.length = 0;
}
