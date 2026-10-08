<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh, Search, VideoPause, VideoPlay, RefreshRight, SetUp } from "@element-plus/icons-vue";
import { listWorkflowDevices, type WorkflowDeviceDescriptor } from "@/api/workflow";
import { taskPauseControls } from "./control";
import { getRuntimeConnection, isCurrentDevicePaired } from "@/runtime/runtime-adapter";
import {
  listTasks,
  getTask,
  enqueueTask,
  prepareOwnedTask,
  enqueueOwnedTask,
  listOwnedDeviceTaskCatalog,
  cancelTask,
  pauseTask,
  resumeTask,
  retryTask,
  recoverTask,
  getTaskProgress,
  getTaskResult,
  getTaskCheckpoint,
  listTaskDefinitions,
	listSourceTaskApprovals,
	decideSourceTaskApproval,
	revokeSourceTaskApproval,
} from "./api";
import {
  type TaskRun,
  type TaskDefinition,
  type TaskRunProgress,
  type TaskRunResult,
  type TaskCheckpoint,
  type TaskRunStatus,
  type EnqueueTaskRequest,
  type OwnedTaskRoleOptions,
  type DeviceTaskCatalogPage,
	type SourceTaskApproval,
  STATUS_LABELS,
  STATUS_TAG_TYPES,
  isTerminal,
  isActive,
} from "./types";

const loading = ref(false);
const hasLocalRuntime = Boolean(window.amitiaDesktop);
const approvalsVisible = ref(false);
const approvalsLoading = ref(false);
const approvalsError = ref("");
const sourceApprovals = ref<SourceTaskApproval[]>([]);
const pendingApprovalCount = computed(() => sourceApprovals.value.filter(approval => approval.status === "pending").length);
let lastApprovalRefresh = 0;
const approvalStatusLabels = { pending: "待审批", approved: "已批准", denied: "已拒绝", claimed: "执行中", revoked: "已撤销" };

async function revokeApproval(approval: SourceTaskApproval) {
  if (approvalsLoading.value) return;
  approvalsLoading.value = true;
  try {
    await revokeSourceTaskApproval(approval);
    ElMessage.success("已撤销单次授权，正在执行的任务将被中断");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error));
  } finally {
    approvalsLoading.value = false;
    await refreshSourceApprovals();
  }
}

async function refreshSourceApprovals() {
  if (approvalsLoading.value) return;
  approvalsLoading.value = true;
  approvalsError.value = "";
  try {
    sourceApprovals.value = await listSourceTaskApprovals();
  } catch (error) {
    sourceApprovals.value = [];
    approvalsError.value = error instanceof Error ? error.message : String(error);
  } finally {
    approvalsLoading.value = false;
    lastApprovalRefresh = Date.now();
  }
}

async function openSourceApprovals() {
  approvalsVisible.value = true;
  await refreshSourceApprovals();
}

async function decideApproval(approval: SourceTaskApproval, approved: boolean) {
  if (approvalsLoading.value) return;
  approvalsLoading.value = true;
  try {
    await decideSourceTaskApproval(approval, approved);
    ElMessage.success(approved ? "已批准这次设备执行，请在发起设备使用原请求重试" : "已拒绝这次设备执行");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error));
  } finally {
    approvalsLoading.value = false;
    await refreshSourceApprovals();
  }
}
const tasks = ref<TaskRun[]>([]);
const definitions = ref<TaskDefinition[]>([]);
const statusFilter = ref<string>("");
const extensionFilter = ref<string>("");
const activeTab = ref<"runs" | "definitions">("runs");
const detailVisible = ref(false);
const detailTask = ref<TaskRun | null>(null);
const detailProgress = ref<TaskRunProgress | null>(null);
const detailResult = ref<TaskRunResult | null>(null);
const detailCheckpoint = ref<TaskCheckpoint | null>(null);
const detailLoading = ref(false);
let detailGeneration = 0;
const enqueueDialogVisible = ref(false);
const enqueueForm = ref<{ taskDefinitionId: string; input: string; priority: number; deviceId: string }>({
  taskDefinitionId: "",
  input: "{}",
  priority: 0,
  deviceId: "",
});
const workflowDevices = ref<WorkflowDeviceDescriptor[]>([]);
const enqueueDefinitions = computed(() => ownedEnqueue.value ? enqueueCatalog.value.map(entry => ({ ...entry.definition, taskId: entry.reference.catalogId, executionPlacement: "device" as const })) : definitions.value);
const selectedEnqueueDefinition = computed(() => enqueueDefinitions.value.find((d) => d.taskId === enqueueForm.value.taskDefinitionId));
const enqueueNeedsDevice = computed(() => ownedEnqueue.value || selectedEnqueueDefinition.value?.executionPlacement === "device");
const onlineWorkflowDevices = computed(() => workflowDevices.value.filter((device) => device.online));
const enqueueLoading = ref(false);
const ownedEnqueue = ref(false);
const enqueueRoles = ref<OwnedTaskRoleOptions | null>(null);
const enqueueRoleId = ref("");
const enqueueRoleLoading = ref(false);
const enqueueRoleError = ref("");
const enqueueCatalog = ref<DeviceTaskCatalogPage["entries"]>([]);
const enqueueCatalogCursor = ref("");
const enqueueCatalogRevision = ref("");
const enqueueCatalogLoading = ref(false);
const enqueueCatalogError = ref("");
let catalogLoadGeneration = 0;
let roleLoadGeneration = 0;
let lastSubmission = { key: "", requestId: "" };
let refreshTimer: ReturnType<typeof setInterval> | null = null;

