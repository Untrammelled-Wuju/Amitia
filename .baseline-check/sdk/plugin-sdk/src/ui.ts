import { AmitiaError, ValidationError, PermissionDeniedError } from "./errors";
import type { UIProviderDefinition, UIProviderEntry } from "./manifest";
import type {
  UIChildrenDecl,
  UIKnownSlotId,
  UISlotEntryKey,
  UISlotInjectFactory,
  UISlotKind,
  UISlotRegisterOptions,
  UISlotRegistrationComponent,
  UISlotComponent,
  UISlotScope,
  UISlotStore,
  UISlotManagedStoreHandle,
  UISlotManagedStoreInstance,
  UISlotStoreFactory,
  UISlotStoreResource,
} from "./slot-contract";

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
  readonly supportedKinds: readonly string[];
  /** DSH-compatible dispatch semantics. When omitted it is derived from multiplicity. */
  readonly kind?: UISlotKind;
  readonly multiplicity?: "single" | "multiple" | "ordered_multiple" | "replaceable_single" | "exclusive";
  readonly layout?: "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal" | "hidden";
  readonly fallbackPolicy?: "none" | "skeleton" | "empty" | "default";
  readonly parentSlotId?: string;
  readonly description?: string;
  readonly declarationEpoch?: number;
  readonly scope?: UISlotScope;
  /** In-memory/runtime inspection seat for the parent-declared common inject face. */
  readonly commonInject?: Readonly<Record<string, unknown>>;
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

/** Explicit compatibility API for old declarative callers. It intentionally is
 * not an overload of register(), so it cannot weaken SlotMap type checking. */
export interface UISlotContributionOptions<
  TProps extends Record<string, unknown> = Record<string, unknown>,
  TStore = unknown,
  TInjected extends Record<string, unknown> = Record<string, unknown>,
> {
  readonly ordering?: number;
  readonly priority?: number;
  readonly props?: TProps;
  readonly children?: readonly UISlotDefinition[];
  readonly store?: TStore | UISlotStoreFactory<TStore>;
  readonly inject?: UISlotInjectFactory<TStore, TInjected>;
  readonly entryKey?: string;
  /** DSH list-cell identity (distinct from stable registration key). */
  readonly cellId?: string;
  readonly label?: string | (() => string);
  readonly select?: (owner: Record<string, unknown>) => unknown | null;
}

/** Legacy manifest entry retained as a named compatibility shape. */
export interface UISlotEntryDefinition<TRenderable = unknown> {
  readonly slotId: string;
  readonly key: string;
  readonly renderable: TRenderable;
  readonly ordering?: number;
  readonly priority?: number;
  readonly props?: Record<string, unknown>;
  readonly children?: readonly UISlotDefinition[];
  readonly store?: unknown | UISlotStoreFactory<unknown>;
  readonly inject?: UISlotInjectFactory<unknown, Record<string, unknown>>;
  readonly entryKey?: string;
  readonly select?: (owner: Record<string, unknown>) => unknown | null;
}

export interface UISlotContributionRegistration<
  T = unknown,
  TStore = unknown,
  TInjected extends Record<string, unknown> = Record<string, unknown>,
> {
  readonly slotId: string;
  readonly key: string;
  readonly renderable: T;
  /** Root-scoped compatibility aliases. Session-scoped state is instantiated per session. */
  readonly store?: TStore;
  readonly injected?: Readonly<TInjected>;
  dispose(): void | Promise<void>;
}

