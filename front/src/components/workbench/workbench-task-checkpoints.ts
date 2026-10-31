import { readWorkbenchTasks, WorkbenchTaskReadError } from './workbench-task-api';

export interface TaskCheckpoint {
  updatedAt: string;
  interactionStatus: string;
  coordinationStatus: string;
  assignmentCount: number;
  completedCount: number;
  failedCount: number;
  parentTerminal: boolean;
  consistent: boolean;
}

export interface TaskCheckpointHistory {
  records: TaskCheckpoint[];
  truncated: boolean;
}

function asCheckpoint(value: unknown): TaskCheckpoint | null {
  if (!value || typeof value !== 'object') return null;
  const data = value as Record<string, unknown>;
  if (typeof data.updatedAt !== 'string' || data.updatedAt.length > 80 || !Number.isFinite(Date.parse(data.updatedAt)) ||
    typeof data.interactionStatus !== 'string' || !data.interactionStatus || data.interactionStatus.length > 64 ||
    typeof data.coordinationStatus !== 'string' || !['planning', 'running', 'waiting', 'aggregating', 'succeeded', 'failed', 'cancelled', 'paused'].includes(data.coordinationStatus) ||
    typeof data.parentTerminal !== 'boolean' || typeof data.consistent !== 'boolean') return null;
  for (const key of ['assignmentCount', 'completedCount', 'failedCount']) {
    const count = data[key];
    if (typeof count !== 'number' || !Number.isSafeInteger(count) || count < 0 || count > 8) return null;
  }
  if ((data.assignmentCount as number) < 1 || (data.completedCount as number) > (data.assignmentCount as number) ||
      (data.failedCount as number) > (data.completedCount as number)) return null;
  return {
    updatedAt: data.updatedAt,
    interactionStatus: data.interactionStatus,
    coordinationStatus: data.coordinationStatus,
    assignmentCount: data.assignmentCount as number,
    completedCount: data.completedCount as number,
    failedCount: data.failedCount as number,
    parentTerminal: data.parentTerminal,
    consistent: data.consistent,
  };
}

export function parseTaskCheckpoints(value: unknown, conversationId: string, coordinationId: string): TaskCheckpointHistory {
  if (!value || typeof value !== 'object') throw new Error('检查点响应格式错误');
  const response = value as Record<string, unknown>;
  if (response.protocolVersion !== 1 || response.conversationId !== conversationId || response.coordinationId !== coordinationId || response.source !== 'sampled-durable-checkpoints' || !Array.isArray(response.records) || response.records.length > 20 || typeof response.truncated !== 'boolean') {
    throw new Error('检查点响应与当前任务不匹配');
  }
  const records = response.records.map(asCheckpoint);
  if (records.some(record => !record)) throw new Error('检查点记录格式错误');
  return { records: records as TaskCheckpoint[], truncated: response.truncated };
}

export async function loadTaskCheckpoints(conversationId: string, coordinationId: string, signal: AbortSignal, characterId = ''): Promise<TaskCheckpointHistory> {
  const response = await readWorkbenchTasks(conversationId, characterId, signal, coordinationId);
  try {
    return parseTaskCheckpoints(response, conversationId, coordinationId);
  } catch (error) {
  console.error("[original-error]", error);
throw new WorkbenchTaskReadError('持久检查点证据无效，旧任务快照已丢弃', true);
  }
}