const filteredTasks = computed(() => {
  return tasks.value.filter((t) => {
    if (statusFilter.value && t.status !== statusFilter.value) return false;
    if (extensionFilter.value && !t.extensionId.includes(extensionFilter.value)) return false;
    return true;
  });
});

const activeTaskCount = computed(() => tasks.value.filter((t) => isActive(t.status)).length);
const succeededCount = computed(() => tasks.value.filter((t) => t.status === "succeeded").length);
const failedCount = computed(() => tasks.value.filter((t) => t.status === "failed" || t.status === "timed_out").length);

function definitionForTask(task: TaskRun): TaskDefinition | undefined {
  return definitions.value.find((definition) => definition.taskId === task.taskDefinitionId);
}

function effectivePlacement(task: TaskRun): "local" | "cloud" | "device" {
  return task.executionPlacement || "local";
}

function canPauseTask(task: TaskRun): boolean {
  return taskPauseControls(task, definitionForTask(task)).pause;
}

function canResumeTask(task: TaskRun): boolean {
  return taskPauseControls(task, definitionForTask(task)).resume;
}

function canCancelTask(task: TaskRun): boolean {
  if (task.readOnly === true) return false;
  const placement = effectivePlacement(task);
  if (placement === "cloud" || placement === "device") {
    return ["queued", "running", "cancelling"].includes(task.status);
  }
  return ["queued", "running", "pausing", "paused"].includes(task.status);
}

function canRecoverTask(task: TaskRun): boolean {
  if (task.readOnly === true) return false;
  return task.status === "recovery_required" || task.status === "manual_intervention";
}

function canRetryTask(task: TaskRun): boolean {
  if (task.readOnly === true) return false;
  if (!isTerminal(task.status) || task.maxAttempts <= 0 || task.attempt >= task.maxAttempts) return false;
  const definition = definitionForTask(task);
  if (!definition) return false;
  const idempotency = definition.idempotency || (definition.idempotent ? "idempotent" : "non_idempotent");
  return idempotency !== "non_idempotent";
}

const statusOptions = [
  { label: "全部", value: "" },
  { label: "排队中", value: "queued" },
  { label: "运行中", value: "running" },
  { label: "暂停中", value: "pausing" },
  { label: "已暂停", value: "paused" },
  { label: "恢复中", value: "resuming" },
  { label: "已成功", value: "succeeded" },
  { label: "已失败", value: "failed" },
  { label: "已取消", value: "cancelled" },
  { label: "需恢复", value: "recovery_required" },
  { label: "需人工干预", value: "manual_intervention" },
];

async function fetchTasks() {
  loading.value = true;
  try {
    const res = await listTasks({});
    tasks.value = res.items || [];
  } catch (e: unknown) {
    ElMessage.error("加载任务列表失败: " + (e instanceof Error ? e.message : String(e)));
  } finally {
    loading.value = false;
  }
}

async function fetchDefinitions() {
  try {
    const res = await listTaskDefinitions();
    definitions.value = res.items || [];
  } catch (e: unknown) {
    ElMessage.error("加载任务定义失败: " + (e instanceof Error ? e.message : String(e)));
  }
}

async function handleRefresh() {
  await Promise.all([fetchTasks(), fetchDefinitions()]);
}

function startAutoRefresh() {
  stopAutoRefresh();
  refreshTimer = setInterval(() => {
    if (hasLocalRuntime && document.visibilityState === "visible" && !approvalsLoading.value && Date.now() - lastApprovalRefresh >= (approvalsVisible.value ? 3000 : 15000)) {
      void refreshSourceApprovals();
    }
    if (activeTaskCount.value > 0) {
      fetchTasks();
    }
  }, 3000);
}

function stopAutoRefresh() {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
}

