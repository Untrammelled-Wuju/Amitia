export type TaskRunStatus =
  | "created"
  | "queued"
  | "starting"
  | "running"
  | "checkpointing"
  | "pausing"
  | "paused"
  | "resuming"
  | "cancelling"
  | "cancelled"
  | "succeeded"
  | "failed"
  | "timed_out"
  | "recovery_required"
  | "manual_intervention";

export type TaskIdempotency = "idempotent" | "conditionally_idempotent" | "non_idempotent";
export type TaskRecoverability = "not_recoverable" | "checkpoint_recoverable" | "restartable_from_beginning" | "manual_recovery";
export type TaskResultPolicy = "inline_json" | "artifact" | "auto";
export type TaskCleanupPolicy = "always" | "on_success" | "on_failure" | "retain_for_debug";

export interface TaskRetryPolicy {
  maxAttempts: number;
  initialBackoff: number;
  maxBackoff: number;
  multiplier: number;
  retryableErrorCodes?: string[];
}

export interface TaskTimeoutPolicy {
  defaultTimeout: number;
  maxTimeout: number;
  hardKillAfter: number;
}

export interface TaskResourceLimits {
  maxMemoryMB: number;
  maxCpuPercent: number;
  maxFileDescriptors: number;
  maxProcesses: number;
}

export interface PermissionRequirement {
  permission: string;
  reason?: string;
  required: boolean;
  scope?: unknown;
}

export interface ScopeRule {
  scopeType: string;
  scopeIds?: string[];
  namespaces?: string[];
}

export interface TaskDefinition {
  taskId: string;
  extensionId: string;
  moduleId: string;
  contributionId?: string;
  runtimeType: string;
  entry: string;
  entryHash?: string;
  inputSchema?: unknown;
  outputSchema?: unknown;
  checkpointSchema?: unknown;
  checkpoint: boolean;
  idempotent: boolean;
  recoverable: boolean;
  idempotency?: TaskIdempotency;
  recoverability?: TaskRecoverability;
  resourceLimits?: TaskResourceLimits;
  permissionRequirements?: PermissionRequirement[];
  allowedNamespaces?: string[];
  scopeRule?: ScopeRule;
  retryPolicy?: TaskRetryPolicy;
  timeoutPolicy?: TaskTimeoutPolicy;
  resultPolicy?: TaskResultPolicy;
  cleanupPolicy?: TaskCleanupPolicy;
  definitionVersion?: number;
  definitionHash?: string;
  version?: string;
  maxDuration?: number;
}

export interface TaskRun {
  taskRunId: string;
  operationId: string;
  invocationId?: string;
  taskDefinitionId: string;
  extensionId: string;
  moduleId: string;
  status: TaskRunStatus;
  priority: number;
  input?: unknown;
  inputHash: string;
  inputArtifactId?: string;
  scopeSnapshotId?: string;
  permissionSnapshotId?: string;
  dependencySnapshotId?: string;
  runtimeInstanceId?: string;
  checkpointId?: string;
  resultArtifactId?: string;
  attempt: number;
  maxAttempts: number;
  createdAt: string;
  queuedAt?: string;
  startedAt?: string;
  finishedAt?: string;
  deadlineAt?: string;
  cancelRequestedAt?: string;
  errorCode?: string;
  errorMessage?: string;
  generation: number;
}

export interface TaskRunProgress {
  taskRunId: string;
  sequence: number;
  current?: number;
  total?: number;
  percentage?: number;
  stage?: string;
  message?: string;
  details?: unknown;
  updatedAt: string;
}

export interface TaskCheckpoint {
  checkpointId: string;
  taskRunId: string;
  version: number;
  payload: unknown;
  payloadHash: string;
  definitionHash?: string;
  inputHash?: string;
  createdAt: string;
}

export interface TaskRunResult {
  taskRunId: string;
  resultType: TaskResultPolicy;
  resultJson?: unknown;
  artifactId?: string;
  resultHash?: string;
  createdAt: string;
}

export interface EnqueueTaskRequest {
  taskDefinitionId: string;
  extensionId?: string;
  moduleId?: string;
  input?: unknown;
  priority?: number;
  operationId?: string;
  scopeSnapshotId?: string;
  permissionSnapshotId?: string;
}

export interface EnqueueTaskResult {
  taskRunId: string;
  status: TaskRunStatus;
  queued: boolean;
  position?: number;
}

export interface ListTasksFilter {
  extensionId?: string;
  status?: string;
  limit?: number;
  offset?: number;
}

export const TERMINAL_STATUSES: TaskRunStatus[] = [
  "succeeded",
  "failed",
  "cancelled",
  "timed_out",
  "manual_intervention",
];

export const ACTIVE_STATUSES: TaskRunStatus[] = [
  "created",
  "queued",
  "starting",
  "running",
  "checkpointing",
  "pausing",
  "paused",
  "resuming",
  "cancelling",
  "recovery_required",
];

export function isTerminal(status: TaskRunStatus): boolean {
  return TERMINAL_STATUSES.includes(status);
}

export function isActive(status: TaskRunStatus): boolean {
  return ACTIVE_STATUSES.includes(status);
}

export const STATUS_LABELS: Record<TaskRunStatus, string> = {
  created: "已创建",
  queued: "排队中",
  starting: "启动中",
  running: "运行中",
  checkpointing: "检查点保存中",
  pausing: "暂停中",
  paused: "已暂停",
  resuming: "恢复中",
  cancelling: "取消中",
  cancelled: "已取消",
  succeeded: "已成功",
  failed: "已失败",
  timed_out: "已超时",
  recovery_required: "需恢复",
  manual_intervention: "需人工干预",
};

export const STATUS_TAG_TYPES: Record<TaskRunStatus, "" | "success" | "warning" | "danger" | "info"> = {
  created: "info",
  queued: "info",
  starting: "warning",
  running: "warning",
  checkpointing: "warning",
  pausing: "warning",
  paused: "info",
  resuming: "warning",
  cancelling: "danger",
  cancelled: "info",
  succeeded: "success",
  failed: "danger",
  timed_out: "danger",
  recovery_required: "warning",
  manual_intervention: "danger",
};
