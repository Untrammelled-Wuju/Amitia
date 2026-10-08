import { apiClient } from "@/composables/useApi";
import { assertTaskIntent, captureTaskIntent, originalTaskIntent, retainTaskAuthority, validateTaskPart } from "./authority";
import type {
  TaskDefinition,
  TaskRun,
  TaskRunProgress,
  TaskCheckpoint,
  TaskRunResult,
  EnqueueTaskRequest,
  EnqueueTaskResult,
  ListTasksFilter,
  OwnedTaskRoleOptions,
  OwnedTaskSubmission,
  DeviceTaskCatalogPage,
	SourceTaskApproval,
} from "./types";

const TASKS_BASE = "/api/extensions/tasks";
const DEFS_BASE = "/api/extensions/task-definitions";

export async function listSourceTaskApprovals(): Promise<SourceTaskApproval[]> {
  const items = (await apiClient.get("/internal/device-mesh/task-approvals")).data;
  if (!Array.isArray(items) || items.length > 64 || items.some(item => {
    const binding = item?.binding;
    const target = binding?.executionTarget;
    return !item?.id || !Number.isSafeInteger(item.revision) || item.revision < 1 || !["pending", "approved", "denied", "claimed", "revoked"].includes(item.status) || !binding?.executionScope?.coreId || !binding.executionScope.targetDeviceId || !binding.taskRunId || !Number.isSafeInteger(binding.taskGeneration) || binding.taskGeneration < 1 || target?.spaceId !== binding.executionScope.coreId || target?.deviceId !== binding.executionScope.targetDeviceId || !target?.runtimeId || !target?.runtimeSessionId || !Number.isSafeInteger(target?.connectionGeneration) || target.connectionGeneration < 1 || !binding.target?.taskId || !Number.isSafeInteger(binding.target.installedGeneration) || binding.target.installedGeneration < 1 || !/^[a-f0-9]{64}$/.test(binding.inputHash) || !Array.isArray(item.permissions) || item.permissions.length > 64 || item.permissions.some((permission: any) => !permission?.permissionId);
  })) {
    throw new Error("目标设备返回的审批身份或资源声明无效");
  }
  return items;
}

export async function decideSourceTaskApproval(approval: SourceTaskApproval, approved: boolean): Promise<void> {
  if (approval.status !== "pending" || !Number.isSafeInteger(approval.revision) || approval.revision < 1) throw new Error("审批已处理，请刷新列表");
  const value = (await apiClient.post(`/internal/device-mesh/task-approvals/${encodeURIComponent(approval.id)}/decision`, { expectedRevision: approval.revision, approved })).data;
  if (value?.id !== approval.id || value?.revision !== approval.revision + 1 || value?.status !== (approved ? "approved" : "denied") || JSON.stringify(value.binding) !== JSON.stringify(approval.binding)) throw new Error("设备未确认本次审批决定，请刷新列表");
}

export async function revokeSourceTaskApproval(approval: SourceTaskApproval): Promise<void> {
  if (!["approved", "claimed"].includes(approval.status) || !Number.isSafeInteger(approval.revision) || approval.revision < 1) throw new Error("审批状态已变化，请刷新列表");
  const value = (await apiClient.post(`/internal/device-mesh/task-approvals/${encodeURIComponent(approval.id)}/revoke`, { expectedRevision: approval.revision })).data;
  if (value?.id !== approval.id || value?.revision !== approval.revision + 1 || value?.status !== "revoked" || JSON.stringify(value.binding) !== JSON.stringify(approval.binding)) throw new Error("设备未确认本次撤销，请刷新列表");
}

export async function listTasks(filter: ListTasksFilter = {}): Promise<{ items: TaskRun[]; total: number }> {
  const intent = await captureTaskIntent();
  const res = await apiClient.get(TASKS_BASE, { params: filter });
  await assertTaskIntent(intent);
  return { ...res.data, items: (res.data.items || []).map((row: TaskRun) => retainTaskAuthority(row, intent)) };
}

export async function getTask(taskRunId: string, expected?: TaskRun): Promise<TaskRun> {
  const intent = expected ? await originalTaskIntent(expected, taskRunId) : await captureTaskIntent();
  if (intent.bound && !expected) throw new Error("请从原任务列表打开详情");
  const res = await apiClient.get(`${TASKS_BASE}/${encodeURIComponent(taskRunId)}`, { params: expected?.executionScope ? { expectedExecutionScope: JSON.stringify(expected.executionScope) } : {} });
  await assertTaskIntent(intent);
  return expected ? validateTaskPart(res.data, expected, intent) : retainTaskAuthority(res.data, intent);
}

