<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div
    ref="rootEl"
    class="messages-area"
    @scroll="$emit('scroll')"
    @wheel="$emit('wheel', $event)"
    @touchstart="$emit('touchStart', $event)"
    @touchmove="$emit('touchMove', $event)"
    @touchend="$emit('touchEnd')"
  >
    <div class="pull-indicator" :class="{ pulling: isPulling, ready: pullReady }">
      <el-icon :size="18" class="pull-icon" :class="{ spin: pullLoading }">
        <Loading v-if="pullLoading" />
        <ArrowDown v-else />
      </el-icon>
      <span>{{ pullText }}</span>
    </div>

    <div v-if="messages.length === 0 && !sending" class="empty-chat">
      <div class="empty-icon"><el-icon :size="48"><ChatDotRound /></el-icon></div>
      <template v-if="workspaceName">
        <p class="empty-text empty-text--project">你好，我是 {{ charName || "Amitia" }}</p>
        <p class="empty-hint">你想在 {{ workspaceName }} 中构建什么？</p>
      </template>
      <template v-else>
        <p class="empty-text">你好，我是 {{ charName || "AI 陪伴角色" }}</p>
        <p class="empty-hint">随时可以和我聊聊天，或者做你想做的事。</p>
      </template>
      <ChatEmptyStateExtensionHost :context="extensionContext || {}" />
    </div>

    <template v-for="item in flowItems" :key="item.key">
      <div
        v-if="item.kind === 'message'"
        :data-message-id="item.message.id"
        class="conversation-flow-item conversation-flow-item--message"
        :class="{
          'conversation-flow-item--entering': item.message.animateIn === true,
          'conversation-flow-item--assistant': item.message.role === 'assistant',
          'conversation-flow-item--continuation': item.isAssistantContinuation === true,
        }"
        @animationend="finishMessageEntrance(item.message, $event)"
      >
        <div
          v-if="item.showRoleSwitchDivider"
          class="conversation-role-switch"
        >
          <span>当前对话角色已切换为 {{ roleSwitchName(item.roleSwitchCharacterId) }}</span>
        </div>
        <ExtensionSlot
          v-if="hasMessageSlotRenderer(item.message)"
          slot-id="chat.message.renderer"
          :context="messageSlotContext(item.message, item)"
          fallback="default"
          layout="stack"
          surface-role="message"
        />
        <UIProviderHost
          v-else
          capability="conversation.message_renderer"
          :provider-id="messageRendererId(item.message)"
          :fallback="ChatBubble"
          :context="{ ...(extensionContext || {}), message: messageContext(item.message, item) }"
          :actions="messageActions(item.message)"
          :message="item.message"
          :char-name="messageCharacterName(item.message)"
          :char-avatar="messageCharacterAvatar(item.message)"
          :character-id="messageCharacterId(item.message)"
          :status="item.message.status"
          :show-avatar="item.showAssistantIdentity !== false"
          :show-header="item.showAssistantIdentity !== false"
          :compact-bottom="item.compactAfter === true"
          @retry="$emit('retry', $event)"
          @reply="$emit('reply', $event)"
          @edit="$emit('edit', $event)"
          @scroll-to-message="(id) => scrollToMessage(id)"
        >
          <template #badges>
            <MessageBadgeExtensionHost
              :message-id="item.message.id"
              :message-type="item.message.type || 'text'"
              :direction="item.message.role === 'user' ? 'outgoing' : item.message.role === 'assistant' ? 'incoming' : 'system'"
              :sender-type="item.message.role === 'user' ? 'user' : item.message.role === 'assistant' ? 'character' : 'system'"
              :character-id="messageCharacterId(item.message)"
              :conversation-id="item.message.conversationId || conversationId"
            />
          </template>
          <template #extension-content>
            <MessageExtensionHost
              :message="messageExtensionSummary(item.message)"
              :character-id="messageCharacterId(item.message)"
              :conversation-id="item.message.conversationId || conversationId"
            />
          </template>
          <template #actions>
            <MessageActionExtensionHost
              :message-id="item.message.id"
              :message-type="item.message.type || 'text'"
              :direction="item.message.role === 'user' ? 'outgoing' : item.message.role === 'assistant' ? 'incoming' : 'system'"
              :sender-type="item.message.role === 'user' ? 'user' : item.message.role === 'assistant' ? 'character' : 'system'"
              :character-id="messageCharacterId(item.message)"
              :conversation-id="item.message.conversationId || conversationId"
            />
          </template>
        </UIProviderHost>
      </div>

      <ConversationProjectionHost
        v-else
        class="conversation-flow-item conversation-flow-item--node"
        :node="item.node"
        :context="extensionContext"
      />
    </template>

    <div
      v-if="pendingRoleSwitchName"
      class="conversation-role-switch conversation-role-switch--pending"
    >
      <span>当前对话角色已切换为 {{ pendingRoleSwitchName }}</span>
    </div>

    <transition name="fade">
      <el-button
        v-if="showScrollBtn"
        :icon="ArrowDown"
        circle
        size="small"
        class="scroll-btn"
        @click="$emit('scrollToBottom')"
      />
    </transition>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ChatDotRound, ArrowDown, Loading } from "@element-plus/icons-vue";
