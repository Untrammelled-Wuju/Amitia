/**
 * Strongly typed UI composition contracts.
 *
 * Amitia intentionally keeps a superset of the DeepSeek Harness slot model:
 * the DSH-style typed composition chain is the core, while schema UI,
 * sandboxed web UI, provider profiles and legacy host surfaces remain valid
 * higher-level capabilities.
 */
export type UISlotScope = "root" | "session-maybe" | "session";
export type UISlotKind = "single" | "list" | "keyed" | "chain";

export interface UISlotObservable<T> {
  getSnapshot(): T;
  subscribe(listener: () => void): () => void;
}

/** Framework managed store handle. Components read state through useStore and
 * mutate only through actions; the raw state object is not part of the strict
 * component contract. Plain stores remain accepted for compatibility. */
/** Compatibility shared handle. Unlike managed handles this object carries one
 * concrete state instance, so the runtime pins it to one scope key at a time. */
export interface UISlotStoreHandle<State, Actions extends Record<string, (...args: any[]) => any> = Record<string, never>> {
  readonly state: State;
  readonly actions: Actions;
  readonly subscribe?: (listener: () => void) => () => void;
}

export type UISlotActionDecl<State> = Record<string, (draft: State, ...args: any[]) => void>;
export type UISlotBakedActions<
  State,
  Actions extends UISlotActionDecl<State>,
> = {
  readonly [K in keyof Actions]: Actions[K] extends (draft: State, ...args: infer Params) => unknown
    ? (...args: Params) => void
    : never;
};

export interface UISlotManagedStoreSpec<
  State,
  Actions extends UISlotActionDecl<State> = UISlotActionDecl<State>,
> {
  readonly init: () => State;
  /** Persistence namespace. Scope key is appended by the framework. */
  readonly persist?: string;
  readonly actions?: Actions;
}

export interface UISlotManagedStoreInstance<
  State,
  Actions extends UISlotActionDecl<State> = UISlotActionDecl<State>,
> {
  readonly __amitiaManagedSlotStoreInstance: true;
  readonly actions: Readonly<UISlotBakedActions<State, Actions>>;
  getSnapshot(): State;
  subscribe(listener: () => void): () => void;
  clearPersisted(): void;
  dispose(): void;
}

export interface UISlotManagedStoreHandle<
  State,
  Actions extends UISlotActionDecl<State> = UISlotActionDecl<State>,
> {
  readonly __amitiaManagedSlotStoreHandle: true;
  readonly spec: UISlotManagedStoreSpec<State, Actions>;
  create(scopeKey?: string): UISlotManagedStoreInstance<State, Actions>;
}

/** Framework-managed immutable-snapshot store. Each create(scopeKey) call gets
 * an independent instance; actions mutate a clone, then atomically publish it. */
export function defineUISlotStore<
  State,
  const Actions extends UISlotActionDecl<State>,
>(
  spec: UISlotManagedStoreSpec<State, Actions> & { readonly actions: Actions },
): UISlotManagedStoreHandle<State, Actions>;
export function defineUISlotStore<State>(
  spec: Omit<UISlotManagedStoreSpec<State, Record<string, never>>, "actions"> & { readonly actions?: undefined },
): UISlotManagedStoreHandle<State, Record<string, never>>;
export function defineUISlotStore<
  State,
  Actions extends UISlotActionDecl<State> = Record<string, never>,
