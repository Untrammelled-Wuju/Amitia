import type {
  ToolContributionDefinition,
  SkillContributionDefinition,
  WorkflowContributionDefinition,
  UIContributionDefinition,
} from "./manifest";
import type { ExtensionID, ModuleID } from "./types";
import { makeExtensionID, makeModuleID } from "./types";
import type { RuntimeScope, RuntimeContext } from "./runtime";
import type { ToolHandler, ToolRegistration } from "./tools";
import { defineTool, listTools, clearTools } from "./tools";
import type { EventSubscriptionSpec } from "./events";
import { defineEvent, listEventSpecs, clearEventSpecs } from "./events";
import type { HookRegistration } from "./hooks";
import {
  defineBeforeHook,
  defineFilterHook,
  defineTransformHook,
  defineObserveHook,
  listHooks,
  clearHooks,
} from "./hooks";
import type { TaskDefinition } from "./tasks";
import { defineTask, listTasks, clearTasks } from "./tasks";
import type { StorageClient } from "./storage";
import { NamespacedStorageClient, type StorageBackend } from "./storage";
import type { SecretReferenceClient } from "./secrets";
import { NamespacedSecretClient, type SecretBackend, InMemorySecretBackend } from "./secrets";
import type { HostClient } from "./host";
import { DefaultHostClient, type HostBridgeLike } from "./host";
import type { UIBridge } from "./ui";
import { AmitiaError, ValidationError, PermissionDeniedError } from "./errors";

export interface ExtensionIdentity {
  readonly extensionId: ExtensionID;
  readonly version: string;
  readonly contractVersion: number;
  readonly platform: string;
  readonly locale: string;
}

export interface ModuleIdentity {
  readonly moduleId: ModuleID;
  readonly kind: string;
}

export interface RuntimeLifecycleClient {
  readonly generation: number;
  readonly startedAt: string;
  readonly signal: AbortSignal;
  readonly abort(reason?: string): void;
}

export interface ExtensionLogger {
  debug(message: string, fields?: Record<string, unknown>): void;
  info(message: string, fields?: Record<string, unknown>): void;
  warn(message: string, fields?: Record<string, unknown>): void;
  error(message: string, fields?: Record<string, unknown>): void;
}

export interface ResourceClient {
  readPackageResource(path: string): Promise<Uint8Array>;
  readHostResource(handle: string): Promise<Uint8Array>;
  writeArtifact(name: string, content: Uint8Array, options?: { kind?: string; mimeType?: string }): Promise<{ handle: string }>;
  selectUserFile(options?: { accept?: string; multiple?: boolean }): Promise<string[]>;
  saveExport(name: string, content: Uint8Array): Promise<string>;
  closeHandle(handle: string): Promise<void>;
}

export interface HandlerBinder {
  bindTool(entryName: string, definition: ToolContributionDefinition, handler: ToolHandler): ToolRegistration;
  bindEvent(entryName: string, spec: EventSubscriptionSpec): EventSubscriptionSpec;
  bindBeforeHook<I>(entryName: string, pipeline: string, stage: string, handler: (input: I, ctx: unknown) => Promise<unknown>): HookRegistration;
  bindFilterHook<I>(entryName: string, pipeline: string, stage: string, handler: (input: I, ctx: unknown) => Promise<unknown>): HookRegistration;
  bindTransformHook<I, O>(entryName: string, pipeline: string, stage: string, handler: (input: I, ctx: unknown) => Promise<O>): HookRegistration<I, O>;
  bindObserveHook<I>(entryName: string, pipeline: string, stage: string, handler: (input: I, ctx: unknown) => Promise<void>): HookRegistration<I, void>;
  bindTask<I, O>(entryName: string, definition: TaskDefinition<I, O>): TaskDefinition<I, O>;
  bindUIAction(entryName: string, actionId: string, handler: unknown): void;
}

export interface ExtensionContext {
  readonly extension: ExtensionIdentity;
  readonly module: ModuleIdentity;
  readonly runtime: RuntimeContext;
  readonly handlers: HandlerBinder;
  readonly host: HostClient;
  readonly storage: StorageClient;
  readonly secrets: SecretReferenceClient;
  readonly resources: ResourceClient;
  readonly events: { emit<T>(type: string, payload: T): Promise<void> };
  readonly logger: ExtensionLogger;
  readonly lifecycle: RuntimeLifecycleClient;
  readonly ui?: UIBridge;
}

export interface ExtensionDefinition {
  readonly activate: (context: ExtensionContext) => Promise<void> | void;
  readonly deactivate?: (reason: string) => Promise<void> | void;
}

export interface ExtensionBootstrap {
  readonly extensionId: string;
  readonly publisher: string;
  readonly version: string;
  readonly contractVersion: number;
  readonly moduleId: string;
  readonly moduleKind: string;
  readonly allowedEntries: string[];
  readonly scope: RuntimeScope;
  readonly traceId: string;
  readonly generation: number;
  readonly platform: string;
  readonly locale: string;
  readonly hostBridge: HostBridgeLike;
  readonly storageBackend: StorageBackend;
  readonly secretBackend?: SecretBackend;
  readonly resourceBackend: ResourceBackend;
  readonly logger: ExtensionLogger;
  readonly signal: AbortSignal;
  readonly uiBridge?: UIBridge;
}

export interface ResourceBackend {
  readPackageResource(path: string): Promise<Uint8Array>;
  readHostResource(handle: string): Promise<Uint8Array>;
  writeArtifact(name: string, content: Uint8Array, options?: { kind?: string; mimeType?: string }): Promise<{ handle: string }>;
  selectUserFile(options?: { accept?: string; multiple?: boolean }): Promise<string[]>;
  saveExport(name: string, content: Uint8Array): Promise<string>;
  closeHandle(handle: string): Promise<void>;
}