async function showDetail(expected: TaskRun) {
  const taskRunId = expected.taskRunId;
  const generation = ++detailGeneration;
  detailVisible.value = true;
  detailLoading.value = true;
  detailTask.value = null;
  detailProgress.value = null;
  detailResult.value = null;
  detailCheckpoint.value = null;
  try {
    const [task, progress, result, checkpoint] = await Promise.all([
      getTask(taskRunId, expected),
      getTaskProgress(taskRunId, expected),
      getTaskResult(taskRunId, expected),
      getTaskCheckpoint(taskRunId, expected),
    ]);
    if (generation !== detailGeneration) return;
    detailTask.value = task;
    detailProgress.value = progress;
    detailResult.value = result;
    detailCheckpoint.value = checkpoint;
  } catch (error) {
    if (generation === detailGeneration) ElMessage.error(error instanceof Error ? error.message : String(error));
  } finally {
    if (generation === detailGeneration) detailLoading.value = false;
  }
}

async function handlePause(task: TaskRun) {
  try {
    await pauseTask(task.taskRunId, task.generation ?? 0, "user_requested", task);
    ElMessage.success("已发送暂停请求");
    await fetchTasks();
    if (detailTask.value?.taskRunId === task.taskRunId) await showDetail(task);
  } catch (e: unknown) {
    ElMessage.error("暂停失败: " + (e instanceof Error ? e.message : String(e)));
  }
}

async function handleResume(task: TaskRun) {
  try {
    await resumeTask(task.taskRunId, task.generation ?? 0, task);
    ElMessage.success("已发送继续请求");
    await fetchTasks();
    if (detailTask.value?.taskRunId === task.taskRunId) await showDetail(task);
  } catch (e: unknown) {
    ElMessage.error("继续失败: " + (e instanceof Error ? e.message : String(e)));
  }
}

async function handleCancel(task: TaskRun) {
  try {
    await ElMessageBox.confirm("确认取消此任务？", "取消任务", { type: "warning" });
    await cancelTask(task.taskRunId, undefined, task);
    ElMessage.success("已发送取消请求");
    fetchTasks();
  } catch (e: unknown) {
    if (e !== "cancel") {
      ElMessage.error("取消失败: " + (e instanceof Error ? e.message : String(e)));
    }
  }
}

async function handleRetry(task: TaskRun) {
  try {
    await retryTask(task.taskRunId, task);
    ElMessage.success("已重新入队");
    fetchTasks();
  } catch (e: unknown) {
    ElMessage.error("重试失败: " + (e instanceof Error ? e.message : String(e)));
  }
}

async function handleRecover(task: TaskRun) {
  try {
    await ElMessageBox.confirm("请先核对任务结果并确认旧执行已停止。是否从已确认的检查点恢复？", "恢复任务", { type: "warning" });
    await recoverTask(task.taskRunId, task);
    ElMessage.success("已提交恢复请求");
    fetchTasks();
  } catch (e: unknown) {
    if (e !== "cancel") {
      ElMessage.error("恢复失败: " + (e instanceof Error ? e.message : String(e)));
    }
  }
}

async function refreshEnqueueDeviceOptions() {
  const def = definitions.value.find((item) => item.taskId === enqueueForm.value.taskDefinitionId);
  if (!ownedEnqueue.value && def?.executionPlacement !== "device") {
    workflowDevices.value = [];
    enqueueForm.value.deviceId = "";
    return;
  }
  try {
    workflowDevices.value = await listWorkflowDevices();
    const online = workflowDevices.value.filter((device) => device.online && !!device.deviceId);
    if (!online.some((device) => device.deviceId === enqueueForm.value.deviceId)) {
      enqueueForm.value.deviceId = online.length === 1 ? online[0].deviceId : "";
    }
  } catch (e: unknown) {
    workflowDevices.value = [];
    enqueueForm.value.deviceId = "";
    ElMessage.warning("设备列表加载失败: " + (e instanceof Error ? e.message : String(e)));
  }
}

async function openEnqueue(def?: TaskDefinition, deviceCatalog = false) {
  const connection = await getRuntimeConnection();
  ownedEnqueue.value = deviceCatalog || def?.executionPlacement === "device" || Boolean(def?.taskId.startsWith("mesh-task-")) || await isCurrentDevicePaired(connection.apiBaseURL);
  enqueueRoles.value = null;
  enqueueRoleId.value = "";
  enqueueRoleError.value = "";
  lastSubmission = { key: "", requestId: "" };
  enqueueForm.value = {
    taskDefinitionId: ownedEnqueue.value ? "" : def?.taskId || "",
    input: "{}",
    priority: 0,
    deviceId: "",
  };
  enqueueDialogVisible.value = true;
  await refreshEnqueueDeviceOptions();
}