import ChatBubble from "./ChatBubble.vue";
import MessageExtensionHost from "./extension/chat/MessageExtensionHost.vue";
import ChatEmptyStateExtensionHost from "./extension/chat/ChatEmptyStateExtensionHost.vue";
import MessageActionExtensionHost from "./extension/chat/MessageActionExtensionHost.vue";
import MessageBadgeExtensionHost from "./extension/chat/MessageBadgeExtensionHost.vue";
import ConversationProjectionHost from "./extension/chat/ConversationProjectionHost.vue";
import ExtensionSlot from "./extension/ExtensionSlot.vue";
import UIProviderHost from "./ui-runtime/UIProviderHost.vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { browserClientPluginRuntime } from "@/ui-runtime/clientPluginRuntime";
import { hasUnifiedSlotItem } from "@/ui-runtime/slotLedger";
import { acknowledgeClientRuntimeSessionState, fetchClientRuntimeSessionState, fetchConversationUIEventsBeforeSequence } from "@/api/extension";
import { createConversationUIEventStream } from "@/composables/useConversationUIEventStream";
import { resolveMessageRenderer } from "@/ui-runtime/messageRendererRegistry";
import {
  getMessageCharacterId,
  getMessageUIKey,
  shouldShowRoleSwitch,
  shouldShowAssistantIdentity,
} from "@/utils/message-order";
import {
  ConversationNodeAssembler,
  compareTimeline,
  durableConversationEvents,
  listProgrammaticConversationNodeDefinitions,
  loadConversationEventJournal,
  mergeConversationEvents,
  messageHistoryEvents,
  subscribeProgrammaticConversationNodeDefinitions,
  subscribeConversationRuntimeEvents,
  type ConversationNode,
  type DurableConversationUIEventRecord,
  type RuntimeConversationEvent,
} from "@/ui-runtime/conversationProjection";

const props = defineProps<{
  messages: any[];
  charName: string;
  charAvatar: string;
  characterId: string;
  sending: boolean;
  showScrollBtn: boolean;
  isPulling: boolean;
  pullReady: boolean;
  pullLoading: boolean;
  pullText: string;
  characters?: any[];
  extensionContext?: Record<string, unknown>;
  providerActions?: Record<string, (input?: unknown) => unknown | Promise<unknown>>;
}>();

const emit = defineEmits<{
  scroll: [];
  wheel: [e: WheelEvent];
  touchStart: [e: TouchEvent];
  touchMove: [e: TouchEvent];
  touchEnd: [];
  retry: [msg: any];
  reply: [msg: any];
  edit: [msg: any];
  scrollToBottom: [];
}>();

const store = useExtensionUIStore();
const rootEl = ref<HTMLElement>();
const runtimeEvents = ref<RuntimeConversationEvent[]>([]);
const durableEvents = ref<RuntimeConversationEvent[]>([]);
const conversationNodes = ref<ConversationNode[]>([]);
const conversationAssembler = new ConversationNodeAssembler();
let unsubscribeConversationEvents: (() => void) | null = null;
let unsubscribeConversationDefinitions: (() => void) | null = null;
let stopCanonicalConversationStream: (() => void) | null = null;
let durableRequestGeneration = 0;

const conversationId = computed(() => String(
  props.extensionContext?.conversationId ?? props.messages[0]?.conversationId ?? "",
));
const workspaceName = computed(() => {
  const workspace = props.extensionContext?.workspace as
    | Record<string, unknown>
    | null
    | undefined;
  return String(workspace?.workspaceName ?? workspace?.name ?? "").trim();
});
const projectionContributions = computed(() => store.getVisibleContributions("chat.conversation.node", {
  ...(props.extensionContext ?? {}),
  conversationId: conversationId.value,
}));

