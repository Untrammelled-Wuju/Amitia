import { readWorkbenchTasks, WorkbenchTaskReadError, type WorkbenchBudgetEvidenceQuery } from './workbench-task-api';

export interface BudgetEvidenceRecord {
  id: string;
  status: string;
  startedAt: string;
  reservedTokens: number;
  reservedCostMicroUSD: number;
  usageReported: boolean;
  budgetOverrun: boolean;
  toolName: string;
  reviewEvidenceRef: string;
  reviewActorId: string;
  reviewRecordedAt: string;
}

export interface BudgetEvidencePage {
  records: BudgetEvidenceRecord[];
  canReview: boolean;
  next?: { afterSequence?: number; afterCreatedAt?: string; afterEffectId?: string };
}

function record(value: unknown, kind: 'model' | 'tool'): BudgetEvidenceRecord {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('预算证据无效');
  const raw = value as Record<string, unknown>;
  const reviewEvidenceRef = raw.reviewEvidenceRef ?? '';
  const reviewActorId = raw.reviewActorId ?? '';
  const reviewRecordedAt = raw.reviewRecordedAt ?? '';
  if (typeof reviewEvidenceRef !== 'string' || reviewEvidenceRef.length > 512 ||
    typeof reviewActorId !== 'string' || reviewActorId.length > 160 ||
    typeof reviewRecordedAt !== 'string' || reviewRecordedAt.length > 80 ||
    (reviewEvidenceRef !== '' && (!reviewActorId || !Number.isFinite(Date.parse(reviewRecordedAt))))) {
    throw new Error('人工复核证据格式无效');
  }
  if (kind === 'model') {
    if (typeof raw.id !== 'string' || raw.id.length === 0 || raw.id.length > 160 ||
      typeof raw.status !== 'string' || !['running', 'interrupted', 'failed', 'cancelled', 'succeeded'].includes(raw.status) ||
      typeof raw.reservedTokens !== 'number' || !Number.isSafeInteger(raw.reservedTokens) || raw.reservedTokens < 0 ||
      typeof raw.reservedCostMicroUSD !== 'number' || !Number.isSafeInteger(raw.reservedCostMicroUSD) || raw.reservedCostMicroUSD < 0 ||
      typeof raw.usageReported !== 'boolean' || (raw.budgetOverrun !== undefined && typeof raw.budgetOverrun !== 'boolean') || (raw.budgetOverrun === true && raw.usageReported === true) || typeof raw.startedAt !== 'string' || raw.startedAt.length > 80) {
      throw new Error('模型预算证据格式无效');
    }
    return { id: raw.id, status: raw.status, startedAt: raw.startedAt, reservedTokens: raw.reservedTokens, reservedCostMicroUSD: raw.reservedCostMicroUSD, usageReported: raw.usageReported, budgetOverrun: raw.budgetOverrun === true, toolName: '', reviewEvidenceRef, reviewActorId, reviewRecordedAt };
  }
  if (typeof raw.EffectID !== 'string' || raw.EffectID.length === 0 || raw.EffectID.length > 255 ||
    typeof raw.ToolName !== 'string' || raw.ToolName.length > 256 ||
    typeof raw.CreatedAt !== 'string' || raw.CreatedAt.length > 80) {
    throw new Error('工具调用预算证据格式无效');
  }
  return { id: raw.EffectID, status: 'unconfirmed', startedAt: raw.CreatedAt, reservedTokens: 0, reservedCostMicroUSD: 0, usageReported: false, budgetOverrun: false, toolName: raw.ToolName, reviewEvidenceRef, reviewActorId, reviewRecordedAt };
}

export function parseBudgetEvidencePage(raw: unknown, expected: { coordinationId: string; parentInteractionId: string; kind: 'model' | 'tool' }): BudgetEvidencePage {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) throw new Error('预算响应格式无效');
  const data = raw as Record<string, unknown>;
  if (data.protocolVersion !== 1 || data.source !== 'durable-budget-ledger' || data.coordinationId !== expected.coordinationId || data.parentInteractionId !== expected.parentInteractionId || data.kind !== expected.kind || !Array.isArray(data.records) || data.records.length > 50 || typeof data.nextPagePossible !== 'boolean') {
    throw new Error('预算响应不属于当前父任务');
  }
  const records = data.records.map(item => record(item, expected.kind));
  if (data.canReview !== undefined && typeof data.canReview !== 'boolean') throw new Error('预算登记权限无效');
  const canReview = data.canReview === true;
  if (!data.nextPagePossible) return { records, canReview };
  if (records.length !== 50) throw new Error('预算分页证据不完整');
  if (expected.kind === 'model') {
    const cursor = data.nextAfterSequence;
    if (typeof cursor !== 'number' || !Number.isSafeInteger(cursor) || cursor < 1) throw new Error('模型预算游标无效');
    return { records, canReview, next: { afterSequence: cursor } };
  }
  if (typeof data.nextAfterCreatedAt !== 'string' || !Number.isFinite(Date.parse(data.nextAfterCreatedAt)) || typeof data.nextAfterEffectId !== 'string' || data.nextAfterEffectId.length === 0 || data.nextAfterEffectId.length > 255) throw new Error('工具预算游标无效');
  return { records, canReview, next: { afterCreatedAt: data.nextAfterCreatedAt, afterEffectId: data.nextAfterEffectId } };
}

export async function loadBudgetEvidence(conversationId: string, characterId: string, coordinationId: string, parentInteractionId: string, kind: 'model' | 'tool', signal: AbortSignal, cursor: BudgetEvidencePage['next'] = undefined): Promise<BudgetEvidencePage> {
  const request: WorkbenchBudgetEvidenceQuery = { parentInteractionId, kind, ...(cursor || {}) };
  const result = await readWorkbenchTasks(conversationId, characterId, signal, coordinationId, request);
  try {
    return parseBudgetEvidencePage(result, { coordinationId, parentInteractionId, kind });
  } catch {
    throw new WorkbenchTaskReadError('预算核对证据校验失败，请重新授权后刷新', true);
  }
}
