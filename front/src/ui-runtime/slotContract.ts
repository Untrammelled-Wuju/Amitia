import type { Component } from "vue";

export type ClientSlotScope = "root" | "session-maybe" | "session";
export type ClientSlotKind = "single" | "list" | "keyed" | "chain";

export interface ClientObservable<T> {
  getSnapshot(): T;
  subscribe(listener: () => void): () => void;
}

/** Declaration-merging locale namespace map for trusted browser plugins. */
export interface ClientLocaleNamespaceMap {}
export type ClientTranslate<K extends string = string> = (key: K, params?: Record<string, unknown>) => string;
export type ClientCommonLocaleKey = ClientLocaleNamespaceMap extends { common: infer C } ? C & string : never;
export type ClientLocaleKeysOf<N extends keyof ClientLocaleNamespaceMap & string> = (ClientLocaleNamespaceMap[N] & string) | ClientCommonLocaleKey;
export type ClientSlotLocaleProps<N> = N extends keyof ClientLocaleNamespaceMap & string
  ? { readonly t: ClientTranslate<ClientLocaleKeysOf<N>> }
  : object;

/** Compatibility shared handle. Unlike managed handles this object carries one
 * concrete state instance, so the runtime pins it to one scope key at a time. */
export interface ClientSlotStoreHandle<State, Actions extends Record<string, (...args: any[]) => any> = Record<string, never>> {
  readonly state: State;
  readonly actions: Actions;
  readonly subscribe?: (listener: () => void) => () => void;
}

export type ClientSlotActionDecl<State> = Record<string, (draft: State, ...args: any[]) => void>;
export type ClientSlotBakedActions<
  State,
  Actions extends ClientSlotActionDecl<State>,
> = {
  readonly [K in keyof Actions]: Actions[K] extends (draft: State, ...args: infer Params) => unknown
    ? (...args: Params) => void
    : never;
};

export interface ClientSlotManagedStoreSpec<
  State,
  Actions extends ClientSlotActionDecl<State> = ClientSlotActionDecl<State>,
> {
  readonly init: () => State;
  /** Persistence namespace. Scope key is appended by the framework. */
  readonly persist?: string;
  readonly actions?: Actions;
}

export interface ClientSlotManagedStoreInstance<
  State,
  Actions extends ClientSlotActionDecl<State> = ClientSlotActionDecl<State>,
> {
  readonly __amitiaManagedSlotStoreInstance: true;
  readonly actions: Readonly<ClientSlotBakedActions<State, Actions>>;
  getSnapshot(): State;
  subscribe(listener: () => void): () => void;
  clearPersisted(): void;
  dispose(): void;
}

export interface ClientSlotManagedStoreHandle<
  State,
  Actions extends ClientSlotActionDecl<State> = ClientSlotActionDecl<State>,
> {
  readonly __amitiaManagedSlotStoreHandle: true;
  readonly spec: ClientSlotManagedStoreSpec<State, Actions>;
  create(scopeKey?: string): ClientSlotManagedStoreInstance<State, Actions>;
}

/** Framework-managed immutable-snapshot store. Each create(scopeKey) call gets
 * an independent instance; actions mutate a clone, then atomically publish it. */
export function defineClientSlotStore<
  State,
  const Actions extends ClientSlotActionDecl<State>,
>(
  spec: ClientSlotManagedStoreSpec<State, Actions> & { readonly actions: Actions },
): ClientSlotManagedStoreHandle<State, Actions>;
export function defineClientSlotStore<State>(
  spec: Omit<ClientSlotManagedStoreSpec<State, Record<string, never>>, "actions"> & { readonly actions?: undefined },
): ClientSlotManagedStoreHandle<State, Record<string, never>>;
export function defineClientSlotStore<
  State,
  Actions extends ClientSlotActionDecl<State> = Record<string, never>,
