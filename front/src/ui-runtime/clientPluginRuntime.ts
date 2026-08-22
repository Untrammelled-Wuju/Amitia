import { markRaw, shallowRef, type Component } from "vue";
import type { UIContributionSnapshot, SlotSnapshot } from "@/stores/extensionUI";
import ClientRuntimeSchemaContribution from "@/components/extension/ClientRuntimeSchemaContribution.vue";
import DynamicClientSandboxContribution from "@/components/extension/DynamicClientSandboxContribution.vue";
import {
  DYNAMIC_CLIENT_CSS_LIMIT,
  DYNAMIC_CLIENT_HTML_LIMIT,
  DYNAMIC_CLIENT_SCRIPT_LIMIT,
  type DynamicClientSandboxCode,
} from "@/ui-runtime/dynamicClientSandbox";
import {
  registerProgrammaticConversationNodeDefinition,
  listProgrammaticConversationNodeDefinitions,
  type ConversationProjectionSpec,
  type ProgrammaticConversationNodeDefinition,
} from "@/ui-runtime/conversationProjection";
import type {
  ClientChildrenDecl,
  ClientKnownSlotId,
  ClientLocaleNamespaceMap,
  ClientSlotComponent,
  ClientSlotEntryKey,
  ClientSlotInjectFactory,
  ClientSlotKind,
  ClientSlotProps,
  ClientSlotRegisterOptions,
  ClientSlotRegistrationComponent,
  ClientSlotRuntimeSpec,
  ClientSlotStore,
  ClientSlotManagedStoreHandle,
  ClientSlotManagedStoreInstance,
  ClientSlotStoreFactory,
  ClientSlotStoreHandle,
  ClientSlotStoreResource,
} from "@/ui-runtime/slotContract";

export type ClientDisposer = () => void | Promise<void>;
export type ClientSlotEffect = void | ClientDisposer | readonly ClientDisposer[] | Iterable<ClientDisposer> | Promise<void | ClientDisposer | readonly ClientDisposer[] | Iterable<ClientDisposer>>;

export interface ClientSlotDefinition {
  slotId: string;
  contractVersion: number;
  supportedKinds: string[];
  /** single/list/keyed/chain dispatch semantics. */
  kind?: ClientSlotKind;
  multiplicity?: SlotSnapshot["multiplicity"];
  layout?: SlotSnapshot["layout"];
  fallbackPolicy?: SlotSnapshot["fallbackPolicy"];
  parentSlotId?: string;
  description?: string;
  scope?: "root" | "session-maybe" | "session";
  declarationEpoch?: number;
  ownerId?: string;
  /** Exact registering entry that owns render authority for dynamic children. */
  ownerEntryId?: string;
  /** Common slot-level inject face supplied by its declaring parent entry. */
  commonInject?: Record<string, unknown>;
}

export interface ClientSlotRegistration {
  definition: ClientSlotDefinition;
  dispose(): void | Promise<void>;
}

/** Legacy/declarative contribution options. Trusted plugins use the strict
 * register({ name, children, store, inject }, component) API instead. */
export interface ClientSlotContributionOptions<
  TProps extends Record<string, unknown> = Record<string, unknown>,
  TStore = unknown,
  TInjected extends Record<string, unknown> = Record<string, unknown>,
> {
  ordering?: number;
  priority?: number;
  props?: TProps;
  children?: readonly ClientSlotDefinition[];
  store?: TStore | ClientSlotStoreFactory<TStore>;
  inject?: ClientSlotInjectFactory<TStore, TInjected>;
  entryKey?: string;
  cellId?: string;
  label?: string | (() => string);
  select?: (owner: Record<string, unknown>) => unknown | null;
  localeNamespace?: string;
}

interface ClientSlotRegistrationEnvironment {
  services: {
    get<T>(serviceId: string): T | undefined;
    list(): string[];
  };
  events: {
    emit<T>(eventType: string, payload: T): Promise<void>;
  };
}

export interface ClientContributionInstance {
  readonly scopeKey: string;
  readonly sessionId?: string;
  /** Compatibility snapshot; strict components consume getSnapshot/useStore. */
  readonly store: unknown;
  readonly actions: Readonly<Record<string, (...args: any[]) => any>>;
  readonly getSnapshot: () => unknown;
  readonly subscribe?: (listener: () => void) => () => void;
  readonly clearPersisted?: () => void;
  readonly injected: Readonly<Record<string, unknown>>;
  dispose(): Promise<void>;
}

export interface ClientSlotContribution {
  contributionId: string;
  pluginId: string;
  key: string;
  slotId: string;
  component: Component;
  ordering: number;
  priority: number;
  props?: Record<string, unknown>;
  /** Dispatch key for `keyed` slots. */
  entryKey?: string;
  /** Stable cell id + display metadata for `list` slots. */
  cellId?: string;
  label?: string | (() => string);
  /** Registration sequence preserves assembly order for chain ties. */
  sequence: number;
  /** Selector for `chain` slots. */
  select?: (owner: Record<string, unknown>) => unknown | null;
  /** Optional locale namespace bound to the strict component as `t`. */
  localeNamespace?: string;
  /** Child keys this exact entry is authorized to render. */
  childSlotIds: ReadonlySet<string>;
  scope: "root" | "session-maybe" | "session";
  strict: boolean;
  active: boolean;
  /** A crashed strict shadowing entry remains registered but yields dispatch. */
  abdicated: boolean;
  storeDecl?: unknown | ClientSlotStoreFactory<unknown>;
  /** Direct managed handle mount, used to release the shared-scope ledger. */
  sharedManagedStoreHandle?: object;
  injectDecl?: ClientSlotInjectFactory<unknown, Record<string, unknown>>;
  environment: ClientSlotRegistrationEnvironment;
  instances: Map<string, ClientContributionInstance>;
  /** Dynamically declared descendants owned by this exact entry. */
  ownedChildEpochs: Array<{ slotId: string; epoch: number }>;
  /** Predeclared Amitia host surfaces claimed by this entry as children. */
  claimedStaticChildren: string[];
  /** Declaration observers may initialize asynchronously; disposal waits them out. */
  childActivations: Promise<unknown>[];
  /** Root-scoped compatibility aliases. */
  store?: unknown;
  actions?: Readonly<Record<string, (...args: any[]) => any>>;
  injected?: Readonly<Record<string, unknown>>;
}

export interface ClientSlotDispatchResult {
  contribution: ClientSlotContribution;
  matched?: unknown;
}

interface SlotSubscriber {
  callback: (definition: ClientSlotDefinition) => ClientSlotEffect;
  cleanup?: ClientDisposer;
}

const EMPTY_ENVIRONMENT: ClientSlotRegistrationEnvironment = {
  services: { get: () => undefined, list: () => [] },
  events: { emit: async () => undefined },
};

export type BrowserLocaleDictionaries = Readonly<Record<string, Readonly<Record<string, string>>>>;

export class BrowserLocaleRuntime {
  private readonly namespaces = new Map<string, { ownerId: string; dictionaries: BrowserLocaleDictionaries }>();
  private currentLocale = typeof navigator !== "undefined" && navigator.language ? navigator.language : "en";
  readonly revision = shallowRef(0);

  register(ownerId: string, namespace: string, dictionaries: BrowserLocaleDictionaries): ClientDisposer {
    const id = namespace.trim();
    if (!id) throw new Error("locale namespace is required");
    const current = this.namespaces.get(id);
    if (current && current.ownerId !== ownerId) throw new Error(`locale namespace ${id} is already owned by ${current.ownerId}`);
    const normalized: Record<string, Readonly<Record<string, string>>> = {};
    for (const [locale, entries] of Object.entries(dictionaries ?? {})) {
      const key = locale.trim();
      if (!key || !entries || typeof entries !== "object") continue;
      normalized[key] = Object.freeze({ ...entries });
    }
    if (Object.keys(normalized).length === 0) throw new Error(`locale namespace ${id} requires at least one dictionary`);
    const record = { ownerId, dictionaries: Object.freeze(normalized) };
    this.namespaces.set(id, record);
    this.revision.value += 1;
    let active = true;
    return () => {
      if (!active) return;
      active = false;
      if (this.namespaces.get(id) === record) {
        this.namespaces.delete(id);
        this.revision.value += 1;
      }
    };
  }

  setLocale(locale: string): void {
    const next = locale.trim();
    if (!next || next === this.currentLocale) return;
    this.currentLocale = next;
    this.revision.value += 1;
  }

  getLocale(): string {
    return this.currentLocale;
  }

  has(namespace: string): boolean {
    return this.namespaces.has(namespace.trim());
  }

  bind(namespace: string): (key: string, params?: Record<string, unknown>) => string {
    const id = namespace.trim();
    const record = this.namespaces.get(id);
    if (!record) throw new Error(`locale namespace ${id} is not registered`);
    return (key, params) => {
      const locale = this.currentLocale;
      const base = locale.split("-")[0] ?? locale;
      const dictionaries = record.dictionaries;
      const selected = dictionaries[locale]
        ?? dictionaries[base]
        ?? dictionaries.en
        ?? dictionaries[Object.keys(dictionaries)[0] ?? ""];
      const common = id === "common" ? undefined : this.namespaces.get("common")?.dictionaries;
      const commonSelected = common
        ? (common[locale] ?? common[base] ?? common.en ?? common[Object.keys(common)[0] ?? ""])
        : undefined;
      let value = selected?.[key] ?? commonSelected?.[key] ?? key;
      for (const [param, raw] of Object.entries(params ?? {})) {
        value = value.replaceAll(`{${param}}`, String(raw));
      }
      return value;
    };
  }
}

export class BrowserSlotRuntime {
  private readonly local = new Map<string, ClientSlotDefinition>();
  private readonly server = new Map<string, ClientSlotDefinition>();
  private readonly epochs = new Map<string, number>();
  private readonly subscribers = new Map<string, Set<SlotSubscriber>>();
  private readonly contributions = new Map<string, Map<string, ClientSlotContribution>>();
  private readonly contributionsById = new Map<string, ClientSlotContribution>();
  private readonly childClaims = new Map<string, {
    ownerEntryId: string;
    ownerId: string;
    parentSlotId: string;
    commonInject?: Record<string, unknown>;
  }>();
  /** DSH invariant: one shared managed handle may mount under one scope kind. */
  private readonly managedStoreScopes = new WeakMap<object, { scope: ClientSlotDefinition["scope"]; count: number }>();
  /** Compatibility concrete-state handles are stricter: one live scope key. */
  private readonly sharedStoreScopes = new WeakMap<object, { scopeKey: string; count: number }>();
  private readonly entryErrorListeners = new Set<(event: { contributionId: string; slotId: string; pluginId: string; error: unknown; abdicated: boolean }) => void>();
  private contributionSequence = 0;
  readonly revision = shallowRef(0);

