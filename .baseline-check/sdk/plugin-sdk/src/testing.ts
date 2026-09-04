import type { HookRegistration, HookContext } from "./hooks";
import type { TaskDefinition, TaskResult, TaskInput } from "./tasks";
import type { StorageBackend, StorageValue } from "./storage";
import type { SecretBackend } from "./secrets";
import { InMemorySecretBackend } from "./secrets";
import type { HostBridgeLike } from "./host";
import { type ExtensionContext, type ExtensionBootstrap, type ResourceBackend, type ExtensionLogger } from "./context";
import { bootstrapExtension, defineExtension, type ExtensionDefinition } from "./context";
import { defineTask } from "./tasks";
import { defineEvent } from "./events";
import { defineTool } from "./tools";
import {
  defineBeforeHook,
  defineFilterHook,
  defineTransformHook,
  defineObserveHook,
} from "./hooks";
import { AmitiaError, ValidationError, PermissionDeniedError, TimeoutError, CancelledError } from "./errors";

export interface MockHostOptions {
  readonly namespace?: string;
  readonly allowedEntries?: string[];
  readonly scope?: {
    scope: "global" | "character" | "conversation";
    characterId?: string;
    conversationId?: string;
    userId?: string;
  };
  readonly secretBackend?: SecretBackend;
  readonly storageBackend?: StorageBackend;
}

export interface MockHost {
  readonly context: ExtensionContext;
  readonly invokeTool: (toolId: string, input: unknown) => Promise<unknown>;
  readonly emitEvent: <T>(type: string, payload: T) => Promise<void>;
  readonly executeHook: <I, O>(registration: HookRegistration<I, O>, input: I) => Promise<O | void>;
  readonly runTask: <I extends TaskInput, O>(definition: TaskDefinition<I, O>, input: I) => Promise<TaskResult<O>>;
  readonly mockPermission: (permission: string, granted: boolean) => void;
  readonly mockScope: (scope: MockHostOptions["scope"]) => void;
  readonly mockStorage: () => StorageBackend;
  readonly mockSecretReference: () => SecretBackend;
  readonly advanceTime: (ms: number) => void;
  readonly simulateCancellation: () => void;
  readonly simulateRuntimeCrash: () => void;
}

export function createMockHost(
  definition: ExtensionDefinition,
  options: MockHostOptions = {},
): MockHost {
  const allowedEntries = new Set(options.allowedEntries ?? ["*"]);
  const scope = options.scope ?? { scope: "global" as const };

  const abortController = new AbortController();
  const logger: ExtensionLogger = {
    debug: () => {},
    info: () => {},
    warn: () => {},
    error: () => {},
  };

  const secretBackend = options.secretBackend ?? new InMemorySecretBackend();
  const storageBackend = options.storageBackend ?? new InMemoryStorageBackend();

  const permissionGrants = new Map<string, boolean>();
  let currentScope = scope;

  const hostBridge: HostBridgeLike = {
    invokeTool: async (_request) => {
      return { ok: true, value: undefined, durationMs: 0 };
    },
    listTools: async () => [],
    networkRequest: async () => ({ status: 200, headers: {}, body: null, ok: true }),
    getMessage: async () => null,
    sendMessage: async (_conversationId, _content) => ({
      messageId: `msg-${Date.now()}`,
      conversationId: _conversationId,
    }),
    showNotification: async () => {},
    clipboardWrite: async () => {},
    clipboardRead: async () => ({ format: "text" }),
  };

  const resourceBackend: ResourceBackend = {
    readPackageResource: async (_path) => new Uint8Array(),
    readHostResource: async (_handle) => new Uint8Array(),
    writeArtifact: async (_name, _content) => ({ handle: `artifact-${Date.now()}` }),
    selectUserFile: async () => [],
    saveExport: async (_name, _content) => `export-${Date.now()}`,
    closeHandle: async () => {},
  };

  const bootstrap: ExtensionBootstrap = {
    extensionId: "mock",
    publisher: "mock",
    version: "0.0.0-mock",
    contractVersion: 1,
    moduleId: "mock-module",
    moduleKind: "tool",
    allowedEntries: Array.from(allowedEntries),
    scope: currentScope as any,
    traceId: `mock-${Date.now()}`,
    generation: 1,
    platform: "test",
    locale: "en",
    hostBridge,
    storageBackend,
    secretBackend,
    resourceBackend,
    logger,
    signal: abortController.signal,
  };

  const { context } = bootstrapExtension(definition, bootstrap);

  return {
    context,
    invokeTool: async (toolId, input) => {
      return context.host.tools.execute({ toolId, input });
    },
    emitEvent: async (type, payload) => {
      await context.events.emit(type, payload);
    },
    executeHook: async <I, O>(registration: HookRegistration<I, O>, input: I) => {
      const hookCtx: HookContext = {
        phase: registration.phase,
        pipeline: registration.pipeline,
        stage: registration.stage,
        traceId: `mock-hook-${Date.now()}`,
        logger: {
          debug: () => {},
          info: () => {},
          warn: () => {},
          error: () => {},
        },
        abort: () => {},
      };
      const handler = registration.handler as (input: I, ctx: HookContext) => Promise<O | void>;
      return handler(input, hookCtx);
    },
    runTask: async (taskDef, input) => {
      const { runTask } = await import("./tasks");
      return runTask(taskDef, input, {
        traceId: `mock-task-${Date.now()}`,
        logger: {
          debug: () => {},
          info: () => {},
          warn: () => {},
          error: () => {},
        },
      });
    },
    mockPermission: (permission, granted) => {
      permissionGrants.set(permission, granted);
    },
    mockScope: (newScope) => {
      currentScope = newScope ?? currentScope;
    },
    mockStorage: () => storageBackend,
    mockSecretReference: () => secretBackend,
    advanceTime: (_ms) => {
      // no-op; tests can override
    },
    simulateCancellation: () => {
      abortController.abort();
    },
    simulateRuntimeCrash: () => {
      abortController.abort("runtime_crash");
    },
  };
}

