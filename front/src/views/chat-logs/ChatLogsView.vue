<template>
  <div class="archive-page">
    <aside class="archive-list">
      <div class="archive-list-header">
        <strong>归档对话</strong>
        <el-button text :loading="loading" aria-label="刷新归档对话" title="刷新归档对话" @click="loadConversations">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </div>
      <div class="archive-filters">
        <el-select
          v-model="projectFilter"
          clearable
          placeholder="全部项目"
          @change="loadConversations"
        >
          <el-option
            v-for="project in projects"
            :key="project.id"
            :label="project.name"
            :value="project.id"
          />
        </el-select>
        <el-input
          v-model="keyword"
          clearable
          placeholder="搜索标题或消息内容"
          @input="scheduleSearch"
        />
      </div>
      <div
        v-for="conversation in conversations"
        :key="conversation.id"
        class="archive-item"
        :class="{ active: conversation.id === selectedId }"
      >
        <button type="button" class="archive-item-main" @click="selectConversation(conversation)">
          <ArchiveConversationIcon class="archive-item-icon" />
          <span class="archive-item-copy">
            <strong>{{ conversationTitle(conversation) }}</strong>
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
            <strong>{{ conversationTitle(selectedConversation) }}</strong>
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
          <ChatBubble
            v-for="message in messages"
            :key="message.id"
            :message="{ ...message, typingDone: true }"
            :char-name="characterName(message.characterId)"
            :char-avatar="characterAvatar(message.characterId)"
            :character-id="message.characterId"
            read-only
          />
          <el-empty v-if="!messageLoading && messages.length === 0" description="暂无消息" :image-size="60" />
        </div>
      </template>
      <el-empty v-else description="选择归档对话查看消息" :image-size="80" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Refresh } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { useApi } from "@/composables/useApi";
import ArchiveConversationIcon from "@/components/ArchiveConversationIcon.vue";
import ChatBubble from "@/components/ChatBubble.vue";
import { useChatStore } from "@/stores/chat";

interface ArchivedConversation {
  id: string;
  title: string;
  messageCount: number;
  archivedAt?: string;
  projectId?: string;
}

interface ArchivedMessage {
  id: string;
  role: string;
  content: string;
  createdAt: string;
  characterId?: string;
  imageUrl?: string;
  videoUrl?: string;
  audioUrl?: string;
  audioDuration?: number;
}

const { get } = useApi();
const chatStore = useChatStore();
const conversations = ref<ArchivedConversation[]>([]);
const messages = ref<ArchivedMessage[]>([]);
const projects = ref<any[]>([]);
const characters = ref<any[]>([]);
const projectFilter = ref("");
const keyword = ref("");
const selectedId = ref("");
const loading = ref(false);
const messageLoading = ref(false);
const restoringId = ref("");
const messageListRef = ref<HTMLElement | null>(null);
let searchTimer: ReturnType<typeof setTimeout> | null = null;

const selectedConversation = computed(
  () => conversations.value.find((item) => item.id === selectedId.value) || null,
);

function formatTime(value?: string) {
  if (!value) return "未知时间";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function characterName(characterId?: string) {
  return (
    characters.value.find((item) => item.id === characterId)?.name ||
    "AI"
  );
}

function characterAvatar(characterId?: string) {
  return characters.value.find((item) => item.id === characterId)?.avatar || "";
}

function conversationTitle(conversation: ArchivedConversation) {
  const title = conversation.title || "新对话";
  const project = projects.value.find(
    (item) => item.id === conversation.projectId,
  );
  return project?.name ? `${project.name} - ${title}` : title;
}

function scheduleSearch() {
  if (searchTimer) clearTimeout(searchTimer);
  searchTimer = setTimeout(loadConversations, 280);
}

async function loadFilters() {
  const [sidebar, characterList] = await Promise.all([
    get<any>("/api/web-chat/sidebar"),
    get<any[]>("/api/characters"),
  ]);
  projects.value = sidebar?.projects || [];
  characters.value = Array.isArray(characterList) ? characterList : [];
}

async function loadConversations() {
  loading.value = true;
  try {
    const response = await get<any>("/api/web-chat/conversations", {
      page: 1,
      pageSize: 200,
      archivedOnly: true,
      projectId: projectFilter.value || undefined,
      keyword: keyword.value.trim() || undefined,
    });
    conversations.value = [...(response?.items || [])].sort((left, right) => {
      const leftTime = Date.parse(left.archivedAt || "");
      const rightTime = Date.parse(right.archivedAt || "");
      const leftOrder = Number.isNaN(leftTime) ? 0 : leftTime;
      const rightOrder = Number.isNaN(rightTime) ? 0 : rightTime;
      return rightOrder - leftOrder;
    });
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
    await chatStore.restoreConversation(conversation.id);
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

onMounted(() => {
  void loadFilters();
  void loadConversations();
});

watch(
  () => chatStore.archivedRevision,
  () => {
    void loadConversations();
  },
);

onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer);
});
</script>

<style scoped>
.archive-page { display: grid; grid-template-columns: 300px minmax(0, 1fr); height: 100%; min-height: 0; background: var(--surface-bg); }
.archive-list { display: flex; flex-direction: column; min-height: 0; overflow-y: auto; padding: 10px; border-right: 1px solid var(--surface-border); }
.archive-list-header, .archive-main-header { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.archive-list-header { min-height: 36px; padding: 0 6px 8px; }
.archive-filters { display: grid; gap: 6px; padding: 0 2px 8px; }
.archive-filters :deep(.el-select), .archive-filters :deep(.el-input) { width: 100%; }
.archive-item { display: flex; align-items: center; gap: 4px; width: 100%; min-height: 52px; padding: 2px 4px 2px 2px; border-radius: 8px; color: var(--text-secondary); }
.archive-item:hover, .archive-item.active { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.archive-item.active { background: var(--workbench-sidebar-active); }
.archive-item-main { display: flex; align-items: center; gap: 9px; min-width: 0; flex: 1; min-height: 48px; padding: 5px 6px; border: 0; border-radius: 8px; background: transparent; color: inherit; cursor: pointer; font: inherit; text-align: left; }
.archive-item-main:focus-visible { outline: 1px solid var(--ac-color-primary); outline-offset: -1px; }
.archive-item-copy { min-width: 0; flex: 1; }
.archive-item-icon { width: 16px; height: 16px; flex: 0 0 auto; }
.archive-item strong, .archive-item small, .archive-main-header strong, .archive-main-header small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.archive-item strong, .archive-main-header strong { font-size: 13px; }
.archive-item small, .archive-main-header small, .message-meta { margin-top: 2px; color: var(--text-muted); font-size: 10px; }
.archive-main { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.archive-main-header { min-height: 52px; padding: 8px 16px; border-bottom: 1px solid var(--surface-border); }
.message-list { display: flex; flex-direction: column; gap: 12px; min-height: 0; flex: 1; overflow-y: auto; padding: 18px; }
@media (max-width: 800px) {
  .archive-page { grid-template-columns: 1fr; }
  .archive-list { max-height: 38vh; border-right: 0; border-bottom: 1px solid var(--surface-border); }
}
</style>