export function defineExtension(definition: ExtensionDefinition): ExtensionDefinition {
  if (typeof definition.activate !== "function") {
    throw new ValidationError("extension.activate must be a function");
  }
  return definition;
}

export function bootstrapExtension(
  definition: ExtensionDefinition,
  bootstrap: ExtensionBootstrap,
): { context: ExtensionContext; cleanup: () => Promise<void> } {
  assertBootstrapValid(bootstrap);
  const namespace = `${bootstrap.publisher}/${bootstrap.extensionId}#${bootstrap.moduleId}`;
  const extensionId = makeExtensionID(bootstrap.publisher, bootstrap.extensionId);
  const moduleId = makeModuleID(extensionId, bootstrap.moduleId);

  const allowedEntries = new Set(bootstrap.allowedEntries);

  const extension: ExtensionIdentity = {
    extensionId,
    version: bootstrap.version,
    contractVersion: bootstrap.contractVersion,
    platform: bootstrap.platform,
    locale: bootstrap.locale,
  };
  const module: ModuleIdentity = {
    moduleId,
    kind: bootstrap.moduleKind,
  };

  const runtime: RuntimeContext = {
    extensionId,
    moduleId,
    generation: bootstrap.generation,
    scope: bootstrap.scope,
    platform: bootstrap.platform,
    locale: bootstrap.locale,
    traceId: bootstrap.traceId,
  };

  const host = new DefaultHostClient(bootstrap.hostBridge, bootstrap.scope, bootstrap.traceId);
  const storage = new NamespacedStorageClient(namespace, bootstrap.storageBackend);
  const secretBackend = bootstrap.secretBackend ?? new InMemorySecretBackend();
  const secrets = new NamespacedSecretClient(namespace, secretBackend);

  const logger = bootstrap.logger;
  let aborted = false;
  const abort = (reason?: string) => {
    aborted = true;
    logger.warn("extension lifecycle aborted", { reason });
  };
  const lifecycle: RuntimeLifecycleClient = {
    generation: bootstrap.generation,
    startedAt: new Date().toISOString(),
    signal: bootstrap.signal,
    abort,
  };

  const events = {
    emit: async <T>(type: string, payload: T): Promise<void> => {
      await bootstrap.hostBridge.sendMessage(`__event__:${type}`, JSON.stringify(payload));
    },
  };

  const resources: ResourceClient = bootstrap.resourceBackend;

  const assertEntry = (entryName: string) => {
    if (!allowedEntries.has(entryName)) {
      throw new PermissionDeniedError(
        `entry '${entryName}' is not declared in manifest`,
        { details: { entryName, allowed: Array.from(allowedEntries) } },
      );
    }
  };

  const handlers: HandlerBinder = {
    bindTool: (entryName, def, handler) => {
      assertEntry(entryName);
      return defineTool(def, handler);
    },
    bindEvent: (entryName, spec) => {
      assertEntry(entryName);
      return defineEvent(spec);
    },
    bindBeforeHook: (entryName, pipeline, stage, handler) => {
      assertEntry(entryName);
      return defineBeforeHook(pipeline, stage, handler as never);
    },
    bindFilterHook: (entryName, pipeline, stage, handler) => {
      assertEntry(entryName);
      return defineFilterHook(pipeline, stage, handler as never);
    },
    bindTransformHook: (entryName, pipeline, stage, handler) => {
      assertEntry(entryName);
      return defineTransformHook(pipeline, stage, handler as never);
    },
    bindObserveHook: (entryName, pipeline, stage, handler) => {
      assertEntry(entryName);
      return defineObserveHook(pipeline, stage, handler as never);
    },
    bindTask: (entryName, taskDef) => {
      assertEntry(entryName);
      return defineTask(taskDef);
    },
    bindUIAction: (entryName, _actionId, _handler) => {
      assertEntry(entryName);
      // UI actions are registered via ui.ts; we only assert entry allowance here
    },
  };

  const context: ExtensionContext = {
    extension,
    module,
    runtime,
    handlers,
    host,
    storage,
    secrets,
    resources,
    events,
    logger,
    lifecycle,
    ui: bootstrap.uiBridge,
  };

  const cleanup = async () => {
    clearTools();
    clearEventSpecs();
    clearHooks();
    clearTasks();
    try {
      if (definition.deactivate) {
        await definition.deactivate(aborted ? "aborted" : "shutdown");
      }
    } catch (cause) {
      logger.error("extension deactivate failed", { error: cause instanceof Error ? cause.message : String(cause) });
    }
  };

  return { context, cleanup };
}

function assertBootstrapValid(bootstrap: ExtensionBootstrap): void {
  if (!bootstrap.extensionId) throw new ValidationError("extensionId is required");
  if (!bootstrap.publisher) throw new ValidationError("publisher is required");
  if (!bootstrap.moduleId) throw new ValidationError("moduleId is required");
  if (!bootstrap.hostBridge) throw new ValidationError("hostBridge is required");
  if (!bootstrap.storageBackend) throw new ValidationError("storageBackend is required");
  if (!bootstrap.resourceBackend) throw new ValidationError("resourceBackend is required");
  if (!bootstrap.logger) throw new ValidationError("logger is required");
  if (!bootstrap.signal) throw new ValidationError("signal is required");
}

export function assertContractVersion(extension: ExtensionIdentity, hostContract: number): void {
  if (extension.contractVersion > hostContract) {
    throw new AmitiaError(
      `extension requires contract v${extension.contractVersion}, host supports v${hostContract}`,
      {
        code: "contract_version_mismatch",
        category: "runtime",
        retryable: false,
        details: { required: extension.contractVersion, host: hostContract },
      },
    );
  }
}

export { listTools, listEventSpecs, listHooks, listTasks };
