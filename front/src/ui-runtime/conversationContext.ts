import { inject, provide, readonly, type InjectionKey, type Ref } from "vue";

export interface ConversationUIActions {
  send(input: { text?: string; imageBase64?: string | null; audioUrl?: string | null; videoUrl?: string | null }): Promise<void>;
  stop(): Promise<void> | void;
  retry(messageId: string): Promise<void> | void;
  regenerate?(messageId?: string): Promise<void> | void;
  delete?(messageId: string): Promise<void> | void;
  createConversation?(): Promise<void> | void;
}

export interface ConversationUIContext {
  conversationId: Ref<string>;
  characterId: Ref<string>;
  messages: Readonly<Ref<readonly unknown[]>>;
  sending: Readonly<Ref<boolean>>;
  generating?: Readonly<Ref<boolean>>;
  offline?: Readonly<Ref<boolean>>;
  actions: ConversationUIActions;
}

export const conversationUIContextKey: InjectionKey<ConversationUIContext> = Symbol("amitia-conversation-ui-context");

export function provideConversationUIContext(context: ConversationUIContext): void {
  provide(conversationUIContextKey, context);
}

export function useConversationUIContext(): ConversationUIContext | null {
  return inject(conversationUIContextKey, null);
}

export function readonlyMessages(messages: Ref<unknown[]>) {
  return readonly(messages) as Readonly<Ref<readonly unknown[]>>;
}