export async function enqueueTask(req: EnqueueTaskRequest): Promise<EnqueueTaskResult> {
  const res = await apiClient.post(TASKS_BASE, req);
  return res.data;
}

export async function prepareOwnedTask(targetDeviceId: string): Promise<OwnedTaskRoleOptions> {
  const before = (await apiClient.get("/api/device-mesh/v1/coordination/me")).data;
  if (!before?.coreId || !before.coordinationAvailable) throw new Error("Core 的设备任务服务尚未就绪");
  const result: OwnedTaskRoleOptions = (await apiClient.get("/api/device-mesh/v1/business/tasks/roles", { params: { targetDeviceId } })).data;
  const after = (await apiClient.get("/api/device-mesh/v1/coordination/me")).data;
  if (!after?.coordinationAvailable || after.coreId !== before.coreId || result?.executionScope?.coreId !== before.coreId || result.executionScope.targetDeviceId !== targetDeviceId || result.roleOwnerId !== result.executionScope.roleOwnerId || result.roleOwnerId !== result.executionScope.resourceOwnerId || !Array.isArray(result.roles) || result.roles.some(role => !role.id || !Number.isSafeInteger(role.revision) || role.revision < 1)) {
    throw new Error("服务提供者或目标数据归属已变化，请刷新后重新选择角色");
  }
  return result;
}

export async function enqueueOwnedTask(req: OwnedTaskSubmission): Promise<EnqueueTaskResult> {
  const result = (await apiClient.post("/api/device-mesh/v1/business/tasks", req)).data;
  const confirmed = result?.executionScope;
  const scopeMatches = confirmed && Object.entries(req.expectedExecutionScope).every(([key, value]) => ["requestId", "turnId", "executionId"].includes(key) || confirmed[key] === value);
  if (!result?.task?.taskRunId || !scopeMatches || confirmed.coreId !== req.expectedCoreId || confirmed.targetDeviceId !== req.targetDeviceId || confirmed.roleId !== req.characterId || confirmed.roleRevision !== req.expectedRoleRevision || confirmed.modeRevision !== req.expectedModeRevision) {
    throw new Error("任务提交结果缺少当前服务提供者的有效确认，请使用原请求重试");
  }
  return result.task;
}

export async function listOwnedDeviceTaskCatalog(options: OwnedTaskRoleOptions, characterId: string, cursor = ""): Promise<DeviceTaskCatalogPage> {
  const role = options.roles.find(candidate => candidate.id === characterId);
  if (!role) throw new Error("请先选择当前数据归属方提供的角色");
  const expected = { ...options.executionScope, roleId: role.id, roleRevision: role.revision };
  const page: DeviceTaskCatalogPage = (await apiClient.post("/api/device-mesh/v1/business/tasks/catalog", { targetDeviceId: expected.targetDeviceId, characterId: role.id, requestId: crypto.randomUUID(), expectedExecutionScope: expected, cursor, limit: 8 })).data;
  const scopeMatches = page?.executionScope && Object.entries(expected).every(([key, value]) => ["requestId", "turnId", "executionId"].includes(key) || page.executionScope[key as keyof typeof expected] === value);
  if (!scopeMatches || !/^[a-f0-9]{64}$/.test(page.revision) || !Array.isArray(page.entries) || page.entries.length > 8 || (page.nextCursor?.length ?? 0) > 1024) throw new Error("设备任务目录缺少有效的服务提供者确认");
  const seen = new Set<string>();
  for (const entry of page.entries) {
    const reference = entry?.reference;
    const pin = entry?.target;
    if (!reference || !pin || reference.coreId !== expected.coreId || reference.deviceId !== expected.targetDeviceId || !/^mesh-task-[a-f0-9]{64}$/.test(reference.catalogId) || seen.has(reference.catalogId) || reference.sourceTaskId !== entry.definition?.taskId || reference.sourceTaskId !== pin.taskId || pin.deviceId !== expected.targetDeviceId || reference.portableFingerprint !== pin.portableFingerprint || !/^[a-f0-9]{64}$/.test(pin.portableFingerprint) || !/^[a-f0-9]{64}$/.test(pin.definitionFingerprint) || !Number.isSafeInteger(pin.installedGeneration) || pin.installedGeneration < 1) throw new Error("设备任务目录来源或安装版本无效");
    seen.add(reference.catalogId);
  }
  return page;
}