export function createExtensionTestRuntime(definition: ExtensionDefinition, options?: MockHostOptions): MockHost {
  return createMockHost(definition, options);
}

export function invokeTool(host: MockHost, toolId: string, input: unknown): Promise<unknown> {
  return host.invokeTool(toolId, input);
}

export function emitEvent<T>(host: MockHost, type: string, payload: T): Promise<void> {
  return host.emitEvent(type, payload);
}

export function executeHook<I, O>(
  host: MockHost,
  registration: HookRegistration<I, O>,
  input: I,
): Promise<O | void> {
  return host.executeHook(registration, input);
}

export function mockPermission(host: MockHost, permission: string, granted: boolean): void {
  host.mockPermission(permission, granted);
}

export function mockScope(host: MockHost, scope: MockHostOptions["scope"]): void {
  host.mockScope(scope);
}

export function mockStorage(host: MockHost): StorageBackend {
  return host.mockStorage();
}

export function mockSecretReference(host: MockHost): SecretBackend {
  return host.mockSecretReference();
}

export function advanceTime(host: MockHost, ms: number): void {
  host.advanceTime(ms);
}

export function simulateCancellation(host: MockHost): void {
  host.simulateCancellation();
}

export function simulateRuntimeCrash(host: MockHost): void {
  host.simulateRuntimeCrash();
}

export class InMemoryStorageBackend implements StorageBackend {
  private readonly store = new Map<string, StorageValue>();

  async get(_namespace: string, key: string): Promise<StorageValue | null> {
    return this.store.get(key) ?? null;
  }
  async set(_namespace: string, key: string, value: unknown, options?: { scope?: any }): Promise<StorageValue> {
    const entry: StorageValue = {
      key,
      value,
      version: (this.store.get(key)?.version ?? 0) + 1,
      updatedAt: new Date().toISOString(),
      scope: options?.scope ?? "extension",
    };
    this.store.set(key, entry);
    return entry;
  }
  async cas(namespace: string, request: import("./storage").StorageCASRequest): Promise<StorageValue> {
    const existing = this.store.get(request.key);
    if (request.expectedVersion !== undefined && existing?.version !== request.expectedVersion) {
      throw new ValidationError("version mismatch");
    }
    return this.set(namespace, request.key, request.newValue, { scope: request.scope });
  }
  async delete(_namespace: string, key: string): Promise<void> {
    this.store.delete(key);
  }
  async list(_namespace: string, query?: { prefix?: string }): Promise<import("./storage").StoragePage> {
    const items: StorageValue[] = [];
    for (const entry of this.store.values()) {
      if (query?.prefix && !entry.key.startsWith(query.prefix)) continue;
      items.push(entry);
    }
    return { items, hasMore: false };
  }
  async transaction<T>(namespace: string, callback: (ctx: any) => Promise<T>): Promise<T> {
    const ctx: any = {
      get: async (key: string) => this.get(namespace, key),
      set: async (key: string, value: unknown) => this.set(namespace, key, value),
      delete: async (key: string) => this.delete(namespace, key),
      list: async () => this.list(namespace),
    };
    return callback(ctx);
  }
}

export { defineExtension, defineTool, defineEvent, defineTask };
export { defineBeforeHook, defineFilterHook, defineTransformHook, defineObserveHook };
export { AmitiaError, ValidationError, PermissionDeniedError, TimeoutError, CancelledError };