  async syncSnapshot(snapshot: UIContributionSnapshot | null): Promise<void> {
    const next = new Map<string, ClientSlotDefinition>();
    for (const slot of snapshot?.slots ?? []) next.set(slot.slotId, fromSnapshot(slot));
    const ids = new Set([...this.server.keys(), ...next.keys()]);
    for (const id of ids) {
      const previous = this.server.get(id);
      const incoming = next.get(id);
      if (sameDefinition(previous, incoming)) continue;
      if (previous && !this.local.has(id)) {
        await this.collapseEntriesForSlot(id);
        await this.deactivate(id);
      }
      if (incoming) this.server.set(id, incoming); else this.server.delete(id);
      this.bumpRevision();
      if (incoming && !this.local.has(id)) await this.activate(id, incoming);
    }
  }

  /** Advanced compatibility primitive. New composition should declare children
   * on registerEntry so declaration and render authority share one lifetime. */
  async declare(input: ClientSlotDefinition): Promise<ClientSlotRegistration> {
    const parentId = input.parentSlotId?.trim();
    const parent = parentId ? this.current(parentId) : undefined;
    if (parentId && !parent) throw new Error(`parent client slot ${parentId} is not declared`);
    if (parent && !clientScopeCanContain(parent.scope ?? "root", input.scope ?? parent.scope ?? "root")) {
      throw new Error(`child slot ${input.slotId} cannot escape parent scope ${parent.scope}`);
    }
    const normalized = normalizeDefinition({
      ...input,
      parentSlotId: parentId || undefined,
      scope: input.scope ?? parent?.scope,
    });
    const id = normalized.slotId;
    if (this.local.has(id)) throw new Error(`client slot ${id} is already declared locally`);
    if (this.current(id)) {
      await this.collapseEntriesForSlot(id);
      await this.deactivate(id);
    }
    const epoch = Math.max(this.epochs.get(id) ?? 0, normalized.declarationEpoch ?? 0) + 1;
    this.epochs.set(id, epoch);
    const definition = { ...normalized, declarationEpoch: epoch };
    this.local.set(id, definition);
    this.bumpRevision();
    await this.activate(id, definition);
    return {
      definition,
      dispose: async () => {
        await this.collapseLocalSlot(id, epoch, true);
      },
    };
  }

  async observe(slotId: string, callback: (definition: ClientSlotDefinition) => ClientSlotEffect): Promise<ClientDisposer> {
    const id = slotId.trim();
    if (!id) throw new Error("slotId is required");
    let subscribers = this.subscribers.get(id);
    if (!subscribers) {
      subscribers = new Set();
      this.subscribers.set(id, subscribers);
    }
    const subscriber: SlotSubscriber = { callback };
    subscribers.add(subscriber);
    const current = this.current(id);
    if (current) subscriber.cleanup = await normalizeSlotEffect(callback(current));
    return async () => {
      subscribers?.delete(subscriber);
      if (subscriber.cleanup) await subscriber.cleanup();
      subscriber.cleanup = undefined;
      if (subscribers?.size === 0) this.subscribers.delete(id);
    };
  }

  /** Slot-declaration lifetime injection. This remains first-class and is not
   * the same thing as the business `inject` field on registerEntry. */
  inject(slotId: string, callback: (definition: ClientSlotDefinition) => ClientSlotEffect): Promise<ClientDisposer> {
    return this.observe(slotId, callback);
  }

  /** DSH-style single composition API used by trusted client plugins. */
  registerEntry<
    K extends ClientKnownSlotId,
    D extends ClientChildrenDecl = {},
    S = ClientSlotStore<K>,
    I extends Record<string, unknown> = {},
  >(
    pluginId: string,
    options: ClientSlotRegisterOptions<K, D, S, I>,
    component: Component,
    environment: ClientSlotRegistrationEnvironment = EMPTY_ENVIRONMENT,
  ): ClientDisposer {
    const children = Object.entries(options.children ?? {}).map(([slotId, raw]) => {
      const spec = raw as ClientSlotRuntimeSpec<ClientKnownSlotId>;
      return normalizeDefinition({
        slotId,
        contractVersion: spec.contractVersion ?? 1,
        supportedKinds: [...(spec.supportedKinds ?? ["panel"])],
        kind: spec.kind,
        multiplicity: spec.multiplicity ?? multiplicityFromClientKind(spec.kind),
        layout: spec.layout,
        fallbackPolicy: spec.fallbackPolicy,
        scope: spec.scope,
        description: spec.description,
        commonInject: spec.inject as Record<string, unknown> | undefined,
      });
    });
    const kindOptions = options as {
      order?: number;
      entryKey?: string;
      id?: string;
      label?: string | (() => string);
      select?: (owner: Record<string, unknown>) => unknown | null;
    };
    return this.registerInternal(pluginId, options.name, options.key, component, {
      ordering: kindOptions.order ?? options.ordering,
      priority: options.priority,
      props: options.ownerDefaults as Record<string, unknown> | undefined,
      children,
      store: options.store as unknown,
      inject: options.inject as ClientSlotInjectFactory<unknown, Record<string, unknown>> | undefined,
      entryKey: kindOptions.entryKey,
      cellId: kindOptions.id,
      label: kindOptions.label,
      select: kindOptions.select,
      localeNamespace: typeof options.locale === "string" ? options.locale : undefined,
    }, environment, true);
  }

  /** Explicit compatibility path. Keeping it separate prevents it from
   * weakening the public SlotMap register() type gate. */
  registerLegacy(
    pluginId: string,
    slotId: string,
    key: string,
    component: Component,
    options: ClientSlotContributionOptions = {},
    environment: ClientSlotRegistrationEnvironment = EMPTY_ENVIRONMENT,
  ): ClientDisposer {
    return this.registerInternal(pluginId, slotId, key, component, options, environment, false);
  }

  /** Internal compatibility alias for older host code/tests. */
  register(
    pluginId: string,
    slotId: string,
    key: string,
    component: Component,
    options: ClientSlotContributionOptions = {},
    environment: ClientSlotRegistrationEnvironment = EMPTY_ENVIRONMENT,
  ): ClientDisposer {
    return this.registerLegacy(pluginId, slotId, key, component, options, environment);
  }

