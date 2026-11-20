<template>
  <div class="active-tasks">
    <ExtensionPageHeader
      title="主动任务"
      description="管理系统定时提醒和角色主动消息。"
      parent-title="任务"
      parent-path="/tasks"
    >
      <template #actions>
        <el-select v-model="kind" size="small" style="width: 140px" @change="loadTasks">
          <el-option label="定时提醒" value="user_reminder" />
          <el-option label="主动消息" value="proactive_task" />
        </el-select>
        <el-button size="small" @click="loadTasks">刷新</el-button>
        <el-button size="small" type="primary" @click="openCreate">新建任务</el-button>
      </template>
    </ExtensionPageHeader>

    <ExtensionSlot
      slot-id="tasks.page.section"
      :context="{ taskKind: kind, route: '/tasks' }"
      fallback="none"
      layout="stack"
      surface-role="main"
    />
    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <div class="task-grid" v-loading="loading">
      <article v-for="task in tasks" :key="task.id" class="task-card">
        <header>
          <strong>{{ task.title }}</strong>
          <el-tag size="small" :type="task.paused ? 'warning' : task.enabled === 1 ? 'success' : 'info'">
            {{ task.state === "expired" ? "已到期" : task.paused ? "已暂停" : task.enabled === 1 ? "已启用" : "已停用" }}
          </el-tag>
        </header>
        <p>{{ task.content || "无任务内容" }}</p>
        <dl>
          <div><dt>执行时间</dt><dd>{{ formatDate(task.localRemindAt || task.remindAt) }}</dd></div>
          <div><dt>重复规则</dt><dd>{{ task.repeatRule || task.recurrence?.type || "none" }}</dd></div>
          <div><dt>状态</dt><dd>{{ task.state === "expired" ? "已到期" : task.paused ? "已暂停" : task.enabled === 1 ? "已启用" : "已停用" }}</dd></div>
        </dl>
        <p v-if="unavailableReason(task)" role="status">{{ unavailableReason(task) }}</p>
        <footer>
          <el-button size="small" @click="openHistory(task)">执行历史</el-button>
          <el-button size="small" @click="runAction(task, task.paused ? 'resume' : 'pause')">
            {{ task.paused ? "恢复" : "暂停" }}
          </el-button>
          <el-button size="small" type="primary" :disabled="!!unavailableReason(task)" :loading="triggering.has(task.id)" @click="trigger(task)">立即执行</el-button>
          <el-button size="small" type="danger" @click="runAction(task, 'cancel')">取消</el-button>
        </footer>
      </article>
      <el-empty v-if="!loading && tasks.length === 0" description="暂无任务" />
    </div>

    <el-dialog v-model="createOpen" title="新建主动任务" width="min(560px, 92vw)">
      <pre v-if="createError" class="task-create-error" role="alert">{{ createError }}</pre>
      <el-form label-position="top">
        <el-form-item label="标题"><el-input v-model="form.title" maxlength="120" /></el-form-item>
        <el-form-item label="所属角色">
          <el-select v-model="form.characterId" aria-label="所属角色" :loading="loadingRoles" :disabled="loadingRoles || saving">
            <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="内容"><el-input v-model="form.content" type="textarea" :rows="4" maxlength="4000" /></el-form-item>
        <el-form-item label="执行时间"><el-input v-model="form.remindAt" type="datetime-local" /></el-form-item>
        <el-form-item label="重复规则">
          <el-select v-model="form.repeatRule" style="width: 100%">
            <el-option label="不重复" value="none" />
            <el-option label="每天" value="daily" />
            <el-option label="每周" value="weekly" />
            <el-option label="每月" value="monthly" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createOpen = false">取消</el-button>
        <el-button type="primary" :loading="saving" :disabled="loadingRoles" @click="createTask">创建</el-button>
      </template>
    </el-dialog>
    <el-dialog v-model="historyOpen" :title="historyTask ? `${historyTask.title} · 执行历史` : '执行历史'" width="min(680px, 92vw)">
      <div v-loading="historyLoading" class="task-history">
        <pre v-if="historyError" class="task-error" role="alert">{{ historyError }}</pre>
        <el-empty v-else-if="!historyLoading && !history.length" description="暂无执行记录" />
        <article v-for="entry in history" :key="entry.id">
          <strong>{{ formatDate(entry.createdAt) }} · {{ executionState(entry.state) }}</strong>
          <p v-if="entry.reason">{{ entry.reason }}</p>
          <RouterLink
            v-if="entry.result?.messageSaved && entry.result.conversationId && entry.result.ownerId"
            :to="{ path: '/chat', query: { conversationId: conversationReference({ id: entry.result.conversationId, ownerId: entry.result.ownerId }), characterId: historyTask?.characterId } }"
          >打开提醒对话</RouterLink>
          <pre v-if="entry.lastError" class="task-error">{{ entry.lastError }}</pre>
          <div v-for="target in entry.result?.targets" :key="`${target.kind}/${target.id}`">
            {{ target.kind === "notification" ? "系统通知" : target.kind === "conversation" ? "会话消息" : target.kind }} · {{ executionState(target.state) }}
            <p v-if="target.kind === 'notification' && target.error?.includes('UNErrorDomain')">请检查 macOS 通知授权和应用签名。系统通知失败的原始信息如下。</p>
            <pre v-if="target.error" class="task-error">{{ target.error }}</pre>
          </div>
        </article>
      </div>
      <template #footer>
        <el-button :loading="historyLoading" @click="loadHistory">刷新历史</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import ExtensionPageHeader from "@/views/extensions/components/ExtensionPageHeader.vue";
