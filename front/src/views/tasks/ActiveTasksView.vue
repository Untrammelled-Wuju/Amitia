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
            {{ task.paused ? "已暂停" : task.enabled === 1 ? "已启用" : "已停用" }}
          </el-tag>
        </header>
        <p>{{ task.content || "无任务内容" }}</p>
        <dl>
          <div><dt>执行时间</dt><dd>{{ formatDate(task.localRemindAt || task.remindAt) }}</dd></div>
          <div><dt>重复规则</dt><dd>{{ task.repeatRule || task.recurrence?.type || "none" }}</dd></div>
          <div><dt>状态</dt><dd>{{ task.state || "-" }}</dd></div>
        </dl>
        <footer>
          <el-button size="small" @click="runAction(task, task.paused ? 'resume' : 'pause')">
            {{ task.paused ? "恢复" : "暂停" }}
          </el-button>
          <el-button size="small" type="primary" @click="trigger(task)">立即执行</el-button>
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
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import ExtensionPageHeader from "@/views/extensions/components/ExtensionPageHeader.vue";
import { apiClient } from "@/composables/useApi";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { originalErrorText } from "@/runtime/original-error";

interface ReminderTask {
  id: string;
  title: string;
  content?: string;
  state?: string;
  paused?: boolean;
  enabled?: number;
  generation?: number;
  remindAt?: string;
  localRemindAt?: string;
  repeatRule?: string;
  recurrence?: { type?: string };
}

const kind = ref("user_reminder");
const tasks = ref<ReminderTask[]>([]);
const loading = ref(false);
const saving = ref(false);
const error = ref("");
const createOpen = ref(false);
const createError = ref("");
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

async function trigger(task: ReminderTask) {
  try {
    await apiClient.post(`/api/reminders/${encodeURIComponent(task.id)}/trigger`, {
      generation: task.generation ?? 0,
    });
    ElMessage.success("任务已提交执行");
    await loadTasks();
  } catch (cause) {
    ElMessage.error(cause instanceof Error ? cause.message : "立即执行失败");
  }
}

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
