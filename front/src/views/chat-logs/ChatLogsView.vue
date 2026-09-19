<template>
  <div class="archive-page">
    <aside class="archive-list">
      <div class="archive-list-header">
        <strong>归档对话</strong>
        <el-button text :loading="loading" aria-label="刷新归档对话" title="刷新归档对话" @click="loadConversations">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </div>
      <div
        v-for="conversation in conversations"
        :key="conversation.id"
        class="archive-item"
        :class="{ active: conversation.id === selectedId }"
      >
        <button type="button" class="archive-item-main" @click="selectConversation(conversation)">
          <el-icon><Box /></el-icon>
          <span class="archive-item-copy">
            <strong>{{ conversation.title || "新对话" }}</strong>
            <small>{{ formatTime(conversation.archivedAt) }} · {{ conversation.messageCount || 0 }} 条</small>
          </span>
        </button>
        <el-button
          text
          size="small"
          :loading="restoringId === conversation.id"
          @click.stop="restoreConversation(conversation)"
        >
          撤销
        </el-button>
      </div>
      <el-empty v-if="!loading && conversations.length === 0" description="暂无归档对话" :image-size="60" />
    </aside>

    <section class="archive-main">
      <template v-if="selectedConversation">
        <header class="archive-main-header">
          <div>
            <strong>{{ selectedConversation.title || "新对话" }}</strong>
            <small>归档于 {{ formatTime(selectedConversation.archivedAt) }}</small>
          </div>
          <el-button
            type="primary"
            plain
            size="small"
            :loading="restoringId === selectedConversation.id"
            @click="restoreConversation(selectedConversation)"
          >
            撤销归档
          </el-button>
        </header>
        <div ref="messageListRef" v-loading="messageLoading" class="message-list">
          <div v-for="message in messages" :key="message.id" class="archive-message" :class="message.role">
            <div class="message-meta">{{ message.role === "user" ? "用户" : "AI" }} · {{ formatTime(message.createdAt) }}</div>
            <div class="message-content">{{ message.content }}</div>
          </div>
          <el-empty v-if="!messageLoading && messages.length === 0" description="暂无消息" :image-size="60" />
        </div>
      </template>
      <el-empty v-else description="选择归档对话查看消息" :image-size="80" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Box, Refresh } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { useApi } from "@/composables/useApi";

interface ArchivedConversation {
  id: string;
  title: string;
  messageCount: number;
  archivedAt?: string;
}

interface ArchivedMessage {
  id: string;
  role: string;
  content: string;
  createdAt: string;
}

const { get, put } = useApi();
const conversations = ref<ArchivedConversation[]>([]);
const messages = ref<ArchivedMessage[]>([]);
const selectedId = ref("");
const loading = ref(false);
const messageLoading = ref(false);
const restoringId = ref("");
const messageListRef = ref<HTMLElement | null>(null);

const selectedConversation = computed(
  () => conversations.value.find((item) => item.id === selectedId.value) || null,
);

function formatTime(value?: string) {
  if (!value) return "未知时间";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

async function loadConversations() {
  loading.value = true;
  try {
    const response = await get<any>("/api/web-chat/conversations", {
      page: 1,
      pageSize: 200,
      archivedOnly: true,
    });
    conversations.value = response?.items || [];
    if (selectedId.value && !conversations.value.some((item) => item.id === selectedId.value)) {
      selectedId.value = "";
      messages.value = [];
    }
  } finally {
    loading.value = false;
  }
}

async function selectConversation(conversation: ArchivedConversation) {
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

async function restoreConversation(conversation: ArchivedConversation) {
  if (restoringId.value) return;
  restoringId.value = conversation.id;
  try {
    await put(`/api/web-chat/conversations/${encodeURIComponent(conversation.id)}`, {
      archived: false,
    });
    conversations.value = conversations.value.filter((item) => item.id !== conversation.id);
    if (selectedId.value === conversation.id) {
      selectedId.value = "";
      messages.value = [];
    }
    ElMessage.success("对话已恢复");
  } finally {
    restoringId.value = "";
  }
}

onMounted(loadConversations);
</script>

<style scoped>
.archive-page { display: grid; grid-template-columns: 300px minmax(0, 1fr); height: 100%; min-height: 0; background: var(--surface-bg); }
.archive-list { display: flex; flex-direction: column; min-height: 0; overflow-y: auto; padding: 10px; border-right: 1px solid var(--surface-border); }
.archive-list-header, .archive-main-header { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.archive-list-header { min-height: 36px; padding: 0 6px 8px; }
.archive-item { display: flex; align-items: center; gap: 4px; width: 100%; min-height: 52px; padding: 2px 4px 2px 2px; border-radius: 8px; color: var(--text-secondary); }
.archive-item:hover, .archive-item.active { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.archive-item.active { background: var(--workbench-sidebar-active); }
.archive-item-main { display: flex; align-items: center; gap: 9px; min-width: 0; flex: 1; min-height: 48px; padding: 5px 6px; border: 0; border-radius: 8px; background: transparent; color: inherit; cursor: pointer; font: inherit; text-align: left; }
.archive-item-main:focus-visible { outline: 1px solid var(--ac-color-primary); outline-offset: -1px; }
.archive-item-copy { min-width: 0; flex: 1; }
.archive-item strong, .archive-item small, .archive-main-header strong, .archive-main-header small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.archive-item strong, .archive-main-header strong { font-size: 13px; }
.archive-item small, .archive-main-header small, .message-meta { margin-top: 2px; color: var(--text-muted); font-size: 10px; }
.archive-main { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.archive-main-header { min-height: 52px; padding: 8px 16px; border-bottom: 1px solid var(--surface-border); }
.message-list { display: flex; flex-direction: column; gap: 12px; min-height: 0; flex: 1; overflow-y: auto; padding: 18px; }
.archive-message { max-width: min(680px, 82%); padding: 9px 12px; border-radius: 10px; background: var(--ac-color-surface); }
.archive-message.user { align-self: flex-end; background: var(--ac-color-primary-bg); }
.message-content { white-space: pre-wrap; word-break: break-word; color: var(--text-primary); font-size: 13px; line-height: 1.55; }
@media (max-width: 800px) {
  .archive-page { grid-template-columns: 1fr; }
  .archive-list { max-height: 38vh; border-right: 0; border-bottom: 1px solid var(--surface-border); }
}
</style>