>(
  spec: UISlotManagedStoreSpec<State, Actions>,
): UISlotManagedStoreHandle<State, Actions> {
  if (typeof spec.init !== "function") throw new Error("slot store init() is required");
  const frozenSpec = Object.freeze({ ...spec, actions: spec.actions ?? ({} as Actions) });
  return Object.freeze({
    __amitiaManagedSlotStoreHandle: true as const,
    spec: frozenSpec,
    create(scopeKey?: string) {
      const persistKey = frozenSpec.persist
        ? `amitia.slot-store:${frozenSpec.persist}${scopeKey ? `:${scopeKey}` : ""}`
        : undefined;
      let state = loadUISlotStoreState(persistKey, frozenSpec.init);
      let active = true;
      const listeners = new Set<() => void>();
      const baked: Record<string, (...args: any[]) => void> = {};
      for (const [name, mutate] of Object.entries(frozenSpec.actions ?? {})) {
        baked[name] = (...args: any[]) => {
          if (!active) throw new Error(`slot store ${frozenSpec.persist ?? "anonymous"} is disposed`);
          const draft = cloneUISlotStoreState(state);
          (mutate as (draft: State, ...params: any[]) => void)(draft, ...args);
          state = draft;
          if (persistKey) writeUISlotStoreState(persistKey, state);
          for (const listener of Array.from(listeners)) listener();
        };
      }
      return {
        __amitiaManagedSlotStoreInstance: true as const,
        actions: Object.freeze(baked) as Readonly<UISlotBakedActions<State, Actions>>,
        getSnapshot: () => state,
        subscribe(listener: () => void) {
          if (!active) return () => undefined;
          listeners.add(listener);
          return () => listeners.delete(listener);
        },
        clearPersisted() {
          if (!persistKey) return;
          try { globalThis.localStorage?.removeItem(persistKey); } catch {}
        },
        dispose() {
          if (!active) return;
          active = false;
          listeners.clear();
        },
      };
    },
  });
}

function cloneUISlotStoreState<State>(state: State): State {
  if (typeof globalThis.structuredClone === "function") return globalThis.structuredClone(state);
  if (state == null || typeof state !== "object") return state;
  return JSON.parse(JSON.stringify(state)) as State;
}

function loadUISlotStoreState<State>(persistKey: string | undefined, init: () => State): State {
  if (persistKey) {
    try {
      const value = globalThis.localStorage?.getItem(persistKey);
      if (value != null) return JSON.parse(value) as State;
    } catch {}
  }
  return cloneUISlotStoreState(init());
}

function writeUISlotStoreState<State>(persistKey: string, state: State): void {
  try { globalThis.localStorage?.setItem(persistKey, JSON.stringify(state)); } catch {}
}

export interface UISlotContract<
  Owner extends Record<string, unknown> = Record<string, unknown>,
  Store = unknown,
  Injected extends Record<string, unknown> = Record<string, unknown>,
  Scope extends UISlotScope = UISlotScope,
  Kind extends UISlotKind = UISlotKind,
  KeyProps extends Record<string, Record<string, unknown>> = Record<string, Record<string, unknown>>,
  Matched = unknown,
  HookContext = never,
> {
  owner: Owner;
  store: Store;
  /** Common slot-level inject face declared by the parent child spec. */
  inject: Injected;
  scope: Scope;
  kind: Kind;
  keyProps: KeyProps;
  matched: Matched;
  hookContext: HookContext;
}

/**
 * Declaration-merging authority for UI slots. Host-provided entries are
 * compatibility surfaces; plugins may add their own keys without modifying
 * the host package.
 */
type UIRootList = UISlotContract<Record<string, unknown>, unknown, Record<string, unknown>, "root", "list">;
type UIRootChain = UISlotContract<Record<string, unknown>, unknown, Record<string, unknown>, "root", "chain">;
type UISessionList = UISlotContract<Record<string, unknown>, unknown, Record<string, unknown>, "session", "list">;
type UISessionChain = UISlotContract<Record<string, unknown>, unknown, Record<string, unknown>, "session", "chain">;
type UISessionMaybeList = UISlotContract<Record<string, unknown>, unknown, Record<string, unknown>, "session-maybe", "list">;
type UISessionMaybeChain = UISlotContract<Record<string, unknown>, unknown, Record<string, unknown>, "session-maybe", "chain">;
type UIRootKeyed = UISlotContract<Record<string, unknown>, unknown, Record<string, unknown>, "root", "keyed", Record<string, Record<string, unknown>>>;

