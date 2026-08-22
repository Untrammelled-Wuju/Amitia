import type { Component } from "vue";

export type ClientSlotScope = "root" | "session-maybe" | "session";

/**
 * Compile-time contract for one UI slot. Plugins may augment ClientSlotMap via
 * TypeScript declaration merging to make dynamically declared child slots fully
 * typed without teaching the host about them ahead of time.
 */
export interface ClientSlotContract<
  Props extends Record<string, unknown> = Record<string, unknown>,
  Store = unknown,
  Injected extends Record<string, unknown> = Record<string, unknown>,
> {
  props: Props;
  store: Store;
  inject: Injected;
}

/**
 * Built-in slot map. This interface is intentionally augmentable:
 *
 * declare module "@/ui-runtime/slotContract" {
 *   interface ClientSlotMap {
 *     "my.plugin.panel.body": ClientSlotContract<MyProps, MyStore, MyInject>;
 *   }
 * }
 */
export interface ClientSlotMap {
  root: ClientSlotContract;
  "extension.center.header.action": ClientSlotContract;
  "extension.center.card.badge": ClientSlotContract;
  "extension.detail.tab": ClientSlotContract;
  "extension.detail.action": ClientSlotContract;
  "extension.settings.page": ClientSlotContract;
  "extension.settings.section": ClientSlotContract;
  "chat.header.action": ClientSlotContract;
  "chat.sidebar.panel": ClientSlotContract;
  "chat.message.action": ClientSlotContract;
  "chat.message.renderer": ClientSlotContract;
  "chat.conversation.node": ClientSlotContract;
  "chat.message.custom_renderer": ClientSlotContract;
  "chat.message.attachment_renderer": ClientSlotContract;
  "chat.message.badge": ClientSlotContract;
  "chat.composer.action": ClientSlotContract;
  "chat.composer.attachment": ClientSlotContract;
  "chat.composer.hint": ClientSlotContract;
  "chat.empty_state.card": ClientSlotContract;
  "chat.status.item": ClientSlotContract;
  "character.detail.tab": ClientSlotContract;
  "character.detail.action": ClientSlotContract;
  "character.sidebar.card": ClientSlotContract;
  "system.status.item": ClientSlotContract;
  "system.settings.section": ClientSlotContract;
  "system.diagnostics.tab": ClientSlotContract;
  "desktop.command": ClientSlotContract;
  "desktop.application_menu.item": ClientSlotContract;
  "desktop.context_menu.item": ClientSlotContract;
  "desktop.tray.item": ClientSlotContract;
  "desktop.window.page": ClientSlotContract;

  "provider.app.shell": ClientSlotContract;
  "provider.app.navigation": ClientSlotContract;
  "provider.app.workspace": ClientSlotContract;
  "provider.route.registry": ClientSlotContract;
  "provider.page.provider": ClientSlotContract;
  "provider.conversation.shell": ClientSlotContract;
  "provider.conversation.header": ClientSlotContract;
  "provider.conversation.messages": ClientSlotContract;
  "provider.conversation.message_renderer": ClientSlotContract;
  "provider.conversation.sidebar": ClientSlotContract;
  "provider.conversation.composer": ClientSlotContract;
  "provider.conversation.overlay": ClientSlotContract;
  "provider.character.shell": ClientSlotContract;
  "provider.character.detail": ClientSlotContract;
  "provider.memory.shell": ClientSlotContract;
  "provider.memory.detail": ClientSlotContract;
  "provider.settings.shell": ClientSlotContract;
  "provider.settings.section": ClientSlotContract;
  "provider.extension.center": ClientSlotContract;
  "provider.extension.page": ClientSlotContract;
  "provider.ui.theme": ClientSlotContract;
  "provider.ui.tokens": ClientSlotContract;
  "provider.ui.icons": ClientSlotContract;
  "provider.ui.components": ClientSlotContract;
}

export type ClientKnownSlotId = keyof ClientSlotMap & string;
export type ClientSlotProps<SlotId extends string> = SlotId extends ClientKnownSlotId
  ? ClientSlotMap[SlotId]["props"]
  : Record<string, unknown>;
export type ClientSlotStore<SlotId extends string> = SlotId extends ClientKnownSlotId
  ? ClientSlotMap[SlotId]["store"]
  : unknown;
export type ClientSlotInjected<SlotId extends string> = SlotId extends ClientKnownSlotId
  ? ClientSlotMap[SlotId]["inject"]
  : Record<string, unknown>;

export interface ClientSlotStoreContext {
  readonly pluginId: string;
  readonly slotId: string;
  readonly key: string;
  readonly scope: ClientSlotScope;
}

export interface ClientSlotStoreResource<T> {
  readonly __amitiaSlotStoreResource: true;
  readonly store: T;
  dispose?(): void | Promise<void>;
}

export function clientSlotStoreResource<T>(
  store: T,
  dispose?: () => void | Promise<void>,
): ClientSlotStoreResource<T> {
  return {
    __amitiaSlotStoreResource: true,
    store,
    ...(dispose ? { dispose } : {}),
  };
}

export type ClientSlotStoreFactory<T> = (
  context: ClientSlotStoreContext,
) => T | ClientSlotStoreResource<T>;

export interface ClientSlotInjectContext<TStore> extends ClientSlotStoreContext {
  readonly store: TStore;
  readonly services: {
    get<T>(serviceId: string): T | undefined;
    list(): string[];
  };
  readonly events: {
    emit<T>(eventType: string, payload: T): Promise<void>;
  };
}

export type ClientSlotInjectFactory<
  TStore,
  TInjected extends Record<string, unknown>,
> = (context: ClientSlotInjectContext<TStore>) => TInjected;

export interface ClientSlotEntryDefinition<SlotId extends string = string> {
  readonly slotId: SlotId;
  readonly key: string;
  readonly component: Component;
  readonly ordering?: number;
  readonly priority?: number;
  readonly props?: ClientSlotProps<SlotId>;
  readonly children?: readonly unknown[];
  readonly store?: ClientSlotStore<SlotId> | ClientSlotStoreFactory<ClientSlotStore<SlotId>>;
  readonly inject?: ClientSlotInjectFactory<ClientSlotStore<SlotId>, ClientSlotInjected<SlotId>>;
}

export function defineClientSlotEntry<SlotId extends string>(
  entry: ClientSlotEntryDefinition<SlotId>,
): ClientSlotEntryDefinition<SlotId> {
  return entry;
}