  private registerInternal(
    pluginId: string,
    slotId: string,
    key: string,
    component: Component,
    options: ClientSlotContributionOptions,
    environment: ClientSlotRegistrationEnvironment,
    strict: boolean,
  ): ClientDisposer {
    const normalizedPluginId = pluginId.trim();
    const normalizedSlotId = slotId.trim();
    const normalizedKey = key.trim();
    if (!normalizedPluginId) throw new Error("pluginId is required");
    if (!normalizedSlotId) throw new Error("slotId is required");
    if (!normalizedKey) throw new Error("client slot contribution key is required");
    if (!component) throw new Error("client slot contribution component is required");
    const parentDefinition = this.current(normalizedSlotId);
    if (!parentDefinition) throw new Error(`client slot ${normalizedSlotId} is not declared`);

    const contributionId = `client:${normalizedPluginId}:${normalizedSlotId}:${normalizedKey}`;
    const childInputs = (options.children ?? []).map((child) => normalizeDefinition({
      ...child,
      parentSlotId: normalizedSlotId,
      scope: child.scope ?? parentDefinition.scope,
      ownerId: normalizedPluginId,
      ownerEntryId: contributionId,
    }));
    const childIds = new Set<string>();
    const childrenToDeclare: ClientSlotDefinition[] = [];
    const staticChildrenToClaim: ClientSlotDefinition[] = [];
    for (const child of childInputs) {
      if (child.slotId === normalizedSlotId) throw new Error(`child slot ${child.slotId} cannot equal its parent`);
      if (childIds.has(child.slotId)) throw new Error(`duplicate child slot ${child.slotId}`);
      if (!clientScopeCanContain(parentDefinition.scope ?? "root", child.scope ?? parentDefinition.scope ?? "root")) {
        throw new Error(`child slot ${child.slotId} cannot escape parent scope ${parentDefinition.scope}`);
      }
      childIds.add(child.slotId);
      const existing = this.currentRaw(child.slotId);
      if (existing) {
        // Amitia keeps predeclared host surfaces as a superset, but strict DSH
        // composition still enforces exactly one declaring parent entry.
        if (existing.parentSlotId && existing.parentSlotId !== normalizedSlotId) {
          throw new Error(`child slot ${child.slotId} belongs to ${existing.parentSlotId}, not ${normalizedSlotId}`);
        }
        if (existing.ownerEntryId && existing.ownerEntryId !== contributionId) {
          throw new Error(`child slot ${child.slotId} is already owned by ${existing.ownerEntryId}`);
        }
        const existingKind = existing.kind ?? clientKindFromMultiplicity(existing.multiplicity);
        const requestedKind = child.kind ?? clientKindFromMultiplicity(child.multiplicity);
        if (strict && existingKind !== requestedKind) {
          throw new Error(`child slot ${child.slotId} kind mismatch: host=${existingKind}, requested=${requestedKind}`);
        }
        if (strict && (existing.scope ?? "root") !== (child.scope ?? "root")) {
          throw new Error(`child slot ${child.slotId} scope mismatch: host=${existing.scope ?? "root"}, requested=${child.scope ?? "root"}`);
        }
        if (strict) {
          const claim = this.childClaims.get(child.slotId);
          if (claim && claim.ownerEntryId !== contributionId) {
            throw new Error(`child slot ${child.slotId} is already declared by ${claim.ownerEntryId}`);
          }
          staticChildrenToClaim.push(child);
        }
        continue;
      }
      childrenToDeclare.push(child);
    }

    const mapKey = `${normalizedPluginId}:${normalizedKey}`;
    let slotContributions = this.contributions.get(normalizedSlotId);
    if (!slotContributions) {
      slotContributions = new Map();
      this.contributions.set(normalizedSlotId, slotContributions);
    }
    const preexisting = slotContributions.get(mapKey);
    if (preexisting?.active) {
      throw new Error(`client slot contribution ${normalizedSlotId}/${mapKey} already registered`);
    }
    if (preexisting) slotContributions.delete(mapKey);
    const preexistingById = this.contributionsById.get(contributionId);
    if (preexistingById?.active) throw new Error(`client contribution ${contributionId} already registered`);
    if (preexistingById) this.contributionsById.delete(contributionId);

    const dispatchKind = parentDefinition.kind ?? clientKindFromMultiplicity(parentDefinition.multiplicity);
    if (strict) {
      if (dispatchKind === "keyed" && !options.entryKey?.trim()) throw new Error(`keyed slot ${normalizedSlotId} requires entryKey`);
      if (dispatchKind === "list" && !options.cellId?.trim()) throw new Error(`list slot ${normalizedSlotId} requires id`);
      if (dispatchKind === "chain" && typeof options.select !== "function") throw new Error(`chain slot ${normalizedSlotId} requires select()`);
      if (dispatchKind !== "chain") {
        const cell = dispatchKind === "keyed" ? options.entryKey!.trim() : dispatchKind === "list" ? options.cellId!.trim() : "__single__";
        const priority = Number.isFinite(options.priority) ? Number(options.priority) : 0;
        for (const existing of slotContributions.values()) {
          if (!existing.strict) continue;
          const existingCell = dispatchKind === "keyed" ? existing.entryKey : dispatchKind === "list" ? existing.cellId : "__single__";
          if (existingCell === cell && existing.priority === priority) {
            throw new Error(`slot ${normalizedSlotId} cell ${cell} already has strict priority ${priority} occupant ${existing.contributionId}`);
          }
        }
      }
    }

    const sharedManagedStoreHandle = isClientSlotManagedStoreHandle(options.store)
      ? options.store as object
      : undefined;
    if (sharedManagedStoreHandle) {
      this.pinManagedStoreRegistration(sharedManagedStoreHandle, parentDefinition.scope ?? "root");
    }

    const contribution: ClientSlotContribution = {
      contributionId,
      pluginId: normalizedPluginId,
      key: normalizedKey,
      slotId: normalizedSlotId,
      component: markRaw(component),
      ordering: Number.isFinite(options.ordering) ? Number(options.ordering) : 0,
      priority: Number.isFinite(options.priority) ? Number(options.priority) : 0,
      props: options.props ? { ...options.props } : undefined,
      entryKey: options.entryKey?.trim() || undefined,
      cellId: options.cellId?.trim() || undefined,
      label: options.label,
      sequence: ++this.contributionSequence,
      select: options.select,
      localeNamespace: options.localeNamespace?.trim() || undefined,
      childSlotIds: childIds,
      scope: parentDefinition.scope ?? "root",
      strict,
      active: true,
      abdicated: false,
      storeDecl: options.store,
      sharedManagedStoreHandle,
      injectDecl: options.inject as ClientSlotInjectFactory<unknown, Record<string, unknown>> | undefined,
      environment,
      instances: new Map(),
      ownedChildEpochs: [],
      claimedStaticChildren: [],
      childActivations: [],
    };

    slotContributions.set(mapKey, contribution);
    this.contributionsById.set(contributionId, contribution);

    // Root entries have a single framework-owned instance and retain the old
    // store/injected aliases for inspection. Session entries are instantiated
    // per session at the outlet.
    if (contribution.scope === "root") {
      try {
        const instance = this.acquireContributionInstance(contributionId);
        contribution.store = instance.store;
        contribution.actions = instance.actions;
        contribution.injected = instance.injected;
      } catch (error) {
        slotContributions.delete(mapKey);
        if (slotContributions.size === 0) this.contributions.delete(normalizedSlotId);
        this.contributionsById.delete(contributionId);
        contribution.active = false;
        if (contribution.sharedManagedStoreHandle) {
          this.releaseManagedStoreRegistration(contribution.sharedManagedStoreHandle, contribution.scope);
        }
        throw error;
      }
    }

    for (const child of staticChildrenToClaim) {
      this.childClaims.set(child.slotId, {
        ownerEntryId: contributionId,
        ownerId: normalizedPluginId,
        parentSlotId: normalizedSlotId,
        ...(child.commonInject ? { commonInject: child.commonInject } : {}),
      });
      contribution.claimedStaticChildren.push(child.slotId);
    }

    const childEpochs = contribution.ownedChildEpochs;
    const childActivations = contribution.childActivations;
    try {
      for (const child of childrenToDeclare) {
        const epoch = Math.max(this.epochs.get(child.slotId) ?? 0, child.declarationEpoch ?? 0) + 1;
        this.epochs.set(child.slotId, epoch);
        const definition = { ...child, declarationEpoch: epoch };
        this.local.set(child.slotId, definition);
        childEpochs.push({ slotId: child.slotId, epoch });
        this.bumpRevision();
        const activation = this.activate(child.slotId, definition);
        childActivations.push(activation.catch(() => undefined));
        activation.catch(async () => {
          await this.collapseLocalSlot(child.slotId, epoch, false);
        });
      }
    } catch (error) {
      awaitableDispose(this.disposeContribution(contribution));
      throw error;
    }

    this.bumpRevision();
    return () => this.disposeContribution(contribution);
  }

  private async disposeContribution(contribution: ClientSlotContribution): Promise<void> {
    if (!contribution.active) return;
    contribution.active = false;
    // Render authority is revoked before any descendant teardown starts.
    this.contributionsById.delete(contribution.contributionId);
    for (const childSlotId of contribution.claimedStaticChildren) {
      const claim = this.childClaims.get(childSlotId);
      if (claim?.ownerEntryId === contribution.contributionId) this.childClaims.delete(childSlotId);
    }
    contribution.claimedStaticChildren.length = 0;
    await Promise.allSettled(contribution.childActivations);
    for (let index = contribution.ownedChildEpochs.length - 1; index >= 0; index--) {
      const child = contribution.ownedChildEpochs[index]!;
      await this.collapseLocalSlot(child.slotId, child.epoch, true);
    }
    const current = this.contributions.get(contribution.slotId);
    current?.delete(`${contribution.pluginId}:${contribution.key}`);
    if (current?.size === 0) this.contributions.delete(contribution.slotId);
    await this.disposeContributionInstances(contribution);
    if (contribution.sharedManagedStoreHandle) {
      this.releaseManagedStoreRegistration(contribution.sharedManagedStoreHandle, contribution.scope);
      contribution.sharedManagedStoreHandle = undefined;
    }
    this.bumpRevision();
  }

  /** Collapse every live entry occupying a declaration. This is the recursive
   * lifecycle axis required by children=declaration+authorization: removing a
   * parent declaration cannot leave child entries, stores, or grandchildren
   * dormant in maps waiting to resurrect on a later declaration epoch. */
  private async collapseEntriesForSlot(slotId: string): Promise<void> {
    const entries = Array.from(this.contributions.get(slotId)?.values() ?? []);
    for (let index = entries.length - 1; index >= 0; index--) {
      await this.disposeContribution(entries[index]!);
    }
    this.contributions.delete(slotId);
  }

  private async collapseLocalSlot(slotId: string, epoch: number, revealServer: boolean): Promise<void> {
    if (this.epochs.get(slotId) !== epoch || !this.local.has(slotId)) return;
    await this.collapseEntriesForSlot(slotId);
    await this.deactivate(slotId);
    if (this.epochs.get(slotId) !== epoch || !this.local.has(slotId)) return;
    this.local.delete(slotId);
    this.bumpRevision();
    if (revealServer) {
      const fallback = this.server.get(slotId);
      if (fallback) await this.activate(slotId, fallback);
    }
  }

  /** Framework outlet calls this to obtain the store+inject instance for the
   * current tree situation. Session stores persist across component remounts
   * and die when the session scope or entry dies. */
  acquireContributionInstance(contributionId: string, sessionId?: string): ClientContributionInstance {
    const contribution = this.contributionsById.get(contributionId);
    if (!contribution || !contribution.active) throw new Error(`stale client slot contribution ${contributionId}`);
    const normalizedSessionId = sessionId?.trim() || undefined;
    const scopeKey = contribution.scope === "root"
      ? "root"
      : contribution.scope === "session"
        ? normalizedSessionId
        : (normalizedSessionId ?? "root");
    if (!scopeKey) throw new Error(`session-scoped contribution ${contributionId} requires sessionId`);
    const existing = contribution.instances.get(scopeKey);
    if (existing) return existing;

    const resourceContext = {
      pluginId: contribution.pluginId,
      slotId: contribution.slotId,
      key: contribution.key,
      scope: contribution.scope,
      ...(normalizedSessionId ? { sessionId: normalizedSessionId } : {}),
    } as const;
    const disposers: ClientDisposer[] = [];
    let store: unknown;
    let actions: Readonly<Record<string, (...args: any[]) => any>> = Object.freeze({});
    let getSnapshot: () => unknown = () => store;
    let subscribe: ((listener: () => void) => () => void) | undefined;
    let clearPersisted: (() => void) | undefined;

    const attachManagedStore = (handle: ClientSlotManagedStoreHandle<unknown, any>) => {
      const managed = handle.create(scopeKey) as ClientSlotManagedStoreInstance<unknown, any>;
      store = managed.getSnapshot();
      getSnapshot = () => managed.getSnapshot();
      actions = Object.freeze({ ...(managed.actions ?? {}) });
      subscribe = managed.subscribe.bind(managed);
      clearPersisted = () => managed.clearPersisted();
      disposers.push(() => managed.dispose());
    };
    const attachSharedStore = (handle: ClientSlotStoreHandle<unknown, Record<string, (...args: any[]) => any>>) => {
      disposers.push(this.pinSharedStoreScope(handle as object, scopeKey));
      store = handle.state;
      getSnapshot = () => handle.state;
      actions = Object.freeze({ ...(handle.actions ?? {}) });
      subscribe = handle.subscribe;
    };

    if (typeof contribution.storeDecl === "function") {
      const created = (contribution.storeDecl as ClientSlotStoreFactory<unknown>)(resourceContext);
      if (isClientSlotStoreResource(created)) {
        store = created.store;
        getSnapshot = () => created.store;
        actions = Object.freeze({ ...(created.actions ?? {}) });
        subscribe = created.subscribe;
        if (created.dispose) disposers.push(() => created.dispose?.());
      } else if (isClientSlotManagedStoreHandle(created)) {
        attachManagedStore(created);
      } else if (isClientSlotStoreHandle(created)) {
        attachSharedStore(created);
      } else {
        store = created;
        getSnapshot = () => store;
      }
    } else if (isClientSlotManagedStoreHandle(contribution.storeDecl)) {
      attachManagedStore(contribution.storeDecl);
    } else if (isClientSlotStoreHandle(contribution.storeDecl)) {
      // Compatibility shared handles remain supported, but unlike managed
      // handles they are forbidden from silently crossing scope instances.
      attachSharedStore(contribution.storeDecl);
    } else {
      store = cloneSessionStoreIfNeeded(contribution.storeDecl, contribution.scope, normalizedSessionId);
      getSnapshot = () => store;
    }

    let injected: Readonly<Record<string, unknown>> = Object.freeze({});
    try {
      if (contribution.injectDecl) {
        const face = contribution.injectDecl({
          ...resourceContext,
          store,
          actions,
          services: contribution.environment.services,
          events: contribution.environment.events,
        });
        injected = Object.freeze({ ...(face ?? {}) });
      }
    } catch (error) {
      for (let index = disposers.length - 1; index >= 0; index--) void disposers[index]?.();
      throw error;
    }

    let active = true;
    const instance: ClientContributionInstance = {
      scopeKey,
      sessionId: normalizedSessionId,
      store,
      actions,
      getSnapshot,
      subscribe,
      clearPersisted,
      injected,
      dispose: async () => {
        if (!active) return;
        active = false;
        for (let index = disposers.length - 1; index >= 0; index--) await disposers[index]?.();
      },
    };
    contribution.instances.set(scopeKey, instance);
    return instance;
  }