import { apiClient } from "@/composables/useApi";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { originalErrorText } from "@/runtime/original-error";
import { conversationReference } from "@/runtime/device-owned-conversation-reference";

interface ReminderTask {
  id: string;
  title: string;
  kind: string;
  content?: string;
  state?: string;
  paused?: boolean;
  enabled?: number;
  generation?: number;
  characterId?: string;
  remindAt?: string;
  localRemindAt?: string;
  repeatRule?: string;
  recurrence?: { type?: string };
}

interface ReminderHistory {
  id: string;
  createdAt: string;
  state: string;
  lastError?: string;
  reason?: string;
  result?: {
    conversationId?: string;
    ownerId?: string;
    messageSaved?: boolean;
    targets?: Array<{ kind: string; id: string; state: string; error?: string }>;
  };
}

const kind = ref("user_reminder");
const tasks = ref<ReminderTask[]>([]);
const loading = ref(false);
const saving = ref(false);
const error = ref("");
const createOpen = ref(false);
const createError = ref("");
const historyOpen = ref(false);
const historyTask = ref<ReminderTask>();
const history = ref<ReminderHistory[]>([]);
const historyLoading = ref(false);
const historyError = ref("");
let historyRequest = 0;
let historyTimer: ReturnType<typeof setTimeout> | undefined;
let followedTrigger = "";
let followUntil = 0;
const triggering = reactive(new Set<string>());
let disposed = false;
const loadingRoles = ref(false);
const roles = ref<Array<{ id: string; name: string }>>([]);
const owned = useDeviceOwnedConversation();
let formCoreId = "";
const form = reactive({ title: "", characterId: "", content: "", remindAt: "", repeatRule: "none" });

function responseData<T>(response: { data?: { data?: T } } | T): T {
  const payload = response as { data?: { data?: T } };
  return (payload.data?.data ?? payload.data ?? response) as T;
}

async function loadTasks() {
  loading.value = true;
  error.value = "";
  try {
    const response = await apiClient.get("/api/reminders", { params: { kind: kind.value } });
    tasks.value = responseData<ReminderTask[]>(response) ?? [];
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "主动任务加载失败";
  } finally {
    loading.value = false;
  }
}

