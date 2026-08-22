import { AmitiaError, ValidationError, PermissionDeniedError } from "./errors";
import type { UIProviderDefinition, UIProviderEntry } from "./manifest";

export interface UIActionRequest {
  readonly actionId: string;
  readonly payload?: unknown;
}

export interface UIActionResponse {
  readonly ok: boolean;
  readonly result?: unknown;
  readonly error?: { code: string; message: string };
}

export interface UIReadyEvent {
  readonly extensionId: string;
  readonly contributionId: string;
  readonly contractVersion: number;
}

export interface UIBridge {
  ready(): Promise<UIReadyEvent>;
  actions: UIActionClient;
  state: UIStateClient;
  host: UIHostClient;
  slots?: UISlotClient;
}

export interface UISlotDefinition {
  readonly slotId: string;
  readonly contractVersion?: number;
  readonly supportedKinds: string[];
  readonly multiplicity?: "single" | "multiple" | "ordered_multiple" | "replaceable_single" | "exclusive";
  readonly layout?: "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal" | "hidden";
  readonly fallbackPolicy?: "none" | "skeleton" | "empty" | "default";
  readonly parentSlotId?: string;
  readonly description?: string;
  readonly declarationEpoch?: number;
  readonly scope?: "root" | "session-maybe" | "session";
}

export interface UISlotRegistration {
  readonly definition: UISlotDefinition;
  dispose(): void | Promise<void>;
}

export type UISlotInjectionCleanup = () => void | Promise<void>;
export type UISlotInjectionEffect =
  | void
  | UISlotInjectionCleanup
  | readonly UISlotInjectionCleanup[]
  | Iterable<UISlotInjectionCleanup>
  | Promise<void | UISlotInjectionCleanup | readonly UISlotInjectionCleanup[] | Iterable<UISlotInjectionCleanup>>;

export interface UISlotContributionOptions {
  readonly ordering?: number;
  readonly priority?: number;
  readonly props?: Record<string, unknown>;
  readonly children?: readonly UISlotDefinition[];
}

export interface UISlotContributionRegistration<T = unknown> {
  readonly slotId: string;
  readonly key: string;
  readonly renderable: T;
  dispose(): void | Promise<void>;
}

export interface UISlotClient {
  declare(definition: UISlotDefinition): Promise<UISlotRegistration>;
  register<T = unknown>(slotId: string, key: string, renderable: T, options?: UISlotContributionOptions): UISlotContributionRegistration<T>;
  list(): Promise<UISlotDefinition[]>;
  /**
   * Depend on the lifetime of a slot declaration. The callback is invoked once
   * for each declaration epoch. If the declaration disappears, its returned
   * cleanup is awaited before a later declaration can activate a new epoch.
   */
  inject(slotId: string, callback: (definition: UISlotDefinition) => UISlotInjectionEffect): Promise<UISlotInjectionCleanup>;
}

export interface UIActionClient {
  invoke(request: UIActionRequest): Promise<UIActionResponse>;
  list(): Promise<UIActionDescriptor[]>;
}

export interface UIActionDescriptor {
  readonly actionId: string;
  readonly title: string;
  readonly description?: string;
  readonly deprecated?: boolean;
}

export interface UIStateClient {
  get<T>(key: string): Promise<T | null>;
  set<T>(key: string, value: T): Promise<void>;
  subscribe(key: string, callback: (value: unknown) => void): Promise<() => void>;
}

export interface UIHostClient {
  openExternal(url: string): Promise<void>;
  copyToClipboard(text: string): Promise<void>;
  setTitle(title: string): Promise<void>;
  notify(message: string, kind?: "info" | "success" | "warning" | "error"): Promise<void>;
}

export type UIActionHandler = (
  payload: unknown,
  context: UIActionContext,
) => Promise<UIActionResponse> | UIActionResponse;

export interface UIActionContext {
  readonly actionId: string;
  readonly traceId: string;
  readonly signal?: AbortSignal;
  readonly logger: UILogger;
}

export interface UILogger {
  debug(message: string, fields?: Record<string, unknown>): void;
  info(message: string, fields?: Record<string, unknown>): void;
  warn(message: string, fields?: Record<string, unknown>): void;
  error(message: string, fields?: Record<string, unknown>): void;
}

const uiActionRegistry = new Map<string, { handler: UIActionHandler; descriptor: UIActionDescriptor }>();