export interface UISlotMap {
  root: UISlotContract<Record<string, unknown>, unknown, Record<string, unknown>, "root", "single">;
  "extension.center.header.action": UIRootList;
  "extension.center.card.badge": UIRootList;
  "extension.detail.tab": UIRootList;
  "extension.detail.action": UIRootList;
  "extension.settings.page": UIRootList;
  "extension.settings.section": UIRootList;
  "chat.header.action": UISessionMaybeList;
  "chat.sidebar.panel": UISessionMaybeList;
  "chat.message.action": UISessionList;
  "chat.message.renderer": UISessionChain;
  "chat.conversation.node": UISessionList;
  "chat.message.custom_renderer": UISessionChain;
  "chat.message.attachment_renderer": UISessionChain;
  "chat.message.badge": UISessionList;
  "chat.composer.action": UISessionMaybeList;
  "chat.composer.attachment": UISessionMaybeList;
  "chat.composer.hint": UISessionMaybeList;
  "chat.empty_state.card": UISessionMaybeList;
  "chat.status.item": UISessionMaybeList;
  "character.detail.tab": UIRootList;
  "character.detail.action": UIRootList;
  "character.sidebar.card": UIRootList;
  "system.status.item": UIRootList;
  "system.settings.section": UIRootList;
  "system.diagnostics.tab": UIRootList;
  "desktop.command": UIRootKeyed;
  "desktop.application_menu.item": UIRootList;
  "desktop.context_menu.item": UIRootList;
  "desktop.tray.item": UIRootList;
  "desktop.window.page": UIRootKeyed;
  "provider.app.shell": UIRootChain;
  "provider.app.navigation": UIRootChain;
  "provider.app.workspace": UIRootChain;
  "provider.route.registry": UIRootList;
  "provider.page.provider": UIRootChain;
  "provider.conversation.shell": UISessionMaybeChain;
  "provider.conversation.header": UISessionMaybeChain;
  "provider.conversation.messages": UISessionMaybeChain;
  "provider.conversation.message_renderer": UISessionChain;
  "provider.conversation.sidebar": UISessionMaybeChain;
  "provider.conversation.composer": UISessionMaybeChain;
  "provider.conversation.overlay": UISessionMaybeList;
  "provider.character.shell": UIRootChain;
  "provider.character.detail": UIRootChain;
  "provider.memory.shell": UIRootChain;
  "provider.memory.detail": UIRootChain;
  "provider.settings.shell": UIRootChain;
  "provider.settings.section": UIRootList;
  "provider.extension.center": UIRootChain;
  "provider.extension.page": UIRootChain;
  "provider.ui.theme": UIRootChain;
  "provider.ui.tokens": UIRootChain;
  "provider.ui.icons": UIRootChain;
  "provider.ui.components": UIRootChain;
}

export interface UILocaleNamespaceMap {}
export type UITranslate<K extends string = string> = (key: K, params?: Record<string, unknown>) => string;
export type UICommonLocaleKey = UILocaleNamespaceMap extends { common: infer C } ? C & string : never;
export type UILocaleKeysOf<N extends keyof UILocaleNamespaceMap & string> = (UILocaleNamespaceMap[N] & string) | UICommonLocaleKey;
export type UISlotLocaleProps<N> = N extends keyof UILocaleNamespaceMap & string ? { readonly t: UITranslate<UILocaleKeysOf<N>> } : object;

export type UIKnownSlotId = keyof UISlotMap & string;
export type UISlotProps<K extends UIKnownSlotId> = UISlotMap[K]["owner"];
export type UISlotStore<K extends UIKnownSlotId> = UISlotMap[K]["store"];
export type UISlotInjected<K extends UIKnownSlotId> = UISlotMap[K]["inject"];
export type UISlotScopeOf<K extends UIKnownSlotId> = UISlotMap[K]["scope"];
export type UISlotKindOf<K extends UIKnownSlotId> = UISlotMap[K]["kind"];
export type UISlotMatched<K extends UIKnownSlotId> = UISlotMap[K]["matched"];
export type UISlotHookContext<K extends UIKnownSlotId> = UISlotMap[K]["hookContext"];
export type UISlotEntryKey<K extends UIKnownSlotId> = UISlotMap[K]["keyProps"] extends infer P extends Record<string, unknown>
  ? keyof P & string
  : string;
export type UISlotKeyProps<
  K extends UIKnownSlotId,
  EntryKey extends UISlotEntryKey<K> = UISlotEntryKey<K>,
> = UISlotKindOf<K> extends "keyed"
  ? UISlotMap[K]["keyProps"] extends Record<string, Record<string, unknown>>
    ? EntryKey extends keyof UISlotMap[K]["keyProps"]
      ? UISlotMap[K]["keyProps"][EntryKey] extends Record<string, unknown> ? UISlotMap[K]["keyProps"][EntryKey] : object
      : object
    : object
  : object;

