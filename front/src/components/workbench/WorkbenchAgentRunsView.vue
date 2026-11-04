<template>
  <div class="agent-runs-view">
    <header>
      <div>
        <strong>子智能体</strong>
        <small>{{ runs.length }} 个运行 · {{ activeChildren.length }} 个子智能体</small>
      </div>
      <button type="button" :disabled="loading" @click="refresh">{{ loading ? "刷新中" : "刷新" }}</button>
    </header>
    <div v-if="error" class="error" role="alert">{{ error }}</div>
    <div v-if="!conversationAvailable" class="empty">发送第一条消息后，主智能体创建的运行会显示在这里。</div>
    <div v-else class="layout">
      <nav aria-label="智能体运行列表">
        <button
          v-for="run in sortedRuns"
          :key="run.id"
          type="button"
          :class="{ active: selectedRunId === run.id }"
          @click="selectRun(run.id)"
        >
          <span class="run-role">{{ run.parentRunId ? run.workerRole || "子智能体" : "主对话" }}</span>
          <strong>{{ run.objective || run.id }}</strong>
          <small><span class="status">{{ statusLabel(run.status) }}</span> · {{ dateTime(run.updatedAt) }}</small>
        </button>
        <div v-if="sortedRuns.length === 0 && !loading" class="empty">尚无智能体运行</div>
      </nav>
      <section v-if="selectedRun" class="conversation">
        <div class="run-heading">
          <div>
            <strong>{{ selectedRun.objective || selectedRun.id }}</strong>
            <small>{{ selectedRun.parentRunId ? `子智能体 · ${selectedRun.workerRole || "worker"}` : "主对话智能体" }} · {{ statusLabel(selectedRun.status) }}</small>
          </div>
        </div>
        <div ref="messageList" class="messages">
          <article
            v-for="message in messages"
            :key="message.id"
            :class="{ mine: message.senderRunId.startsWith('user:') }"
          >
            <small>{{ senderLabel(message) }} · {{ messageKindLabel(message.kind) }} · {{ dateTime(message.createdAt) }}</small>
            <p>{{ message.content }}</p>
          </article>
          <div v-if="messages.length === 0 && !messageLoading" class="empty">暂无消息</div>
        </div>
        <form class="composer" @submit.prevent="send">
          <textarea v-model="draft" maxlength="16000" rows="3" placeholder="向该智能体发送消息" />
          <div>
            <small>消息会持久化，并可由该运行在下一轮读取。</small>
            <button type="submit" :disabled="!draft.trim() || sending">{{ sending ? "发送中" : "发送" }}</button>
          </div>
        </form>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from "vue";
import { useWorkbenchStore } from "@/stores/workbench";
import { listAgentRunMessages, listAgentRuns, sendAgentRunMessage, type AgentRun, type AgentRunMessage } from "@/api/agent-runs";
import { originalErrorMessage } from "@/runtime/original-error";

const store = useWorkbenchStore();
const runs = ref<AgentRun[]>([]);
const messages = ref<AgentRunMessage[]>([]);
const selectedRunId = ref("");
const draft = ref("");
const loading = ref(false);
const messageLoading = ref(false);
const sending = ref(false);
const error = ref("");
const messageList = ref<HTMLElement | null>(null);
let pollTimer: number | null = null;
let disposed = false;
let initialized = false;

const conversationAvailable = computed(() => Boolean(store.conversationId && store.conversationId !== "__draft__"));
const sortedRuns = computed(() => [...runs.value].sort((left, right) => {
  const depth = left.depth - right.depth;
  return depth || String(left.createdAt).localeCompare(String(right.createdAt));
}));
const selectedRun = computed(() => runs.value.find((run) => run.id === selectedRunId.value) || null);
const activeChildren = computed(() => runs.value.filter((run) => run.parentRunId && ["planning", "running", "waiting", "paused"].includes(run.status)));

function statusLabel(status: string) {
  return ({ planning: "规划中", running: "运行中", waiting: "等待中", succeeded: "已完成", failed: "失败", cancelled: "已取消", paused: "已暂停" } as Record<string, string>)[status] || status;
}

function messageKindLabel(kind: string) {
  return ({ instruction: "指令", progress: "进度", question: "问题", result: "结果", error: "错误", user: "用户" } as Record<string, string>)[kind] || kind;
}

function senderLabel(message: AgentRunMessage) {
  if (message.senderRunId.startsWith("user:")) return "你";
  const run = runs.value.find((item) => item.id === message.senderRunId);
  if (!run) return "智能体";
  return run.parentRunId ? run.workerRole || "子智能体" : "主对话智能体";
}

function dateTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

async function refresh() {
  if (loading.value) return;
  if (!conversationAvailable.value) {
    runs.value = [];
    messages.value = [];
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    runs.value = await listAgentRuns(store.conversationId);
    if (!selectedRunId.value || !runs.value.some((run) => run.id === selectedRunId.value)) {
      selectedRunId.value = runs.value.find((run) => !run.parentRunId)?.id || runs.value[0]?.id || "";
    }
    if (selectedRunId.value) await loadMessages();
  } catch (cause) {
    console.error("[original-error]", cause);
    error.value = originalErrorMessage(cause);
  } finally {
    loading.value = false;
  }
}