type FlowItem =
  | {
      kind: "message";
      key: string;
      message: any;
      sequence?: number;
      timestamp: string;
      showAssistantIdentity?: boolean;
      isAssistantContinuation?: boolean;
      compactAfter?: boolean;
      showRoleSwitchDivider?: boolean;
      roleSwitchCharacterId?: string;
    }
  | { kind: "node"; key: string; node: ConversationNode; sequence?: number; timestamp: string };

const flowItems = computed<FlowItem[]>(() => {
  const items: FlowItem[] = props.messages.map((message, index) => ({
    kind: "message",
    key: `message:${getMessageUIKey(message, index)}`,
    message,
    sequence: finiteNumber(message?.seq ?? message?.sequence),
    timestamp: String(message?.createdAt ?? message?.timestamp ?? ""),
  }));
  for (const node of conversationNodes.value) {
    items.push({
      kind: "node",
      key: `node:${node.nodeId}`,
      node,
      sequence: node.anchorSeq,
      timestamp: node.anchorTimestamp,
    });
  }
  const ordered = items.sort((a, b) => {
    const anchorOrder = compareAnchorOrder(a, b);
    if (anchorOrder !== 0) return anchorOrder;
    const order = compareTimeline(a.sequence, a.timestamp, b.sequence, b.timestamp);
    if (order !== 0) return order;
    if (a.kind !== b.kind) return a.kind === "message" ? -1 : 1;
    return a.key.localeCompare(b.key);
  });
  let previousMessage: any = null;
  for (const item of ordered) {
    if (item.kind !== "message") {
      previousMessage = null;
      continue;
    }
    item.showAssistantIdentity = shouldShowAssistantIdentity(
      item.message,
      previousMessage,
    );
    item.isAssistantContinuation =
      item.message?.role === "assistant" &&
      item.showAssistantIdentity === false;
    item.showRoleSwitchDivider = shouldShowRoleSwitch(
      previousMessage,
      item.message,
    );
    item.roleSwitchCharacterId = getMessageCharacterId(item.message);
    previousMessage = item.message;
  }
  for (let index = 0; index < ordered.length; index += 1) {
    const item = ordered[index];
    const next = ordered[index + 1];
    if (item.kind !== "message") continue;
    item.compactAfter =
      item.message?.role === "assistant" &&
      next?.kind === "message" &&
      next.isAssistantContinuation === true;
  }
  return ordered;
});

function resolveCharacter(characterId: unknown) {
  const id = String(characterId || "").trim();
  if (!id) {
    return {
      id: props.characterId,
      name: props.charName || "未知角色",
      avatar: props.charAvatar,
    };
  }
  const character = props.characters?.find(
    (item: any) => String(item?.id || "") === id,
  );
  if (character) {
    return {
      id,
      name: String(character.name || "未知角色"),
      avatar: String(character.avatar || ""),
    };
  }
  if (id === props.characterId) {
    return {
      id,
      name: props.charName || "未知角色",
      avatar: props.charAvatar,
    };
  }
  return { id, name: "未知角色", avatar: "" };
}

function messageCharacterId(msg: any) {
  return resolveCharacter(getMessageCharacterId(msg)).id;
}

function messageCharacterName(msg: any) {
  return resolveCharacter(getMessageCharacterId(msg)).name;
}

function messageCharacterAvatar(msg: any) {
  return resolveCharacter(getMessageCharacterId(msg)).avatar;
}

function roleSwitchName(characterId: unknown) {
  return resolveCharacter(characterId).name;
}

const pendingRoleSwitchName = computed(() => {
  const lastMessage = props.messages[props.messages.length - 1];
  if (!lastMessage || !props.characterId) return "";
  if (!shouldShowRoleSwitch(lastMessage, { characterId: props.characterId })) {
    return "";
  }
  return resolveCharacter(props.characterId).name;
});

function compareAnchorOrder(a: FlowItem, b: FlowItem): number {
  if (a.kind !== "message" || b.kind !== "message") return 0;
  const aId = String(a.message?.id || "");
  const bId = String(b.message?.id || "");
  const aAnchor = String(a.message?.anchorMessageId || "");
  const bAnchor = String(b.message?.anchorMessageId || "");
  if (aAnchor && aAnchor === bId) return 1;
  if (bAnchor && bAnchor === aId) return -1;
  return 0;
}