export interface UISlotRuntimeSpec<K extends UIKnownSlotId = UIKnownSlotId> {
  readonly kind: UISlotKindOf<K>;
  readonly scope: UISlotScopeOf<K>;
  readonly supportedKinds?: readonly string[];
  readonly layout?: "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal" | "hidden";
  readonly fallbackPolicy?: "none" | "skeleton" | "empty" | "default";
  readonly description?: string;
  readonly contractVersion?: number;
  readonly multiplicity?: "single" | "multiple" | "ordered_multiple" | "replaceable_single" | "exclusive";
  /** Optional explicit key domain metadata for runtime inspection. */
  readonly keys?: readonly UISlotEntryKey<K>[];
  /** Common slot-level inject face supplied by the declaring parent. */
  readonly inject?: UISlotInjected<K>;
}

export type UIChildrenDecl = Partial<{ [K in UIKnownSlotId]: UISlotRuntimeSpec<K> }>;
export type UIChildKeys<D extends UIChildrenDecl> = keyof D & UIKnownSlotId;
export type UIChainChildKeys<D extends UIChildrenDecl> = {
  [K in UIChildKeys<D>]: D[K] extends { kind: "chain" } ? K : never
}[UIChildKeys<D>];
export type UINonChainChildKeys<D extends UIChildrenDecl> = Exclude<UIChildKeys<D>, UIChainChildKeys<D>>;

export type UISlotRenderOptions<
  K extends UIKnownSlotId = UIKnownSlotId,
  EntryKey extends UISlotEntryKey<K> = UISlotEntryKey<K>,
> = {
  /** DSH-compatible keyed dispatch selector. */
  readonly entryKey?: EntryKey;
  /** Amitia compatibility alias for entryKey. */
  readonly key?: EntryKey;
  /** List cell filter (DSH `only`). */
  readonly only?: string;
  /** Renderer-specific fallback payload. */
  readonly fallback?: unknown;
};

type UISlotRenderArgs<K extends UIKnownSlotId, EntryKey extends UISlotEntryKey<K>> =
  [UISlotHookContext<K>] extends [never]
    ? [options?: UISlotRenderOptions<K, EntryKey>]
    : [options: UISlotRenderOptions<K, EntryKey> & { readonly hookContext: UISlotHookContext<K> }];

type UISlotChainRenderOptions<K extends UIKnownSlotId> = {
  readonly fallback?: unknown;
  readonly overlay?: boolean;
} & ([UISlotHookContext<K>] extends [never] ? object : { readonly hookContext: UISlotHookContext<K> });

type UISlotChainRenderArgs<K extends UIKnownSlotId> =
  [UISlotHookContext<K>] extends [never]
    ? [options?: UISlotChainRenderOptions<K>]
    : [options: UISlotChainRenderOptions<K>];

type UISessionChildKeys<D extends UIChildrenDecl> = {
  [K in UIChildKeys<D>]: D[K] extends { scope: "session" } ? K : never
}[UIChildKeys<D>];

export type UISlotRenderAuthority<D extends UIChildrenDecl> = {
  renderSlot<
    K extends UINonChainChildKeys<D>,
    EntryKey extends UISlotEntryKey<K> = UISlotEntryKey<K>,
  >(
    name: K,
    owner: UISlotProps<K> & UISlotKeyProps<K, EntryKey>,
    ...args: UISlotRenderArgs<K, EntryKey>
  ): unknown;
  readonly __renders?: ((name: UIChildKeys<D>) => void) | undefined;
} & ([UIChainChildKeys<D>] extends [never] ? object : {
  renderSlotChain<K extends UIChainChildKeys<D>>(
    name: K,
    owner: UISlotProps<K>,
    ...args: UISlotChainRenderArgs<K>
  ): unknown;
}) & ([UISessionChildKeys<D>] extends [never] ? object : {
  readonly SessionProvider: (props: UISessionAreaProps) => unknown;
});

export interface UISlotStoreContext {
  readonly pluginId: string;
  readonly slotId: string;
  readonly key: string;
  readonly scope: UISlotScope;
  /** Present for session/session-maybe instances mounted in a session tree. */
  readonly sessionId?: string;
}

