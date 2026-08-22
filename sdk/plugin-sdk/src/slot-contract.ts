/**
 * Type-level UI slot contract. Extension packages can declaration-merge
 * UISlotMap so child-slot names, props, state and injected business faces are
 * checked at build time while still allowing runtime-declared slots.
 */
export interface UISlotContract<
  Props extends Record<string, unknown> = Record<string, unknown>,
  Store = unknown,
  Injected extends Record<string, unknown> = Record<string, unknown>,
> {
  props: Props;
  store: Store;
  inject: Injected;
}

export interface UISlotMap {
  root: UISlotContract;
  "extension.center.header.action": UISlotContract;
  "extension.center.card.badge": UISlotContract;
  "extension.detail.tab": UISlotContract;
  "extension.detail.action": UISlotContract;
  "extension.settings.page": UISlotContract;
  "extension.settings.section": UISlotContract;
  "chat.header.action": UISlotContract;
  "chat.sidebar.panel": UISlotContract;
  "chat.message.action": UISlotContract;
  "chat.message.renderer": UISlotContract;
  "chat.conversation.node": UISlotContract;
  "chat.message.custom_renderer": UISlotContract;
  "chat.message.attachment_renderer": UISlotContract;
  "chat.message.badge": UISlotContract;
  "chat.composer.action": UISlotContract;
  "chat.composer.attachment": UISlotContract;
  "chat.composer.hint": UISlotContract;
  "chat.empty_state.card": UISlotContract;
  "chat.status.item": UISlotContract;
  "character.detail.tab": UISlotContract;
  "character.detail.action": UISlotContract;
  "character.sidebar.card": UISlotContract;
  "system.status.item": UISlotContract;
  "system.settings.section": UISlotContract;
  "system.diagnostics.tab": UISlotContract;
  "desktop.command": UISlotContract;
  "desktop.application_menu.item": UISlotContract;
  "desktop.context_menu.item": UISlotContract;
  "desktop.tray.item": UISlotContract;
  "desktop.window.page": UISlotContract;
  "provider.app.shell": UISlotContract;
  "provider.app.navigation": UISlotContract;
  "provider.app.workspace": UISlotContract;
  "provider.route.registry": UISlotContract;
  "provider.page.provider": UISlotContract;
  "provider.conversation.shell": UISlotContract;
  "provider.conversation.header": UISlotContract;
  "provider.conversation.messages": UISlotContract;
  "provider.conversation.message_renderer": UISlotContract;
  "provider.conversation.sidebar": UISlotContract;
  "provider.conversation.composer": UISlotContract;
  "provider.conversation.overlay": UISlotContract;
  "provider.character.shell": UISlotContract;
  "provider.character.detail": UISlotContract;
  "provider.memory.shell": UISlotContract;
  "provider.memory.detail": UISlotContract;
  "provider.settings.shell": UISlotContract;
  "provider.settings.section": UISlotContract;
  "provider.extension.center": UISlotContract;
  "provider.extension.page": UISlotContract;
  "provider.ui.theme": UISlotContract;
  "provider.ui.tokens": UISlotContract;
  "provider.ui.icons": UISlotContract;
  "provider.ui.components": UISlotContract;
}

export type UIKnownSlotId = keyof UISlotMap & string;
export type UISlotProps<SlotId extends string> = SlotId extends UIKnownSlotId
  ? UISlotMap[SlotId]["props"]
  : Record<string, unknown>;
export type UISlotStore<SlotId extends string> = SlotId extends UIKnownSlotId
  ? UISlotMap[SlotId]["store"]
  : unknown;
export type UISlotInjected<SlotId extends string> = SlotId extends UIKnownSlotId
  ? UISlotMap[SlotId]["inject"]
  : Record<string, unknown>;

export type UISlotScope = "root" | "session-maybe" | "session";

export interface UISlotStoreContext {
  readonly pluginId: string;
  readonly slotId: string;
  readonly key: string;
  readonly scope: UISlotScope;
}

export interface UISlotStoreResource<T> {
  readonly __amitiaSlotStoreResource: true;
  readonly store: T;
  dispose?(): void | Promise<void>;
}

export function uiSlotStoreResource<T>(
  store: T,
  dispose?: () => void | Promise<void>,
): UISlotStoreResource<T> {
  return {
    __amitiaSlotStoreResource: true,
    store,
    ...(dispose ? { dispose } : {}),
  };
}

export type UISlotStoreFactory<T> = (context: UISlotStoreContext) => T | UISlotStoreResource<T>;

export interface UISlotInjectContext<TStore> extends UISlotStoreContext {
  readonly store: TStore;
  readonly services: {
    get<T>(serviceId: string): T | undefined;
    list(): string[];
  };
  readonly events: {
    emit<T>(eventType: string, payload: T): Promise<void>;
  };
}

export type UISlotInjectFactory<TStore, TInjected extends Record<string, unknown>> = (
  context: UISlotInjectContext<TStore>,
) => TInjected;

export interface TypedUISlotEntryDefinition<SlotId extends string, TRenderable = unknown> {
  readonly slotId: SlotId;
  readonly key: string;
  readonly renderable: TRenderable;
  readonly props?: UISlotProps<SlotId>;
  readonly store?: UISlotStore<SlotId> | UISlotStoreFactory<UISlotStore<SlotId>>;
  readonly inject?: UISlotInjectFactory<UISlotStore<SlotId>, UISlotInjected<SlotId>>;
}

export function defineUISlotContractEntry<SlotId extends string, TRenderable = unknown>(
  entry: TypedUISlotEntryDefinition<SlotId, TRenderable>,
): TypedUISlotEntryDefinition<SlotId, TRenderable> {
  return entry;
}