  private pinManagedStoreRegistration(handle: object, scope: NonNullable<ClientSlotDefinition["scope"]>): void {
    const existing = this.managedStoreScopes.get(handle);
    if (existing && existing.scope !== scope) {
      throw new Error(`managed slot store handle is already mounted under scope ${existing.scope}; cannot mount under ${scope}`);
    }
    if (existing) existing.count += 1;
    else this.managedStoreScopes.set(handle, { scope, count: 1 });
  }

  private releaseManagedStoreRegistration(handle: object, scope: NonNullable<ClientSlotDefinition["scope"]>): void {
    const existing = this.managedStoreScopes.get(handle);
    if (!existing || existing.scope !== scope) return;
    existing.count -= 1;
    if (existing.count <= 0) this.managedStoreScopes.delete(handle);
  }

  private pinSharedStoreScope(handle: object, scopeKey: string): ClientDisposer {
    const existing = this.sharedStoreScopes.get(handle);
    if (existing && existing.scopeKey !== scopeKey) {
      throw new Error(`shared slot store handle is already pinned to scope ${existing.scopeKey}; cannot reuse it in ${scopeKey}`);
    }
    if (existing) existing.count += 1;
    else this.sharedStoreScopes.set(handle, { scopeKey, count: 1 });
    let active = true;
    return () => {
      if (!active) return;
      active = false;
      const current = this.sharedStoreScopes.get(handle);
      if (!current || current.scopeKey !== scopeKey) return;
      current.count -= 1;
      if (current.count <= 0) this.sharedStoreScopes.delete(handle);
    };
  }

  async disposeSessionRuntime(sessionId: string): Promise<void> {
    const scopeKey = sessionId.trim();
    if (!scopeKey) return;
    for (const contribution of this.contributionsById.values()) {
      const instance = contribution.instances.get(scopeKey);
      if (!instance || contribution.scope === "root") continue;
      contribution.instances.delete(scopeKey);
      instance.clearPersisted?.();
      await instance.dispose();
    }
  }

  private async disposeContributionInstances(contribution: ClientSlotContribution): Promise<void> {
    const instances = Array.from(contribution.instances.values());
    contribution.instances.clear();
    for (let index = instances.length - 1; index >= 0; index--) await instances[index]?.dispose();
  }

  isAuthorizedChild(contributionId: string, childSlotId: string): boolean {
    const contribution = this.contributionsById.get(contributionId);
    if (!contribution || !contribution.active || !contribution.childSlotIds.has(childSlotId)) return false;
    const definition = this.current(childSlotId);
    if (!definition) return false;
    if (definition.parentSlotId && definition.parentSlotId !== contribution.slotId) return false;
    if (definition.ownerEntryId && definition.ownerEntryId !== contributionId) return false;
    return true;
  }

  assertRenderAuthority(contributionId: string, childSlotId: string): void {
    if (!this.isAuthorizedChild(contributionId, childSlotId)) {
      throw new Error(`client contribution ${contributionId} is not authorized to render child slot ${childSlotId}`);
    }
  }

  list(): ClientSlotDefinition[] {
    this.revision.value;
    const ids = new Set([...this.server.keys(), ...this.local.keys()]);
    return Array.from(ids)
      .map((id) => this.current(id))
      .filter((definition): definition is ClientSlotDefinition => !!definition)
      .sort((a, b) => a.slotId.localeCompare(b.slotId));
  }

  getDefinition(slotId: string): ClientSlotDefinition | undefined {
    this.revision.value;
    return this.current(slotId);
  }

  listContributions(slotId: string): ClientSlotContribution[] {
    this.revision.value;
    if (!this.current(slotId)) return [];
    return Array.from(this.contributions.get(slotId)?.values() ?? [])
      .filter((entry) => entry.active)
      .sort(compareClientStable);
  }

  dispatchContributions(
    slotId: string,
    owner: Record<string, unknown> = {},
    dispatchKey?: string,
    listOnly?: string,
  ): ClientSlotDispatchResult[] {
    const definition = this.current(slotId);
    if (!definition) return [];
    const entries = this.listContributions(slotId);
    const kind = definition.kind ?? clientKindFromMultiplicity(definition.multiplicity);
    const strict = entries.filter((entry) => entry.strict && !entry.abdicated);
    if (strict.length > 0) {
      if (kind === "keyed") {
        if (!dispatchKey) return [];
        const winner = strict.filter((entry) => entry.entryKey === dispatchKey).sort(compareStrictShadow)[0];
        return winner ? [{ contribution: winner }] : [];
      }
      if (kind === "list") {
        const cells = new Map<string, ClientSlotContribution>();
        for (const entry of [...strict].sort(compareStrictShadow)) {
          const cell = entry.cellId ?? entry.key;
          if (!cells.has(cell)) cells.set(cell, entry);
        }
        const winners = Array.from(cells.values()).sort(compareStrictListDisplay);
        return winners
          .filter((entry) => !listOnly || entry.cellId === listOnly)
          .map((contribution) => ({ contribution }));
      }
      if (kind === "single") {
        const winner = [...strict].sort(compareStrictShadow)[0];
        return winner ? [{ contribution: winner }] : [];
      }
      const ordered = [...strict].sort(compareClientChain);
      for (const contribution of ordered) {
        if (!contribution.select) continue;
        const matched = contribution.select(owner);
        if (matched != null) return [{ contribution, matched }];
      }
      return [];
    }
    // Legacy entries keep Amitia's historical behavior when no strict DSH-style
    // entry occupies the slot. This preserves the existing Provider/Schema/UI ecosystem.
    if (kind === "keyed") {
      if (!dispatchKey) return [];
      return entries.filter((entry) => entry.entryKey === dispatchKey).map((contribution) => ({ contribution }));
    }
    if (kind === "chain") {
      const ordered = [...entries].sort(compareLegacyChain);
      for (const contribution of ordered) {
        if (!contribution.select) continue;
        const matched = contribution.select(owner);
        if (matched != null) return [{ contribution, matched }];
      }
      return [];
    }
    return entries.map((contribution) => ({ contribution }));
  }

  onEntryError(listener: (event: { contributionId: string; slotId: string; pluginId: string; error: unknown; abdicated: boolean }) => void): ClientDisposer {
    this.entryErrorListeners.add(listener);
    return () => {
      this.entryErrorListeners.delete(listener);
    };
  }

  /** Report a render crash. Shadowing strict entries can abdicate without being
   * unregistered, allowing the next lower-priority survivor to render. */
  reportEntryError(
    contributionId: string,
    error: unknown,
    options: { abdicate?: boolean } = {},
  ): boolean {
    const contribution = this.contributionsById.get(contributionId);
    if (!contribution || !contribution.active) return false;
    const definition = this.current(contribution.slotId);
    const kind = definition?.kind ?? clientKindFromMultiplicity(definition?.multiplicity);
    const canAbdicate = contribution.strict && options.abdicate === true && kind !== "chain";
    if (canAbdicate && !contribution.abdicated) {
      contribution.abdicated = true;
      this.bumpRevision();
    }
    const event = {
      contributionId,
      slotId: contribution.slotId,
      pluginId: contribution.pluginId,
      error,
      abdicated: canAbdicate,
    };
    for (const listener of Array.from(this.entryErrorListeners)) {
      try { listener(event); } catch { /* diagnostics must not block failover */ }
    }
    return canAbdicate;
  }

  /** Amitia superset recovery hook: explicitly re-enable an abdicated entry. */
  reviveContribution(contributionId: string): boolean {
    const contribution = this.contributionsById.get(contributionId);
    if (!contribution || !contribution.active || !contribution.abdicated) return false;
    contribution.abdicated = false;
    this.bumpRevision();
    return true;
  }

  inspect() {
    return this.list().map((definition) => ({
      ...definition,
      contributions: this.listContributions(definition.slotId).map((entry) => ({
        contributionId: entry.contributionId,
        pluginId: entry.pluginId,
        key: entry.key,
        entryKey: entry.entryKey,
        localeNamespace: entry.localeNamespace,
        ordering: entry.ordering,
        priority: entry.priority,
        strict: entry.strict,
        abdicated: entry.abdicated,
        childSlots: Array.from(entry.childSlotIds).sort(),
        instances: Array.from(entry.instances.keys()).sort(),
      })),
    }));
  }

