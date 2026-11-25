<template>
  <section class="host-channel-messages" data-amitia-ui="channels.host-messages">
    <header class="host-messages-heading">
      <div>
        <h2>{{ channelLabel }}</h2>
        <p>展示宿主已保存的渠道会话和消息。</p>
      </div>
      <el-button :loading="loading" @click="loadConversations()">刷新</el-button>
    </header>

    <el-alert v-if="error" type="warning" show-icon :closable="false"><pre class="host-load-error">{{ error }}</pre></el-alert>

    <div class="host-message-stats">
      <div class="host-stat-card"><span>已加载会话</span><strong>{{ conversations.length }}</strong></div>
      <div class="host-stat-card"><span>群聊 / 私聊</span><strong>{{ groupCount }} / {{ privateCount }}</strong></div>
      <div class="host-stat-card"><span>已加载会话的消息数</span><strong>{{ totalMessages }}</strong></div>
      <div class="host-stat-card"><span>最近活动</span><strong>{{ lastActivity }}</strong></div>
    </div>

    <div v-if="conversations.length" class="host-message-layout" :class="{ 'host-message-layout--single': singleConversation }">
      <aside v-if="!singleConversation" class="host-conversation-list" aria-label="渠道会话">
        <button
          v-for="conversation in conversations"
          :key="conversation.id"
          type="button"
          class="host-conversation-item"
          :class="{ active: conversation.id === selectedId }"
          @click="selectConversation(conversation)"
        >
          <span class="host-conversation-title">
            <span class="host-peer-kind" :data-kind="conversation.peerKind || 'unknown'">{{ peerKindLabel(conversation) }}</span>
            <strong>{{ conversation.title || conversation.peerId || "未命名会话" }}</strong>
          </span>
          <small>{{ conversation.messageCount || 0 }} 条 · {{ conversation.updatedAt || "—" }}</small>
        </button>
        <ConversationListEnd :enabled="canLoadNext" @reach="loadConversations(true)" />
      </aside>
      <div class="host-message-pane">
        <template v-if="selectedConversation">
          <header class="host-conversation-header">
            <strong>
              <span class="host-peer-kind" :data-kind="selectedConversation.peerKind || 'unknown'">{{ peerKindLabel(selectedConversation) }}</span>
              {{ selectedConversation.title || selectedConversation.peerId }}
            </strong>
            <span>{{ selectedConversation.messageCount || 0 }} 条消息</span>
          </header>
          <div ref="messageListRef" v-loading="messageLoading" class="host-message-list">
            <UIProviderHost
              v-for="message in messages"
              :key="message.id"
              capability="conversation.message_renderer"
              :provider-id="messageRendererId(message)"
              :fallback="ChannelMessageBubble"
              :context="messageContext(message)"
              :message="channelMessage(message)"
            />
            <el-empty v-if="!messageLoading && messages.length === 0" description="暂无消息" :image-size="60" />
          </div>
        </template>
        <el-empty v-else description="选择渠道会话查看消息" :image-size="70" />
      </div>
    </div>
    <ConversationListEnd v-else-if="listCursor" :enabled="canLoadNext" @reach="loadConversations(true)" />
    <el-empty v-else-if="!loading && !error" description="宿主中暂无该渠道的消息" :image-size="72" />
  </section>
</template>

<script setup lang="ts">
import ConversationListEnd from "@/components/ConversationListEnd.vue";
import { originalErrorText } from "@/runtime/original-error";
import { computed, onMounted, ref, watch } from "vue";
import UIProviderHost from "@/components/ui-runtime/UIProviderHost.vue";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { useChannelPresentations } from "@/ui-runtime/channelPresentation";
import { resolveMessageRenderer } from "@/ui-runtime/messageRendererRegistry";
import { buildChannelConversationGroups } from "./channelConversationGroups";
import { normalizeChannelMessage, resolveConversationChannelId } from "./channelMessageMedia";
import ChannelMessageBubble from "./ChannelMessageBubble.vue";

interface HostConversation {
  peerKind: string;
  id: string;
  title: string;
  peerId: string;
  messageCount: number;
  updatedAt: string;
  characterId?: string;
  channel: string;
}

