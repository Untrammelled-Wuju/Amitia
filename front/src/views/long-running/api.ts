// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { useApi } from "@/composables/useApi";
import { coreConfigurationRequestConfig } from "@/composables/useCoreConfigurationAccess";
const { get, post, put } = useApi();

export interface TaskMaintenanceStatus {
  source: 'task-runtime';
  running: boolean;
  activeCount: number;
  localProcessCount: number;
  truncated: boolean;
  tasks: Array<{ id: string; title: string; status: string; updatedAt: string; placement: string }>;
}

export interface TaskMaintenanceConfig {
  source: 'task-runtime';
  runtimeProfile: string;
  maxTasks: number;
  timeout: { disabled: boolean; seconds: number };
  revision: number;
  timeoutRevision: number;
  applied: boolean;
}

export async function fetchStatusApi(context: string) {
  const value = await get<TaskMaintenanceStatus>("/api/runtime/tasks/status", undefined, coreConfigurationRequestConfig(context));
  if (value?.source !== 'task-runtime' || !Array.isArray(value.tasks) || !Number.isInteger(value.activeCount)) throw new Error('任务执行状态无效');
  return value;
}

export function requireAppliedConfig(value: TaskMaintenanceConfig): TaskMaintenanceConfig {
  if (value?.source !== 'task-runtime' || value.applied !== true || !Number.isInteger(value.revision) || value.revision < 1 || !Number.isInteger(value.timeoutRevision) || value.timeoutRevision < 0 || !Number.isInteger(value.maxTasks) || value.maxTasks < 1 || value.maxTasks > 20 || typeof value.timeout?.disabled !== 'boolean' || !Number.isInteger(value.timeout.seconds) || value.timeout.seconds < 30 || value.timeout.seconds > 1800 || value.timeout.seconds % 30 !== 0) throw new Error('执行器未确认配置生效');
  return value;
}

export async function fetchConfigApi(context: string) {
  return requireAppliedConfig(await get<TaskMaintenanceConfig>("/api/runtime/tasks/config", undefined, coreConfigurationRequestConfig(context)));
}

export async function saveConfigApi(data: TaskMaintenanceConfig, context: string) {
  const saved = requireAppliedConfig(await put<TaskMaintenanceConfig>("/api/runtime/tasks/config", {
    maxTasks: data.maxTasks,
    timeout: data.timeout,
    expectedRevision: data.revision,
    expectedTimeoutRevision: data.timeoutRevision,
  }, coreConfigurationRequestConfig(context)));
  if (saved.maxTasks !== data.maxTasks || saved.timeout.disabled !== data.timeout.disabled || saved.timeout.seconds !== data.timeout.seconds || saved.revision !== data.revision + 1 || saved.timeoutRevision !== data.timeoutRevision + 1) throw new Error('执行器返回的配置与本次保存不一致，请重新加载');
  return saved;
}

export function cleanupTempApi(context: string) {
  return post<{ deleted: number; freedBytes: number }>(
    "/api/runtime/cleanup-temp", undefined, coreConfigurationRequestConfig(context),
  );
}

export function rotateLogsApi(context: string) {
  return post<{ rotated: string[]; skipped: string[] }>(
    "/api/runtime/rotate-logs", undefined, coreConfigurationRequestConfig(context),
  );
}

export function checkDbIntegrityApi(context: string) {
  return post<{ ok: boolean; errors: string[] }>(
    "/api/runtime/check-db-integrity", undefined, coreConfigurationRequestConfig(context),
  );
}