  private currentRaw(id: string): ClientSlotDefinition | undefined {
    return this.local.get(id) ?? this.server.get(id);
  }

  private current(id: string): ClientSlotDefinition | undefined {
    const base = this.currentRaw(id);
    if (!base) return undefined;
    const claim = this.childClaims.get(id);
    if (!claim) return base;
    return {
      ...base,
      ownerEntryId: claim.ownerEntryId,
      ownerId: claim.ownerId,
      parentSlotId: claim.parentSlotId,
      ...(claim.commonInject ? { commonInject: claim.commonInject } : {}),
    };
  }

  private async deactivate(id: string): Promise<void> {
    for (const subscriber of this.subscribers.get(id) ?? []) {
      if (subscriber.cleanup) await subscriber.cleanup();
      subscriber.cleanup = undefined;
    }
  }

  private async activate(id: string, definition: ClientSlotDefinition): Promise<void> {
    for (const subscriber of this.subscribers.get(id) ?? []) {
      if (subscriber.cleanup) await subscriber.cleanup();
      subscriber.cleanup = await normalizeSlotEffect(subscriber.callback(definition));
    }
  }

  private bumpRevision(): void {
    this.revision.value += 1;
  }
}

function compareClientStable(a: ClientSlotContribution, b: ClientSlotContribution): number {
  return a.ordering - b.ordering || b.priority - a.priority || a.contributionId.localeCompare(b.contributionId);
}

function compareClientChain(a: ClientSlotContribution, b: ClientSlotContribution): number {
  return a.priority - b.priority || a.sequence - b.sequence;
}

function compareLegacyChain(a: ClientSlotContribution, b: ClientSlotContribution): number {
  return b.priority - a.priority || a.ordering - b.ordering || a.contributionId.localeCompare(b.contributionId);
}

function compareStrictShadow(a: ClientSlotContribution, b: ClientSlotContribution): number {
  return a.priority - b.priority || a.sequence - b.sequence;
}

function compareStrictListDisplay(a: ClientSlotContribution, b: ClientSlotContribution): number {
  return a.ordering - b.ordering || a.sequence - b.sequence;
}

function clientKindFromMultiplicity(multiplicity: ClientSlotDefinition["multiplicity"]): ClientSlotKind {
  if (multiplicity === "single" || multiplicity === "replaceable_single" || multiplicity === "exclusive") return "single";
  return "list";
}

function multiplicityFromClientKind(kind: ClientSlotKind): NonNullable<ClientSlotDefinition["multiplicity"]> {
  return kind === "single" ? "replaceable_single" : "ordered_multiple";
}

function isClientSlotManagedStoreHandle(value: unknown): value is ClientSlotManagedStoreHandle<unknown, any> {
  return !!value
    && typeof value === "object"
    && (value as { __amitiaManagedSlotStoreHandle?: unknown }).__amitiaManagedSlotStoreHandle === true
    && typeof (value as { create?: unknown }).create === "function";
}

function isClientSlotStoreHandle(value: unknown): value is ClientSlotStoreHandle<unknown, Record<string, (...args: any[]) => any>> {
  return !!value
    && typeof value === "object"
    && "state" in value
    && "actions" in value
    && !!(value as { actions?: unknown }).actions
    && typeof (value as { actions?: unknown }).actions === "object";
}

function cloneSessionStoreIfNeeded(value: unknown, scope: ClientSlotContribution["scope"], sessionId?: string): unknown {
  if (value == null || scope === "root" || !sessionId) return value;
  if (typeof structuredClone === "function") {
    try { return structuredClone(value); } catch { /* fall through */ }
  }
  if (Array.isArray(value)) return [...value];
  if (typeof value === "object" && Object.getPrototypeOf(value) === Object.prototype) return { ...(value as Record<string, unknown>) };
  return value;
}

function awaitableDispose(task: Promise<void>): void {
  void task.catch((error) => console.error("[ClientSlotRuntime] contribution rollback failed", error));
}

async function normalizeSlotEffect(effect: ClientSlotEffect): Promise<ClientDisposer | undefined> {
  const resolved = await effect;
  if (!resolved) return undefined;
  if (typeof resolved === "function") return resolved;

  const disposers: ClientDisposer[] = [];
  try {
    for (const disposer of resolved) {
      if (typeof disposer !== "function") throw new Error("slot effect iterable must yield disposer functions");
      disposers.push(disposer);
    }
  } catch (error) {
    for (let index = disposers.length - 1; index >= 0; index--) {
      try { await disposers[index]?.(); } catch { /* preserve original setup failure */ }
    }
    throw error;
  }
  return async () => {
    for (let index = disposers.length - 1; index >= 0; index--) await disposers[index]?.();
  };
}

class ClientFiber {
  private readonly disposers: ClientDisposer[] = [];
  private disposed = false;
  constructor(readonly id: string) {}
  get resourceCount(): number { return this.disposers.length; }
  get isDisposed(): boolean { return this.disposed; }
  own(disposer: ClientDisposer): ClientDisposer {
    if (this.disposed) void disposer(); else this.disposers.push(disposer);
    return disposer;
  }
  async dispose(): Promise<void> {
    if (this.disposed) return;
    this.disposed = true;
    for (let index = this.disposers.length - 1; index >= 0; index--) await this.disposers[index]?.();
    this.disposers.length = 0;
  }
}

class ClientPackageRunSupersededError extends Error {
  constructor(id: string, version: string) {
    super(`client package ${id}@${version} run was superseded by a newer lifecycle mutation`);
    this.name = "ClientPackageRunSupersededError";
  }
}

export type BrowserConversationNodeDefinition<State = unknown> = Omit<
  ProgrammaticConversationNodeDefinition<State>,
  "id" | "contributionId" | "extensionId"
>;

export interface BrowserClientPluginContext {
  pluginId: string;
  services: {
    provide<T>(id: string, value: T): ClientDisposer;
    get<T>(id: string): T | undefined;
    list(): string[];
  };
  events: {
    on<T>(type: string, handler: (payload: T) => void | Promise<void>): ClientDisposer;
    emit<T>(type: string, payload: T): Promise<void>;
    list(): string[];
  };
  locale: {
    register<N extends keyof ClientLocaleNamespaceMap & string>(
      namespace: N,
      dictionaries: Readonly<Record<string, Readonly<Record<ClientLocaleNamespaceMap[N] & string, string>>>>,
    ): ClientDisposer;
    setCurrent(locale: string): void;
    current(): string;
  };
  slots: {
    declare(definition: ClientSlotDefinition): Promise<ClientSlotRegistration>;
    observe(slotId: string, callback: (definition: ClientSlotDefinition) => ClientSlotEffect): Promise<ClientDisposer>;
    /** Slot-declaration lifetime dependency. */
    inject(slotId: string, callback: (definition: ClientSlotDefinition) => ClientSlotEffect): Promise<ClientDisposer>;
    register<
      K extends ClientKnownSlotId,
      const D extends ClientChildrenDecl = {},
      S = ClientSlotStore<K>,
      I extends Record<string, unknown> = {},
      N extends keyof ClientLocaleNamespaceMap & string | undefined = undefined,
      const EntryKey extends ClientSlotEntryKey<K> = ClientSlotEntryKey<K>,
      C extends ClientSlotComponent<never> = ClientSlotComponent<never>,
    >(
      options: ClientSlotRegisterOptions<K, D, S, I, N, EntryKey>,
      component: ClientSlotRegistrationComponent<K, D, S, I, N, EntryKey, C>,
    ): ClientDisposer;
    /** Explicit compatibility path for declarative/legacy packages. */
    registerLegacy(
      slotId: string,
      key: string,
      component: Component,
      options?: ClientSlotContributionOptions,
    ): ClientDisposer;
    onEntryError(callback: (event: { contributionId: string; slotId: string; pluginId: string; error: unknown; abdicated: boolean }) => void): ClientDisposer;
    revive(contributionId: string): boolean;
    list(): ClientSlotDefinition[];
  };
  conversationNodes: {
    register<State>(
      key: string,
      component: Component,
      definition: BrowserConversationNodeDefinition<State>,
      options?: ClientSlotContributionOptions,
    ): ClientDisposer;
  };
}

export interface BrowserClientPluginDefinition {
  id: string;
  version?: string;
  atomicUpdate?: boolean;
  setup(context: BrowserClientPluginContext): void | ClientDisposer | Promise<void | ClientDisposer>;
}

export interface BrowserDeclarativeClientContribution {
  slotId: string;
  key: string;
  sourceExtensionId?: string;
  sourceContributionId?: string;
  clientCode?: DynamicClientSandboxCode;
  title?: string;
  kind?: string;
  ordering?: number;
  priority?: number;
  children?: ClientSlotDefinition[];
}

export interface BrowserDeclarativeConversationNode {
  key: string;
  sourceExtensionId: string;
  sourceContributionId: string;
  projection: ConversationProjectionSpec;
  title?: string;
  ordering?: number;
}

export interface BrowserDeclarativeClientPackage {
  id: string;
  version: string;
  description?: string;
  contributions?: BrowserDeclarativeClientContribution[];
  conversationNodes?: BrowserDeclarativeConversationNode[];
}

export interface BrowserClientRuntimeSessionPackage {
  id: string;
  versions: BrowserDeclarativeClientPackage[];
  approvedVersions?: string[];
  activeVersion?: string;
  targetVersion?: string;
  transitionState?: string;
  transitionMode?: "run" | "update" | string;
  runId?: string;
  pluginRunId?: string;
  running?: boolean;
}

export interface BrowserClientRuntimeSessionState {
  userId?: string;
  conversationId: string;
  revision?: number;
  packages: BrowserClientRuntimeSessionPackage[];
}

