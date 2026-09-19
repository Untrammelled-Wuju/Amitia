<template>
  <div class="channel-page">
    <aside class="channel-list">
      <div class="channel-list-header">
        <strong>渠道消息</strong>
        <el-button text :loading="loading" aria-label="刷新渠道消息" title="刷新渠道消息" @click="loadConversations">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </div>
      <button
        v-for="conversation in conversations"
        :key="conversation.id"
        type="button"
        class="channel-item"
        :class="{ active: conversation.id === selectedId }"
        @click="selectConversation(conversation)"
      >
        <el-icon><Connection /></el-icon>
        <span>
          <strong>{{ conversation.title || channelLabel(conversation.channel) }}</strong>
          <small>{{ channelLabel(conversation.channel) }} · {{ conversation.messageCount || 0 }} 条</small>
        </span>
      </button>
      <el-empty v-if="!loading && conversations.length === 0" description="暂无渠道消息" :image-size="60" />
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
          <div v-for="message in messages" :key="message.id" class="channel-message" :class="message.role">
            <div class="message-meta">{{ message.role }} · {{ formatTime(message.createdAt) }}</div>
            <div class="message-content">{{ message.content }}</div>
          </div>
          <el-empty v-if="!messageLoading && messages.length === 0" description="暂无消息" :image-size="60" />
        </div>
      </template>
      <el-empty v-else description="选择渠道会话查看消息" :image-size="80" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Connection, Refresh } from "@element-plus/icons-vue";
import { useApi } from "@/composables/useApi";

const { get } = useApi();
const conversations = ref<any[]>([]);
const messages = ref<any[]>([]);
const selectedId = ref("");
const loading = ref(false);
const messageLoading = ref(false);
const messageListRef = ref<HTMLElement | null>(null);

function channelLabel(channel: string) {
  const labels: Record<string, string> = {
    qq: "QQ",
    wechat: "微信",
    wechat_personal: "个人微信",
  };
  return labels[channel] || channel || "渠道";
}

function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

const selectedConversation = computed(
  () => conversations.value.find((item) => item.id === selectedId.value) || null,
);

async function loadConversations() {
  loading.value = true;
  try {
    const response = await get<any>("/api/web-chat/channels");
    conversations.value = response?.items || [];
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
    const response = await get<any>(
      `/api/web-chat/conversations/${encodeURIComponent(conversation.id)}/messages`,
      { page: 1, pageSize: 200 },
    );
    messages.value = response?.items || [];
    requestAnimationFrame(() => {
      if (messageListRef.value) messageListRef.value.scrollTop = messageListRef.value.scrollHeight;
    });
  } finally {
    messageLoading.value = false;
  }
}

onMounted(loadConversations);
</script>

<style scoped>
.channel-page { display: grid; grid-template-columns: 300px minmax(0, 1fr); height: 100%; min-height: 0; background: var(--surface-bg); }
.channel-list { display: flex; flex-direction: column; min-height: 0; overflow-y: auto; padding: 10px; border-right: 1px solid var(--surface-border); }
.channel-list-header, .channel-main-header { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.channel-list-header { min-height: 36px; padding: 0 6px 8px; }
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
.channel-message { max-width: min(680px, 82%); padding: 9px 12px; border-radius: 10px; background: var(--ac-color-surface); }
.channel-message.user { align-self: flex-end; background: var(--ac-color-primary-bg); }
.message-content { white-space: pre-wrap; word-break: break-word; color: var(--text-primary); font-size: 13px; line-height: 1.55; }
@media (max-width: 800px) {
  .channel-page { grid-template-columns: 1fr; }
  .channel-list { max-height: 38vh; border-right: 0; border-bottom: 1px solid var(--surface-border); }
}
</style>