function rebuildConversationEventLog() {
  const id = conversationId.value;
  if (!id) {
    runtimeEvents.value = [];
    conversationAssembler.setContributions([]);
    conversationAssembler.replaceEvents([]);
    conversationNodes.value = [];
    return;
  }
  runtimeEvents.value = mergeConversationEvents(
    messageHistoryEvents(props.messages, id),
    durableEvents.value.filter((event) => event.conversationId === id),
    loadConversationEventJournal(id),
    runtimeEvents.value.filter((event) => event.conversationId === id && event.source !== "history" && event.source !== "durable"),
  );
  conversationAssembler.setContributions(projectionContributions.value);
  conversationAssembler.setProgrammaticDefinitions(listProgrammaticConversationNodeDefinitions());
  conversationAssembler.replaceEvents(runtimeEvents.value);
  conversationNodes.value = conversationAssembler.nodes();
}

async function loadDurableConversationEventWindow(id: string) {
  const generation = ++durableRequestGeneration;
  if (!id) {
    stopCanonicalConversationStream?.();
    stopCanonicalConversationStream = null;
    durableEvents.value = [];
    rebuildConversationEventLog();
    return;
  }
  const records: DurableConversationUIEventRecord[] = [];
  const pageSize = 2000;
  let beforeSequence = 0;
  try {
    while (true) {
      const page = await fetchConversationUIEventsBeforeSequence(id, beforeSequence, pageSize);
      if (generation !== durableRequestGeneration || id !== conversationId.value) return;
      const items = page.items ?? [];
      if (items.length > 0) {
        records.unshift(...items);
        const firstSequence = finiteNumber(items[0]?.sequence) ?? 0;
        if (firstSequence <= 0 || firstSequence === beforeSequence) break;
        beforeSequence = firstSequence;
      }
      if (items.length < pageSize) break;
    }
    durableEvents.value = durableConversationEvents(records, id);
  } catch {
    if (generation !== durableRequestGeneration || id !== conversationId.value) return;
    durableEvents.value = [];
  }
  rebuildConversationEventLog();
  if (generation !== durableRequestGeneration || id !== conversationId.value) return;
  startCanonicalConversationStream(id);
}

function startCanonicalConversationStream(id: string) {
  stopCanonicalConversationStream?.();
  stopCanonicalConversationStream = null;
  if (!id) return;
  const afterSequence = durableEvents.value.reduce((max, event) => Math.max(max, finiteNumber(event.sequence) ?? 0), 0);
  stopCanonicalConversationStream = createConversationUIEventStream({
    conversationId: id,
    afterSequence,
    onEvent: (record) => {
      if (id !== conversationId.value) return;
      const event = durableConversationEvents([record], id)[0];
      if (!event) return;
      durableEvents.value = mergeConversationEvents(durableEvents.value, [event]);
      runtimeEvents.value = mergeConversationEvents(runtimeEvents.value, [event]);
      conversationAssembler.append(event);
      conversationNodes.value = conversationAssembler.nodes();
    },
  });
}

function consumeConversationEvent(event: RuntimeConversationEvent) {
  if (!event.conversationId || event.conversationId !== conversationId.value) return;
  runtimeEvents.value = mergeConversationEvents(runtimeEvents.value, [event]);
  conversationAssembler.append(event);
  conversationNodes.value = conversationAssembler.nodes();
}

let clientRuntimeSessionGeneration = 0;
async function loadClientRuntimeSession(id: string) {
  const generation = ++clientRuntimeSessionGeneration;
  const scopeGeneration = await browserClientPluginRuntime.activateConversationScope(id);
  if (!id) return;
  try {
    const state = await fetchClientRuntimeSessionState(id);
    if (generation !== clientRuntimeSessionGeneration || id !== conversationId.value) return;
    const applied = await browserClientPluginRuntime.synchronizeSession(state as any, {
      expectedScopeGeneration: scopeGeneration,
    });
    if (!applied || generation !== clientRuntimeSessionGeneration || id !== conversationId.value) return;

    const hasPendingActivation = (state.packages ?? []).some((item) => {
      const transition = String(item.transitionState ?? "").toLowerCase();
      return !!item.running && !!item.targetVersion && (transition === "starting" || transition === "awaiting_client");
    });
    if (hasPendingActivation) {
      const committed = await acknowledgeClientRuntimeSessionState(id, Number(state.revision ?? 0));
      if (generation !== clientRuntimeSessionGeneration || id !== conversationId.value) return;
      await browserClientPluginRuntime.synchronizeSession(committed as any, {
        expectedScopeGeneration: scopeGeneration,
      });
    }
  } catch {
  }
}