export interface UIActionRegistration {
  readonly actionId: string;
  dispose(): void;
}

export function defineUIAction(
  actionId: string,
  handler: UIActionHandler,
  descriptor?: Partial<UIActionDescriptor>,
): UIActionRegistration {
  if (!actionId) {
    throw new ValidationError("actionId is required");
  }
  if (typeof handler !== "function") {
    throw new ValidationError("handler must be a function");
  }
  uiActionRegistry.set(actionId, {
    handler,
    descriptor: {
      actionId,
      title: descriptor?.title ?? actionId,
      description: descriptor?.description,
      deprecated: descriptor?.deprecated,
    },
  });
  return {
    actionId,
    dispose: () => {
      uiActionRegistry.delete(actionId);
    },
  };
}

export function unregisterUIAction(actionId: string): boolean {
  return uiActionRegistry.delete(actionId);
}

export function listUIActions(): UIActionDescriptor[] {
  return Array.from(uiActionRegistry.values()).map((r) => r.descriptor);
}

export function clearUIActions(): void {
  uiActionRegistry.clear();
}

export function createAmitiaUI(bridge: UIBridge): UIBridge {
  return bridge;
}

function slotScopeCanContain(
  parent: NonNullable<UISlotDefinition["scope"]>,
  child: NonNullable<UISlotDefinition["scope"]>,
): boolean {
  if (parent === "session") return child === "session";
  if (parent === "session-maybe") return child === "session-maybe" || child === "session";
  return true;
}

export function defineUISlot(definition: UISlotDefinition): UISlotDefinition {
  if (!definition.slotId?.trim()) throw new ValidationError("ui slot slotId is required");
  if (!Array.isArray(definition.supportedKinds) || definition.supportedKinds.length === 0) {
    throw new ValidationError(`ui slot ${definition.slotId} requires supportedKinds`);
  }
  if (definition.parentSlotId === definition.slotId) {
    throw new ValidationError(`ui slot ${definition.slotId} cannot parent itself`);
  }
  if (definition.scope && definition.scope !== "root" && definition.scope !== "session-maybe" && definition.scope !== "session") {
    throw new ValidationError(`ui slot ${definition.slotId} has invalid scope ${definition.scope}`);
  }
  return {
    ...definition,
    slotId: definition.slotId.trim(),
    contractVersion: definition.contractVersion ?? 1,
    multiplicity: definition.multiplicity ?? "ordered_multiple",
    layout: definition.layout ?? "stack",
    fallbackPolicy: definition.fallbackPolicy ?? "empty",
    scope: definition.scope ?? "root",
  };
}

export class InMemoryUIBridge implements UIBridge {
  private readonly stateValues = new Map<string, unknown>();
  private readonly subscribers = new Map<string, Set<(value: unknown) => void>>();
  private readonly slotValues = new Map<string, UISlotDefinition>();
  private readonly slotEpochs = new Map<string, number>();
  private readonly slotContributions = new Map<string, Map<string, UISlotContributionRegistration>>();
  private readonly slotSubscribers = new Map<string, Set<{
    callback: (definition: UISlotDefinition) => UISlotInjectionEffect;
    cleanup?: UISlotInjectionCleanup;
  }>>();

  async ready(): Promise<UIReadyEvent> {
    return {
      extensionId: "in-memory",
      contributionId: "in-memory",
      contractVersion: 1,
    };
  }

  readonly actions: UIActionClient = {
    invoke: async (request: UIActionRequest): Promise<UIActionResponse> => {
      const entry = uiActionRegistry.get(request.actionId);
      if (!entry) {
        return { ok: false, error: { code: "action_not_found", message: `action ${request.actionId} not found` } };
      }
      const ctx: UIActionContext = {
        actionId: request.actionId,
        traceId: `ui-${Date.now()}`,
        logger: noopUILogger,
      };
      return entry.handler(request.payload, ctx);
    },
    list: async (): Promise<UIActionDescriptor[]> => listUIActions(),
  };

  readonly state: UIStateClient = {
    get: async <T>(key: string): Promise<T | null> => {
      return (this.stateValues.get(key) as T) ?? null;
    },
    set: async <T>(key: string, value: T): Promise<void> => {
      this.stateValues.set(key, value);
      const subs = this.subscribers.get(key);
      if (subs) {
        for (const cb of subs) cb(value);
      }
    },
    subscribe: async (key: string, callback: (value: unknown) => void): Promise<(() => void)> => {
      let subs = this.subscribers.get(key);
      if (!subs) {
        subs = new Set();
        this.subscribers.set(key, subs);
      }
      subs.add(callback);
      return () => {
        subs?.delete(callback);
      };
    },
  };