export class BrowserClientPluginRuntime {
  private readonly definitions = new Map<string, BrowserClientPluginDefinition>();
  private readonly packages = new Map<string, Map<string, BrowserClientPluginDefinition>>();
  private readonly packageOrder = new Map<string, string[]>();
  private readonly activePackageVersion = new Map<string, string>();
  private readonly packageMutationEpoch = new Map<string, number>();
  private readonly running = new Map<string, ClientFiber>();
  private readonly services = new Map<string, unknown>();
  private readonly serviceOwners = new Map<string, string>();
  private readonly events = new Map<string, Set<(payload: unknown) => void | Promise<void>>>();
  private readonly eventOwners = new Map<string, Map<(payload: unknown) => void | Promise<void>, string>>();
  private activeConversationScope = "";
  private activeConversationScopeGeneration = 0;
  private readonly sessionManagedPackages = new Map<string, Map<string, { packageId: string; version: string }>>();
  private readonly sessionRevisions = new Map<string, number>();
  private readonly sessionSyncQueues = new Map<string, Promise<void>>();
  readonly slots = new BrowserSlotRuntime();
  readonly locale = new BrowserLocaleRuntime();

  setActiveConversationScope(conversationId: string): void {
    const scope = conversationId.trim();
    if (scope === this.activeConversationScope) return;
    const previous = this.activeConversationScope;
    this.activeConversationScope = scope;
    this.activeConversationScopeGeneration += 1;
    if (previous) void this.slots.disposeSessionRuntime(previous);
  }

  async activateConversationScope(conversationId: string): Promise<number> {
    const scope = conversationId.trim();
    if (scope !== this.activeConversationScope) {
      const previous = this.activeConversationScope;
      this.activeConversationScope = scope;
      this.activeConversationScopeGeneration += 1;
      if (previous) await this.slots.disposeSessionRuntime(previous);
    }
    const generation = this.activeConversationScopeGeneration;
    for (const [managedScope, packageVersions] of Array.from(this.sessionManagedPackages.entries())) {
      if (managedScope === scope) continue;
      for (const entry of packageVersions.values()) {
        await this.undefinePackage(entry.packageId, entry.version);
      }
      this.sessionManagedPackages.delete(managedScope);
      this.sessionRevisions.delete(managedScope);
    }
    return generation;
  }

  isActiveConversationScope(conversationId: string): boolean {
    const normalized = conversationId.trim();
    return !!normalized && normalized === this.activeConversationScope;
  }

  private scopeGenerationMatches(scope: string, expectedGeneration: number): boolean {
    return scope === this.activeConversationScope && expectedGeneration === this.activeConversationScopeGeneration;
  }

  scopedPackageId(conversationId: string, packageId: string): string {
    const scope = conversationId.trim();
    const id = packageId.trim();
    if (!scope || !id) throw new Error("conversation scope and package id are required");
    return `session:${encodeURIComponent(scope)}:${id}`;
  }

  defineScopedDeclarativePackage(conversationId: string, spec: BrowserDeclarativeClientPackage): ClientDisposer {
    const scopedId = this.scopedPackageId(conversationId, spec.id);
    const dispose = this.defineDeclarativePackage({ ...spec, id: scopedId });
    let versions = this.sessionManagedPackages.get(conversationId);
    if (!versions) { versions = new Map(); this.sessionManagedPackages.set(conversationId, versions); }
    versions.set(managedPackageToken(scopedId, spec.version), { packageId: scopedId, version: spec.version });
    return dispose;
  }

  async synchronizeSession(
    state: BrowserClientRuntimeSessionState,
    options: { activate?: boolean; expectedScopeGeneration?: number } = {},
  ): Promise<boolean> {
    const scope = state.conversationId?.trim();
    if (!scope) return false;

    let scopeGeneration: number;
    if (options.activate === true) {
      scopeGeneration = await this.activateConversationScope(scope);
    } else {
      if (!this.isActiveConversationScope(scope)) return false;
      scopeGeneration = options.expectedScopeGeneration ?? this.activeConversationScopeGeneration;
      if (!this.scopeGenerationMatches(scope, scopeGeneration)) return false;
    }

    const previous = this.sessionSyncQueues.get(scope) ?? Promise.resolve();
    let applied = false;
    const next = previous.catch(() => {}).then(async () => {
      applied = await this.applySessionSnapshot(state, scope, scopeGeneration);
    });
    this.sessionSyncQueues.set(scope, next);
    try {
      await next;
      return applied;
    } finally {
      if (this.sessionSyncQueues.get(scope) === next) this.sessionSyncQueues.delete(scope);
    }
  }

  getSessionRevision(conversationId: string): number | undefined {
    return this.sessionRevisions.get(conversationId.trim());
  }

  private async applySessionSnapshot(
    state: BrowserClientRuntimeSessionState,
    scope: string,
    expectedScopeGeneration: number,
  ): Promise<boolean> {
    if (!this.scopeGenerationMatches(scope, expectedScopeGeneration)) return false;
    const incomingRevision = normalizeSessionRevision(state.revision);
    const currentRevision = this.sessionRevisions.get(scope);
    if (currentRevision !== undefined && incomingRevision <= currentRevision) return false;

    const desiredTokens = new Set<string>();
    for (const record of state.packages ?? []) {
      if (!this.scopeGenerationMatches(scope, expectedScopeGeneration)) return false;
      const scopedId = this.scopedPackageId(scope, record.id);
      for (const version of record.versions ?? []) {
        desiredTokens.add(managedPackageToken(scopedId, version.version));
        const existing = this.packages.get(scopedId)?.has(version.version);
        if (!existing) this.defineScopedDeclarativePackage(scope, { ...version, id: record.id });
      }
      const transitionState = record.transitionState?.trim().toLowerCase() || "";
      const transitionMode = record.transitionMode?.trim().toLowerCase() || "";
      const transitionWantsTarget = transitionState === "starting" || transitionState === "awaiting_client";
      const desiredVersion = transitionWantsTarget
        ? (record.targetVersion?.trim() || record.activeVersion?.trim())
        : record.activeVersion?.trim();
      const forceRestart = transitionWantsTarget
        && transitionMode === "run"
        && !!record.targetVersion?.trim()
        && record.targetVersion?.trim() === record.activeVersion?.trim();
      try {
        if (record.running && desiredVersion) {
          if (forceRestart) await this.restartPackage(scopedId, desiredVersion);
          else await this.runPackage(scopedId, desiredVersion);
        } else if (this.running.has(scopedId)) {
          await this.stopPackage(scopedId);
        }
      } catch (error) {
        if (!this.scopeGenerationMatches(scope, expectedScopeGeneration)) return false;
        throw error;
      }
      if (!this.scopeGenerationMatches(scope, expectedScopeGeneration)) return false;
    }

    if (!this.scopeGenerationMatches(scope, expectedScopeGeneration)) return false;
    const managed = this.sessionManagedPackages.get(scope) ?? new Map<string, { packageId: string; version: string }>();
    for (const [token, entry] of Array.from(managed.entries())) {
      if (desiredTokens.has(token)) continue;
      try {
        await this.undefinePackage(entry.packageId, entry.version);
      } catch (error) {
        if (!this.scopeGenerationMatches(scope, expectedScopeGeneration)) return false;
        throw error;
      }
      if (!this.scopeGenerationMatches(scope, expectedScopeGeneration)) return false;
      managed.delete(token);
    }
    if (managed.size === 0) this.sessionManagedPackages.delete(scope);
    if (!this.scopeGenerationMatches(scope, expectedScopeGeneration)) return false;
    this.sessionRevisions.set(scope, incomingRevision);
    return true;
  }

  define(definition: BrowserClientPluginDefinition): ClientDisposer {
    const id = definition.id?.trim();
    if (!id) throw new Error("client plugin id is required");
    if (this.definitions.has(id)) throw new Error(`client plugin ${id} already defined`);
    this.definitions.set(id, { ...definition, id });
    return () => this.undefine(id);
  }

  definePackage(definition: BrowserClientPluginDefinition & { version: string }): ClientDisposer {
    const id = definition.id?.trim();
    const version = definition.version?.trim();
    if (!id || !version) throw new Error("client package id and version are required");
    let versions = this.packages.get(id);
    if (!versions) { versions = new Map(); this.packages.set(id, versions); }
    if (versions.has(version)) throw new Error(`client package ${id}@${version} already exists`);
    versions.set(version, Object.freeze({ ...definition, id, version }));
    const order = this.packageOrder.get(id) ?? [];
    order.push(version);
    this.packageOrder.set(id, order);
    return () => this.undefinePackage(id, version);
  }

  defineDeclarativePackage(spec: BrowserDeclarativeClientPackage): ClientDisposer {
    const normalized = normalizeDeclarativePackage(spec);
    return this.definePackage({
      id: normalized.id,
      version: normalized.version,
      atomicUpdate: true,
      setup: async (ctx) => {
        for (const contribution of normalized.contributions ?? []) {
          const component = contribution.clientCode ? DynamicClientSandboxContribution : ClientRuntimeSchemaContribution;
          await ctx.slots.observe(contribution.slotId, () => ctx.slots.registerLegacy(
            contribution.slotId,
            contribution.key,
            component,
            {
              ordering: contribution.ordering ?? 0,
              priority: contribution.priority ?? 0,
              children: contribution.children ?? [],
              props: contribution.clientCode
                ? {
                    clientCode: contribution.clientCode,
                    title: contribution.title ?? normalized.description ?? "Dynamic UI",
                  }
                : {
                    sourceExtensionId: contribution.sourceExtensionId,
                    sourceContributionId: contribution.sourceContributionId,
                    title: contribution.title ?? normalized.description ?? "Dynamic UI",
                    kind: contribution.kind ?? "panel",
                  },
            },
          ));
        }
        for (const node of normalized.conversationNodes ?? []) {
          await ctx.slots.observe("chat.conversation.node", () => ctx.conversationNodes.register(
            node.key,
            ClientRuntimeSchemaContribution,
            declarativeConversationNodeDefinition(node.projection),
            {
              ordering: node.ordering ?? 0,
              props: {
                sourceExtensionId: node.sourceExtensionId,
                sourceContributionId: node.sourceContributionId,
                title: node.title ?? normalized.description ?? "Conversation Node",
                kind: "conversation_node",
              },
            },
          ));
        }
      },
    });
  }

