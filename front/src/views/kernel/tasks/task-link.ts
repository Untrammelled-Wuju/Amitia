export interface LinkedTaskRun {
  taskRunId: string;
}

export interface TaskAuthorizationSnapshot extends LinkedTaskRun {
  executionScope?: unknown;
  managementExecutionScope?: unknown;
  ownerId?: string;
  readOnly?: boolean;
  generation?: number;
  attempt?: number;
  status?: string;
  taskDefinitionId?: string;
  inputHash?: string;
  scopeSnapshotId?: string;
  permissionSnapshotId?: string;
  dependencySnapshotId?: string;
  runtimeInstanceId?: string;
  checkpointId?: string;
  resultArtifactId?: string;
  deadlineAt?: string;
  cancelRequestedAt?: string;
  startedAt?: string;
  finishedAt?: string;
}

export function resolveLinkedTask<T extends LinkedTaskRun>(requested: unknown, authorizedTasks: readonly T[]): T | undefined {
  if (typeof requested !== 'string' || !/^[A-Za-z0-9_.:-]{1,160}$/.test(requested)) return undefined;
  const matches = authorizedTasks.filter(task => task.taskRunId === requested);
  return matches.length === 1 ? matches[0] : undefined;
}

export function currentAuthorizedTask<T extends TaskAuthorizationSnapshot>(previous: T | null, authorizedTasks: readonly T[]): T | undefined {
  if (!previous) return undefined;
  const current = resolveLinkedTask(previous.taskRunId, authorizedTasks);
  if (!current ||
      current.ownerId !== previous.ownerId ||
      current.readOnly !== previous.readOnly ||
      current.generation !== previous.generation ||
      current.attempt !== previous.attempt ||
      current.status !== previous.status ||
      current.taskDefinitionId !== previous.taskDefinitionId ||
      current.inputHash !== previous.inputHash ||
      current.scopeSnapshotId !== previous.scopeSnapshotId ||
      current.permissionSnapshotId !== previous.permissionSnapshotId ||
      current.dependencySnapshotId !== previous.dependencySnapshotId ||
      current.runtimeInstanceId !== previous.runtimeInstanceId ||
      current.checkpointId !== previous.checkpointId ||
      current.resultArtifactId !== previous.resultArtifactId ||
      current.deadlineAt !== previous.deadlineAt ||
      current.cancelRequestedAt !== previous.cancelRequestedAt ||
      current.startedAt !== previous.startedAt ||
      current.finishedAt !== previous.finishedAt ||
      JSON.stringify(current.executionScope) !== JSON.stringify(previous.executionScope) ||
      JSON.stringify(current.managementExecutionScope) !== JSON.stringify(previous.managementExecutionScope)) return undefined;
  return current;
}

export function detailStillAuthorized<T extends TaskAuthorizationSnapshot>(
  previouslyOpened: T | null,
  authorizedTasks: readonly T[],
): boolean {
  return !!currentAuthorizedTask(previouslyOpened, authorizedTasks);
}

export function currentControlTarget<T extends TaskAuthorizationSnapshot>(previous: T, authorizedTasks: readonly T[]): T | undefined {
  const current = currentAuthorizedTask(previous, authorizedTasks);
  return current && current.status === previous.status && current.readOnly !== true ? current : undefined;
}

export function taskEvidenceMatches(
  task: TaskAuthorizationSnapshot,
  progress?: { taskRunId?: string } | null,
  result?: { taskRunId?: string; artifactId?: string } | null,
  checkpoint?: { taskRunId?: string; checkpointId?: string } | null,
): boolean {
  if (!task.taskRunId || [progress, result, checkpoint].some(part => part != null && part.taskRunId !== task.taskRunId)) return false;
  if (result && task.resultArtifactId && result.artifactId !== task.resultArtifactId) return false;
  if (checkpoint && task.checkpointId && checkpoint.checkpointId !== task.checkpointId) return false;
  return true;
}