watch(() => [conversationId.value, props.messages, projectionContributions.value] as const, rebuildConversationEventLog, { deep: true });
watch(conversationId, (id) => {
  void loadDurableConversationEventWindow(id);
  void loadClientRuntimeSession(id);
}, { immediate: true });

function messageContext(
  msg: any,
  presentation?: {
    showAssistantIdentity?: boolean;
    isAssistantContinuation?: boolean;
    compactAfter?: boolean;
  },
) {
  const messageType = msg.msgType || msg.msg_type || msg.type || "text";
  return {
    messageId: msg.id,
    type: messageType,
    messageType,
    extensionType: msg.extensionType || msg.extension_type || "",
    role: msg.role,
    status: msg.status,
    content: msg.content || msg.text || "",
    altText: msg.altText || msg.alt_text || "",
    imageUrl: msg.imageUrl || msg.image_url || "",
    videoUrl: msg.videoUrl || msg.video_url || "",
    audioUrl: msg.audioUrl || msg.audio_url || "",
    originalAssetReference: msg.originalAssetReference || msg.original_asset_reference || "",
    fallbackAssetReference: msg.fallbackAssetReference || msg.fallback_asset_reference || "",
    isAnimated: !!(msg.isAnimated || msg.is_animated),
    width: msg.width || msg.media_width || 0,
    height: msg.height || msg.media_height || 0,
    attachments: msg.attachments || [],
    metadata: msg.metadata || {},
    characterId: messageCharacterId(msg),
    characterName: messageCharacterName(msg),
    characterAvatar: messageCharacterAvatar(msg),
    showAssistantIdentity: presentation?.showAssistantIdentity !== false,
    isAssistantContinuation: presentation?.isAssistantContinuation === true,
    compactBottom: presentation?.compactAfter === true,
  };
}

function messageSlotContext(
  msg: any,
  presentation?: {
    showAssistantIdentity?: boolean;
    isAssistantContinuation?: boolean;
    compactAfter?: boolean;
  },
) {
  const messageType = msg.msgType || msg.msg_type || msg.type || "text";
  return {
    ...(props.extensionContext ?? {}),
    messageId: msg.id,
    messageType,
    direction: msg.role === "user" ? "outgoing" : msg.role === "assistant" ? "incoming" : "system",
    senderType: msg.role === "user" ? "user" : msg.role === "assistant" ? "character" : "system",
    characterId: messageCharacterId(msg),
    conversationId: msg.conversationId || conversationId.value,
    message: messageContext(msg, presentation),
    showAssistantIdentity: presentation?.showAssistantIdentity !== false,
    isAssistantContinuation: presentation?.isAssistantContinuation === true,
    compactBottom: presentation?.compactAfter === true,
  };
}

function hasMessageSlotRenderer(msg: any): boolean {
  browserClientPluginRuntime.slots.revision.value;
  const slotId = "chat.message.renderer";
  const contract = store.slotsById.get(slotId) ?? browserClientPluginRuntime.slots.getDefinition(slotId);
  return hasUnifiedSlotItem(
    contract,
    store.getVisibleContributions(slotId, messageSlotContext(msg)),
    browserClientPluginRuntime.slots.listContributions(slotId),
  );
}

function messageRendererId(msg: any): string | undefined {
  return resolveMessageRenderer(
    store.getProviders("conversation.message_renderer"),
    store.getResolvedProvider("conversation.message_renderer"),
    msg,
    store.snapshot?.providerContext,
  )?.providerId;
}

function messageActions(msg: any) {
  return {
    ...(props.providerActions ?? {}),
    "conversation.retry": async () => emit("retry", msg),
    "conversation.reply": async () => emit("reply", msg),
    "conversation.edit": async () => emit("edit", msg),
    "conversation.scrollToMessage": async (input?: unknown) => scrollToMessage(String((input as any)?.messageId ?? input ?? msg.id)),
  };
}