  readonly host: UIHostClient = {
    openExternal: async (_url: string): Promise<void> => {
      // no-op in memory
    },
    copyToClipboard: async (_text: string): Promise<void> => {
      // no-op in memory
    },
    setTitle: async (_title: string): Promise<void> => {},
    notify: async (_message: string, _kind?: "info" | "success" | "warning" | "error"): Promise<void> => {
      // no-op in memory
    },
  };

  readonly slots: UISlotClient = {
    declare: async (input: UISlotDefinition): Promise<UISlotRegistration> => {
      const parentId = input.parentSlotId?.trim();
      const parent = parentId ? this.slotValues.get(parentId) : undefined;
      if (parentId && !parent) throw new ValidationError(`parent ui slot ${parentId} is not declared`);
      if (parent && !slotScopeCanContain(parent.scope ?? "root", input.scope ?? parent.scope ?? "root")) {
        throw new ValidationError(`child slot ${input.slotId} cannot escape parent scope ${parent.scope}`);
      }
      const definition = defineUISlot({
        ...input,
        parentSlotId: parentId || undefined,
        scope: input.scope ?? parent?.scope,
      });
      const previousEpoch = this.slotEpochs.get(definition.slotId) ?? 0;
      if (this.slotValues.has(definition.slotId)) {
        await this.deactivateSlotEpoch(definition.slotId);
      }
      const epoch = previousEpoch + 1;
      this.slotEpochs.set(definition.slotId, epoch);
      const activeDefinition = { ...definition, declarationEpoch: epoch };
      this.slotValues.set(definition.slotId, activeDefinition);
      await this.activateSlotEpoch(definition.slotId, activeDefinition);
      return {
        definition: activeDefinition,
        dispose: async () => {
          if (this.slotEpochs.get(definition.slotId) !== epoch) return;
          await this.deactivateSlotEpoch(definition.slotId);
          this.slotValues.delete(definition.slotId);
        },
      };
    },
    register: <T = unknown>(slotId: string, key: string, renderable: T, options: UISlotContributionOptions = {}): UISlotContributionRegistration<T> => {
      const id = slotId.trim();
      const contributionKey = key.trim();
      if (!id || !contributionKey) throw new ValidationError("ui slot registration requires slotId and key");
      if (!this.slotValues.has(id)) throw new ValidationError(`ui slot ${id} is not declared`);

      const parentDefinition = this.slotValues.get(id)!;
      const childDefinitions = (options.children ?? []).map((child) => {
        if (!slotScopeCanContain(parentDefinition.scope ?? "root", child.scope ?? parentDefinition.scope ?? "root")) {
          throw new ValidationError(`child slot ${child.slotId} cannot escape parent scope ${parentDefinition.scope}`);
        }
        return defineUISlot({
          ...child,
          parentSlotId: id,
          scope: child.scope ?? parentDefinition.scope,
        });
      });
      const childIds = new Set<string>();
      for (const child of childDefinitions) {
        if (child.slotId === id) throw new ValidationError(`child slot ${child.slotId} cannot equal its parent`);
        if (childIds.has(child.slotId)) throw new ValidationError(`duplicate child slot ${child.slotId}`);
        if (this.slotValues.has(child.slotId)) throw new ValidationError(`child slot ${child.slotId} is already declared`);
        childIds.add(child.slotId);
      }

      let entries = this.slotContributions.get(id);
      if (!entries) { entries = new Map(); this.slotContributions.set(id, entries); }
      if (entries.has(contributionKey)) throw new ValidationError(`ui slot contribution ${id}/${contributionKey} already registered`);

      const childEpochs: Array<{ slotId: string; epoch: number }> = [];
      for (const child of childDefinitions) {
        const epoch = Math.max(this.slotEpochs.get(child.slotId) ?? 0, child.declarationEpoch ?? 0) + 1;
        this.slotEpochs.set(child.slotId, epoch);
        const activeDefinition = { ...child, declarationEpoch: epoch };
        this.slotValues.set(child.slotId, activeDefinition);
        childEpochs.push({ slotId: child.slotId, epoch });
        void this.activateSlotEpoch(child.slotId, activeDefinition).catch(async () => {
          if (this.slotEpochs.get(child.slotId) !== epoch) return;
          await this.deactivateSlotEpoch(child.slotId);
          this.slotValues.delete(child.slotId);
        });
      }

      let active = true;
      const registration: UISlotContributionRegistration<T> = {
        slotId: id,
        key: contributionKey,
        renderable,
        dispose: async () => {
          if (!active) return;
          active = false;
          for (let index = childEpochs.length - 1; index >= 0; index--) {
            const child = childEpochs[index]!;
            if (this.slotEpochs.get(child.slotId) !== child.epoch || !this.slotValues.has(child.slotId)) continue;
            await this.deactivateSlotEpoch(child.slotId);
            this.slotValues.delete(child.slotId);
          }
          const current = this.slotContributions.get(id);
          current?.delete(contributionKey);
          if (current?.size === 0) this.slotContributions.delete(id);
        },
      };
      entries.set(contributionKey, registration as UISlotContributionRegistration);
      return registration;
    },
    list: async (): Promise<UISlotDefinition[]> => Array.from(this.slotValues.values()),
    inject: async (slotId: string, callback: (definition: UISlotDefinition) => UISlotInjectionEffect): Promise<UISlotInjectionCleanup> => {
      const id = slotId.trim();
      if (!id) throw new ValidationError("ui slot injection requires slotId");
      let subscribers = this.slotSubscribers.get(id);
      if (!subscribers) {
        subscribers = new Set();
        this.slotSubscribers.set(id, subscribers);
      }
      const subscriber = { callback, cleanup: undefined as UISlotInjectionCleanup | undefined };
      subscribers.add(subscriber);
      const current = this.slotValues.get(id);
      if (current) subscriber.cleanup = await normalizeUISlotInjectionEffect(callback(current));
      return async () => {
        subscribers?.delete(subscriber);
        if (subscriber.cleanup) await subscriber.cleanup();
        subscriber.cleanup = undefined;
        if (subscribers?.size === 0) this.slotSubscribers.delete(id);
      };
    },
  };