async function refreshEnqueueRoles() {
  const ticket = ++roleLoadGeneration;
  const target = enqueueForm.value.deviceId;
  enqueueRoles.value = null;
  enqueueRoleId.value = "";
  enqueueRoleError.value = "";
  if (!ownedEnqueue.value || !target || !enqueueDialogVisible.value) {
    enqueueRoleLoading.value = false;
    return;
  }
  enqueueRoleLoading.value = true;
  try {
    const result = await prepareOwnedTask(target);
    if (ticket !== roleLoadGeneration || !enqueueDialogVisible.value) return;
    enqueueRoles.value = result;
    if (result.roles.length === 1) enqueueRoleId.value = result.roles[0].id;
    else if (result.roles.some(role => role.id === result.selectedRole)) enqueueRoleId.value = result.selectedRole;
    if (!result.roles.length) enqueueRoleError.value = "目标数据来源没有可用角色，拒绝调用";
  } catch (error) {
    if (ticket === roleLoadGeneration) enqueueRoleError.value = error instanceof Error ? error.message : String(error);
  } finally {
    if (ticket === roleLoadGeneration) enqueueRoleLoading.value = false;
  }
}

async function refreshEnqueueCatalog(more = false) {
  const ticket = ++catalogLoadGeneration;
  const options = enqueueRoles.value;
  const roleId = enqueueRoleId.value;
  const cursor = more ? enqueueCatalogCursor.value : "";
  if (!more) {
    enqueueCatalog.value = [];
    enqueueCatalogCursor.value = "";
    enqueueCatalogRevision.value = "";
    enqueueForm.value.taskDefinitionId = "";
  }
  enqueueCatalogError.value = "";
  if (!ownedEnqueue.value || !options || !roleId || !enqueueDialogVisible.value || (more && !cursor)) {
    enqueueCatalogLoading.value = false;
    return;
  }
  enqueueCatalogLoading.value = true;
  try {
    const page = await listOwnedDeviceTaskCatalog(options, roleId, cursor);
    if (ticket !== catalogLoadGeneration || enqueueRoles.value !== options || enqueueRoleId.value !== roleId || !enqueueDialogVisible.value) return;
    if (more && (page.revision !== enqueueCatalogRevision.value || page.entries.some(entry => enqueueCatalog.value.some(existing => existing.reference.catalogId === entry.reference.catalogId)))) throw new Error("设备任务目录已更新，请刷新角色与服务后重新选择");
    if (enqueueCatalog.value.length + page.entries.length > 256) throw new Error("设备任务目录超过上限");
    enqueueCatalog.value = more ? [...enqueueCatalog.value, ...page.entries] : page.entries;
    enqueueCatalogCursor.value = page.nextCursor || "";
    enqueueCatalogRevision.value = page.revision;
  } catch (error) {
    if (ticket === catalogLoadGeneration) {
      enqueueCatalog.value = [];
      enqueueCatalogCursor.value = "";
      enqueueForm.value.taskDefinitionId = "";
      enqueueCatalogError.value = error instanceof Error ? error.message : String(error);
    }
  } finally {
    if (ticket === catalogLoadGeneration) enqueueCatalogLoading.value = false;
  }
}

async function handleEnqueue() {
  if (!enqueueForm.value.taskDefinitionId) {
    ElMessage.warning("请选择任务定义");
    return;
  }
  let input: unknown;
  try {
    input = JSON.parse(enqueueForm.value.input);
  } catch {
    ElMessage.error("输入 JSON 格式无效");
    return;
  }
  if (enqueueNeedsDevice.value && !enqueueForm.value.deviceId) {
    ElMessage.warning("设备任务需要选择在线设备");
    return;
  }
  if (ownedEnqueue.value && (!enqueueNeedsDevice.value || !enqueueRoles.value || !enqueueRoleId.value || enqueueRoleLoading.value)) {
    ElMessage.warning("绑定设备任务需要在线目标设备与有效角色，请先刷新角色");
    return;
  }
  enqueueLoading.value = true;
  try {
    const def = selectedEnqueueDefinition.value;
    const req: EnqueueTaskRequest = {
      taskDefinitionId: enqueueForm.value.taskDefinitionId,
      extensionId: def?.extensionId,
      moduleId: def?.moduleId,
      input,
      priority: enqueueForm.value.priority,
      deviceId: enqueueNeedsDevice.value ? enqueueForm.value.deviceId : undefined,
    };
    let result;
    if (ownedEnqueue.value) {
      const options = enqueueRoles.value!;
      const role = options.roles.find(item => item.id === enqueueRoleId.value);
      if (!role) throw new Error("角色已失效，请刷新角色");
      const current = await prepareOwnedTask(enqueueForm.value.deviceId);
      const expected = options.executionScope;
      const actual = current.executionScope;
      if (actual.coreId !== expected.coreId || actual.modeRevision !== expected.modeRevision || actual.providerEpoch !== expected.providerEpoch || actual.permissionRevision !== expected.permissionRevision || actual.targetPermissionRevision !== expected.targetPermissionRevision || actual.targetProviderEpoch !== expected.targetProviderEpoch || actual.resourceOwnerId !== expected.resourceOwnerId || !current.roles.some(item => item.id === role.id && item.revision === role.revision)) {
        throw new Error(actual.coreId !== expected.coreId ? `云端服务提供者已从「${expected.coreId}」切换为「${actual.coreId}」，请刷新后重新提交。` : "目标角色、权限或统筹状态已变化，请刷新后重新提交");
      }
      const expectedExecutionScope = { ...expected, requestId: "", turnId: "", executionId: "", roleId: role.id, roleRevision: role.revision };
      const catalog = enqueueCatalog.value.find(entry => entry.reference.catalogId === req.taskDefinitionId);
      if (!catalog || catalog.reference.deviceId !== enqueueForm.value.deviceId || catalog.reference.coreId !== expected.coreId) throw new Error("任务目录来源已失效，请重新选择设备任务");
      const body = { taskDefinitionId: req.taskDefinitionId, taskCatalogReference: catalog.reference, targetDeviceId: enqueueForm.value.deviceId, characterId: role.id, input, expectedCoreId: expected.coreId, expectedModeRevision: expected.modeRevision, expectedRoleRevision: role.revision, expectedExecutionScope };
      const key = JSON.stringify(body);
      if (lastSubmission.key !== key) lastSubmission = { key, requestId: crypto.randomUUID() };
      result = await enqueueOwnedTask({ ...body, requestId: lastSubmission.requestId });
    } else {
      result = await enqueueTask(req);
    }
    ElMessage.success(`${result.queued ? "任务已入队" : STATUS_LABELS[result.status]}: ${result.taskRunId}`);
    enqueueDialogVisible.value = false;
    fetchTasks();
  } catch (e: unknown) {
    ElMessage.error("入队失败: " + (e instanceof Error ? e.message : String(e)));
  } finally {
    enqueueLoading.value = false;
  }
}

