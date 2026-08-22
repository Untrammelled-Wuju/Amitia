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
  ClientKnownSlotId,
  ClientSlotEntryDefinition,
  ClientSlotInjectFactory,
  ClientSlotInjected,
  ClientSlotProps,
  ClientSlotStore,
  ClientSlotStoreFactory,
  ClientSlotStoreResource,
} from "@/ui-runtime/slotContract";

export type ClientDisposer = () => void | Promise<void>;
export type ClientSlotEffect = void | ClientDisposer | readonly ClientDisposer[] | Iterable<ClientDisposer> | Promise<void | ClientDisposer | readonly ClientDisposer[] | Iterable<ClientDisposer>>;

export interface ClientSlotDefinition {
  slotId: string;
  contractVersion: number;
  supportedKinds: string[];
  multiplicity?: SlotSnapshot["multiplicity"];
  layout?: SlotSnapshot["layout"];
  fallbackPolicy?: SlotSnapshot["fallbackPolicy"];
  parentSlotId?: string;
  description?: string;
  scope?: "root" | "session-maybe" | "session";
  declarationEpoch?: number;
  ownerId?: string;
}

export interface ClientSlotRegistration {
  definition: ClientSlotDefinition;
  dispose(): void | Promise<void>;
}

export interface ClientSlotContributionOptions<
  TProps extends Record<string, unknown> = Record<string, unknown>,
  TStore = unknown,
  TInjected extends Record<string, unknown> = Record<string, unknown>,