  private async deactivateSlotEpoch(slotId: string): Promise<void> {
    for (const subscriber of this.slotSubscribers.get(slotId) ?? []) {
      if (subscriber.cleanup) await subscriber.cleanup();
      subscriber.cleanup = undefined;
    }
  }

  private async activateSlotEpoch(slotId: string, definition: UISlotDefinition): Promise<void> {
    for (const subscriber of this.slotSubscribers.get(slotId) ?? []) {
      if (subscriber.cleanup) await subscriber.cleanup();
      subscriber.cleanup = await normalizeUISlotInjectionEffect(subscriber.callback(definition));
    }
  }
}

async function normalizeUISlotInjectionEffect(effect: UISlotInjectionEffect): Promise<UISlotInjectionCleanup | undefined> {
  const resolved = await effect;
  if (!resolved) return undefined;
  if (typeof resolved === "function") return resolved;
  const cleanups: UISlotInjectionCleanup[] = [];
  try {
    for (const cleanup of resolved) {
      if (typeof cleanup !== "function") throw new ValidationError("slot injection iterable must yield cleanup functions");
      cleanups.push(cleanup);
    }
  } catch (error) {
    for (let index = cleanups.length - 1; index >= 0; index--) {
      try { await cleanups[index]?.(); } catch { /* retain setup failure */ }
    }
    throw error;
  }
  return async () => {
    for (let index = cleanups.length - 1; index >= 0; index--) await cleanups[index]?.();
  };
}

const noopUILogger: UILogger = {
  debug: () => {},
  info: () => {},
  warn: () => {},
  error: () => {},
};

export function mapUIError(cause: unknown): AmitiaError {
  if (cause instanceof AmitiaError) return cause;
  const message = cause instanceof Error ? cause.message : String(cause);
  if (/permission|denied|forbidden/i.test(message)) {
    return new PermissionDeniedError(message);
  }
  return new ValidationError(message);
}

export function assertUIActionAllowed(actionId: string, allowed: string[]): void {
  if (!allowed.includes(actionId)) {
    throw new PermissionDeniedError(`UI action ${actionId} not allowed`);
  }
}


/**
 * Define a replaceable UI provider contribution with SDK-side fail-fast validation.
 * The manifest contribution id remains the authority for extension/module identity;
 * the host normalizes and verifies those fields again during installation.
 */