  async runPackage(id: string, version?: string): Promise<void> {
    const versions = this.packages.get(id);
    if (!versions || versions.size === 0) throw new Error(`client package ${id} is not defined`);
    const selected = version || this.packageOrder.get(id)?.at(-1);
    const definition = selected ? versions.get(selected) : undefined;
    if (!definition || !selected) throw new Error(`client package ${id}@${selected ?? "?"} is not defined`);
    if (this.activePackageVersion.get(id) === selected && this.running.has(id)) return;

    const mutationEpoch = this.bumpPackageMutationEpoch(id);
    const previousDefinition = this.definitions.get(id);
    const previousVersion = this.activePackageVersion.get(id);
    const previousFiber = this.running.get(id);

    if (previousFiber && definition.atomicUpdate) {
      const candidatePluginId = `${id}#candidate:${selected}:${mutationEpoch}`;
      const candidateFiber = new ClientFiber(`client-plugin:${candidatePluginId}`);
      try {
        const cleanup = await definition.setup(this.context(candidatePluginId, candidateFiber));
        if (typeof cleanup === "function") candidateFiber.own(cleanup);
      } catch (error) {
        await candidateFiber.dispose();
        throw error;
      }
      if (!this.isPackageMutationCurrent(id, mutationEpoch) || this.packages.get(id)?.get(selected) !== definition) {
        await candidateFiber.dispose();
        throw new ClientPackageRunSupersededError(id, selected);
      }
      this.definitions.set(id, { ...definition, id });
      this.running.set(id, candidateFiber);
      this.activePackageVersion.set(id, selected);
      try {
        await previousFiber.dispose();
      } catch {
      }
      return;
    }

    const previousRunning = !!previousFiber;
    if (previousDefinition) await this.undefine(id);
    if (!this.isPackageMutationCurrent(id, mutationEpoch)) {
      throw new ClientPackageRunSupersededError(id, selected);
    }
    this.define(definition);
    try {
      await this.run(id);
      if (!this.isPackageMutationCurrent(id, mutationEpoch)
        || this.packages.get(id)?.get(selected) !== definition
        || !this.running.has(id)
        || this.definitions.get(id)?.version !== selected) {
        throw new ClientPackageRunSupersededError(id, selected);
      }
      this.activePackageVersion.set(id, selected);
    } catch (error) {
      if (this.isPackageMutationCurrent(id, mutationEpoch)) {
        await this.undefine(id);
        this.activePackageVersion.delete(id);
        if (!(error instanceof ClientPackageRunSupersededError) && previousDefinition) {
          this.define(previousDefinition);
          if (previousVersion) this.activePackageVersion.set(id, previousVersion);
          if (previousRunning) await this.run(id);
        }
      }
      throw error;
    }
  }

  async restartPackage(id: string, version?: string): Promise<void> {
    await this.stopPackage(id);
    await this.runPackage(id, version);
  }

  async rollbackPackage(id: string): Promise<string> {
    const order = this.packageOrder.get(id) ?? [];
    const active = this.activePackageVersion.get(id);
    const currentIndex = active ? order.lastIndexOf(active) : order.length;
    if (currentIndex <= 0) throw new Error(`client package ${id} has no rollback version`);
    const previous = order[currentIndex - 1]!;
    await this.runPackage(id, previous);
    return previous;
  }

  async stopPackage(id: string): Promise<void> {
    this.bumpPackageMutationEpoch(id);
    await this.stop(id);
  }

  async undefinePackage(id: string, version: string): Promise<void> {
    this.bumpPackageMutationEpoch(id);
    if (this.activePackageVersion.get(id) === version
      || this.definitions.get(id)?.version === version) {
      await this.undefine(id);
      this.activePackageVersion.delete(id);
    }
    const versions = this.packages.get(id);
    versions?.delete(version);
    if (versions?.size === 0) this.packages.delete(id);
    const order = (this.packageOrder.get(id) ?? []).filter((item) => item !== version);
    if (order.length) this.packageOrder.set(id, order); else this.packageOrder.delete(id);
  }

  async run(id: string): Promise<void> {
    if (this.running.has(id)) return;
    const definition = this.definitions.get(id);
    if (!definition) throw new Error(`client plugin ${id} is not defined`);
    const fiber = new ClientFiber(`client-plugin:${id}`);
    this.running.set(id, fiber);
    try {
      const cleanup = await definition.setup(this.context(id, fiber));
      if (typeof cleanup === "function") fiber.own(cleanup);
    } catch (error) {
      this.running.delete(id);
      await fiber.dispose();
      throw error;
    }
  }

  async stop(id: string): Promise<void> {
    const fiber = this.running.get(id);
    if (!fiber) return;
    this.running.delete(id);
    await fiber.dispose();
  }

  async undefine(id: string): Promise<void> {
    await this.stop(id);
    this.definitions.delete(id);
  }

  inspect() {
    const services = Array.from(this.services.keys()).sort();
    const events = Array.from(this.events.keys()).filter((type) => (this.events.get(type)?.size ?? 0) > 0).sort();
    return {
      plugins: Array.from(this.definitions.values()).map((definition) => ({
        id: definition.id,
        version: definition.version ?? "0",
        state: this.running.has(definition.id) ? "running" as const : "defined" as const,
      })),
      fibers: Array.from(this.running.entries()).map(([id, fiber]) => ({
        id: fiber.id,
        pluginId: id,
        resourceCount: fiber.resourceCount,
        disposed: fiber.isDisposed,
      })),
      services,
      serviceDetails: services.map((id) => ({ id, ownerPluginId: this.serviceOwners.get(id) ?? "" })),
      events,
      eventDetails: events.map((type) => ({
        type,
        subscribers: Array.from(this.eventOwners.get(type)?.values() ?? []).sort(),
      })),
      slots: this.slots.inspect(),
      conversationNodes: listProgrammaticConversationNodeDefinitions().map((definition) => ({
        id: definition.id,
        contributionId: definition.contributionId,
        extensionId: definition.extensionId,
      })),
      packages: Array.from(this.packages.entries()).map(([id, versions]) => ({
        id,
        versions: this.packageOrder.get(id) ?? Array.from(versions.keys()),
        activeVersion: this.activePackageVersion.get(id),
        running: this.running.has(id),
      })),
      activeConversationScope: this.activeConversationScope,
      sessionRevisions: Object.fromEntries(this.sessionRevisions.entries()),
    };
  }

  private bumpPackageMutationEpoch(id: string): number {
    const next = (this.packageMutationEpoch.get(id) ?? 0) + 1;
    this.packageMutationEpoch.set(id, next);
    return next;
  }

  private isPackageMutationCurrent(id: string, epoch: number): boolean {
    return (this.packageMutationEpoch.get(id) ?? 0) === epoch;
  }

  private context(pluginId: string, fiber: ClientFiber): BrowserClientPluginContext {
    return {
      pluginId,
      services: {
        provide: <T>(id: string, value: T) => {
          if (!id.trim()) throw new Error("service id is required");
          if (this.services.has(id)) throw new Error(`service ${id} already provided`);
          this.services.set(id, value);
          this.serviceOwners.set(id, pluginId);
          return fiber.own(() => {
            if (this.services.get(id) === value) {
              this.services.delete(id);
              this.serviceOwners.delete(id);
            }
          });
        },
        get: <T>(id: string) => this.services.get(id) as T | undefined,
        list: () => Array.from(this.services.keys()).sort(),
      },
      events: {
        on: <T>(type: string, handler: (payload: T) => void | Promise<void>) => {
          let handlers = this.events.get(type);
          if (!handlers) { handlers = new Set(); this.events.set(type, handlers); }
          const wrapped = handler as (payload: unknown) => void | Promise<void>;
          handlers.add(wrapped);
          let owners = this.eventOwners.get(type);
          if (!owners) { owners = new Map(); this.eventOwners.set(type, owners); }
          owners.set(wrapped, pluginId);
          return fiber.own(() => {
            handlers?.delete(wrapped);
            owners?.delete(wrapped);
            if (handlers?.size === 0) {
              this.events.delete(type);
              this.eventOwners.delete(type);
            }
          });
        },
        emit: async <T>(type: string, payload: T) => {
          for (const handler of Array.from(this.events.get(type) ?? [])) await handler(payload);
        },
        list: () => Array.from(this.events.keys()).sort(),
      },
      locale: {
        register: (namespace, dictionaries) => fiber.own(this.locale.register(pluginId, namespace, dictionaries)),
        setCurrent: (locale) => this.locale.setLocale(locale),
        current: () => this.locale.getLocale(),
      },
      slots: {
        declare: async (definition) => {
          const registration = await this.slots.declare({ ...definition, ownerId: pluginId });
          fiber.own(registration.dispose);
          return registration;
        },
        observe: async (slotId, callback) => {
          const dispose = await this.slots.observe(slotId, callback);
          fiber.own(dispose);
          return dispose;
        },
        inject: async (slotId, callback) => {
          const dispose = await this.slots.inject(slotId, callback);
          fiber.own(dispose);
          return dispose;
        },
        register: ((options: ClientSlotRegisterOptions<ClientKnownSlotId, ClientChildrenDecl, unknown, Record<string, unknown>>, component: Component) => {
          const environment: ClientSlotRegistrationEnvironment = {
            services: {
              get: <T>(serviceId: string) => this.services.get(serviceId) as T | undefined,
              list: () => Array.from(this.services.keys()).sort(),
            },
            events: {
              emit: async <T>(eventType: string, payload: T) => {
                for (const handler of Array.from(this.events.get(eventType) ?? [])) await handler(payload);
              },
            },
          };
          const dispose = this.slots.registerEntry(pluginId, options as never, component, environment);
          return fiber.own(dispose);
        }) as BrowserClientPluginContext["slots"]["register"],
        registerLegacy: (slotId, key, component, options) => {
          const environment: ClientSlotRegistrationEnvironment = {
            services: {
              get: <T>(serviceId: string) => this.services.get(serviceId) as T | undefined,
              list: () => Array.from(this.services.keys()).sort(),
            },
            events: {
              emit: async <T>(eventType: string, payload: T) => {
                for (const handler of Array.from(this.events.get(eventType) ?? [])) await handler(payload);
              },
            },
          };
          return fiber.own(this.slots.registerLegacy(pluginId, slotId, key, component, options, environment));
        },
        onEntryError: (callback) => fiber.own(this.slots.onEntryError(callback)),
        revive: (contributionId) => this.slots.reviveContribution(contributionId),
        list: () => this.slots.list(),
      },
      conversationNodes: {
        register: <State>(
          key: string,
          component: Component,
          definition: BrowserConversationNodeDefinition<State>,
          options?: ClientSlotContributionOptions,
        ) => {
          const normalizedKey = key.trim();
          if (!normalizedKey) throw new Error("conversation node key is required");
          const slotId = "chat.conversation.node";
          const contributionId = `client:${pluginId}:${slotId}:${normalizedKey}`;
          const disposeView = this.slots.registerLegacy(
            pluginId,
            slotId,
            normalizedKey,
            component,
            options,
            {
              services: {
                get: <T>(serviceId: string) => this.services.get(serviceId) as T | undefined,
                list: () => Array.from(this.services.keys()).sort(),
              },
              events: {
                emit: async <T>(eventType: string, payload: T) => {
                  for (const handler of Array.from(this.events.get(eventType) ?? [])) await handler(payload);
                },
              },
            },
          );
          let disposeDefinition: ClientDisposer;
          try {
            disposeDefinition = registerProgrammaticConversationNodeDefinition({
              ...definition,
              id: `${pluginId}:${normalizedKey}`,
              contributionId,
              extensionId: pluginId,
            });
          } catch (error) {
            void disposeView();
            throw error;
          }
          let active = true;
          return fiber.own(async () => {
            if (!active) return;
            active = false;
            await disposeDefinition();
            await disposeView();
          });
        },
      },
    };
  }
}