> {
  ordering?: number;
  priority?: number;
  props?: TProps;
  children?: readonly ClientSlotDefinition[];
  /** Stable state owned by this contribution. It is disposed with the contribution/fiber. */
  store?: TStore | ClientSlotStoreFactory<TStore>;
  /** Business capability face injected into this contribution only. */
  inject?: ClientSlotInjectFactory<TStore, TInjected>;
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
  store?: unknown;
  injected?: Record<string, unknown>;
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

interface SlotSubscriber {
  callback: (definition: ClientSlotDefinition) => ClientSlotEffect;
  cleanup?: ClientDisposer;
}

export class BrowserSlotRuntime {
  private readonly local = new Map<string, ClientSlotDefinition>();
  private readonly server = new Map<string, ClientSlotDefinition>();
  private readonly epochs = new Map<string, number>();
  private readonly subscribers = new Map<string, Set<SlotSubscriber>>();
  private readonly contributions = new Map<string, Map<string, ClientSlotContribution>>();
  readonly revision = shallowRef(0);

  async syncSnapshot(snapshot: UIContributionSnapshot | null): Promise<void> {
    const next = new Map<string, ClientSlotDefinition>();
    for (const slot of snapshot?.slots ?? []) next.set(slot.slotId, fromSnapshot(slot));
    const ids = new Set([...this.server.keys(), ...next.keys()]);
    for (const id of ids) {
      const previous = this.server.get(id);
      const incoming = next.get(id);
      if (sameDefinition(previous, incoming)) continue;
      if (previous && !this.local.has(id)) await this.deactivate(id);
      if (incoming) this.server.set(id, incoming); else this.server.delete(id);
      this.bumpRevision();
      if (incoming && !this.local.has(id)) await this.activate(id, incoming);
    }
  }

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
    if (this.current(id)) await this.deactivate(id);
    const epoch = Math.max(this.epochs.get(id) ?? 0, normalized.declarationEpoch ?? 0) + 1;
    this.epochs.set(id, epoch);
    const definition = { ...normalized, declarationEpoch: epoch };
    this.local.set(id, definition);
    this.bumpRevision();
    await this.activate(id, definition);
    return {
      definition,
      dispose: async () => {
        if (this.epochs.get(id) !== epoch || !this.local.has(id)) return;
        await this.deactivate(id);
        this.local.delete(id);
        this.bumpRevision();
        const fallback = this.server.get(id);
        if (fallback) await this.activate(id, fallback);
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

  /** @deprecated Use observe(). Business injection now belongs to register({ inject }). */
  inject(slotId: string, callback: (definition: ClientSlotDefinition) => ClientSlotEffect): Promise<ClientDisposer> {
    return this.observe(slotId, callback);
  }

  register<SlotId extends ClientKnownSlotId>(
    pluginId: string,
    slotId: SlotId,
    key: string,
    component: Component,
    options?: ClientSlotContributionOptions<ClientSlotProps<SlotId>, ClientSlotStore<SlotId>, ClientSlotInjected<SlotId>>,
    environment?: ClientSlotRegistrationEnvironment,
  ): ClientDisposer;
  register(
    pluginId: string,
    slotId: string,
    key: string,
    component: Component,
    options?: ClientSlotContributionOptions,
    environment?: ClientSlotRegistrationEnvironment,
  ): ClientDisposer;
  register<SlotId extends string>(
    pluginId: string,
    entry: ClientSlotEntryDefinition<SlotId> & { children?: readonly ClientSlotDefinition[] },
    environment?: ClientSlotRegistrationEnvironment,
  ): ClientDisposer;
  register(
    pluginId: string,
    slotOrEntry: string | (ClientSlotEntryDefinition<string> & { children?: readonly ClientSlotDefinition[] }),
    keyOrEnvironment?: string | ClientSlotRegistrationEnvironment,
    component?: Component,
    options: ClientSlotContributionOptions = {},
    legacyEnvironment?: ClientSlotRegistrationEnvironment,
  ): ClientDisposer {
    const normalizedPluginId = pluginId.trim();
    const entryMode = typeof slotOrEntry !== "string";
    const slotId = entryMode ? slotOrEntry.slotId : slotOrEntry;
    const key = entryMode ? slotOrEntry.key : String(keyOrEnvironment ?? "");
    const resolvedComponent = entryMode ? slotOrEntry.component : component;
    const resolvedOptions: ClientSlotContributionOptions = entryMode
      ? {
          ordering: slotOrEntry.ordering,
          priority: slotOrEntry.priority,
          props: slotOrEntry.props,
          children: slotOrEntry.children,
          store: slotOrEntry.store,
          inject: slotOrEntry.inject,
        }
      : options;
    const environment = entryMode
      ? (keyOrEnvironment && typeof keyOrEnvironment !== "string" ? keyOrEnvironment : undefined)
      : legacyEnvironment;
    const normalizedSlotId = slotId.trim();
    const normalizedKey = key.trim();
    if (!normalizedPluginId) throw new Error("pluginId is required");
    if (!normalizedSlotId) throw new Error("slotId is required");
    if (!normalizedKey) throw new Error("client slot contribution key is required");
    if (!resolvedComponent) throw new Error("client slot contribution component is required");
    if (!this.current(normalizedSlotId)) throw new Error(`client slot ${normalizedSlotId} is not declared`);
    const parentDefinition = this.current(normalizedSlotId);
    const childInputs = (resolvedOptions.children ?? []).map((child) => normalizeDefinition({
      ...child,
      parentSlotId: normalizedSlotId,
      scope: child.scope ?? parentDefinition?.scope,
    }));
    const childIds = new Set<string>();
    for (const child of childInputs) {
      if (child.slotId === normalizedSlotId) throw new Error(`child slot ${child.slotId} cannot equal its parent`);
      if (childIds.has(child.slotId)) throw new Error(`duplicate child slot ${child.slotId}`);
      if (this.current(child.slotId)) throw new Error(`child slot ${child.slotId} is already declared`);
      if (parentDefinition && !clientScopeCanContain(parentDefinition.scope ?? "root", child.scope ?? parentDefinition.scope ?? "root")) {
        throw new Error(`child slot ${child.slotId} cannot escape parent scope ${parentDefinition.scope}`);
      }
      childIds.add(child.slotId);
    }
    const mapKey = `${normalizedPluginId}:${normalizedKey}`;
    let slotContributions = this.contributions.get(normalizedSlotId);
    if (!slotContributions) {
      slotContributions = new Map();
      this.contributions.set(normalizedSlotId, slotContributions);
    }
    if (slotContributions.has(mapKey)) {
      throw new Error(`client slot contribution ${normalizedSlotId}/${mapKey} already registered`);
    }
    const resourceDisposers: ClientDisposer[] = [];
    const resourceContext = {
      pluginId: normalizedPluginId,
      slotId: normalizedSlotId,
      key: normalizedKey,
      scope: parentDefinition?.scope ?? "root",
    } as const;
    let store: unknown;
    if (typeof resolvedOptions.store === "function") {
      const created = (resolvedOptions.store as ClientSlotStoreFactory<unknown>)(resourceContext);
      if (isClientSlotStoreResource(created)) {
        store = created.store;
        if (created.dispose) resourceDisposers.push(() => created.dispose?.());
      } else {
        store = created;
      }
    } else {
      store = resolvedOptions.store;
    }
    let injected: Record<string, unknown> | undefined;
    try {
      if (resolvedOptions.inject) {
        injected = (resolvedOptions.inject as ClientSlotInjectFactory<unknown, Record<string, unknown>>)({
          ...resourceContext,
          store,
          services: environment?.services ?? { get: () => undefined, list: () => [] },
          events: environment?.events ?? { emit: async () => undefined },
        });
      }
    } catch (error) {
      for (let index = resourceDisposers.length - 1; index >= 0; index--) void resourceDisposers[index]?.();
      throw error;
    }
    const contribution: ClientSlotContribution = {
      contributionId: `client:${normalizedPluginId}:${normalizedSlotId}:${normalizedKey}`,
      pluginId: normalizedPluginId,
      key: normalizedKey,
      slotId: normalizedSlotId,
      component: markRaw(resolvedComponent),
      ordering: Number.isFinite(resolvedOptions.ordering) ? Number(resolvedOptions.ordering) : 0,
      priority: Number.isFinite(resolvedOptions.priority) ? Number(resolvedOptions.priority) : 0,
      props: resolvedOptions.props ? { ...resolvedOptions.props } : undefined,
      store,
      injected,
    };
    slotContributions.set(mapKey, contribution);
    const childEpochs: Array<{ slotId: string; epoch: number }> = [];
    const childActivations: Promise<unknown>[] = [];
    for (const child of childInputs) {
      const epoch = Math.max(this.epochs.get(child.slotId) ?? 0, child.declarationEpoch ?? 0) + 1;
      this.epochs.set(child.slotId, epoch);
      const definition = { ...child, ownerId: normalizedPluginId, declarationEpoch: epoch };
      this.local.set(child.slotId, definition);
      childEpochs.push({ slotId: child.slotId, epoch });
      this.bumpRevision();
      const activation = this.activate(child.slotId, definition);
      childActivations.push(activation.catch(() => undefined));
      activation.catch(async () => {
        if (this.epochs.get(child.slotId) !== epoch) return;
        await this.deactivate(child.slotId);
        this.local.delete(child.slotId);
        this.bumpRevision();
      });
    }
    this.bumpRevision();
    let active = true;
    return async () => {
      if (!active) return;
      active = false;
      await Promise.allSettled(childActivations);
      for (let index = childEpochs.length - 1; index >= 0; index--) {
        const child = childEpochs[index]!;
        if (this.epochs.get(child.slotId) !== child.epoch || !this.local.has(child.slotId)) continue;
        await this.deactivate(child.slotId);
        this.local.delete(child.slotId);
        this.bumpRevision();
        const fallback = this.server.get(child.slotId);
        if (fallback) await this.activate(child.slotId, fallback);
      }
      const current = this.contributions.get(normalizedSlotId);
      current?.delete(mapKey);
      if (current?.size === 0) this.contributions.delete(normalizedSlotId);
      for (let index = resourceDisposers.length - 1; index >= 0; index--) await resourceDisposers[index]?.();
      this.bumpRevision();
    };
  }

  list(): ClientSlotDefinition[] {
    this.revision.value;
    const values = new Map(this.server);
    for (const [id, definition] of this.local) values.set(id, definition);
    return Array.from(values.values()).sort((a, b) => a.slotId.localeCompare(b.slotId));
  }

  getDefinition(slotId: string): ClientSlotDefinition | undefined {
    this.revision.value;
    return this.current(slotId);
  }

  listContributions(slotId: string): ClientSlotContribution[] {
    this.revision.value;
    if (!this.current(slotId)) return [];
    return Array.from(this.contributions.get(slotId)?.values() ?? [])
      .sort((a, b) => a.ordering - b.ordering || a.contributionId.localeCompare(b.contributionId));
  }

  inspect() {
    return this.list().map((definition) => ({
      ...definition,
      contributions: this.listContributions(definition.slotId).map((entry) => ({
        contributionId: entry.contributionId,
        pluginId: entry.pluginId,
        key: entry.key,
        ordering: entry.ordering,
        priority: entry.priority,
      })),
    }));
  }

  private current(id: string): ClientSlotDefinition | undefined {
    return this.local.get(id) ?? this.server.get(id);
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
    for (let index = disposers.length - 1; index >= 0; index--) {
      await disposers[index]?.();
    }
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
  slots: {
    declare(definition: ClientSlotDefinition): Promise<ClientSlotRegistration>;
    observe(slotId: string, callback: (definition: ClientSlotDefinition) => ClientSlotEffect): Promise<ClientDisposer>;
    /** @deprecated Use observe(). Business injection is configured on register(). */
    inject(slotId: string, callback: (definition: ClientSlotDefinition) => ClientSlotEffect): Promise<ClientDisposer>;
    register<SlotId extends ClientKnownSlotId>(
      slotId: SlotId,
      key: string,
      component: Component,
      options?: ClientSlotContributionOptions<ClientSlotProps<SlotId>, ClientSlotStore<SlotId>, ClientSlotInjected<SlotId>>,
    ): ClientDisposer;
    register(
      slotId: string,
      key: string,
      component: Component,
      options?: ClientSlotContributionOptions,
    ): ClientDisposer;
    register<SlotId extends string>(
      entry: ClientSlotEntryDefinition<SlotId> & { children?: readonly ClientSlotDefinition[] },
    ): ClientDisposer;
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

  setActiveConversationScope(conversationId: string): void {
    const scope = conversationId.trim();
    if (scope === this.activeConversationScope) return;
    this.activeConversationScope = scope;
    this.activeConversationScopeGeneration += 1;
  }

  async activateConversationScope(conversationId: string): Promise<number> {
    const scope = conversationId.trim();
    if (scope !== this.activeConversationScope) {
      this.activeConversationScope = scope;
      this.activeConversationScopeGeneration += 1;
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
          await ctx.slots.observe(contribution.slotId, () => ctx.slots.register(
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
          const dispose = await this.slots.observe(slotId, callback);
          fiber.own(dispose);
          return dispose;
        },
        register: ((slotOrEntry: string | (ClientSlotEntryDefinition<string> & { children?: readonly ClientSlotDefinition[] }), key?: string, component?: Component, options?: ClientSlotContributionOptions) => {
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
          const dispose = typeof slotOrEntry === "string"
            ? this.slots.register(pluginId, slotOrEntry as ClientKnownSlotId, key ?? "", component!, options as ClientSlotContributionOptions, environment)
            : this.slots.register(pluginId, slotOrEntry, environment);
          return fiber.own(dispose);
        }) as BrowserClientPluginContext["slots"]["register"],
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
          const disposeView = this.slots.register(
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