export function defineUIProvider(definition: UIProviderDefinition): UIProviderDefinition {
  if (!definition.providerId?.trim()) {
    throw new ValidationError("ui provider providerId is required");
  }
  if (!definition.capability) {
    throw new ValidationError("ui provider capability is required");
  }
  const entries = definition.entries ?? {};
  if (Object.keys(entries).length === 0) {
    throw new ValidationError("ui provider requires at least one platform entry");
  }
  for (const [platform, entry] of Object.entries(entries)) {
    assertUIProviderEntry(platform, entry, definition.capability);
  }
  assertUIProviderMetadata(definition);
  return {
    ...definition,
    providerId: definition.providerId.trim(),
    mode: definition.mode ?? "replace",
    entries: { ...entries },
  };
}

function assertUIProviderMetadata(definition: UIProviderDefinition): void {
  const metadata = definition.metadata;
  if (!metadata) return;
  if (definition.capability === "route.registry" && metadata.routes !== undefined) {
    if (!Array.isArray(metadata.routes)) throw new ValidationError("route.registry metadata.routes must be an array");
    for (const route of metadata.routes) {
      if (!route || typeof route !== "object" || !("id" in route) || !("path" in route) || !("providerId" in route)) {
        throw new ValidationError("route.registry routes require object entries with id, path and providerId");
      }
    }
  }
  if (["app.navigation", "route.registry"].includes(definition.capability) && metadata.navigationItems !== undefined) {
    if (!Array.isArray(metadata.navigationItems)) throw new ValidationError("app.navigation metadata.navigationItems must be an array");
    for (const item of metadata.navigationItems) {
      if (!item?.id || !item.label || !item.route?.startsWith("/")) {
        throw new ValidationError("navigation items require id, label and an absolute route");
      }
    }
  }
  if (definition.capability === "conversation.message_renderer") {
    for (const key of ["messageTypes", "roles", "mimeTypes", "extensionTypes"] as const) {
      const value = metadata[key];
      if (value !== undefined && !Array.isArray(value)) {
        throw new ValidationError(`conversation.message_renderer metadata.${key} must be an array`);
      }
    }
  }
  if (definition.capability === "ui.components" && metadata.componentVariants !== undefined) {
    if (!metadata.componentVariants || typeof metadata.componentVariants !== "object" || Array.isArray(metadata.componentVariants)) {
      throw new ValidationError("ui.components metadata.componentVariants must be an object");
    }
  }
  if (definition.capability === "ui.icons") {
    for (const key of ["iconAliases", "iconExports", "iconGlyphs"] as const) {
      const value = metadata[key];
      if (value !== undefined && (!value || typeof value !== "object" || Array.isArray(value))) {
        throw new ValidationError(`ui.icons metadata.${key} must be an object`);
      }
    }
  }
}

function assertUIProviderEntry(
  platform: string,
  entry: UIProviderEntry,
  capability: UIProviderDefinition["capability"],
): void {
  if (!platform.trim()) throw new ValidationError("ui provider platform key is required");
  switch (entry.type) {
    case "builtin_native":
      throw new ValidationError("builtin_native is reserved for host built-in providers");
    case "declarative": {
      const allowed = new Set<UIProviderDefinition["capability"]>([
        "app.navigation",
        "route.registry",
        "ui.theme",
        "ui.tokens",
        "ui.icons",
        "ui.components",
      ]);
      if (!allowed.has(capability)) {
        throw new ValidationError(`declarative entry is not supported for ${capability}`);
      }
      return;
    }
    case "web_module":
      if (["android", "ios", "mobile"].includes(platform.trim().toLowerCase())) {
        throw new ValidationError(`web_module cannot be used for Flutter AOT platform ${platform}; use schema_renderer or sandbox web`);
      }
      if (!entry.path?.trim()) throw new ValidationError(`web_module path required for ${platform}`);
      return;
    case "schema_renderer":
      if (!entry.contributionId?.trim()) {
        throw new ValidationError(`schema_renderer contributionId required for ${platform}`);
      }
      return;
    case "web_restricted":
    case "web_isolated":
      if (!entry.contributionId?.trim()) {
        throw new ValidationError(`${entry.type} contributionId required for ${platform}`);
      }
      return;
    default:
      throw new ValidationError(`unsupported UI provider entry type for ${platform}`);
  }
}