const props = defineProps<{ channelId: string }>();
const owned = useDeviceOwnedConversation();
const store = useExtensionUIStore();
const { presentations, byChannel } = useChannelPresentations();
const conversations = ref<HostConversation[]>([]);
const messages = ref<any[]>([]);
const selectedId = ref("");
const loading = ref(false);
const messageLoading = ref(false);
const error = ref("");
const messageListRef = ref<HTMLElement | null>(null);
let generation = 0;

const singleConversation = computed(() => Boolean(
  presentations.value.find((item) => item.channelId === props.channelId)?.singleConversation,
));

const channelLabel = computed(() =>
  presentations.value.find((item) => item.channelId === props.channelId)?.displayName || props.channelId || "渠道",
);
const selectedConversation = computed(() => conversations.value.find((item) => item.id === selectedId.value) || null);
const groupCount = computed(() => conversations.value.filter((item) => item.peerKind === "group").length);
const privateCount = computed(() => conversations.value.filter((item) => item.peerKind !== "group").length);

const totalMessages = computed(() => conversations.value.reduce((total, item) => total + Number(item.messageCount || 0), 0));
const lastActivity = computed(() => conversations.value
  .map((item) => item.updatedAt)
  .filter(Boolean)
  .sort((left, right) => right.localeCompare(left))[0] || "—");

function channelMessage(message: any) {
  return normalizeChannelMessage(message, selectedConversation.value?.channel || props.channelId);
}

function messageContext(message: any) {
  return {
    channelId: props.channelId,
    conversationId: selectedConversation.value?.id || "",
    readOnly: true,
    message: channelMessage(message),
  };
}

function messageRendererId(message: any) {
  return resolveMessageRenderer(
    store.getProviders("conversation.message_renderer"),
    store.getResolvedProvider("conversation.message_renderer"),
    channelMessage(message),
    store.snapshot?.providerContext,
  )?.providerId;
}

function resolvePeerKind(item: Record<string, unknown>): string {
  const explicit = String(item.peerKind || "").trim();
  if (explicit === "group" || explicit === "private") return explicit;
  return "";
}

function peerKindLabel(conversation: HostConversation): string {
  if (conversation.peerKind === "group") return "群聊";
  if (conversation.peerKind === "private") return "私聊";
  return "单聊";
}

const listCursor = ref<Awaited<ReturnType<typeof owned.conversationPage>>["cursor"]>();
const canLoadNext = computed(() => Boolean(listCursor.value) && !loading.value && !error.value && (!singleConversation.value || !conversations.value.length));

async function loadConversations(append = false) {
  if (append && !canLoadNext.value) return;
  if (!append) listCursor.value = undefined;
  const ticket = ++generation;
  loading.value = true;
  error.value = "";
  try {
    if (!props.channelId) throw new Error("无法识别当前渠道");
    if (!append && !await owned.refresh()) throw new Error(owned.notice.value || "设备消息服务尚未就绪");
    const role = owned.selectInitialRole(localStorage.getItem(`webchat-char-id:${owned.coreId.value}:${owned.roleOwnerId.value}`) || "");
    const page = await owned.conversationPage(role, "", append ? listCursor.value : undefined, { channel: props.channelId });
    const groups = buildChannelConversationGroups(presentations.value, page.rows);
    if (ticket !== generation) return;
    const group = groups.find((item) => item.channel === props.channelId);
    const items = singleConversation.value ? (group?.items || []).slice(0, 1) : (group?.items || []);
    const next = items.map((item: Record<string, unknown>) => ({
      id: String(item.id || ""),
      peerKind: resolvePeerKind(item),
      title: String(item.title || item.displayName || "未命名会话"),
      peerId: String(item.peerId || item.externalConversationId || item.id || "—"),
      messageCount: Number(item.messageCount || 0),
      updatedAt: String(item.updatedAt || item.lastMessageAt || "—"),
      characterId: String(item.characterId || ""),
      channel: String(item.channel || props.channelId),
    }));
    const rows = new Map((append ? conversations.value : []).map((row) => [row.id, row]));
    for (const row of next) rows.set(row.id, row);
    conversations.value = [...rows.values()];
    listCursor.value = page.cursor;
    if (selectedId.value && !conversations.value.some((item) => item.id === selectedId.value)) {
      selectedId.value = "";
      messages.value = [];
    }
    if (!selectedId.value && conversations.value[0]) await selectConversation(conversations.value[0]);
  } catch (cause) {
    if (ticket === generation) {
      error.value = originalErrorText(cause);
    }
  } finally {
    if (ticket === generation) loading.value = false;
  }
}