export interface UISlotStoreResource<T, Actions extends Record<string, (...args: any[]) => any> = Record<string, never>> {
  readonly __amitiaSlotStoreResource: true;
  readonly store: T;
  readonly actions?: Actions;
  readonly subscribe?: (listener: () => void) => () => void;
  dispose?(): void | Promise<void>;
}

export function uiSlotStoreResource<T, Actions extends Record<string, (...args: any[]) => any> = Record<string, never>>(
  store: T,
  options?: (() => void | Promise<void>) | {
    readonly actions?: Actions;
    readonly subscribe?: (listener: () => void) => () => void;
    readonly dispose?: () => void | Promise<void>;
  },
): UISlotStoreResource<T, Actions> {
  const normalized = typeof options === "function" ? { dispose: options } : options ?? {};
  return {
    __amitiaSlotStoreResource: true,
    store,
    ...(normalized.actions ? { actions: normalized.actions } : {}),
    ...(normalized.subscribe ? { subscribe: normalized.subscribe } : {}),
    ...(normalized.dispose ? { dispose: normalized.dispose } : {}),
  };
}

export type UISlotStoreFactory<T> = (context: UISlotStoreContext) => T | UISlotStoreResource<T, any>;

export interface UISlotServiceReader {
  get<T>(serviceId: string): T | undefined;
  list(): string[];
}

export interface UISlotEventWriter {
  emit<T>(eventType: string, payload: T): Promise<void>;
}

export interface UISlotInjectContext<TStore> extends UISlotStoreContext {
  /** Runtime state is available only to inject; managed/shared store handles are unwrapped. */
  readonly store: StoreState<TStore>;
  readonly actions: Readonly<StoreActions<TStore>>;
  readonly services: UISlotServiceReader;
  readonly events: UISlotEventWriter;
}

export type UISlotInjectFactory<TStore, TInjected extends Record<string, unknown>> = (
  context: UISlotInjectContext<TStore>,
) => TInjected;

export type UISlotHookSource = UISlotObservable<unknown> | ((standard: Record<string, unknown>, hookContext: unknown) => (...args: any[]) => unknown);
export type UISlotHookSources = Record<string, UISlotHookSource>;
export interface UISlotReadonlyValue<T> { readonly value: T }
type BoundHookName<K extends string> = `use${Capitalize<K>}`;
type BindHookSources<H> = H extends Record<string, unknown> ? {
  [K in keyof H & string as BoundHookName<K>]: H[K] extends UISlotObservable<infer T>
    ? <Selected = T>(selector?: (value: T) => Selected) => UISlotReadonlyValue<Selected>
    : H[K] extends (...args: any[]) => infer Hook
      ? Hook extends (...args: any[]) => unknown ? Hook : never
      : never
} : object;
export type UISlotInjectFace<I extends Record<string, unknown>> = Omit<I, "hooks"> & (
  I extends { hooks: infer H } ? BindHookSources<H> : object
);

type StoreState<S> = S extends UISlotManagedStoreHandle<infer State, any> ? State
  : S extends UISlotStoreHandle<infer State, any> ? State
    : S;
type StoreActions<S> = S extends UISlotManagedStoreHandle<infer State, infer Actions> ? UISlotBakedActions<State, Actions>
  : S extends UISlotStoreHandle<any, infer Actions> ? Actions
    : Record<string, (...args: any[]) => any>;

export type UISlotStoreProps<S> = {
  useStore<Selected = StoreState<S>>(selector?: (state: StoreState<S>) => Selected): UISlotReadonlyValue<Selected>;
  readonly actions: Readonly<StoreActions<S>>;
};

export type UISlotMatchedProps<K extends UIKnownSlotId> = UISlotKindOf<K> extends "chain" ? { readonly matched: UISlotMatched<K> } : object;

export type UISlotSelectorHook<T> = <Selected = T>(selector?: (value: T) => Selected) => UISlotReadonlyValue<Selected>;

export interface UISessionAreaProps {
  readonly empty?: () => unknown;
  readonly children: (sessionId: string) => unknown;
}

