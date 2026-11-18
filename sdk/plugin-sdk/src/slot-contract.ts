/**
 * Strongly typed UI composition contracts.
 *
 * Amitia intentionally keeps a superset of the DeepSeek Harness slot model:
 * the DSH-style typed composition chain is the core, while schema UI,
 * same-page client modules, provider profiles and host surfaces remain valid
 * higher-level capabilities.
 */
export type UISlotScope = "root" | "session-maybe" | "session";
export type UISlotKind = "single" | "list" | "keyed" | "chain" | "hoc";

export interface UISlotObservable<T> {
  getSnapshot(): T;
  subscribe(listener: () => void): () => void;
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
          try { globalThis.localStorage?.removeItem(persistKey); } catch (error) {
  console.error("[original-error]", error);
}
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
    } catch (error) {
  console.error("[original-error]", error);
}
  }
  return cloneUISlotStoreState(init());
}

function writeUISlotStoreState<State>(persistKey: string, state: State): void {
  try { globalThis.localStorage?.setItem(persistKey, JSON.stringify(state)); } catch (error) {
  console.error("[original-error]", error);
}
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
  allowRootChild?: boolean;
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
export type UIHostPlatform = "windows" | "macos" | "linux" | "web" | "android" | "ios" | "mobile";
export type UIHostKind = "web" | "desktop" | "mobile";
export type UIHostOS = "windows" | "macos" | "linux" | "android" | "ios" | "unknown";
export type UIHostSurfaceRole = "header" | "status" | "sidebar" | "message" | "composer" | "main" | "overlay";

export interface UIRootOwner extends Record<string, unknown> {
  slotId?: string;
  route?: string;
  platform?: UIHostPlatform;
  host?: UIHostKind;
  os?: UIHostOS;
  locale?: string;
  capabilities?: readonly string[];
}

export interface UIExtensionOwner extends UIRootOwner {
  extensionId?: string;
  moduleId?: string;
  contributionId?: string;
}

export interface UISessionOwner extends UIRootOwner {
  sessionId?: string;
  conversationId: string;
  characterId?: string;
}

export interface UIMessageOwner extends UISessionOwner {
  messageId: string;
  messageType: string;
  direction?: "incoming" | "outgoing" | "system";
  senderType?: "user" | "character" | "system" | "extension";
  extensionType?: string;
  message?: Readonly<Record<string, unknown>>;
}

export interface UIConversationNodeOwner extends UISessionOwner {
  nodeId?: string;
  nodeType?: string;
  eventType?: string;
  node?: Readonly<Record<string, unknown>>;
}

export interface UICharacterOwner extends UIRootOwner {
  characterId: string;
  tabId?: string;
}

export interface UIDesktopOwner extends UIRootOwner {
  commandId?: string;
  windowId?: string;
  menuId?: string;
  selection?: Readonly<Record<string, unknown>>;
}

export interface UIProviderOwner extends UIRootOwner {
  capability: string;
  providerId?: string;
  providerMode?: "replace" | "compose" | "augment";
}

export interface UIProviderSessionOwner extends UISessionOwner {
  capability: string;
  providerId?: string;
  providerMode?: "replace" | "compose" | "augment";
}

type UINoInject = Record<string, never>;
type UISingle<Owner extends Record<string, unknown>, Scope extends UISlotScope = "root"> = UISlotContract<Owner, unknown, UINoInject, Scope, "single">;
type UIList<Owner extends Record<string, unknown>, Scope extends UISlotScope = "root"> = UISlotContract<Owner, unknown, UINoInject, Scope, "list">;
type UIChain<Owner extends Record<string, unknown>, Scope extends UISlotScope = "root"> = UISlotContract<Owner, unknown, UINoInject, Scope, "chain">;
type UIKeyed<Owner extends Record<string, unknown>, Scope extends UISlotScope = "root"> = UISlotContract<Owner, unknown, UINoInject, Scope, "keyed", Record<string, Owner>>;
type UIHoc<Owner extends Record<string, unknown>, Scope extends UISlotScope = "root"> = UISlotContract<Owner, unknown, UINoInject, Scope, "hoc">;

export interface UISlotMap {
  root: UISlotContract<UIRootOwner, unknown, UINoInject, "root", "single">;
  "extension.center.header.action": UIList<UIExtensionOwner>;
  "extension.center.card.badge": UIList<UIExtensionOwner>;
  "extension.detail.tab": UIList<UIExtensionOwner>;
  "extension.detail.action": UIList<UIExtensionOwner>;
  "extension.settings.page": UIList<UIExtensionOwner>;
  "extension.settings.section": UIList<UIExtensionOwner>;
  "extension.plugin.tab": UIList<UIRootOwner>;
  "extension.plugin.item": UIList<UIRootOwner>;
  "extension.plugin.bundle.activation": UIKeyed<UIRootOwner>;
  "extension.plugin.bundle.config": UIKeyed<UIRootOwner>;
  "extension.plugin.row.config": UIKeyed<UIExtensionOwner>;
  "extension.settings.tab": UIList<UIRootOwner>;
  "chat.header.action": UIList<UISessionOwner, "session-maybe">;
  "chat.header.leading": UISingle<UIRootOwner>;
  "chat.header.lineage": UISingle<UISessionOwner, "session">;
  "chat.header.corner": UISingle<UISessionOwner, "session">;
  "chat.conversation.root": UISingle<UISessionOwner, "session-maybe">;
  "chat.conversation.session": UISingle<UISessionOwner, "session">;
  "chat.conversation.header": UISingle<UISessionOwner, "session-maybe">;
  "chat.conversation.session.header": UISingle<UISessionOwner, "session">;
  "chat.conversation.subagent.footer": UIList<UISessionOwner, "session">;
  "chat.conversation.views": UIList<UISessionOwner, "session">;
  "chat.composer.root": UIChain<UISessionOwner, "session">;
  "chat.composer.bar": UISingle<UISessionOwner, "session-maybe">;
  "chat.composer.attachments": UISingle<UISessionOwner, "session-maybe">;
  "chat.empty.brand_mark": UISingle<UIRootOwner>;
  "chat.empty.frame": UIHoc<UIRootOwner>;
  "chat.empty.options": UIList<UISessionOwner, "session">;
  "chat.empty.workspace": UISingle<UIRootOwner>;
  "chat.empty.workspace.directory_flow": UISingle<UIRootOwner>;
  "chat.empty.agent_preset": UISingle<UISessionOwner, "session-maybe">;
  "chat.sidebar.panel": UIList<UISessionOwner, "session-maybe">;
  "chat.message.action": UIList<UIMessageOwner, "session">;
  "chat.message.renderer": UIChain<UIMessageOwner, "session">;
  "chat.message.images": UISingle<UIMessageOwner, "session">;
  "chat.message.user.images": UISingle<UIMessageOwner, "session">;
  "chat.message.user.text": UIKeyed<UIMessageOwner, "session">;
  "chat.approval.detail": UISingle<UISessionOwner, "session">;
  "chat.message.command": UIKeyed<UIMessageOwner, "session">;
  "chat.conversation.flow": UISingle<UISessionOwner, "session">;
  "chat.message.markdown.fence": UIHoc<UIMessageOwner, "session">;
  "chat.message.reasoning.body": UISingle<UIMessageOwner, "session">;
  "chat.message.text.interaction": UIKeyed<UIMessageOwner, "session">;
  "chat.message.trajectory.images": UISingle<UISessionOwner, "session">;
  "chat.message.tool.images": UISingle<UIMessageOwner, "session">;
  "chat.message.tool.cordis": UIKeyed<UIMessageOwner, "session">;
  "chat.conversation.node": UIList<UIConversationNodeOwner, "session">;
  "chat.message.custom_renderer": UIChain<UIMessageOwner, "session">;
  "chat.message.attachment_renderer": UIChain<UIMessageOwner, "session">;
  "chat.message.badge": UIList<UIMessageOwner, "session">;
  "chat.composer.action": UIList<UISessionOwner, "session-maybe">;
  "chat.composer.attachment": UIList<UISessionOwner, "session-maybe">;
  "chat.composer.hint": UIList<UISessionOwner, "session-maybe">;
  "chat.composer.dock.top": UIList<UISessionOwner, "session">;
  "chat.composer.dock.bottom": UIList<UISessionOwner, "session">;
  "chat.composer.left": UIList<UISessionOwner, "session">;
  "chat.composer.right": UIList<UISessionOwner, "session">;
  "chat.composer.overlay": UIList<UISessionOwner, "session">;
  "chat.composer.model": UISingle<UISessionOwner, "session">;
  "chat.composer.model.label": UISingle<UISessionOwner, "session">;
  "chat.composer.model.options": UIList<UISessionOwner, "session">;
  "chat.composer.permission": UISingle<UISessionOwner, "session">;
  "chat.empty_state.card": UIList<UISessionOwner, "session-maybe">;
  "chat.status.item": UIList<UISessionOwner, "session-maybe">;
  "character.detail.tab": UIList<UICharacterOwner>;
  "character.detail.action": UIList<UICharacterOwner>;
  "character.sidebar.card": UIList<UICharacterOwner>;
  "memory.page.section": UIList<UIRootOwner>;
  "tasks.page.section": UIList<UIRootOwner>;
  "system.status.item": UIList<UIRootOwner>;
  "system.settings.section": UIList<UIRootOwner>;
  "system.diagnostics.tab": UIList<UIRootOwner>;
  "app.shell.quota_notice": UIChain<UIRootOwner>;
  "app.workspace.toast": UIKeyed<UIRootOwner>;
  "app.shell.bottom": UISingle<UIRootOwner>;
  "app.rightbar": UISingle<UIRootOwner>;
  "app.rightbar.session": UISingle<UISessionOwner, "session">;
  "desktop.context_menu.content": UIKeyed<UIRootOwner>;
  "desktop.context_menu.icon": UIKeyed<UIRootOwner>;
  "workbench.file.actions": UIList<UISessionOwner, "session">;
  "workbench.review.file.actions": UIList<UISessionOwner, "session">;
  "settings.model.provider": UIKeyed<UIRootOwner>;
  "settings.model.footer": UIList<UIRootOwner>;
  "settings.shell": UISingle<UIRootOwner>;
  "settings.shell.header": UISingle<UIRootOwner>;
  "settings.shell.action": UIList<UIRootOwner>;
  "settings.shell.close": UISingle<UIRootOwner>;
  "settings.shell.launcher": UISingle<UIRootOwner>;
  "settings.shell.launcher.notifications": UISingle<UIRootOwner>;
  "settings.onboarding": UIList<UIRootOwner>;
  "settings.section.icon": UIList<UIRootOwner>;
  "settings.shell.trigger": UISingle<UIRootOwner>;
  "settings.model.sign-in": UISingle<UIRootOwner>;
  "ui.portal.overlay": UIList<UIRootOwner>;
  "ui.portal.modal": UIList<UIRootOwner>;
  "ui.portal.drawer": UIList<UIRootOwner>;
  "ui.portal.toast": UIList<UIRootOwner>;
  "ui.sidebar.right.pane.tab": UIList<UIRootOwner, "session-maybe">;
  "ui.sidebar.right.pane.tab.title": UIList<UIRootOwner, "session-maybe">;
  "ui.sidebar.right.pane.document": UIKeyed<UIRootOwner, "session">;
  "ui.sidebar.right.pane.document.action": UIKeyed<UIRootOwner, "session">;
  "ui.sidebar.right.pane.document.actions": UIList<UIRootOwner, "session">;
  "ui.sidebar.right.pane.document.pdf": UIKeyed<UIRootOwner, "session">;
  "ui.sidebar.right.pane.document.unpreviewable": UIList<UIRootOwner, "session">;
  "ui.sidebar.right.pane.files.actions": UIList<UISessionOwner, "session">;
  "ui.sidebar.right.pane.guide": UIChain<UISessionOwner, "session">;
  "ui.sidebar.right.pane.guide.actions": UIList<UIRootOwner>;
  "ui.sidebar.right.pane.guide.entry": UIKeyed<UISessionOwner, "session">;
  "ui.sidebar.right.pane.tab.menu.item": UIList<UISessionOwner, "session">;
  "desktop.command": UIKeyed<UIDesktopOwner>;
  "desktop.application_menu.item": UIList<UIDesktopOwner>;
  "desktop.context_menu.item": UIList<UIDesktopOwner>;
  "desktop.tray.item": UIList<UIDesktopOwner>;
  "desktop.window.page": UIKeyed<UIDesktopOwner>;
  "provider.app.shell": UIChain<UIProviderOwner>;
  "provider.app.navigation": UIChain<UIProviderOwner>;
  "provider.app.workspace": UIChain<UIProviderOwner>;
  "provider.app.workspace.sidebar": UIChain<UIProviderOwner>;
  "provider.route.registry": UIList<UIProviderOwner>;
  "provider.page.provider": UIChain<UIProviderOwner>;
  "provider.conversation.shell": UIChain<UIProviderSessionOwner, "session-maybe">;
  "provider.conversation.header": UIChain<UIProviderSessionOwner, "session-maybe">;
  "provider.conversation.messages": UIChain<UIProviderSessionOwner, "session-maybe">;
  "provider.conversation.message_renderer": UIChain<UIMessageOwner, "session">;
  "provider.conversation.sidebar": UIChain<UIProviderSessionOwner, "session-maybe">;
  "provider.conversation.composer": UIChain<UIProviderSessionOwner, "session-maybe">;
  "provider.conversation.overlay": UIList<UIProviderSessionOwner, "session-maybe">;
  "provider.character.shell": UIChain<UIProviderOwner>;
  "provider.character.detail": UIChain<UIProviderOwner>;
  "provider.memory.shell": UIChain<UIProviderOwner>;
  "provider.memory.detail": UIChain<UIProviderOwner>;
  "provider.settings.shell": UIChain<UIProviderOwner>;
  "provider.settings.section": UIList<UIProviderOwner>;
  "provider.extension.center": UIChain<UIProviderOwner>;
  "provider.extension.page": UIChain<UIProviderOwner>;
  "provider.ui.theme": UIChain<UIProviderOwner>;
  "provider.ui.tokens": UIChain<UIProviderOwner>;
  "provider.ui.icons": UIChain<UIProviderOwner>;
  "provider.ui.components": UIChain<UIProviderOwner>;
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
export type UISlotDynamicOwner = UIRootOwner & Record<string, unknown>;

export interface UISlotDynamicDefinition {
  readonly supportedKinds?: readonly string[];
  readonly kind: UISlotKind;
  readonly scope?: UISlotScope;
  readonly layout?: "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal" | "hidden";
  readonly fallbackPolicy?: "none" | "skeleton" | "empty" | "default";
  readonly contractVersion?: number;
  readonly description?: string;
}

export interface UISlotDynamicRegisterOptions<K extends string> {
  readonly name: K;
  readonly key: string;
  readonly ordering?: number;
  readonly priority?: number;
  readonly props?: Readonly<Record<string, unknown>>;
  readonly children?: Readonly<Record<string, UISlotDynamicDefinition>>;
  readonly store?: unknown;
  readonly inject?: (context: UISlotInjectContext<unknown>) => Record<string, unknown>;
  readonly entryKey?: string;
  readonly cellId?: string;
  readonly label?: string | (() => string);
  readonly select?: (owner: Record<string, unknown>) => unknown | null;
}

export type UISlotDynamicComponentProps<K extends string = string> = UISlotDynamicOwner & {
  readonly slotId: K;
  readonly pluginId: string;
  readonly contributionId: string;
  readonly useGlobal: UISlotSelectorHook<Record<string, unknown>>;
  readonly useSession: UISlotSelectorHook<Record<string, unknown> | undefined>;
  useStore<Selected = unknown>(selector?: (value: unknown) => Selected): UISlotReadonlyValue<Selected>;
  readonly actions: Readonly<Record<string, (...args: any[]) => any>>;
  renderSlot(slotId: string, owner: Record<string, unknown>, options?: UISlotRenderOptions<UIKnownSlotId>): unknown;
  renderSlotChain?(slotId: string, owner: Record<string, unknown>, options?: UISlotChainRenderOptions<UIKnownSlotId>): unknown;
};
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
  : S;
type StoreActions<S> = S extends UISlotManagedStoreHandle<infer State, infer Actions> ? UISlotBakedActions<State, Actions>
  : Record<string, (...args: any[]) => any>;

export type UISlotStoreProps<S> = {
  useStore<Selected = StoreState<S>>(selector?: (state: StoreState<S>) => Selected): UISlotReadonlyValue<Selected>;
  readonly actions: Readonly<StoreActions<S>>;
};

export type UISlotMatchedProps<K extends UIKnownSlotId> = UISlotKindOf<K> extends "chain" ? { readonly matched: UISlotMatched<K> } : object;
export type UISlotHocProps<K extends UIKnownSlotId> = UISlotKindOf<K> extends "hoc" ? { readonly children: unknown } : object;

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
  & UISlotHocProps<K>
  & UISlotLocaleProps<N>;

export type UISlotKindRegisterOptions<K extends UIKnownSlotId, EntryKey extends UISlotEntryKey<K> = UISlotEntryKey<K>> =
  UISlotKindOf<K> extends "keyed" ? { readonly entryKey: EntryKey; readonly priority?: number }
    : UISlotKindOf<K> extends "list" ? { readonly id: string; readonly order?: number; readonly label?: string | (() => string); readonly priority?: number }
      : UISlotKindOf<K> extends "chain" ? { readonly select: (owner: UISlotProps<K>) => UISlotMatched<K> | null; readonly priority?: number }
        : UISlotKindOf<K> extends "hoc" ? { readonly id?: string; readonly entryKey?: EntryKey; readonly order?: number; readonly priority?: number }
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
