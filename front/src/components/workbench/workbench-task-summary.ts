export interface TaskSummary {
  total: number;
  active: number;
  paused: number;
  succeeded: number;
  failed: number;
  runtimeObserved: number;
  checkpointOnly: number;
  needsReconciliation: number;
  codingWorkersRequiringReview: number;
}

const keys = ['total', 'active', 'paused', 'succeeded', 'failed', 'runtimeObserved', 'checkpointOnly', 'needsReconciliation', 'codingWorkersRequiringReview'] as const;

export function normalizeTaskSummary(value: unknown): TaskSummary {
  const source = value && typeof value === 'object' ? value as Record<string, unknown> : {};
  const result: Record<string, number> = {};
  for (const key of keys) {
    const raw = source[key];
    result[key] = typeof raw === 'number' && Number.isSafeInteger(raw) && raw >= 0 ? raw : 0;
  }
  return result as unknown as TaskSummary;
}