export interface UISlotClient {
  /** Advanced compatibility primitive. DSH-style composition should declare children in register(). */
  declare(definition: UISlotDefinition): Promise<UISlotRegistration>;
  /** Strong single composition API. No string catch-all overload is provided. */
  register<
    K extends UIKnownSlotId,
    const D extends UIChildrenDecl = {},
    S = UISlotStore<K>,
    I extends Record<string, unknown> = {},
    N extends keyof import("./slot-contract").UILocaleNamespaceMap & string | undefined = undefined,
    const EntryKey extends UISlotEntryKey<K> = UISlotEntryKey<K>,
    C extends UISlotComponent<never> = UISlotComponent<never>,
  >(
    options: UISlotRegisterOptions<K, D, S, I, N, EntryKey>,
    renderable: UISlotRegistrationComponent<K, D, S, I, N, EntryKey, C>,
  ): UISlotContributionRegistration<C, S, I>;
  /** Explicit compatibility escape hatch for old manifests/declarative packages. */
  registerLegacy<T = unknown>(
    slotId: string,
    key: string,
    renderable: T,
    options?: UISlotContributionOptions,
  ): UISlotContributionRegistration<T>;
  list(): Promise<UISlotDefinition[]>;
  /** Slot-declaration lifetime injection; retained as a first-class API. */
  inject(slotId: string, callback: (definition: UISlotDefinition) => UISlotInjectionEffect): Promise<UISlotInjectionCleanup>;
  /** Alias with clearer semantics for callers that prefer it. */
  observe(slotId: string, callback: (definition: UISlotDefinition) => UISlotInjectionEffect): Promise<UISlotInjectionCleanup>;
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


function multiplicityFromSlotKind(kind: import("./slot-contract").UISlotKind): NonNullable<UISlotDefinition["multiplicity"]> {
  if (kind === "single") return "replaceable_single";
  return "ordered_multiple";
}

function slotKindFromMultiplicity(multiplicity: UISlotDefinition["multiplicity"]): import("./slot-contract").UISlotKind {
  if (multiplicity === "single" || multiplicity === "replaceable_single" || multiplicity === "exclusive") return "single";
  return "list";
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
    kind: definition.kind ?? slotKindFromMultiplicity(definition.multiplicity),
    multiplicity: definition.multiplicity ?? multiplicityFromSlotKind(definition.kind ?? "list"),
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

  constructor() {
    // Root is the sole foundational composition seat. Amitia may expose
    // additional built-in host surfaces, but strict plugin trees can always
    // bootstrap from this DSH-compatible root without out-of-band setup.
    const root = defineUISlot({
      slotId: "root",
      contractVersion: 1,
      supportedKinds: ["panel", "schema_page", "web_page"],
      kind: "single",
      multiplicity: "replaceable_single",
      layout: "stack",
      fallbackPolicy: "default",
      scope: "root",
      declarationEpoch: 1,
    });
    this.slotValues.set("root", root);
    this.slotEpochs.set("root", 1);
  }

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
        await this.collapseSlotEntries(definition.slotId);
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
          await this.collapseSlotEpoch(definition.slotId, epoch);
        },
      };
    },
    register: ((options: UISlotRegisterOptions<UIKnownSlotId, UIChildrenDecl, unknown, Record<string, unknown>>, renderable: unknown) => {
      const children: UISlotDefinition[] = Object.entries(options.children ?? {}).map(([slotId, raw]) => {
        const spec = raw as NonNullable<UIChildrenDecl[UIKnownSlotId]>;
        return {
          slotId,
          contractVersion: spec.contractVersion ?? 1,
          supportedKinds: [...(spec.supportedKinds ?? ["panel"])],
          kind: spec.kind,
          scope: spec.scope,
          multiplicity: spec.multiplicity ?? multiplicityFromSlotKind(spec.kind),
          layout: spec.layout,
          fallbackPolicy: spec.fallbackPolicy,
          description: spec.description,
          commonInject: spec.inject as Readonly<Record<string, unknown>> | undefined,
        };
      });
      return this.slots.registerLegacy(options.name, options.key, renderable, {
        ordering: (options as { order?: number }).order ?? options.ordering,
        priority: options.priority,
        props: options.ownerDefaults as Record<string, unknown> | undefined,
        children,
        store: options.store as unknown,
        inject: options.inject as UISlotInjectFactory<unknown, Record<string, unknown>> | undefined,
        entryKey: (options as { entryKey?: string }).entryKey,
        cellId: (options as { id?: string }).id,
        label: (options as { label?: string | (() => string) }).label,
        select: (options as { select?: (owner: Record<string, unknown>) => unknown | null }).select,
      });
    }) as UISlotClient["register"],
    registerLegacy: ((
      slotId: string,
      key: string,
      renderable: unknown,
      options: UISlotContributionOptions = {},
    ): UISlotContributionRegistration<unknown> => {
      const id = slotId.trim();
      const contributionKey = key.trim();
      const contributionRenderable = renderable;
      const resolvedOptions = options;
      if (!id || !contributionKey) throw new ValidationError("ui slot registration requires slotId and key");
      if (!this.slotValues.has(id)) throw new ValidationError(`ui slot ${id} is not declared`);

      const parentDefinition = this.slotValues.get(id)!;
      const childDefinitions = (resolvedOptions.children ?? []).map((child) => {
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

      const resourceDisposers: UISlotInjectionCleanup[] = [];
      const resourceContext = {
        pluginId: "in-memory",
        slotId: id,
        key: contributionKey,
        scope: parentDefinition.scope ?? "root",
      } as const;
      let store: unknown;
      let actions: Readonly<Record<string, (...args: any[]) => any>> = {};
      const attachManagedStore = (handle: UISlotManagedStoreHandle<unknown, any>) => {
        const managed = handle.create(resourceContext.scope) as UISlotManagedStoreInstance<unknown, any>;
        store = managed.getSnapshot();
        actions = Object.freeze({ ...(managed.actions ?? {}) });
        resourceDisposers.push(() => managed.dispose());
      };
      if (typeof resolvedOptions.store === "function") {
        const created = (resolvedOptions.store as UISlotStoreFactory<unknown>)(resourceContext);
        if (isUISlotStoreResource(created)) {
          store = created.store;
          actions = Object.freeze({ ...(created.actions ?? {}) });
          if (created.dispose) resourceDisposers.push(() => created.dispose?.());
        } else if (isUISlotManagedStoreHandle(created)) {
          attachManagedStore(created);
        } else {
          store = created;
        }
      } else if (isUISlotManagedStoreHandle(resolvedOptions.store)) {
        attachManagedStore(resolvedOptions.store);
      } else {
        store = resolvedOptions.store;
      }
      let injected: Record<string, unknown> | undefined;
      try {
        if (resolvedOptions.inject) {
          injected = (resolvedOptions.inject as UISlotInjectFactory<unknown, Record<string, unknown>>)({
            ...resourceContext,
            store,
            actions,
            services: { get: () => undefined, list: () => [] },
            events: { emit: async () => undefined },
          });
        }
      } catch (error) {
        for (let index = resourceDisposers.length - 1; index >= 0; index--) void resourceDisposers[index]?.();
        throw error;
      }

      const childEpochs: Array<{ slotId: string; epoch: number }> = [];
      for (const child of childDefinitions) {
        const epoch = Math.max(this.slotEpochs.get(child.slotId) ?? 0, child.declarationEpoch ?? 0) + 1;
        this.slotEpochs.set(child.slotId, epoch);
        const activeDefinition = { ...child, declarationEpoch: epoch };
        this.slotValues.set(child.slotId, activeDefinition);
        childEpochs.push({ slotId: child.slotId, epoch });
        void this.activateSlotEpoch(child.slotId, activeDefinition).catch(async () => {
          await this.collapseSlotEpoch(child.slotId, epoch);
        });
      }

      let active = true;
      const registration: UISlotContributionRegistration<unknown> = {
        slotId: id,
        key: contributionKey,
        renderable: contributionRenderable,
        store,
        injected: injected ? Object.freeze({ ...injected }) : undefined,
        dispose: async () => {
          if (!active) return;
          active = false;
          for (let index = childEpochs.length - 1; index >= 0; index--) {
            const child = childEpochs[index]!;
            await this.collapseSlotEpoch(child.slotId, child.epoch);
          }
          const current = this.slotContributions.get(id);
          current?.delete(contributionKey);
          if (current?.size === 0) this.slotContributions.delete(id);
          for (let index = resourceDisposers.length - 1; index >= 0; index--) await resourceDisposers[index]?.();
        },
      };
      entries.set(contributionKey, registration as UISlotContributionRegistration);
      return registration;
    }) as UISlotClient["registerLegacy"],
    list: async (): Promise<UISlotDefinition[]> => Array.from(this.slotValues.values()),
    observe: async (slotId: string, callback: (definition: UISlotDefinition) => UISlotInjectionEffect): Promise<UISlotInjectionCleanup> => {
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
    inject: async (slotId: string, callback: (definition: UISlotDefinition) => UISlotInjectionEffect): Promise<UISlotInjectionCleanup> => {
      return this.slots.observe(slotId, callback);
    },
  };

  private async collapseSlotEntries(slotId: string): Promise<void> {
    const entries = Array.from(this.slotContributions.get(slotId)?.values() ?? []);
    for (let index = entries.length - 1; index >= 0; index--) {
      await entries[index]!.dispose();
    }
    this.slotContributions.delete(slotId);
  }

  private async collapseSlotEpoch(slotId: string, epoch: number): Promise<void> {
    if (this.slotEpochs.get(slotId) !== epoch || !this.slotValues.has(slotId)) return;
    await this.collapseSlotEntries(slotId);
    await this.deactivateSlotEpoch(slotId);
    if (this.slotEpochs.get(slotId) !== epoch) return;
    this.slotValues.delete(slotId);
  }

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

function isUISlotManagedStoreHandle(value: unknown): value is UISlotManagedStoreHandle<unknown, any> {
  return !!value
    && typeof value === "object"
    && (value as { __amitiaManagedSlotStoreHandle?: unknown }).__amitiaManagedSlotStoreHandle === true
    && typeof (value as { create?: unknown }).create === "function";
}

function isUISlotStoreResource(value: unknown): value is UISlotStoreResource<unknown> {
  return !!value && typeof value === "object" && (value as { __amitiaSlotStoreResource?: unknown }).__amitiaSlotStoreResource === true;
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
