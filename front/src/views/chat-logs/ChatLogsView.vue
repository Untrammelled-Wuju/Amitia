<template>
  <section class="chat-logs-page" data-amitia-ui="chat-logs.page">
    <header class="chat-logs-toolbar">
      <div class="toolbar-heading">
        <h1>归档对话</h1>
        <p>仅显示已归档会话。</p>
      </div>
      <el-select v-model="roleId" placeholder="选择角色" class="role-select" @change="loadConversations">
        <el-option v-for="role in owned.roles.value" :key="role.id" :label="role.name || role.id" :value="role.id" />
      </el-select>
      <el-input v-model="keyword" clearable placeholder="搜索标题" class="search-field" @keyup.enter="loadConversations" />
      <el-button :loading="loading" type="primary" @click="loadConversations">刷新归档</el-button>
    </header>

    <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" />

    <div class="chat-logs-layout">
      <aside class="conversation-list">
        <button
          v-for="conversation in conversations"
          :key="conversation.id"
          type="button"
          class="conversation-item"
          :class="{ active: selectedId === conversation.id }"
          @click="openConversation(conversation)"
        >
          <span class="conversation-main">
            <strong>{{ conversation.title || "未命名对话" }}</strong>
            <small>{{ formatDate(conversation.archivedAt || conversation.updatedAt) }}</small>
          </span>
          <span class="conversation-actions" @click.stop>
            <el-button size="small" @click="restoreConversation(conversation)">撤销归档</el-button>
            <el-button size="small" type="danger" plain @click="deleteConversation(conversation)">永久删除</el-button>
          </span>
        </button>
        <el-empty v-if="!loading && conversations.length === 0" description="暂无归档对话" :image-size="56" />
      </aside>

      <main class="archive-detail">
        <template v-if="selectedConversation">
          <header class="detail-heading">
            <div>
              <h2>{{ selectedConversation.title || "未命名对话" }}</h2>
              <p>{{ messages.length }} 条消息</p>
            </div>
            <div class="detail-actions">
              <el-button :loading="summaryLoading" @click="generateSummary">生成摘要</el-button>
              <el-button :loading="exporting" @click="exportCurrent">导出 JSON</el-button>
            </div>
          </header>
          <el-alert v-if="summaryText" :title="summaryText" type="info" :closable="false" class="summary-panel" />
          <div v-loading="messageLoading" class="message-list">
            <article v-for="message in messages" :key="message.uiKey || message.id" class="message-row" :class="message.role">
              <span class="message-role">{{ message.role === "user" ? "我" : "角色" }}</span>
              <div class="message-content">
                <p>{{ message.content }}</p>
                <small>{{ formatDate(message.createdAt) }}</small>
              </div>
            </article>
            <el-empty v-if="!messageLoading && messages.length === 0" description="暂无消息" :image-size="48" />
          </div>
        </template>
        <el-empty v-else description="从左侧选择归档对话" :image-size="64" />
      </main>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";

const owned = useDeviceOwnedConversation();
const roleId = ref("");
const keyword = ref("");
const conversations = ref<any[]>([]);
const selectedConversation = ref<any | null>(null);
const messages = ref<any[]>([]);
const loading = ref(false);
const messageLoading = ref(false);
const summaryLoading = ref(false);
const exporting = ref(false);
const summaryText = ref("");
const error = ref("");
const selectedId = ref("");

function formatDate(value: unknown) {
  const raw = String(value || "").trim();
  if (!raw) return "";
  const date = new Date(raw);
  if (Number.isNaN(date.valueOf())) return raw;
  return date.toLocaleString();
}

async function loadConversations() {
  loading.value = true;
  error.value = "";
  try {
    await owned.refresh();
    roleId.value = roleId.value || owned.selectInitialRole();
    if (!roleId.value) throw new Error("没有可用的对话角色");
    const rows = await owned.conversations(roleId.value, keyword.value);
    conversations.value = rows.filter((row) => String(row.archivedAt || "").trim());
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : String(caught);
    conversations.value = [];
  } finally {
    loading.value = false;
  }
}

async function openConversation(conversation: any) {
  selectedConversation.value = conversation;
  selectedId.value = conversation.id;
  messages.value = [];
  summaryText.value = "";
  messageLoading.value = true;
  try {
    messages.value = await owned.allMessages(conversation.id, roleId.value);
  } catch (caught) {
    ElMessage.error(caught instanceof Error ? caught.message : String(caught));
  } finally {
    messageLoading.value = false;
  }
}