async function openCreate() {
  if (loadingRoles.value || saving.value) return;
  form.title = "";
  form.characterId = "";
  form.content = "";
  form.remindAt = "";
  form.repeatRule = "none";
  formCoreId = "";
  createError.value = "";
  roles.value = [];
  createOpen.value = true;
  loadingRoles.value = true;
  try {
    if (!await owned.refresh()) throw new Error("当前统筹中心角色目录不可用");
    formCoreId = owned.coreId.value;
    roles.value = owned.roles.value.map(({ id, name }) => ({ id, name }));
    if (roles.value.length === 1) form.characterId = roles.value[0].id;
    if (roles.value.length === 0) ElMessage.warning("请先创建角色，再新建任务");
  } catch (cause) {
    createError.value = originalErrorText(cause);
  } finally {
    loadingRoles.value = false;
  }
}

async function createTask() {
  if (saving.value || loadingRoles.value) return;
  if (!form.title.trim()) return ElMessage.warning("请输入任务标题");
  if (!roles.value.some(role => role.id === form.characterId)) return ElMessage.warning("请选择所属角色");
  if (!form.remindAt) return ElMessage.warning("请选择执行时间");
  const characterId = form.characterId;
  createError.value = "";
  saving.value = true;
  try {
    if (!await owned.refresh()) throw new Error("当前统筹中心角色目录不可用");
    if (owned.coreId.value !== formCoreId || !owned.roles.value.some(role => role.id === characterId)) {
      throw new Error("统筹中心或所属角色已变化，请重新打开新建任务表单");
    }
    await apiClient.post("/api/reminders", {
      title: form.title.trim(),
      characterId,
      content: form.content.trim(),
      remindAt: new Date(form.remindAt).toISOString(),
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
      kind: kind.value,
      repeatRule: form.repeatRule,
      recurrence: { type: form.repeatRule },
      enabled: true,
      systemNotification: true,
      requestId: crypto.randomUUID(),
      ...(kind.value === "proactive_task" && {
        policy: {
          enabled: true, activeLevel: 40, quietStart: "23:00", quietEnd: "07:00",
          minInterval: 60, maxPerDay: 6, maxDailyCalls: 10, ruleMaxPerDay: 1,
          unrepliedSlowdownEnabled: true, unrepliedSlowdownAfter: 2,
          unrepliedCooldownMultiplier: 2, unrepliedRecoveryOnReply: true,
        },
      }),
    });
    createOpen.value = false;
    ElMessage.success("任务已创建");
    await loadTasks();
  } catch (cause) {
    createError.value = originalErrorText(cause);
  } finally {
    saving.value = false;
  }
}

async function runAction(task: ReminderTask, action: string) {
  try {
    await apiClient.post(`/api/reminders/${encodeURIComponent(task.id)}/action`, {
      action,
      generation: task.generation ?? 0,
    });
    ElMessage.success("任务状态已更新");
    await loadTasks();
  } catch (cause) {
    ElMessage.error(cause instanceof Error ? cause.message : "任务操作失败");
  }
}

function unavailableReason(task: ReminderTask) {
  if (task.state === "expired") return "任务已到期，无法立即执行。需要再次提醒时请新建任务。";
  if (task.paused) return "任务已暂停，恢复后可立即执行。";
  if (task.enabled !== 1) return "任务已停用，启用后可立即执行。";
  return "";
}

async function trigger(task: ReminderTask) {
  if (triggering.has(task.id) || unavailableReason(task)) return;
  triggering.add(task.id);
  try {
    const response = await apiClient.post(`/api/reminders/${encodeURIComponent(task.id)}/trigger`, {
      generation: task.generation ?? 0,
    });
    const { triggerId } = responseData<{ triggerId: string }>(response);
    if (disposed) return;
    openHistory(task, triggerId);
    void loadTasks();
  } catch (cause) {
    if (!disposed) {
      stopFollowing();
      historyTask.value = task;
      history.value = [];
      historyLoading.value = false;
      historyOpen.value = true;
      historyError.value = originalErrorText(cause);
    }
  } finally {
    triggering.delete(task.id);
  }
}

