<template>
  <div class="channel-page">
    <aside class="channel-list">
      <div class="channel-list-header">
        <strong>渠道消息</strong>
        <el-button text :loading="loading" aria-label="刷新渠道消息" title="刷新渠道消息" @click="loadConversations">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </div>
      <section v-for="group in channelGroups" :key="group.channel" class="channel-group">
        <div class="channel-group-title">
          <el-icon><Connection /></el-icon>
          <span>{{ group.label }}</span>
          <small>{{ group.items.length }}</small>
        </div>
        <div v-if="group.items.length === 0" class="channel-group-empty">暂无会话</div>
        <button
          v-for="conversation in group.items"
          :key="conversation.id"
          type="button"
          class="channel-item"
          :class="{ active: conversation.id === selectedId }"
          @click="selectConversation(conversation)"
        >
          <span>
            <strong>{{ conversation.title || group.label }}</strong>
            <small>{{ group.label }} · {{ conversation.messageCount || 0 }} 条</small>
          </span>
        </button>
      </section>
      <el-empty v-if="!loading && channelGroups.length === 0" description="暂无渠道消息" :image-size="60" />
    </aside>

    <section class="channel-main">
      <template v-if="selectedConversation">
        <header class="channel-main-header">
          <div>
            <strong>{{ selectedConversation.title || channelLabel(selectedConversation.channel) }}</strong>
            <small>{{ selectedConversation.peerId || selectedConversation.id }}</small>
          </div>
          <el-tag type="info" size="small">{{ channelLabel(selectedConversation.channel) }}</el-tag>
        </header>
        <div ref="messageListRef" class="message-list">
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
      <el-empty v-else description="选择渠道会话查看消息" :image-size="80" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { Connection, Refresh } from "@element-plus/icons-vue";
import { useRouter } from "vue-router";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import UIProviderHost from "@/components/ui-runtime/UIProviderHost.vue";
import { resolveMessageRenderer } from "@/ui-runtime/messageRendererRegistry";
import { useChannelPresentations } from "@/ui-runtime/channelPresentation";
import { normalizeChannelMessage, resolveConversationChannelId } from "./channelMessageMedia";
import { buildChannelConversationGroups } from "./channelConversationGroups";
import ChannelMessageBubble from "./ChannelMessageBubble.vue";

const owned = useDeviceOwnedConversation();
const router = useRouter();
const { presentations, byChannel, store } = useChannelPresentations();
const conversations = ref<any[]>([]);
const messages = ref<any[]>([]);
const selectedId = ref("");
const loading = ref(false);
const messageLoading = ref(false);
const messageListRef = ref<HTMLElement | null>(null);

function channelLabel(channel: string) {
  return byChannel.value.get(channel)?.displayName || channel || "渠道";
}

function channelMessage(message: any) {
  return normalizeChannelMessage(message, selectedConversation.value?.channel || "");
}

const channelGroups = computed(() => buildChannelConversationGroups(presentations.value, conversations.value));

function messageContext(message: any) {
  return {
    channelId: selectedConversation.value?.channel || "",
    conversationId: selectedConversation.value?.id || "",
    readOnly: true,
    message: channelMessage(message),
  };
}

function messageRendererId(message: any) {
  const payload = channelMessage(message);
  return resolveMessageRenderer(
    store.getProviders("conversation.message_renderer"),
    store.getResolvedProvider("conversation.message_renderer"),
    payload,
    store.snapshot?.providerContext,
  )?.providerId;
}

const selectedConversation = computed(
  () => conversations.value.find((item) => item.id === selectedId.value) || null,
);

async function loadConversations() {
  loading.value = true;
  try {
    if (!await owned.refresh()) throw new Error(owned.notice.value || "设备对话服务尚未就绪");
    const role = owned.selectInitialRole(localStorage.getItem(`webchat-char-id:${owned.coreId.value}:${owned.roleOwnerId.value}`) || "");
    conversations.value = (await owned.conversations(role))
      .map((item) => ({ ...item, channel: resolveConversationChannelId(item, new Set(byChannel.value.keys())) }))
      .filter((item) => item.channel && item.channel !== "web");
    if (selectedId.value && !conversations.value.some((item) => item.id === selectedId.value)) {
      selectedId.value = "";
      messages.value = [];
    }
  } finally {
    loading.value = false;
  }
}

async function selectConversation(conversation: any) {
  selectedId.value = conversation.id;
  messageLoading.value = true;
  try {
    messages.value = await owned.allMessages(conversation.id, conversation.characterId || owned.selectInitialRole());
    requestAnimationFrame(() => {
      if (messageListRef.value) messageListRef.value.scrollTop = messageListRef.value.scrollHeight;
    });
  } finally {
    messageLoading.value = false;
  }
}

async function ensureChannelEntryAvailable() {
  if (!store.snapshot) await store.refreshSnapshot(true).catch(() => {});
  if (store.snapshot && presentations.value.length === 0) {
    await router.replace({ path: "/chat" });
  }
}

watch(presentations, (items) => {
  if (store.snapshot && items.length === 0) {
    void router.replace({ path: "/chat" });
  }
});

onMounted(() => {
  void ensureChannelEntryAvailable();
  void loadConversations();
});
</script>

<style scoped>
.channel-page { display: grid; grid-template-columns: 300px minmax(0, 1fr); height: 100%; min-height: 0; background: var(--surface-bg); }
.channel-list { display: flex; flex-direction: column; min-height: 0; overflow-y: auto; padding: 10px; border-right: 1px solid var(--surface-border); }
.channel-list-header, .channel-main-header { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.channel-list-header { min-height: 36px; padding: 0 6px 8px; }
.channel-group + .channel-group { margin-top: 8px; }
.channel-group-title { display: flex; align-items: center; gap: 6px; padding: 5px 8px; color: var(--text-muted); font-size: 11px; }
.channel-group-title span { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.channel-group-title small { font-size: 10px; }
.channel-group-empty { padding: 4px 8px 8px 30px; color: var(--text-muted); font-size: 11px; }
.channel-item { display: flex; align-items: flex-start; gap: 9px; width: 100%; min-height: 48px; padding: 8px; border: 0; border-radius: 8px; background: transparent; color: var(--text-secondary); cursor: pointer; text-align: left; }
.channel-item:hover, .channel-item.active { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.channel-item.active { background: var(--workbench-sidebar-active); }
.channel-item > span { min-width: 0; flex: 1; }
.channel-item strong, .channel-item small, .channel-main-header strong, .channel-main-header small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.channel-item strong, .channel-main-header strong { font-size: 13px; }
.channel-item small, .channel-main-header small, .message-meta { margin-top: 2px; color: var(--text-muted); font-size: 10px; }
.channel-main { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.channel-main-header { min-height: 52px; padding: 8px 16px; border-bottom: 1px solid var(--surface-border); }
.message-list { display: flex; flex-direction: column; gap: 12px; min-height: 0; flex: 1; overflow-y: auto; padding: 18px; }
@media (max-width: 800px) {
  .channel-page { grid-template-columns: 1fr; }
  .channel-list { max-height: 38vh; border-right: 0; border-bottom: 1px solid var(--surface-border); }
}
</style>