async function restoreConversation(conversation: any) {
  try {
    await owned.edit("conversation", conversation.id, { archived: false }, { characterId: roleId.value });
    ElMessage.success("已撤销归档");
    if (selectedId.value === conversation.id) {
      selectedConversation.value = null;
      selectedId.value = "";
      messages.value = [];
    }
    await loadConversations();
  } catch (caught) {
    ElMessage.error(caught instanceof Error ? caught.message : String(caught));
  }
}

async function deleteConversation(conversation: any) {
  try {
    await ElMessageBox.confirm("永久删除该对话及消息，记忆会保留。此操作无法撤销。", "永久删除", { type: "warning" });
    await owned.edit("conversation", conversation.id, {}, { deleted: true, characterId: roleId.value });
    ElMessage.success("归档对话已永久删除");
    if (selectedId.value === conversation.id) {
      selectedConversation.value = null;
      selectedId.value = "";
      messages.value = [];
    }
    await loadConversations();
  } catch (caught) {
    if (caught === "cancel" || caught === "close") return;
    ElMessage.error(caught instanceof Error ? caught.message : String(caught));
  }
}

async function generateSummary() {
  if (!selectedConversation.value) return;
  summaryLoading.value = true;
  try {
    const viewId = await owned.prepareSummaryGeneration(selectedConversation.value.id, roleId.value);
    const result = await owned.generateConversationSummary(selectedConversation.value.id, viewId);
    summaryText.value = String(result.summaryText || result.summary || "");
    ElMessage.success("摘要已生成");
  } catch (caught) {
    ElMessage.error(caught instanceof Error ? caught.message : String(caught));
  } finally {
    summaryLoading.value = false;
  }
}

async function exportCurrent() {
  if (!selectedConversation.value) return;
  exporting.value = true;
  try {
    const result = await owned.exportConversation(selectedConversation.value.id, roleId.value, "json");
    const url = URL.createObjectURL(new Blob([result.content], { type: result.mimeType }));
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = result.filename;
    anchor.click();
    URL.revokeObjectURL(url);
  } catch (caught) {
    ElMessage.error(caught instanceof Error ? caught.message : String(caught));
  } finally {
    exporting.value = false;
  }
}

onMounted(loadConversations);
</script>

<style scoped>
.chat-logs-page { min-height: 100%; display: grid; grid-template-rows: auto auto minmax(0, 1fr); gap: 14px; }
.chat-logs-toolbar { display: grid; grid-template-columns: minmax(180px, 1fr) minmax(150px, 220px) minmax(160px, 280px) auto; align-items: end; gap: 10px; }
.toolbar-heading h1 { margin: 0; color: var(--text-primary); font-size: var(--ac-font-size-xl); }
.toolbar-heading p { margin: 4px 0 0; color: var(--text-muted); font-size: var(--ac-font-size-sm); }
.chat-logs-layout { min-height: 0; display: grid; grid-template-columns: minmax(260px, 360px) minmax(0, 1fr); gap: 14px; }
.conversation-list, .archive-detail { min-height: 0; overflow: auto; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-md); background: var(--ac-color-surface); }
.conversation-list { padding: 8px; }
.conversation-item { width: 100%; display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px; border: 0; border-bottom: 1px solid var(--surface-border); background: transparent; color: var(--text-primary); cursor: pointer; text-align: left; }
.conversation-item:hover, .conversation-item.active { background: var(--workbench-sidebar-hover); }
.conversation-main { min-width: 0; display: grid; gap: 4px; }
.conversation-main strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.conversation-main small, .detail-heading p, .message-content small { color: var(--text-muted); }
.conversation-actions { display: flex; flex-shrink: 0; gap: 4px; }
.archive-detail { padding: 16px; }
.detail-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 14px; }
.detail-heading h2 { margin: 0; font-size: var(--ac-font-size-lg); }
.detail-heading p { margin: 4px 0 0; font-size: var(--ac-font-size-sm); }
.detail-actions { display: flex; gap: 8px; }
.summary-panel { margin-bottom: 14px; }
.message-list { display: grid; gap: 12px; }
.message-row { display: grid; grid-template-columns: 42px minmax(0, 1fr); gap: 10px; }
.message-role { color: var(--text-muted); font-size: var(--ac-font-size-sm); }
.message-content { padding: 10px 12px; border-radius: var(--ac-radius-sm); background: var(--ac-color-surface-secondary); }
.message-content p { margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; }
.message-content small { display: block; margin-top: 6px; }
@media (max-width: 767px) {
  .chat-logs-toolbar { grid-template-columns: 1fr; }
  .chat-logs-layout { grid-template-columns: 1fr; }
  .conversation-list { max-height: 320px; }
  .conversation-item { align-items: flex-start; flex-direction: column; }
}
</style>