function messageExtensionSummary(msg: any) {
  const messageType = msg.msgType || msg.msg_type || msg.type || "text";
  return {
    messageId: msg.id,
    type: messageType,
    messageType,
    direction: msg.role === "user" ? "outgoing" : msg.role === "assistant" ? "incoming" : "system",
    senderType: msg.role === "user" ? "user" : msg.role === "assistant" ? "character" : "system",
    createdAt: msg.createdAt || "",
    status: msg.status || "sent",
    hasText: !!(msg.content || msg.text),
    attachmentTypes: msg.attachments?.map((a: any) => a.type) || [],
    extensionType: msg.extensionType || msg.extension_type || "",
    content: msg.content || msg.text || "",
    metadata: msg.metadata || {},
  } as const;
}

function finiteNumber(value: unknown): number | undefined {
  const number = Number(value);
  return Number.isFinite(number) ? number : undefined;
}

function finishMessageEntrance(message: any, event: AnimationEvent) {
  if (event.target !== event.currentTarget) return;
  message.animateIn = false;
}

function scrollToMessage(messageId: string) {
  if (!rootEl.value) return;
  const el = rootEl.value.querySelector(`[data-message-id="${messageId}"]`);
  if (el) {
    el.scrollIntoView({ behavior: "smooth", block: "center" });
    el.classList.add("highlight-flash");
    setTimeout(() => el.classList.remove("highlight-flash"), 2000);
  }
}

onMounted(() => {
  if (!store.snapshot) void store.refreshSnapshot();
  rebuildConversationEventLog();
  unsubscribeConversationEvents = subscribeConversationRuntimeEvents(consumeConversationEvent);
  unsubscribeConversationDefinitions = subscribeProgrammaticConversationNodeDefinitions(rebuildConversationEventLog);
});

onBeforeUnmount(() => {
  unsubscribeConversationEvents?.();
  unsubscribeConversationDefinitions?.();
  stopCanonicalConversationStream?.();
  stopCanonicalConversationStream = null;
});
defineExpose({ rootEl });
</script>

<style scoped>
.messages-area {
  width: 100%;
  margin: 0 auto;
  flex: 1 1 0;
  min-height: 0;
  overflow-y: scroll;
  overscroll-behavior-y: contain;
  padding: 0 32px 28px;
  position: relative;
}

.empty-chat {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 100%;
  text-align: center;
  padding: 40px 20px;
  margin: auto 0;
}

.empty-icon {
  color: var(--ac-color-text-muted);
  margin-bottom: 14px;
  opacity: 0.58;
}

.empty-text {
  margin-bottom: 7px;
  color: var(--text-primary);
  font-size: 18px;
  font-weight: 520;
  letter-spacing: -0.2px;
}
.empty-text--project {
  font-weight: 650;
}

.empty-hint {
  max-width: 360px;
  color: var(--text-muted);
  font-size: 12px;
}

.empty-chat :deep(.extension-slot) { width: min(100%, 680px); margin-top: 20px; }



.messages-area > [data-message-id] { width: min(100%, 820px); margin: 0 auto; }

.conversation-flow-item--assistant {
  margin-bottom: 0;
}

.conversation-role-switch {
  width: min(100%, 820px);
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 2px auto 20px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
  line-height: 1.4;
}

.conversation-role-switch::before,
.conversation-role-switch::after {
  content: "";
  flex: 1 1 0;
  height: 1px;
  background: var(--ac-color-border);
}

.conversation-role-switch--pending {
  margin-top: 12px;
}

.conversation-role-switch span {
  flex: 0 0 auto;
  text-align: center;
}

.conversation-flow-item--entering { animation: conversationMessageIn 0.25s ease-out; }

@keyframes conversationMessageIn {
  from { opacity: 0; transform: translateY(6px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (max-width: 768px) { .messages-area { padding: 12px 8px; } }

.scroll-btn {
  position: sticky;
  bottom: 8px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10;
  box-shadow: var(--ac-shadow-md);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--ac-transition-fast);
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.pull-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 0;
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  transition: opacity var(--ac-transition-fast);
  opacity: 0;
  height: 0;
  overflow: hidden;
}

.pull-indicator.pulling {
  opacity: 0.6;
  height: 36px;
  padding: 8px 0;
}
.pull-indicator.ready {
  opacity: 1;
  color: var(--ac-color-primary);
  height: 36px;
  padding: 8px 0;
}

.pull-icon.spin {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 768px) {
  .messages-area {
    padding: 12px 12px;
  }
}

.highlight-flash {
  animation: highlight-fade 2s ease-out;
}

@keyframes highlight-fade {
  0% {
    background-color: var(--tp-primary-light-9);
  }
  100% {
    background-color: transparent;
  }
}
</style>
