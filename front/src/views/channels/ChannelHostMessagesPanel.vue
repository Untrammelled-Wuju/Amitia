<template>
  <section class="host-channel-messages" data-amitia-ui="channels.host-messages">
    <header class="host-messages-heading">
      <div>
        <h2>{{ channelLabel }}</h2>
        <p>展示宿主已保存的渠道会话和消息。</p>
      </div>
      <el-button :loading="loading" @click="loadConversations">刷新</el-button>
    </header>

    <el-alert v-if="error" :title="error" type="warning" show-icon :closable="false" />

    <div class="host-message-stats">
      <div class="host-stat-card"><span>会话数</span><strong>{{ conversations.length }}</strong></div>
      <div class="host-stat-card"><span>宿主消息数</span><strong>{{ totalMessages }}</strong></div>
      <div class="host-stat-card"><span>最近活动</span><strong>{{ lastActivity }}</strong></div>
    </div>

    <div v-if="conversations.length" class="host-message-layout">
      <aside class="host-conversation-list" aria-label="渠道会话">
        <button
          v-for="conversation in conversations"
          :key="conversation.id"
          type="button"
          class="host-conversation-item"
          :class="{ active: conversation.id === selectedId }"
          @click="selectConversation(conversation)"
        >
          <strong>{{ conversation.title || conversation.peerId || "未命名会话" }}</strong>
          <small>{{ conversation.messageCount || 0 }} 条 · {{ conversation.updatedAt || "—" }}</small>
        </button>
      </aside>
      <div class="host-message-pane">
        <template v-if="selectedConversation">
          <header class="host-conversation-header">
            <strong>{{ selectedConversation.title || selectedConversation.peerId }}</strong>
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
    <el-empty v-else-if="!loading" description="宿主中暂无该渠道的消息" :image-size="72" />
  </section>
</template>

<script setup lang="ts">
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

const channelLabel = computed(() =>
  presentations.value.find((item) => item.channelId === props.channelId)?.displayName || props.channelId || "渠道",
);
const selectedConversation = computed(() => conversations.value.find((item) => item.id === selectedId.value) || null);
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

async function loadConversations() {
  const ticket = ++generation;
  loading.value = true;
  error.value = "";
  try {
    if (!props.channelId) throw new Error("无法识别当前渠道");
    if (!await owned.refresh()) throw new Error(owned.notice.value || "设备消息服务尚未就绪");
    const role = owned.selectInitialRole(localStorage.getItem(`webchat-char-id:${owned.coreId.value}:${owned.roleOwnerId.value}`) || "");
    const groups = buildChannelConversationGroups(presentations.value, await owned.conversations(role));
    if (ticket !== generation) return;
    const group = groups.find((item) => item.channel === props.channelId);
    conversations.value = (group?.items || []).map((item: Record<string, unknown>) => ({
      id: String(item.id || ""),
      title: String(item.title || item.displayName || "未命名会话"),
      peerId: String(item.peerId || item.externalConversationId || item.id || "—"),
      messageCount: Number(item.messageCount || 0),
      updatedAt: String(item.updatedAt || item.lastMessageAt || "—"),
      characterId: String(item.characterId || ""),
      channel: String(item.channel || props.channelId),
    }));
    if (selectedId.value && !conversations.value.some((item) => item.id === selectedId.value)) {
      selectedId.value = "";
      messages.value = [];
    }
    if (!selectedId.value && conversations.value[0]) await selectConversation(conversations.value[0]);
  } catch (cause) {
    if (ticket === generation) {
      conversations.value = [];
      messages.value = [];
      error.value = cause instanceof Error ? cause.message : "宿主消息加载失败";
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

onMounted(loadConversations);
watch(() => props.channelId, loadConversations);
</script>

<style scoped>
.host-channel-messages { min-height: 0; height: 100%; overflow-y: auto; padding: 16px 20px 24px; }
.host-messages-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 14px; }
.host-messages-heading h2 { margin: 0; font-size: 18px; }
.host-messages-heading p { margin: 5px 0 0; color: var(--text-secondary); font-size: var(--ac-font-size-sm); }
.host-message-stats { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin-bottom: 14px; }
.host-stat-card { display: grid; gap: 7px; padding: 14px; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-md); background: var(--surface-bg); }
.host-stat-card span { color: var(--text-muted); font-size: var(--ac-font-size-xs); }
.host-stat-card strong { color: var(--text-primary); font-size: 20px; font-weight: 650; overflow-wrap: anywhere; }
.host-message-layout { display: grid; grid-template-columns: minmax(220px, 300px) minmax(0, 1fr); min-height: 420px; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-md); overflow: hidden; background: var(--surface-bg); }
.host-conversation-list { min-height: 0; overflow-y: auto; padding: 8px; border-right: 1px solid var(--surface-border); background: var(--workbench-bg); }
.host-conversation-item { display: grid; gap: 4px; width: 100%; padding: 10px; border: 0; border-radius: 8px; background: transparent; color: var(--text-secondary); cursor: pointer; font: inherit; text-align: left; }
.host-conversation-item:hover, .host-conversation-item.active { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.host-conversation-item strong, .host-conversation-item small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.host-conversation-item strong { font-size: var(--ac-font-size-sm); }
.host-conversation-item small { color: var(--text-muted); font-size: 11px; }
.host-message-pane { min-width: 0; min-height: 0; display: flex; flex-direction: column; }
.host-conversation-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 46px; padding: 8px 14px; border-bottom: 1px solid var(--surface-border); }
.host-conversation-header span { color: var(--text-muted); font-size: var(--ac-font-size-xs); }
.host-message-list { min-height: 0; flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 10px; padding: 14px; }
@media (max-width: 720px) {
  .host-channel-messages { padding: 14px; }
  .host-message-stats { grid-template-columns: 1fr; }
  .host-message-layout { grid-template-columns: 1fr; }
  .host-conversation-list { max-height: 220px; border-right: 0; border-bottom: 1px solid var(--surface-border); }
  .host-message-pane { min-height: 360px; }
}
</style>