function stopFollowing() {
  clearTimeout(historyTimer);
  followedTrigger = "";
  historyRequest++;
}

function openHistory(task: ReminderTask, triggerId = "") {
  stopFollowing();
  followedTrigger = triggerId;
  followUntil = Date.now() + 45_000;
  historyTask.value = task;
  history.value = [];
  historyError.value = "";
  historyLoading.value = false;
  historyOpen.value = true;
  void loadHistory();
}

const pendingStates = new Set(["waiting", "due", "leased", "triggering", "running", "retry_wait", "retry", "queued", "pending", "accepted"]);

async function loadHistory() {
  const task = historyTask.value;
  if (!task || !historyOpen.value) return;
  clearTimeout(historyTimer);
  const request = ++historyRequest;
  historyError.value = "";
  historyLoading.value = !history.value.length;
  try {
    const response = await apiClient.get("/api/reminders/trigger-history", { params: { id: task.id, kind: task.kind } });
    if (request !== historyRequest) return;
    const items = responseData<{ items: ReminderHistory[] }>(response).items;
    const entry = items.find(item => item.id === followedTrigger);
    history.value = entry ? [entry, ...items.filter(item => item !== entry)] : items;
    if (followedTrigger && Date.now() < followUntil &&
        (!entry || pendingStates.has(entry.state) || entry.result?.targets?.some(target => pendingStates.has(target.state)))) {
      historyTimer = setTimeout(loadHistory, 1000);
    }
  } catch (cause) {
    if (request === historyRequest) historyError.value = originalErrorText(cause);
  } finally {
    if (request === historyRequest) historyLoading.value = false;
  }
}

function executionState(state: string) {
  const labels: Record<string, string> = {
    waiting: "待执行", due: "待执行", leased: "执行中", triggering: "执行中", running: "执行中",
    queued: "待投递", pending: "待投递", retry: "等待重试", retry_wait: "等待重试", accepted: "通知渠道已受理",
    completed: "已完成", partial: "部分完成", skipped: "已跳过",
    saved: "已保存", delivered: "已投递", displayed: "已显示", clicked: "已打开",
    cancelled: "已取消", expired: "已过期", blocked: "已阻断", failed: "失败", display_failed: "显示失败", suppressed: "未弹出",
  };
  return labels[state] ?? state;
}

watch(historyOpen, (open) => {
  if (!open) stopFollowing();
});

onUnmounted(() => {
  disposed = true;
  stopFollowing();
});

function formatDate(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

onMounted(loadTasks);
</script>

<style scoped>
.active-tasks { width: min(1180px, 100%); margin: 0 auto; padding: 24px; }
.task-create-error { white-space: pre-wrap; overflow-wrap: anywhere; color: var(--el-color-danger); }
.task-error { margin: 4px 0; white-space: pre-wrap; overflow-wrap: anywhere; color: var(--el-color-danger); }
.task-history { min-height: 48px; max-height: 50vh; overflow-y: auto; }
.task-history article { padding: 8px 0; border-bottom: 1px solid var(--surface-border); }
.task-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 14px; margin-top: 18px; }
.task-card { display: grid; gap: 14px; padding: 18px; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-md); background: var(--surface-bg); }
.task-card header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.task-card p { margin: 0; color: var(--text-secondary); line-height: 1.6; white-space: pre-wrap; }
.task-card dl { display: grid; gap: 8px; margin: 0; }
.task-card dl div { display: flex; justify-content: space-between; gap: 16px; font-size: 12px; }
.task-card dt { color: var(--text-muted); }
.task-card dd { margin: 0; color: var(--text-primary); text-align: right; overflow-wrap: anywhere; }
.task-card footer { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; }
@media (max-width: 767px) { .active-tasks { padding: 16px; } }
</style>