function formatTime(t?: string): string {
  if (!t) return "-";
  return new Date(t).toLocaleString("zh-CN");
}

function formatDuration(start?: string, end?: string): string {
  if (!start) return "-";
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const diff = Math.max(0, e - s);
  if (diff < 1000) return `${diff}ms`;
  if (diff < 60000) return `${(diff / 1000).toFixed(1)}s`;
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m${Math.floor((diff % 60000) / 1000)}s`;
  return `${Math.floor(diff / 3600000)}h${Math.floor((diff % 3600000) / 60000)}m`;
}

function formatPayload(payload: unknown): string {
  try {
    return JSON.stringify(payload, null, 2);
  } catch {
    return String(payload);
  }
}

const progressPercentage = computed(() => {
  if (!detailProgress.value) return 0;
  if (detailProgress.value.percentage != null) return detailProgress.value.percentage;
  if (detailProgress.value.current != null && detailProgress.value.total != null && detailProgress.value.total > 0) {
    return Math.min(100, Math.round((detailProgress.value.current / detailProgress.value.total) * 100));
  }
  return 0;
});

watch(activeTab, (val) => {
  if (val === "definitions" && definitions.value.length === 0) {
    fetchDefinitions();
  }
});

watch(
  () => enqueueForm.value.taskDefinitionId,
  () => {
    if (enqueueDialogVisible.value && !ownedEnqueue.value) void refreshEnqueueDeviceOptions();
  },
);

watch(() => [enqueueForm.value.deviceId, ownedEnqueue.value, enqueueDialogVisible.value], () => { void refreshEnqueueRoles(); });
watch(() => [enqueueRoles.value, enqueueRoleId.value, enqueueDialogVisible.value], () => { void refreshEnqueueCatalog(); });

onMounted(() => {
  handleRefresh();
  if (hasLocalRuntime) void refreshSourceApprovals();
  startAutoRefresh();
});

onUnmounted(() => {
  roleLoadGeneration++;
  catalogLoadGeneration++;
  stopAutoRefresh();
});
</script>

<template>
  <div class="task-center">
    <div class="task-header">
      <div class="header-left">
        <h2 class="page-title">任务运行时</h2>
        <div class="stats-bar">
          <el-tag type="info">总计 {{ tasks.length }}</el-tag>
          <el-tag type="warning">活跃 {{ activeTaskCount }}</el-tag>
          <el-tag type="success">成功 {{ succeededCount }}</el-tag>
          <el-tag type="danger">失败 {{ failedCount }}</el-tag>
        </div>
      </div>
      <div class="header-right">
		<el-button v-if="hasLocalRuntime" @click="openSourceApprovals">本机资源审批{{ pendingApprovalCount ? `（${pendingApprovalCount} 待处理）` : "" }}</el-button>
        <el-button @click="openEnqueue()">执行任务</el-button>
        <el-button @click="openEnqueue(undefined, true)">执行设备任务</el-button>
        <el-button :icon="Refresh" @click="handleRefresh" :loading="loading">刷新</el-button>
      </div>
    </div>

    <el-dialog v-model="approvalsVisible" title="本机设备任务资源审批" width="min(760px, 94vw)">
      <el-alert title="批准仅用于当前请求的一次执行，最长 30 分钟；拒绝后需重新发起请求。" type="info" :closable="false" />
      <el-alert v-if="approvalsError" :title="approvalsError" type="error" :closable="false" />
      <div v-loading="approvalsLoading">
        <el-empty v-if="!sourceApprovals.length && !approvalsError" description="没有当前绑定下有效的设备任务审批" />
        <el-card v-for="approval in sourceApprovals" :key="approval.id" style="margin-top: 12px">
          <div>任务：{{ approval.binding.target.taskId }} · 安装代次 {{ approval.binding.target.installedGeneration }}</div>
          <div>插件：{{ approval.binding.target.extensionId }} / {{ approval.binding.target.moduleId }}</div>
          <div>Core：{{ approval.binding.executionScope.coreId }}</div>
          <div>发起设备：{{ approval.binding.executionScope.initiatorDeviceId }} · 角色 {{ approval.binding.executionScope.roleId }}</div>
          <div>执行代次：{{ approval.binding.taskGeneration }} · 数据保存至 {{ approval.binding.executionScope.resourceOwnerId }}</div>
          <div>资源权限：{{ approval.permissions.map(permission => permission.permissionId).join("、") }}</div>
          <div>请求内容指纹：{{ approval.binding.inputHash }}</div>
          <div>审批状态：{{ approvalStatusLabels[approval.status] }} · 有效至 {{ approval.expiresAt }}</div>
          <el-button v-if="approval.status === 'approved' || approval.status === 'claimed'" :disabled="approvalsLoading" type="danger" @click="revokeApproval(approval)">撤销单次授权</el-button>
          <template v-if="approval.status === 'pending'">
            <el-button :disabled="approvalsLoading" type="primary" @click="decideApproval(approval, true)">批准本次执行</el-button>
            <el-button :disabled="approvalsLoading" type="danger" @click="decideApproval(approval, false)">拒绝</el-button>
          </template>
        </el-card>
      </div>
      <template #footer><el-button :loading="approvalsLoading" @click="refreshSourceApprovals">刷新审批</el-button></template>
    </el-dialog>
    <el-tabs v-model="activeTab" class="task-tabs">
      <el-tab-pane label="任务运行" name="runs">
        <div class="filter-bar">
          <el-select v-model="statusFilter" placeholder="状态筛选" clearable style="width: 160px">
            <el-option v-for="opt in statusOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
          <el-input v-model="extensionFilter" placeholder="扩展ID筛选" clearable style="width: 240px" :prefix-icon="Search" />
        </div>

        <el-table :data="filteredTasks" v-loading="loading" style="width: 100%" row-key="taskRunId" @row-click="(row: TaskRun) => showDetail(row)">
          <el-table-column prop="taskRunId" label="任务运行ID" width="200" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="mono-text">{{ row.taskRunId.substring(0, 16) }}...</span>
            </template>
          </el-table-column>
          <el-table-column prop="taskDefinitionId" label="任务定义" width="180" show-overflow-tooltip />
          <el-table-column prop="extensionId" label="扩展" width="180" show-overflow-tooltip />
          <el-table-column prop="status" label="状态" width="120">
            <template #default="{ row }">
              <el-tag :type="STATUS_TAG_TYPES[row.status as TaskRunStatus]" size="small">
                {{ STATUS_LABELS[row.status as TaskRunStatus] }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="priority" label="优先级" width="80" />
          <el-table-column prop="attempt" label="尝试" width="80">
            <template #default="{ row }">
              {{ row.attempt }}/{{ row.maxAttempts }}
            </template>
          </el-table-column>
          <el-table-column label="耗时" width="100">
            <template #default="{ row }">
              {{ formatDuration(row.startedAt, row.finishedAt) }}
            </template>
          </el-table-column>
          <el-table-column prop="createdAt" label="创建时间" width="180">
            <template #default="{ row }">
              {{ formatTime(row.createdAt) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="280" fixed="right">
            <template #default="{ row }">
              <el-button v-if="canPauseTask(row)" size="small" :icon="VideoPause" @click.stop="handlePause(row)">暂停</el-button>
              <el-button v-if="canResumeTask(row)" size="small" type="primary" :icon="VideoPlay" @click.stop="handleResume(row)">继续</el-button>
              <el-button v-if="canCancelTask(row)" size="small" type="danger" @click.stop="handleCancel(row)">取消</el-button>
              <el-button v-if="canRecoverTask(row)" size="small" type="warning" :icon="RefreshRight" @click.stop="handleRecover(row)">恢复</el-button>
              <el-button v-if="canRetryTask(row)" size="small" :icon="VideoPlay" @click.stop="handleRetry(row)">重试</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="任务定义" name="definitions">
        <el-table :data="definitions" style="width: 100%" row-key="taskId">
          <el-table-column prop="taskId" label="任务ID" width="200" show-overflow-tooltip />
          <el-table-column prop="extensionId" label="扩展" width="200" show-overflow-tooltip />
          <el-table-column prop="moduleId" label="模块" width="120" show-overflow-tooltip />
          <el-table-column prop="runtimeType" label="运行时" width="140" />
          <el-table-column prop="entry" label="入口" width="200" show-overflow-tooltip />
          <el-table-column label="特性" width="160">
            <template #default="{ row }">
              <el-tag v-if="row.checkpoint" size="small" type="info" style="margin-right: 4px">检查点</el-tag>
              <el-tag v-if="row.recoverable" size="small" type="success" style="margin-right: 4px">可恢复</el-tag>
              <el-tag v-if="row.idempotent" size="small">幂等</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="primary" :icon="SetUp" @click="openEnqueue(row)">执行</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="detailVisible" title="任务详情" width="780px" destroy-on-close>
      <div v-loading="detailLoading">
        <template v-if="detailTask">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="运行ID">{{ detailTask.taskRunId }}</el-descriptions-item>
            <el-descriptions-item label="状态">
              <el-tag :type="STATUS_TAG_TYPES[detailTask.status]" size="small">{{ STATUS_LABELS[detailTask.status] }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="任务定义">{{ detailTask.taskDefinitionId }}</el-descriptions-item>
            <el-descriptions-item label="扩展">{{ detailTask.extensionId }}</el-descriptions-item>
            <el-descriptions-item label="尝试次数">{{ detailTask.attempt }} / {{ detailTask.maxAttempts }}</el-descriptions-item>
            <el-descriptions-item label="优先级">{{ detailTask.priority }}</el-descriptions-item>
            <el-descriptions-item label="创建时间">{{ formatTime(detailTask.createdAt) }}</el-descriptions-item>
            <el-descriptions-item label="开始时间">{{ formatTime(detailTask.startedAt) }}</el-descriptions-item>
            <el-descriptions-item label="完成时间">{{ formatTime(detailTask.finishedAt) }}</el-descriptions-item>
            <el-descriptions-item label="截止时间">{{ formatTime(detailTask.deadlineAt) }}</el-descriptions-item>
            <el-descriptions-item v-if="detailTask.errorCode" label="错误代码">
              <el-tag type="danger" size="small">{{ detailTask.errorCode }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item v-if="detailTask.errorMessage" label="错误信息" :span="2">{{ detailTask.errorMessage }}</el-descriptions-item>
          </el-descriptions>

          <div v-if="detailProgress" class="detail-section">
            <h4>进度</h4>
            <el-progress :percentage="progressPercentage" :status="detailTask.status === 'succeeded' ? 'success' : detailTask.status === 'failed' ? 'exception' : undefined" />
            <div class="progress-info">
              <span v-if="detailProgress.current != null && detailProgress.total != null">{{ detailProgress.current }} / {{ detailProgress.total }}</span>
              <span v-if="detailProgress.stage">{{ detailProgress.stage }}</span>
              <span v-if="detailProgress.message">{{ detailProgress.message }}</span>
            </div>
          </div>

          <div v-if="detailResult" class="detail-section">
            <h4>结果</h4>
            <el-descriptions :column="1" border>
              <el-descriptions-item label="类型">{{ detailResult.resultType }}</el-descriptions-item>
              <el-descriptions-item v-if="detailResult.artifactId" label="产物ID">{{ detailResult.artifactId }}</el-descriptions-item>
              <el-descriptions-item v-if="detailResult.resultHash" label="哈希">{{ detailResult.resultHash }}</el-descriptions-item>
            </el-descriptions>
            <div v-if="detailResult.resultJson" class="json-block">
              <pre>{{ formatPayload(detailResult.resultJson) }}</pre>
            </div>
          </div>

          <div v-if="detailCheckpoint" class="detail-section">
            <h4>检查点 (v{{ detailCheckpoint.version }})</h4>
            <div class="json-block">
              <pre>{{ formatPayload(detailCheckpoint.payload) }}</pre>
            </div>
          </div>

          <div class="detail-section">
            <h4>输入</h4>
            <div class="json-block">
              <pre>{{ formatPayload(detailTask.input) }}</pre>
            </div>
          </div>

          <div class="detail-actions">
            <el-button v-if="canPauseTask(detailTask)" :icon="VideoPause" @click="handlePause(detailTask)">暂停任务</el-button>
            <el-button v-if="canResumeTask(detailTask)" type="primary" :icon="VideoPlay" @click="handleResume(detailTask)">继续任务</el-button>
            <el-button v-if="canCancelTask(detailTask)" type="danger" @click="handleCancel(detailTask)">取消任务</el-button>
            <el-button v-if="canRecoverTask(detailTask)" type="warning" :icon="RefreshRight" @click="handleRecover(detailTask)">恢复任务</el-button>
            <el-button v-if="canRetryTask(detailTask)" :icon="VideoPlay" @click="handleRetry(detailTask)">重试任务</el-button>
          </div>
        </template>
      </div>
    </el-dialog>

    <el-dialog v-model="enqueueDialogVisible" title="执行任务" width="560px" :close-on-click-modal="!enqueueLoading" :close-on-press-escape="!enqueueLoading" :show-close="!enqueueLoading">
      <el-form label-width="100px" :disabled="enqueueLoading">
        <el-form-item label="任务定义">
          <el-select v-model="enqueueForm.taskDefinitionId" :loading="enqueueCatalogLoading" :disabled="ownedEnqueue && (!enqueueRoleId || enqueueCatalogLoading)" placeholder="选择任务定义" style="width: 100%">
            <el-option v-for="def in enqueueDefinitions" :key="def.taskId" :label="ownedEnqueue ? `${enqueueCatalog.find(entry => entry.reference.catalogId === def.taskId)?.reference.sourceTaskId} · ${def.version || '当前版本'} (${def.extensionId})` : `${def.taskId} (${def.extensionId})`" :value="def.taskId" />
          </el-select>
          <div v-if="enqueueCatalogError" class="form-tip">{{ enqueueCatalogError }}</div>
          <div v-if="ownedEnqueue && enqueueRoleId && !enqueueCatalogLoading && !enqueueCatalogError && !enqueueCatalog.length" class="form-tip">目标设备没有可用的已安装任务。</div>
          <el-button v-if="ownedEnqueue && enqueueCatalogCursor" :loading="enqueueCatalogLoading" @click="refreshEnqueueCatalog(true)">加载更多设备任务</el-button>
        </el-form-item>
        <el-form-item v-if="enqueueNeedsDevice" label="执行设备">
          <el-select v-model="enqueueForm.deviceId" placeholder="选择在线设备" style="width: 100%">
            <el-option
              v-for="device in onlineWorkflowDevices"
              :key="device.deviceId"
              :label="device.label || `${device.platform || 'device'} · ${device.deviceId}`"
              :value="device.deviceId"
            />
          </el-select>
          <div v-if="onlineWorkflowDevices.length === 0" class="form-tip">当前没有在线设备，设备任务无法入队。</div>
        </el-form-item>
        <template v-if="ownedEnqueue">
          <el-form-item label="执行角色">
            <el-select v-model="enqueueRoleId" :loading="enqueueRoleLoading" :disabled="enqueueLoading || enqueueRoleLoading" placeholder="选择目标数据来源的角色" style="width: 100%">
              <el-option v-for="role in enqueueRoles?.roles || []" :key="role.id" :label="role.name" :value="role.id" />
            </el-select>
            <div v-if="enqueueRoles" class="form-tip">Core：{{ enqueueRoles.executionScope.coreId }} · 新数据所有者：{{ enqueueRoles.roleOwnerId }}</div>
            <div v-if="enqueueRoleError" class="form-tip">{{ enqueueRoleError }}</div>
            <el-button :disabled="enqueueLoading" :loading="enqueueRoleLoading" @click="refreshEnqueueRoles">刷新角色与服务</el-button>
          </el-form-item>
        </template>
        <el-form-item v-else label="优先级">
          <el-input-number v-model="enqueueForm.priority" :min="0" :max="10" />
        </el-form-item>
        <el-form-item v-if="ownedEnqueue && (selectedEnqueueDefinition?.permissionRequirements?.length || selectedEnqueueDefinition?.permissionRequirementStrings?.length)" label="资源权限">
          <el-alert title="本机资源权限须由目标设备批准，提交时会再次确认。" type="info" :closable="false" />
        </el-form-item>
        <el-form-item label="输入(JSON)">
          <el-input v-model="enqueueForm.input" type="textarea" :rows="6" placeholder='{"key":"value"}' />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button :disabled="enqueueLoading" @click="enqueueDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="enqueueLoading"
          :disabled="(enqueueNeedsDevice && !enqueueForm.deviceId) || (ownedEnqueue && (!enqueueNeedsDevice || !enqueueRoleId || enqueueRoleLoading))"
          @click="handleEnqueue"
        >入队</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.task-center {
  padding: 20px;
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 20px;
}

.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
}

.stats-bar {
  display: flex;
  gap: 8px;
}

.task-tabs {
  margin-top: 8px;
}

.filter-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.mono-text {
  font-family: 'Courier New', monospace;
  font-size: 13px;
}

.detail-section {
  margin-top: 20px;
}

.detail-section h4 {
  margin-bottom: 12px;
  color: var(--el-text-color-primary);
}

.progress-info {
  display: flex;
  gap: 16px;
  margin-top: 8px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.json-block {
  background: var(--el-fill-color-light);
  border-radius: 4px;
  padding: 12px;
  max-height: 240px;
  overflow: auto;
}

.json-block pre {
  margin: 0;
  font-family: 'Courier New', monospace;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
}

.detail-actions {
  margin-top: 24px;
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}

.form-tip {
  width: 100%;
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
}

:deep(.el-table__row) {
  cursor: pointer;
}
</style>