async function selectConversation(conversation: HostConversation) {
  selectedId.value = conversation.id;
  messageLoading.value = true;
  try {
    messages.value = await owned.allMessages(conversation.id, conversation.characterId || owned.selectInitialRole());
    requestAnimationFrame(() => {
      if (messageListRef.value) messageListRef.value.scrollTop = messageListRef.value.scrollHeight;
    });
  } catch (cause) {
    messages.value = [];
    error.value = cause instanceof Error ? cause.message : "宿主消息加载失败";
  } finally {
    messageLoading.value = false;
  }
}

onMounted(() => void loadConversations());
watch(() => props.channelId, () => {
  conversations.value = [];
  messages.value = [];
  selectedId.value = "";
  void loadConversations();
});
</script>

<style scoped>
.host-channel-messages { display: flex; flex-direction: column; min-height: 0; height: 100%; overflow: hidden; padding: 16px 20px 24px; }
.host-channel-messages > .el-alert { flex: 0 0 auto; }
.host-messages-heading { flex: 0 0 auto; display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 14px; }
.host-messages-heading h2 { margin: 0; font-size: 18px; }
.host-messages-heading p { margin: 5px 0 0; color: var(--text-secondary); font-size: var(--ac-font-size-sm); }
.host-message-stats { flex: 0 0 auto; display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; margin-bottom: 14px; }
.host-stat-card { display: grid; gap: 7px; padding: 14px; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-md); background: var(--surface-bg); }
.host-stat-card span { color: var(--text-muted); font-size: var(--ac-font-size-xs); }
.host-stat-card strong { color: var(--text-primary); font-size: 20px; font-weight: 650; overflow-wrap: anywhere; }
.host-load-error { white-space: pre-wrap; overflow-wrap: anywhere; user-select: text; max-height: 12rem; overflow-y: auto; }
.host-message-layout { flex: 1 1 auto; display: grid; grid-template-columns: minmax(220px, 300px) minmax(0, 1fr); min-height: 0; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-md); overflow: hidden; background: var(--surface-bg); }
.host-message-layout--single { grid-template-columns: minmax(0, 1fr); }
.host-conversation-list { min-height: 0; overflow-y: auto; padding: 8px; border-right: 1px solid var(--surface-border); background: var(--workbench-bg); }
.host-conversation-item { display: grid; gap: 4px; width: 100%; padding: 10px; border: 0; border-radius: 8px; background: transparent; color: var(--text-secondary); cursor: pointer; font: inherit; text-align: left; }
.host-conversation-item:hover, .host-conversation-item.active { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.host-conversation-title { display: flex; align-items: center; gap: 6px; min-width: 0; }
.host-conversation-item strong, .host-conversation-item small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.host-peer-kind { flex-shrink: 0; padding: 1px 7px; border-radius: 999px; background: color-mix(in srgb, var(--ac-color-primary) 12%, transparent); color: var(--ac-color-primary); font-size: 10px; font-weight: 600; }
.host-peer-kind[data-kind="private"] { background: color-mix(in srgb, var(--ac-color-success, #16a34a) 14%, transparent); color: var(--ac-color-success, #16a34a); }
.host-conversation-header strong { display: flex; align-items: center; gap: 6px; min-width: 0; }
.host-conversation-item strong { font-size: var(--ac-font-size-sm); }
.host-conversation-item small { color: var(--text-muted); font-size: 11px; }
.host-message-pane { min-width: 0; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.host-conversation-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 46px; padding: 8px 14px; border-bottom: 1px solid var(--surface-border); }
.host-conversation-header span { color: var(--text-muted); font-size: var(--ac-font-size-xs); }
.host-message-list { min-height: 0; flex: 1 1 auto; overflow-y: auto; overflow-x: hidden; display: flex; flex-direction: column; gap: 10px; padding: 14px; }
@media (max-width: 720px) {
  .host-channel-messages { padding: 14px; }
  .host-message-stats { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .host-load-error { white-space: pre-wrap; overflow-wrap: anywhere; user-select: text; max-height: 12rem; overflow-y: auto; }
.host-message-layout { grid-template-columns: 1fr; }
  .host-conversation-list { max-height: 200px; border-right: 0; border-bottom: 1px solid var(--surface-border); }
  .host-message-pane { min-height: 0; }
}
</style>