export type UISlotStandardProps<K extends UIKnownSlotId> = {
  readonly slotId: K;
  readonly pluginId: string;
  readonly contributionId: string;
  /** Amitia global object-layer selector seat; equivalent in capability to DSH's global standard kit. */
  readonly useGlobal: UISlotSelectorHook<Record<string, unknown>>;
} & (UISlotScopeOf<K> extends "session"
  ? { readonly sessionId: string; readonly useSession: UISlotSelectorHook<Record<string, unknown> | undefined> }
  : UISlotScopeOf<K> extends "session-maybe"
    ? { readonly sessionId?: string; readonly useSession: UISlotSelectorHook<Record<string, unknown> | undefined> }
    : { readonly sessionId?: string; readonly useSession: UISlotSelectorHook<Record<string, unknown> | undefined> });

export type UISlotComponentProps<
  K extends UIKnownSlotId,
  D extends UIChildrenDecl = {},
  S = UISlotStore<K>,
  I extends Record<string, unknown> = UISlotInjected<K>,
  N extends keyof UILocaleNamespaceMap & string | undefined = undefined,
  EntryKey extends UISlotEntryKey<K> = UISlotEntryKey<K>,
> = UISlotProps<K>
  & UISlotKeyProps<K, NoInfer<EntryKey>>
  & UISlotStandardProps<K>
  & UISlotRenderAuthority<D>
  & UISlotStoreProps<S>
  & UISlotInjectFace<UISlotInjected<K>>
  & UISlotInjectFace<I>
  & UISlotMatchedProps<K>
  & UISlotLocaleProps<N>;

export type UISlotKindRegisterOptions<K extends UIKnownSlotId, EntryKey extends UISlotEntryKey<K> = UISlotEntryKey<K>> =
  UISlotKindOf<K> extends "keyed" ? { readonly entryKey: EntryKey; readonly priority?: number }
    : UISlotKindOf<K> extends "list" ? { readonly id: string; readonly order?: number; readonly label?: string | (() => string); readonly priority?: number }
      : UISlotKindOf<K> extends "chain" ? { readonly select: (owner: UISlotProps<K>) => UISlotMatched<K> | null; readonly priority?: number }
        : { readonly priority?: number };

export type UISlotRegisterOptions<
  K extends UIKnownSlotId,
  D extends UIChildrenDecl = {},
  S = UISlotStore<K>,
  I extends Record<string, unknown> = UISlotInjected<K>,
  N extends keyof UILocaleNamespaceMap & string | undefined = undefined,
  EntryKey extends UISlotEntryKey<K> = UISlotEntryKey<K>,
> = {
  readonly name: K;
  /** Stable registration identity; independent from keyed/list dispatch cells. */
  readonly key: string;
  readonly ordering?: number;
  readonly ownerDefaults?: Partial<UISlotProps<K>>;
  readonly children?: D;
  readonly store?: S | UISlotStoreFactory<S>;
  readonly inject?: UISlotInjectFactory<S, I>;
  readonly locale?: N;
  readonly registrant?: string;
} & UISlotKindRegisterOptions<K, EntryKey>;

export type UISlotComponent<P> = (props: P) => unknown;

export type UISlotRendersCheck<C, D extends UIChildrenDecl> =
  [UIChildKeys<D>] extends [never] ? unknown
    : C extends (props: infer P) => unknown
      ? "renderSlot" extends keyof P ? unknown
        : "renderSlotChain" extends keyof P ? unknown
          : { readonly "children declared but component consumes no renderSlot": UIChildKeys<D> }
      : unknown;

export type UISlotRegistrationComponent<
  K extends UIKnownSlotId,
  D extends UIChildrenDecl,
  S,
  I extends Record<string, unknown>,
  N extends keyof UILocaleNamespaceMap & string | undefined,
  EntryKey extends UISlotEntryKey<K>,
  C,
> = C & UISlotComponent<UISlotComponentProps<K, D, S, I, N, EntryKey>> & UISlotRendersCheck<C, D>;

/** Compatibility shape for manifests/declarative packages. New trusted plugins
 * should use UISlotRegisterOptions + slots.register(). */
export type TypedUISlotEntryDefinition<K extends UIKnownSlotId, TRenderable = unknown> = UISlotRegisterOptions<K> & {
  readonly renderable: TRenderable;
};

export function defineUISlotContractEntry<K extends UIKnownSlotId, TRenderable = unknown>(
  entry: TypedUISlotEntryDefinition<K, TRenderable>,
): TypedUISlotEntryDefinition<K, TRenderable> {
  return entry;
}