async function loadMessages() {
  if (!selectedRunId.value) return;
  messageLoading.value = true;
  try {
    messages.value = await listAgentRunMessages(selectedRunId.value);
    await nextTick();
    if (messageList.value) messageList.value.scrollTop = messageList.value.scrollHeight;
  } catch (cause) {
    console.error("[original-error]", cause);
    error.value = originalErrorMessage(cause);
  } finally {
    messageLoading.value = false;
  }
}

async function selectRun(runId: string) {
  selectedRunId.value = runId;
  draft.value = "";
  await loadMessages();
}

async function send() {
  if (!selectedRun.value || !draft.value.trim() || sending.value) return;
  sending.value = true;
  error.value = "";
  try {
    await sendAgentRunMessage(selectedRun.value.id, draft.value.trim());
    draft.value = "";
    await loadMessages();
  } catch (cause) {
    console.error("[original-error]", cause);
    error.value = originalErrorMessage(cause);
  } finally {
    sending.value = false;
  }
}

function startPolling() {
  stopPolling();
  pollTimer = window.setInterval(() => {
    if (!disposed) void refresh();
  }, 2500);
}

function stopPolling() {
  if (pollTimer !== null) window.clearInterval(pollTimer);
  pollTimer = null;
}

watch(() => store.conversationId, () => {
  selectedRunId.value = "";
  void refresh();
});

onMounted(() => {
  initialized = true;
  void refresh();
  startPolling();
});

onActivated(() => {
  if (!initialized) return;
  void refresh();
  startPolling();
});

onDeactivated(stopPolling);

onBeforeUnmount(() => {
  disposed = true;
  stopPolling();
});
</script>

<style scoped>
.agent-runs-view { display: flex; flex-direction: column; height: 100%; min-height: 0; color: var(--text-primary); }
.agent-runs-view > header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 12px; border-bottom: 1px solid var(--surface-border); }
.agent-runs-view > header strong, .agent-runs-view > header small { display: block; }
.agent-runs-view > header small { margin-top: 2px; color: var(--text-muted); font-size: 11px; }
.agent-runs-view button { border: 1px solid var(--surface-border); border-radius: 7px; background: transparent; color: inherit; cursor: pointer; font: inherit; }
.layout { display: grid; grid-template-columns: minmax(190px, 34%) minmax(0, 1fr); flex: 1; min-height: 0; }
.layout > nav { display: flex; flex-direction: column; gap: 6px; min-height: 0; overflow-y: auto; padding: 8px; border-right: 1px solid var(--surface-border); }
.layout > nav > button { display: grid; gap: 3px; padding: 9px; text-align: left; }
.layout > nav > button.active { background: var(--workbench-sidebar-active); border-color: color-mix(in srgb, var(--ac-color-primary) 35%, var(--surface-border)); }
.layout > nav strong, .layout > nav small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.layout > nav small { color: var(--text-muted); font-size: 11px; }
.run-role { color: var(--ac-color-primary); font-size: 10px; font-weight: 650; }
.conversation { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.run-heading { padding: 10px 12px; border-bottom: 1px solid var(--surface-border); }
.run-heading strong, .run-heading small { display: block; }
.run-heading small { margin-top: 3px; color: var(--text-muted); font-size: 11px; }
.messages { flex: 1; min-height: 0; overflow-y: auto; padding: 12px; }
.messages article { max-width: 88%; margin-bottom: 10px; padding: 9px 10px; border: 1px solid var(--surface-border); border-radius: 9px; background: var(--surface-bg); }
.messages article.mine { margin-left: auto; background: color-mix(in srgb, var(--ac-color-primary) 9%, var(--surface-bg)); }
.messages small { color: var(--text-muted); font-size: 10px; }
.messages p { margin: 5px 0 0; white-space: pre-wrap; overflow-wrap: anywhere; font-size: 13px; line-height: 1.55; }
.composer { display: grid; gap: 7px; padding: 10px 12px; border-top: 1px solid var(--surface-border); }
.composer textarea { width: 100%; box-sizing: border-box; resize: vertical; min-height: 62px; padding: 8px 9px; border: 1px solid var(--surface-border); border-radius: 8px; background: var(--surface-bg); color: inherit; font: inherit; font-size: 13px; }
.composer > div { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.composer small { color: var(--text-muted); font-size: 10px; }
.composer button { padding: 6px 13px; }
.empty { padding: 18px 12px; color: var(--text-muted); font-size: 12px; text-align: center; }
.error { padding: 9px 12px; border-bottom: 1px solid color-mix(in srgb, var(--ac-color-danger) 35%, var(--surface-border)); color: var(--ac-color-danger); white-space: pre-wrap; overflow-wrap: anywhere; user-select: text; font-size: 11px; }
@media (max-width: 720px) {
  .layout { grid-template-columns: 1fr; }
  .layout > nav { max-height: 32%; border-right: 0; border-bottom: 1px solid var(--surface-border); }
}
</style>
