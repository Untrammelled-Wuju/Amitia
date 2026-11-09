import type {
  ToolContributionDefinition,
} from "./manifest";
import type { ExtensionID, ModuleID } from "./types";
import { makeExtensionID, makeModuleID } from "./types";
import type {
  ContributionClient,
  ContributionRegistration,
  AdapterClient,
  RuntimeScope,
  RuntimeContext,
  ServiceClient,
  ServiceLease,
} from "./runtime";
import type { ToolHandler, ToolRegistration } from "./tools";
import { defineTool, listTools, unregisterTool } from "./tools";
import type { EventSubscriptionSpec } from "./events";
import { defineEvent, listEventSpecs, unregisterEventSpec } from "./events";
import type { HookRegistration } from "./hooks";
import {
  defineBeforeHook,
  defineFilterHook,
  defineTransformHook,
  defineObserveHook,
  listHooks,
  unregisterHook,
} from "./hooks";
import type { TaskDefinition, TaskInput } from "./tasks";
import { defineTask, listTasks, unregisterTask } from "./tasks";
import type { StorageClient } from "./storage";
import { NamespacedStorageClient, type StorageBackend } from "./storage";
import type { SecretReferenceClient } from "./secrets";
import { NamespacedSecretClient, type SecretBackend, InMemorySecretBackend } from "./secrets";
import type { HostClient } from "./host";
import { DefaultHostClient, type HostBridgeLike } from "./host";
import type { UIBridge, UISlotClient, UIActionHandler } from "./ui";
import { defineUIAction } from "./ui";
import type { ExtensionFiber } from "./fiber";
import { RuntimeFiber } from "./fiber";
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
  abort(reason?: string): void;
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
  bindTask<I extends TaskInput, O>(entryName: string, definition: TaskDefinition<I, O>): TaskDefinition<I, O>;
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
  readonly contributions: ContributionClient;
  readonly services: ServiceClient;
  readonly adapters: AdapterClient;
  readonly logger: ExtensionLogger;
  readonly lifecycle: RuntimeLifecycleClient;
  readonly fiber: ExtensionFiber;
  readonly ui?: UIBridge;
  readonly slots?: UISlotClient;
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
  const fiber = new RuntimeFiber(namespace);
  const callHost = async (method: string, input: unknown): Promise<unknown> => {
    if (!bootstrap.hostBridge.call) throw new Error("host API transport is unavailable");
    return bootstrap.hostBridge.call(method, input);
  };
  const contributions: ContributionClient = {
    register: async (request): Promise<ContributionRegistration> => {
      const result = await callHost("plugin.contribution.register", request) as { id?: string };
      const id = String(result?.id ?? request.id);
      const registration = {
        id,
        async dispose() {
          await callHost("plugin.contribution.unregister", { id: request.id });
        },
      };
      fiber.own(() => registration.dispose());
      return registration;
    },
    unregister: async (id) => {
      await callHost("plugin.contribution.unregister", { id });
    },
  };
  const services: ServiceClient = {
    provide: async (offer) => {
      await callHost("plugin.service.provide", offer);
      const withdraw = async () => {
        await callHost("plugin.service.unprovide", { serviceId: offer.serviceId });
      };
      fiber.own(withdraw);
      return withdraw;
    },
    require: async (requirement): Promise<ServiceLease> => {
      const result = await callHost("plugin.service.require", requirement) as { providerIds?: unknown[] };
      const lease: ServiceLease = {
        serviceId: requirement.serviceId,
        providerIds: Array.isArray(result?.providerIds) ? result.providerIds.map(String) : [],
        async release() {
          await callHost("plugin.service.release", { serviceId: requirement.serviceId });
        },
      };
      fiber.own(() => lease.release());
      return lease;
    },
  };
  const adapters: AdapterClient = {
    invoke: async (adapterId, contractVersion, request) => {
      return await callHost("plugin.adapter.invoke", {
        adapterId,
        contractVersion,
        ...request,
      }) as { invocationId: string; output?: unknown };
    },
    cancel: async (adapterId, contractVersion, invocationId, reason) => {
      await callHost("plugin.adapter.cancel", {
        adapterId,
        contractVersion,
        invocationId,
        reason,
      });
    },
  };

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
      const registration = defineTool(def, handler);
      fiber.own(() => { unregisterTool(def.toolId); });
      return registration;
    },
    bindEvent: (entryName, spec) => {
      assertEntry(entryName);
      const registration = defineEvent(spec);
      fiber.own(() => { unregisterEventSpec(spec.eventType); });
      return registration;
    },
    bindBeforeHook: (entryName, pipeline, stage, handler) => {
      assertEntry(entryName);
      const registration = defineBeforeHook(pipeline, stage, handler as never);
      fiber.own(() => { unregisterHook(registration); });
      return registration;
    },
    bindFilterHook: (entryName, pipeline, stage, handler) => {
      assertEntry(entryName);
      const registration = defineFilterHook(pipeline, stage, handler as never);
      fiber.own(() => { unregisterHook(registration); });
      return registration;
    },
    bindTransformHook: <I, O>(entryName: string, pipeline: string, stage: string, handler: (input: I, ctx: unknown) => Promise<O>): HookRegistration<I, O> => {
      assertEntry(entryName);
      const registration = defineTransformHook<I, O>(pipeline, stage, handler as never);
      fiber.own(() => { unregisterHook(registration); });
      return registration;
    },
    bindObserveHook: <I>(entryName: string, pipeline: string, stage: string, handler: (input: I, ctx: unknown) => Promise<void>): HookRegistration<I, void> => {
      assertEntry(entryName);
      const registration = defineObserveHook<I>(pipeline, stage, handler as never);
      fiber.own(() => { unregisterHook(registration); });
      return registration;
    },
    bindTask: (entryName, taskDef) => {
      assertEntry(entryName);
      const registration = defineTask(taskDef);
      fiber.own(() => { unregisterTask(taskDef.taskId); });
      return registration;
    },
    bindUIAction: (entryName, actionId, handler) => {
      assertEntry(entryName);
      const registration = defineUIAction(actionId, handler as UIActionHandler);
      fiber.own(() => registration.dispose());
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
    contributions,
    services,
    adapters,
    logger,
    lifecycle,
    fiber,
    ui: bootstrap.uiBridge,
    slots: bootstrap.uiBridge?.slots,
  };

  const cleanup = async () => {
    try {
      if (definition.deactivate) {
        await definition.deactivate(aborted ? "aborted" : "shutdown");
      }
    } catch (cause) {
      logger.error("extension deactivate failed", { error: cause instanceof Error ? cause.message : String(cause) });
    } finally {
      await fiber.dispose();
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