async function controlTask(id: string, action: string, body: Record<string, unknown>, expected?: TaskRun): Promise<TaskRun> {
  const intent = expected ? await originalTaskIntent(expected, id, true) : await captureTaskIntent();
  if (intent.bound && !expected) throw new Error("缺少任务原始范围，请重新打开列表");
  const result = (await apiClient.post(`${TASKS_BASE}/${encodeURIComponent(id)}/${action}`, { ...body, ...(expected?.executionScope ? { expectedExecutionScope: expected.executionScope } : {}) })).data;
  await assertTaskIntent(intent);
  return expected ? validateTaskPart(result, expected, intent, action === "retry") : result;
}
export async function cancelTask(taskRunId: string, reason?: string, expected?: TaskRun): Promise<TaskRun> {
  return controlTask(taskRunId, "cancel", { reason: reason || "user_requested" }, expected);
}
export async function pauseTask(taskRunId: string, generation: number, reason = "user_requested", expected?: TaskRun): Promise<TaskRun> {
  return controlTask(taskRunId, "pause", { generation, reason }, expected);
}
export async function resumeTask(taskRunId: string, generation: number, expected?: TaskRun): Promise<TaskRun> {
  return controlTask(taskRunId, "resume", { generation, resumeKind: "resume" }, expected);
}
export async function retryTask(taskRunId: string, expected?: TaskRun): Promise<TaskRun> {
  return controlTask(taskRunId, "retry", {}, expected);
}
export async function recoverTask(taskRunId: string, expected?: TaskRun): Promise<TaskRun> {
  return controlTask(taskRunId, "recover", {}, expected);
}
async function taskPart(id: string, part: string, expected?: TaskRun) {
  const intent = expected ? await originalTaskIntent(expected, id) : await captureTaskIntent();
  if (intent.bound && !expected) throw new Error("请从原任务列表打开结果");
  const result = (await apiClient.get(`${TASKS_BASE}/${encodeURIComponent(id)}/${part}`, { params: expected?.executionScope ? { expectedExecutionScope: JSON.stringify(expected.executionScope) } : {} })).data;
  await assertTaskIntent(intent);
  return expected && result ? validateTaskPart(result, expected, intent) : result;
}
export async function getTaskProgress(taskRunId: string, expected?: TaskRun): Promise<TaskRunProgress | null> {
  return taskPart(taskRunId, "progress", expected);
}
export async function getTaskResult(taskRunId: string, expected?: TaskRun): Promise<TaskRunResult | null> {
  return taskPart(taskRunId, "result", expected);
}
export async function getTaskCheckpoint(taskRunId: string, expected?: TaskRun): Promise<TaskCheckpoint | null> {
  return taskPart(taskRunId, "checkpoint", expected);
}

export async function downloadTaskArtifact(expected: TaskRun, artifactId?: string, hash?: string): Promise<Blob> {
  const intent = await originalTaskIntent(expected, expected.taskRunId);
  if (intent.bound && !/^[a-f0-9]{64}$/.test(hash || "")) throw new Error("产物缺少原所有者的有效哈希确认");
  const result = await apiClient.get(`${TASKS_BASE}/${encodeURIComponent(expected.taskRunId)}/result/artifact`, { responseType: "blob", params: { artifactId, ...(expected.executionScope ? { expectedExecutionScope: JSON.stringify(expected.executionScope) } : {}) } });
  await assertTaskIntent(intent);
  const blob = result.data as Blob;
  if (hash) {
    const digest = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", await blob.arrayBuffer())), value => value.toString(16).padStart(2, "0")).join("");
    if (digest !== hash) throw new Error("下载产物哈希与原任务结果不一致");
    await assertTaskIntent(intent);
  }
  return blob;
}

export async function listTaskDefinitions(extensionId?: string): Promise<{ items: TaskDefinition[]; total: number }> {
  const res = await apiClient.get(DEFS_BASE, { params: { extensionId } });
  return res.data;
}

export async function getTaskDefinition(defId: string): Promise<TaskDefinition> {
  const res = await apiClient.get(`${DEFS_BASE}/${encodeURIComponent(defId)}`);
  return res.data;
}

export async function createTaskDefinition(def: TaskDefinition): Promise<TaskDefinition> {
  const res = await apiClient.post(DEFS_BASE, def);
  return res.data;
}