>(
  spec: ClientSlotManagedStoreSpec<State, Actions>,
): ClientSlotManagedStoreHandle<State, Actions> {
  if (typeof spec.init !== "function") throw new Error("slot store init() is required");
  const frozenSpec = Object.freeze({ ...spec, actions: spec.actions ?? ({} as Actions) });
  return Object.freeze({
    __amitiaManagedSlotStoreHandle: true as const,
    spec: frozenSpec,
    create(scopeKey?: string) {
      const persistKey = frozenSpec.persist
        ? `amitia.slot-store:${frozenSpec.persist}${scopeKey ? `:${scopeKey}` : ""}`
        : undefined;
      let state = loadClientSlotStoreState(persistKey, frozenSpec.init);
      let active = true;
      const listeners = new Set<() => void>();
      const baked: Record<string, (...args: any[]) => void> = {};
      for (const [name, mutate] of Object.entries(frozenSpec.actions ?? {})) {
        baked[name] = (...args: any[]) => {
          if (!active) throw new Error(`slot store ${frozenSpec.persist ?? "anonymous"} is disposed`);
          const draft = cloneClientSlotStoreState(state);
          (mutate as (draft: State, ...params: any[]) => void)(draft, ...args);
          state = draft;
          if (persistKey) writeClientSlotStoreState(persistKey, state);
          for (const listener of Array.from(listeners)) listener();
        };
      }
      return {
        __amitiaManagedSlotStoreInstance: true as const,
        actions: Object.freeze(baked) as Readonly<ClientSlotBakedActions<State, Actions>>,
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

function cloneClientSlotStoreState<State>(state: State): State {
  if (typeof globalThis.structuredClone === "function") return globalThis.structuredClone(state);
  if (state == null || typeof state !== "object") return state;
  return JSON.parse(JSON.stringify(state)) as State;
}

function loadClientSlotStoreState<State>(persistKey: string | undefined, init: () => State): State {
  if (persistKey) {
    try {
      const value = globalThis.localStorage?.getItem(persistKey);
      if (value != null) return JSON.parse(value) as State;
    } catch {}
  }
  return cloneClientSlotStoreState(init());
}

function writeClientSlotStoreState<State>(persistKey: string, state: State): void {
  try { globalThis.localStorage?.setItem(persistKey, JSON.stringify(state)); } catch {}
}

export interface ClientSlotContract<
  Owner extends Record<string, unknown> = Record<string, unknown>,
  Store = unknown,
  Injected extends Record<string, unknown> = Record<string, unknown>,
  Scope extends ClientSlotScope = ClientSlotScope,
  Kind extends ClientSlotKind = ClientSlotKind,
  KeyProps extends Record<string, Record<string, unknown>> = Record<string, Record<string, unknown>>,
  Matched = unknown,
  HookContext = never,
> {
  owner: Owner;
  store: Store;
  inject: Injected;
  scope: Scope;
  kind: Kind;
  keyProps: KeyProps;
  matched: Matched;
  hookContext: HookContext;
}

/** Declaration-merging authority for trusted browser plugins. Built-in keys are
 * compatibility surfaces; plugins can add new typed descendants freely. */
export type ClientHostPlatform = "windows" | "macos" | "linux" | "web";
export type ClientHostKind = "web" | "desktop";
export type ClientHostOS = "windows" | "macos" | "linux" | "unknown";

export interface ClientRootOwner extends Record<string, unknown> {
  slotId?: string;
  route?: string;
  platform?: ClientHostPlatform;
  host?: ClientHostKind;
  os?: ClientHostOS;
  locale?: string;
  capabilities?: readonly string[];
}

export interface ClientExtensionOwner extends ClientRootOwner {
  extensionId?: string;
  moduleId?: string;
  contributionId?: string;
}

export interface ClientSessionOwner extends ClientRootOwner {
  sessionId?: string;
  conversationId: string;
  characterId?: string;
}

export interface ClientMessageOwner extends ClientSessionOwner {
  messageId: string;
  messageType: string;
  direction?: "incoming" | "outgoing" | "system";
  senderType?: "user" | "character" | "system" | "extension";
  extensionType?: string;
  message?: Readonly<Record<string, unknown>>;
}

export interface ClientConversationNodeOwner extends ClientSessionOwner {
  nodeId?: string;
  nodeType?: string;
  eventType?: string;
  node?: Readonly<Record<string, unknown>>;
}

export interface ClientCharacterOwner extends ClientRootOwner {
  characterId: string;
  tabId?: string;
}

export interface ClientDesktopOwner extends ClientRootOwner {
  commandId?: string;
  windowId?: string;
  menuId?: string;
  selection?: Readonly<Record<string, unknown>>;
}

export interface ClientProviderOwner extends ClientRootOwner {
  capability: string;
  providerId?: string;
  providerMode?: "replace" | "compose" | "augment";
}

export interface ClientProviderSessionOwner extends ClientSessionOwner {
  capability: string;
  providerId?: string;
  providerMode?: "replace" | "compose" | "augment";
}

type ClientNoInject = Record<string, never>;
type ClientList<Owner extends Record<string, unknown>, Scope extends ClientSlotScope = "root"> = ClientSlotContract<Owner, unknown, ClientNoInject, Scope, "list">;
type ClientChain<Owner extends Record<string, unknown>, Scope extends ClientSlotScope = "root"> = ClientSlotContract<Owner, unknown, ClientNoInject, Scope, "chain">;
type ClientKeyed<Owner extends Record<string, unknown>, Scope extends ClientSlotScope = "root"> = ClientSlotContract<Owner, unknown, ClientNoInject, Scope, "keyed", Record<string, Owner>>;

export interface ClientSlotMap {
  root: ClientSlotContract<ClientRootOwner, unknown, ClientNoInject, "root", "single">;
  "extension.center.header.action": ClientList<ClientExtensionOwner>;
  "extension.center.card.badge": ClientList<ClientExtensionOwner>;
  "extension.detail.tab": ClientList<ClientExtensionOwner>;
  "extension.detail.action": ClientList<ClientExtensionOwner>;
  "extension.settings.page": ClientList<ClientExtensionOwner>;
  "extension.settings.section": ClientList<ClientExtensionOwner>;
  "chat.header.action": ClientList<ClientSessionOwner, "session-maybe">;
  "chat.sidebar.panel": ClientList<ClientSessionOwner, "session-maybe">;
  "chat.message.action": ClientList<ClientMessageOwner, "session">;
  "chat.message.renderer": ClientChain<ClientMessageOwner, "session">;
  "chat.conversation.node": ClientList<ClientConversationNodeOwner, "session">;
  "chat.message.custom_renderer": ClientChain<ClientMessageOwner, "session">;
  "chat.message.attachment_renderer": ClientChain<ClientMessageOwner, "session">;
  "chat.message.badge": ClientList<ClientMessageOwner, "session">;
  "chat.composer.action": ClientList<ClientSessionOwner, "session-maybe">;
  "chat.composer.attachment": ClientList<ClientSessionOwner, "session-maybe">;
  "chat.composer.hint": ClientList<ClientSessionOwner, "session-maybe">;
  "chat.empty_state.card": ClientList<ClientSessionOwner, "session-maybe">;
  "chat.status.item": ClientList<ClientSessionOwner, "session-maybe">;
  "character.detail.tab": ClientList<ClientCharacterOwner>;
  "character.detail.action": ClientList<ClientCharacterOwner>;
  "character.sidebar.card": ClientList<ClientCharacterOwner>;
  "system.status.item": ClientList<ClientRootOwner>;
  "system.settings.section": ClientList<ClientRootOwner>;
  "system.diagnostics.tab": ClientList<ClientRootOwner>;
  "desktop.command": ClientKeyed<ClientDesktopOwner>;
  "desktop.application_menu.item": ClientList<ClientDesktopOwner>;
  "desktop.context_menu.item": ClientList<ClientDesktopOwner>;
  "desktop.tray.item": ClientList<ClientDesktopOwner>;
  "desktop.window.page": ClientKeyed<ClientDesktopOwner>;
  "provider.app.shell": ClientChain<ClientProviderOwner>;
  "provider.app.navigation": ClientChain<ClientProviderOwner>;
  "provider.app.workspace": ClientChain<ClientProviderOwner>;
  "provider.route.registry": ClientList<ClientProviderOwner>;
  "provider.page.provider": ClientChain<ClientProviderOwner>;
  "provider.conversation.shell": ClientChain<ClientProviderSessionOwner, "session-maybe">;
  "provider.conversation.header": ClientChain<ClientProviderSessionOwner, "session-maybe">;
  "provider.conversation.messages": ClientChain<ClientProviderSessionOwner, "session-maybe">;
  "provider.conversation.message_renderer": ClientChain<ClientMessageOwner, "session">;
  "provider.conversation.sidebar": ClientChain<ClientProviderSessionOwner, "session-maybe">;
  "provider.conversation.composer": ClientChain<ClientProviderSessionOwner, "session-maybe">;
  "provider.conversation.overlay": ClientList<ClientProviderSessionOwner, "session-maybe">;
  "provider.character.shell": ClientChain<ClientProviderOwner>;
  "provider.character.detail": ClientChain<ClientProviderOwner>;
  "provider.memory.shell": ClientChain<ClientProviderOwner>;
  "provider.memory.detail": ClientChain<ClientProviderOwner>;
  "provider.settings.shell": ClientChain<ClientProviderOwner>;
  "provider.settings.section": ClientList<ClientProviderOwner>;
  "provider.extension.center": ClientChain<ClientProviderOwner>;
  "provider.extension.page": ClientChain<ClientProviderOwner>;
  "provider.ui.theme": ClientChain<ClientProviderOwner>;
  "provider.ui.tokens": ClientChain<ClientProviderOwner>;
  "provider.ui.icons": ClientChain<ClientProviderOwner>;
  "provider.ui.components": ClientChain<ClientProviderOwner>;
}

export type ClientKnownSlotId = keyof ClientSlotMap & string;
export type ClientSlotProps<K extends ClientKnownSlotId> = ClientSlotMap[K]["owner"];
export type ClientSlotStore<K extends ClientKnownSlotId> = ClientSlotMap[K]["store"];
export type ClientSlotInjected<K extends ClientKnownSlotId> = ClientSlotMap[K]["inject"];
export type ClientSlotScopeOf<K extends ClientKnownSlotId> = ClientSlotMap[K]["scope"];
export type ClientSlotKindOf<K extends ClientKnownSlotId> = ClientSlotMap[K]["kind"];
export type ClientSlotMatched<K extends ClientKnownSlotId> = ClientSlotMap[K]["matched"];
export type ClientSlotHookContext<K extends ClientKnownSlotId> = ClientSlotMap[K]["hookContext"];
export type ClientSlotEntryKey<K extends ClientKnownSlotId> = ClientSlotMap[K]["keyProps"] extends infer P extends Record<string, unknown>
  ? keyof P & string
  : string;
export type ClientSlotKeyProps<
  K extends ClientKnownSlotId,
  EntryKey extends ClientSlotEntryKey<K> = ClientSlotEntryKey<K>,
> = ClientSlotKindOf<K> extends "keyed"
  ? ClientSlotMap[K]["keyProps"] extends Record<string, Record<string, unknown>>
    ? EntryKey extends keyof ClientSlotMap[K]["keyProps"]
      ? ClientSlotMap[K]["keyProps"][EntryKey] extends Record<string, unknown>
        ? ClientSlotMap[K]["keyProps"][EntryKey]
        : object
      : object
    : object
  : object;

export interface ClientSlotRuntimeSpec<K extends ClientKnownSlotId = ClientKnownSlotId> {
  readonly kind: ClientSlotKindOf<K>;
  readonly scope: ClientSlotScopeOf<K>;
  readonly supportedKinds?: readonly string[];
  readonly layout?: "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal" | "hidden";
  readonly fallbackPolicy?: "none" | "skeleton" | "empty" | "default";
  readonly description?: string;
  readonly contractVersion?: number;
  readonly multiplicity?: "single" | "multiple" | "ordered_multiple" | "replaceable_single" | "exclusive";
  readonly keys?: readonly ClientSlotEntryKey<K>[];
  readonly inject?: ClientSlotInjected<K>;
}

export type ClientChildrenDecl = Partial<{ [K in ClientKnownSlotId]: ClientSlotRuntimeSpec<K> }>;
export type ClientChildKeys<D extends ClientChildrenDecl> = keyof D & ClientKnownSlotId;
export type ClientChainChildKeys<D extends ClientChildrenDecl> = {
  [K in ClientChildKeys<D>]: D[K] extends { kind: "chain" } ? K : never
}[ClientChildKeys<D>];
export type ClientNonChainChildKeys<D extends ClientChildrenDecl> = Exclude<ClientChildKeys<D>, ClientChainChildKeys<D>>;

type ClientSlotOccurrenceContextOption<K extends ClientKnownSlotId> = [ClientSlotHookContext<K>] extends [never]
  ? { readonly hookContext?: never }
  : { readonly hookContext: ClientSlotHookContext<K> };

export type ClientSlotRenderOptions<
  K extends ClientKnownSlotId = ClientKnownSlotId,
  EntryKey extends ClientSlotEntryKey<K> = ClientSlotEntryKey<K>,
> = {
  readonly entryKey?: EntryKey;
  /** Amitia compatibility alias for entryKey. */
  readonly key?: EntryKey;
  readonly only?: string;
  readonly fallback?: unknown;
} & ClientSlotOccurrenceContextOption<K>;

type ClientSessionChildKeys<D extends ClientChildrenDecl> = {
  [K in ClientChildKeys<D>]: D[K] extends { scope: "session" } ? K : never
}[ClientChildKeys<D>];

export interface ClientSessionAreaProps {
  readonly empty?: () => unknown;
  readonly children: (sessionId: string) => unknown;
}

type ClientSlotRenderArgs<K extends ClientKnownSlotId, EntryKey extends ClientSlotEntryKey<K>> =
  [ClientSlotHookContext<K>] extends [never]
    ? [options?: ClientSlotRenderOptions<K, EntryKey>]
    : [options: ClientSlotRenderOptions<K, EntryKey>];

type ClientSlotChainRenderOptions<K extends ClientKnownSlotId> = {
  readonly fallback?: unknown;
  readonly overlay?: boolean;
} & ClientSlotOccurrenceContextOption<K>;

type ClientSlotChainRenderArgs<K extends ClientKnownSlotId> =
  [ClientSlotHookContext<K>] extends [never]
    ? [options?: ClientSlotChainRenderOptions<K>]
    : [options: ClientSlotChainRenderOptions<K>];

export type ClientSlotRenderAuthority<D extends ClientChildrenDecl> = {
  renderSlot<
    K extends ClientNonChainChildKeys<D>,
    EntryKey extends ClientSlotEntryKey<K> = ClientSlotEntryKey<K>,
  >(
    name: K,
    owner: ClientSlotProps<K> & ClientSlotKeyProps<K, EntryKey>,
    ...args: ClientSlotRenderArgs<K, EntryKey>
  ): unknown;
  readonly __renders?: ((name: ClientChildKeys<D>) => void) | undefined;
} & ([ClientChainChildKeys<D>] extends [never] ? object : {
  renderSlotChain<K extends ClientChainChildKeys<D>>(
    name: K,
    owner: ClientSlotProps<K>,
    ...args: ClientSlotChainRenderArgs<K>
  ): unknown;
}) & ([ClientSessionChildKeys<D>] extends [never] ? object : {
  readonly SessionProvider: (props: ClientSessionAreaProps) => unknown;
});

export interface ClientSlotStoreContext {
  readonly pluginId: string;
  readonly slotId: string;
  readonly key: string;
  readonly scope: ClientSlotScope;
  readonly sessionId?: string;
}

export interface ClientSlotStoreResource<T, Actions extends Record<string, (...args: any[]) => any> = Record<string, never>> {
  readonly __amitiaSlotStoreResource: true;
  readonly store: T;
  readonly actions?: Actions;
  readonly subscribe?: (listener: () => void) => () => void;
  dispose?(): void | Promise<void>;
}

export function clientSlotStoreResource<T, Actions extends Record<string, (...args: any[]) => any> = Record<string, never>>(
  store: T,
  options?: (() => void | Promise<void>) | {
    readonly actions?: Actions;
    readonly subscribe?: (listener: () => void) => () => void;
    readonly dispose?: () => void | Promise<void>;
  },
): ClientSlotStoreResource<T, Actions> {
  const normalized = typeof options === "function" ? { dispose: options } : options ?? {};
  return {
    __amitiaSlotStoreResource: true,
    store,
    ...(normalized.actions ? { actions: normalized.actions } : {}),
    ...(normalized.subscribe ? { subscribe: normalized.subscribe } : {}),
    ...(normalized.dispose ? { dispose: normalized.dispose } : {}),
  };
}

export type ClientSlotStoreFactory<T> = (context: ClientSlotStoreContext) => T | ClientSlotStoreResource<T, any>;

export interface ClientSlotInjectContext<TStore> extends ClientSlotStoreContext {
  readonly store: TStore;
  readonly actions: Readonly<Record<string, (...args: any[]) => any>>;
  readonly services: {
    get<T>(serviceId: string): T | undefined;
    list(): string[];
  };
  readonly events: {
    emit<T>(eventType: string, payload: T): Promise<void>;
  };
}

export type ClientSlotInjectFactory<TStore, TInjected extends Record<string, unknown>> = (
  context: ClientSlotInjectContext<TStore>,
) => TInjected;

export type ClientSlotHookSource = ClientObservable<unknown>
  | ((standard: Record<string, unknown>, hookContext: unknown) => (...args: any[]) => unknown);
export type ClientSlotHookSources = Record<string, ClientSlotHookSource>;
type ClientBoundHookName<K extends string> = `use${Capitalize<K>}`;
type ClientBindHookSources<H> = H extends Record<string, unknown> ? {
  [K in keyof H & string as ClientBoundHookName<K>]: H[K] extends ClientObservable<infer T>
    ? <Selected = T>(selector?: (value: T) => Selected, equality?: (previous: Selected, next: Selected) => boolean) => ClientReadonlyValue<Selected>
    : H[K] extends (...args: any[]) => infer Hook
      ? Hook extends (...args: any[]) => unknown ? Hook : never
      : never
} : object;

export type ClientSlotInjectFace<I extends Record<string, unknown>> = Omit<I, "hooks"> & (
  I extends { hooks: infer H } ? ClientBindHookSources<H> : object
);

export interface ClientReadonlyValue<T> { readonly value: T }
export type ClientSlotSelectorHook<T> = <Selected = T>(
  selector?: (value: T) => Selected,
  equality?: (previous: Selected, next: Selected) => boolean,
) => ClientReadonlyValue<Selected>;

type ClientStoreState<S> = S extends ClientSlotManagedStoreHandle<infer State, any> ? State
  : S extends ClientSlotStoreHandle<infer State, any> ? State
    : S;
type ClientStoreActions<S> = S extends ClientSlotManagedStoreHandle<infer State, infer Actions> ? ClientSlotBakedActions<State, Actions>
  : S extends ClientSlotStoreHandle<any, infer Actions> ? Actions
    : Record<string, (...args: any[]) => any>;

export type ClientSlotStoreProps<S> = {
  useStore<Selected = ClientStoreState<S>>(
    selector?: (state: ClientStoreState<S>) => Selected,
    equality?: (previous: Selected, next: Selected) => boolean,
  ): ClientReadonlyValue<Selected>;
  readonly actions: Readonly<ClientStoreActions<S>>;
};

export type ClientSlotMatchedProps<K extends ClientKnownSlotId> = ClientSlotKindOf<K> extends "chain"
  ? { readonly matched: ClientSlotMatched<K> }
  : object;

export type ClientSlotStandardProps<K extends ClientKnownSlotId> = {
  readonly slotId: K;
  readonly pluginId: string;
  readonly contributionId: string;
  readonly useGlobal: ClientSlotSelectorHook<Record<string, unknown>>;
} & (ClientSlotScopeOf<K> extends "session"
  ? { readonly sessionId: string; readonly useSession: ClientSlotSelectorHook<Record<string, unknown> | undefined> }
  : { readonly sessionId?: string; readonly useSession: ClientSlotSelectorHook<Record<string, unknown> | undefined> });

export type ClientSlotComponentProps<
  K extends ClientKnownSlotId,
  D extends ClientChildrenDecl = {},
  S = ClientSlotStore<K>,
  I extends Record<string, unknown> = ClientSlotInjected<K>,
  N extends keyof ClientLocaleNamespaceMap & string | undefined = undefined,
  EntryKey extends ClientSlotEntryKey<K> = ClientSlotEntryKey<K>,
> = ClientSlotProps<K>
  & ClientSlotKeyProps<K, NoInfer<EntryKey>>
  & ClientSlotStandardProps<K>
  & ClientSlotRenderAuthority<D>
  & ClientSlotStoreProps<S>
  & ClientSlotInjectFace<ClientSlotInjected<K>>
  & ClientSlotInjectFace<I>
  & ClientSlotMatchedProps<K>
  & ClientSlotLocaleProps<N>;

export type ClientSlotComponent<P> = (props: P) => unknown;
export type ClientSlotRendersCheck<C, D extends ClientChildrenDecl> =
  [ClientChildKeys<D>] extends [never] ? unknown
    : C extends (props: infer P) => unknown
      ? "renderSlot" extends keyof P ? unknown
        : "renderSlotChain" extends keyof P ? unknown
          : { readonly "children declared but component consumes no renderSlot": ClientChildKeys<D> }
      : unknown;

export type ClientSlotRegistrationComponent<
  K extends ClientKnownSlotId,
  D extends ClientChildrenDecl,
  S,
  I extends Record<string, unknown>,
  N extends keyof ClientLocaleNamespaceMap & string | undefined,
  EntryKey extends ClientSlotEntryKey<K>,
  C,
> = C
  & ClientSlotComponent<ClientSlotComponentProps<K, D, S, I, N, EntryKey>>
  & ClientSlotRendersCheck<C, D>;

export type ClientSlotKindRegisterOptions<
  K extends ClientKnownSlotId,
  EntryKey extends ClientSlotEntryKey<K> = ClientSlotEntryKey<K>,
> =
  ClientSlotKindOf<K> extends "keyed" ? { readonly entryKey: EntryKey; readonly priority?: number }
    : ClientSlotKindOf<K> extends "list" ? { readonly id: string; readonly order?: number; readonly label?: string | (() => string); readonly priority?: number }
      : ClientSlotKindOf<K> extends "chain" ? { readonly select: (owner: ClientSlotProps<K>) => ClientSlotMatched<K> | null; readonly priority?: number }
        : { readonly priority?: number };

export type ClientSlotRegisterOptions<
  K extends ClientKnownSlotId,
  D extends ClientChildrenDecl = {},
  S = ClientSlotStore<K>,
  I extends Record<string, unknown> = ClientSlotInjected<K>,
  N extends keyof ClientLocaleNamespaceMap & string | undefined = undefined,
  EntryKey extends ClientSlotEntryKey<K> = ClientSlotEntryKey<K>,
> = {
  readonly name: K;
  /** Stable Amitia registration identity, independent from keyed dispatch entryKey. */
  readonly key: string;
  readonly ordering?: number;
  readonly ownerDefaults?: Partial<ClientSlotProps<K>>;
  readonly children?: D;
  readonly store?: S | ClientSlotStoreFactory<S>;
  readonly inject?: ClientSlotInjectFactory<S, I>;
  readonly locale?: N;
} & ClientSlotKindRegisterOptions<K, EntryKey>;

export interface ClientSlotEntryDefinition<K extends ClientKnownSlotId = ClientKnownSlotId> {
  readonly options: ClientSlotRegisterOptions<K>;
  readonly component: Component;
}

export function defineClientSlotEntry<K extends ClientKnownSlotId>(
  entry: ClientSlotEntryDefinition<K>,
): ClientSlotEntryDefinition<K> {
  return entry;
}
