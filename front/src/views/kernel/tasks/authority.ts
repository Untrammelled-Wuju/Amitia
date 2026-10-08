import { apiClient } from "@/composables/useApi";
import { getDeploymentConfig, getRuntimeConnection, isCurrentDevicePaired } from "@/runtime/runtime-adapter";
import type { TaskRun } from "./types";

export type TaskAuthority = { executionScope?: Record<string, unknown>; managementExecutionScope?: Record<string, unknown>; ownerId?: string; readOnly?: boolean };
type Intent = { epoch: number; base: string; bound: boolean; policy: string; coreId: string };
const intents = new WeakMap<object, Intent>();
let epoch = 0;
if (typeof window !== "undefined") {
  for (const event of ["amitia:runtime-connection-changed", "amitia:execution-scope-changed"]) window.addEventListener(event, () => { epoch++; });
}
export function taskScopeStamp(scope: Record<string, unknown>) {
  return JSON.stringify(Object.keys(scope).filter(key => !["requestId", "turnId", "executionId"].includes(key)).sort().map(key => [key, scope[key]]));
}
export async function captureTaskIntent(): Promise<Intent> {
  const start = epoch;
  const connection = await getRuntimeConnection();
  const deployment = await getDeploymentConfig();
  const bound = deployment.mode === "cloud" || await isCurrentDevicePaired(connection.apiBaseURL);
  let coreId = "", policy = "";
  if (bound) {
    const state = (await apiClient.get("/api/device-mesh/v1/coordination/me")).data;
    const value = state?.policy;
    if (!state?.coordinationAvailable || typeof state.coreId !== "string" || !state.coreId || typeof value?.coordinated !== "boolean" || [value.providerEpoch, value.modeRevision, value.permissionRevision].some(n => !Number.isSafeInteger(n) || n < 1)) throw new Error("任务 Core 服务尚未就绪");
    coreId = state.coreId;
    policy = JSON.stringify([value.providerEpoch, value.modeRevision, value.permissionRevision, value.coordinated]);
  }
  if (start !== epoch) throw new Error("任务服务已切换，请重新打开列表");
  return { epoch: start, base: connection.apiBaseURL, bound, policy, coreId };
}
export async function assertTaskIntent(intent: Intent) {
  if (intent.epoch !== epoch || JSON.stringify(intent) !== JSON.stringify(await captureTaskIntent())) throw new Error("任务原服务或权限范围已变化，请重新打开列表");
}
export function retainTaskAuthority<T extends TaskAuthority>(value: T, intent: Intent): T {
  if (intent.bound) {
    const scope = value?.executionScope, management = value?.managementExecutionScope;
    if (!scope || !management || scope.coreId !== intent.coreId || management.coreId !== intent.coreId || value.ownerId !== scope.resourceOwnerId || typeof value.readOnly !== "boolean" || JSON.stringify([management.providerEpoch, management.modeRevision, management.permissionRevision, management.coordinated]) !== intent.policy) throw new Error("任务响应缺少原所有者和有效管理范围");
  }
  const row = Object.freeze({ ...value, ...(value.executionScope ? { executionScope: Object.freeze({ ...value.executionScope }) } : {}), ...(value.managementExecutionScope ? { managementExecutionScope: Object.freeze({ ...value.managementExecutionScope }) } : {}) }) as T;
  intents.set(row, intent);
  return row;
}
export async function originalTaskIntent(task: TaskRun, id: string, write = false) {
  const intent = intents.get(task);
  if (!intent || task.taskRunId !== id || write && task.readOnly === true) throw new Error("请从原任务列表选择有效记录，历史任务只读");
  await assertTaskIntent(intent);
  return intent;
}
export function validateTaskPart<T extends TaskAuthority & { taskRunId?: string }>(value: T, expected: TaskRun, intent: Intent, retry = false): T {
  if (intent.bound && (!value || (retry ? !value.taskRunId : value.taskRunId !== expected.taskRunId) || !value.executionScope || !expected.executionScope || taskScopeStamp(value.executionScope) !== taskScopeStamp(expected.executionScope))) throw new Error("任务响应归属发生变化，迟到结果已丢弃");
  return retainTaskAuthority(value, intent);
}