function isClientSlotStoreResource(value: unknown): value is ClientSlotStoreResource<unknown> {
  return !!value && typeof value === "object" && (value as { __amitiaSlotStoreResource?: unknown }).__amitiaSlotStoreResource === true;
}

function normalizeDeclarativePackage(spec: BrowserDeclarativeClientPackage): BrowserDeclarativeClientPackage {
  const id = spec.id?.trim();
  const version = spec.version?.trim();
  if (!id || !version) throw new Error("declarative client package id and version are required");
  const contributions = (spec.contributions ?? []).map((item) => {
    const slotId = item.slotId?.trim();
    const key = item.key?.trim();
    const sourceExtensionId = item.sourceExtensionId?.trim();
    const sourceContributionId = item.sourceContributionId?.trim();
    const clientCode = normalizeDynamicClientCode(item.clientCode);
    const hasSchemaSource = !!sourceExtensionId && !!sourceContributionId;
    if (!slotId || !key || (!hasSchemaSource && !clientCode)) {
      throw new Error(`client package ${id}@${version} contribution ${key || "?"} requires a published schema source or clientCode`);
    }
    if ((sourceExtensionId && !sourceContributionId) || (!sourceExtensionId && sourceContributionId)) {
      throw new Error(`client package ${id}@${version} contribution ${key} requires both sourceExtensionId and sourceContributionId`);
    }
    const children = (item.children ?? []).map((child) => normalizeDefinition(child));
    return {
      ...item,
      slotId,
      key,
      sourceExtensionId: sourceExtensionId || undefined,
      sourceContributionId: sourceContributionId || undefined,
      clientCode,
      ordering: Number.isFinite(item.ordering) ? Number(item.ordering) : 0,
      priority: Number.isFinite(item.priority) ? Number(item.priority) : 0,
      children,
    };
  });
  const conversationNodes = (spec.conversationNodes ?? []).map((item) => {
    const key = item.key?.trim();
    const sourceExtensionId = item.sourceExtensionId?.trim();
    const sourceContributionId = item.sourceContributionId?.trim();
    if (!key || !sourceExtensionId || !sourceContributionId) {
      throw new Error(`client package ${id}@${version} has an invalid conversation node`);
    }
    if (!item.projection?.eventTypes?.length || !item.projection.startEvents?.length) {
      throw new Error(`conversation node ${key} requires eventTypes and startEvents`);
    }
    return { ...item, key, sourceExtensionId, sourceContributionId, projection: { ...item.projection } };
  });
  return { ...spec, id, version, contributions, conversationNodes };
}

function normalizeDynamicClientCode(value: DynamicClientSandboxCode | undefined): DynamicClientSandboxCode | undefined {
  if (!value) return undefined;
  const html = typeof value.html === "string" ? value.html : "";
  const css = typeof value.css === "string" ? value.css : "";
  const script = typeof value.script === "string" ? value.script : "";
  if (!html && !css && !script) throw new Error("clientCode requires html, css, or script");
  if (html.length > DYNAMIC_CLIENT_HTML_LIMIT) throw new Error(`clientCode.html exceeds ${DYNAMIC_CLIENT_HTML_LIMIT} characters`);
  if (css.length > DYNAMIC_CLIENT_CSS_LIMIT) throw new Error(`clientCode.css exceeds ${DYNAMIC_CLIENT_CSS_LIMIT} characters`);
  if (script.length > DYNAMIC_CLIENT_SCRIPT_LIMIT) throw new Error(`clientCode.script exceeds ${DYNAMIC_CLIENT_SCRIPT_LIMIT} characters`);
  const minHeight = normalizeDynamicClientHeight(value.minHeight, 32, 1200);
  const maxHeight = normalizeDynamicClientHeight(value.maxHeight, minHeight ?? 32, 2400);
  if (minHeight !== undefined && maxHeight !== undefined && maxHeight < minHeight) {
    throw new Error("clientCode.maxHeight must be greater than or equal to minHeight");
  }
  return { html, css, script, ...(minHeight !== undefined ? { minHeight } : {}), ...(maxHeight !== undefined ? { maxHeight } : {}) };
}

function normalizeDynamicClientHeight(value: unknown, minimum: number, maximum: number): number | undefined {
  if (value === undefined || value === null) return undefined;
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric < minimum || numeric > maximum) {
    throw new Error(`dynamic client height must be between ${minimum} and ${maximum}`);
  }
  return Math.round(numeric);
}

function declarativeConversationNodeDefinition(
  projection: ConversationProjectionSpec,
): BrowserConversationNodeDefinition<Record<string, unknown>> {
  const startEvents = new Set(projection.startEvents ?? []);
  const endEvents = new Set(projection.endEvents ?? []);
  const eventTypes = new Set(projection.eventTypes);
  const maxEvents = projection.maxEvents;
  return {
    maxEvents,
    match(event) {
      if (!eventTypes.has(event.eventType)) return null;
      const keyValue = projection.keyPath ? readPath(event.payload, projection.keyPath) : event.id;
      const contextId = runtimeText(keyValue) || event.id;
      return {
        contextId,
        phase: startEvents.has(event.eventType) ? "start" : endEvents.has(event.eventType) ? "end" : "update",
      };
    },
    create(event) {
      return { lastPayload: event.payload, lastEventType: event.eventType };
    },
    update(state, event) {
      return { ...state, lastPayload: event.payload, lastEventType: event.eventType };
    },
    project(context) {
      let title = "";
      if (projection.titlePath) {
        for (let index = context.events.length - 1; index >= 0; index--) {
          title = runtimeText(readPath(context.events[index]!.payload, projection.titlePath));
          if (title) break;
        }
      }
      return {
        nodeType: projection.nodeType || "conversation_node",
        title: title || undefined,
        payload: context.lastEvent.payload,
      };
    },
  };
}

function readPath(value: unknown, path: string): unknown {
  let current = value;
  for (const segment of path.split(".")) {
    if (!current || typeof current !== "object" || Array.isArray(current)) return undefined;
    current = (current as Record<string, unknown>)[segment];
  }
  return current;
}

function runtimeText(value: unknown): string {
  return typeof value === "string" ? value.trim() : value == null ? "" : String(value);
}

function managedPackageToken(packageId: string, version: string): string {
  return `${packageId.length}:${packageId}${version.length}:${version}`;
}

function normalizeSessionRevision(value: unknown): number {
  const numeric = Number(value);
  if (!Number.isSafeInteger(numeric) || numeric < 0) return 0;
  return numeric;
}

export const browserClientPluginRuntime = new BrowserClientPluginRuntime();

export function syncBrowserClientSlots(snapshot: UIContributionSnapshot | null): Promise<void> {
  return browserClientPluginRuntime.slots.syncSnapshot(snapshot);
}

function fromSnapshot(slot: SlotSnapshot): ClientSlotDefinition {
  return normalizeDefinition({
    slotId: slot.slotId,
    contractVersion: slot.contractVersion,
    supportedKinds: slot.supportedKinds ?? [],
    kind: slot.kind,
    multiplicity: slot.multiplicity,
    layout: slot.layout,
    fallbackPolicy: slot.fallbackPolicy,
    parentSlotId: slot.parentSlotId,
    description: slot.description,
    scope: slot.scope ?? "root",
    ownerId: slot.ownerExtension,
    declarationEpoch: slot.declarationEpoch,
  });
}

function clientScopeCanContain(
  parent: NonNullable<ClientSlotDefinition["scope"]>,
  child: NonNullable<ClientSlotDefinition["scope"]>,
): boolean {
  if (parent === "session") return child === "session";
  if (parent === "session-maybe") return child === "session-maybe" || child === "session";
  return true;
}

function normalizeDefinition(definition: ClientSlotDefinition): ClientSlotDefinition {
  const slotId = definition.slotId?.trim();
  if (!slotId) throw new Error("slotId is required");
  const scope = definition.scope ?? "root";
  if (scope !== "root" && scope !== "session-maybe" && scope !== "session") {
    throw new Error(`client slot ${slotId} has invalid scope ${scope}`);
  }
  return {
    ...definition,
    slotId,
    contractVersion: definition.contractVersion || 1,
    supportedKinds: [...definition.supportedKinds],
    kind: definition.kind ?? clientKindFromMultiplicity(definition.multiplicity),
    multiplicity: definition.multiplicity ?? multiplicityFromClientKind(definition.kind ?? "list"),
    scope,
    declarationEpoch: definition.declarationEpoch && definition.declarationEpoch > 0 ? definition.declarationEpoch : undefined,
  };
}

function sameDefinition(a?: ClientSlotDefinition, b?: ClientSlotDefinition): boolean {
  return JSON.stringify(a ?? null) === JSON.stringify(b ?? null);
}

declare global {
  interface Window {
    amitiaClientPlugins?: BrowserClientPluginRuntime;
  }
}
