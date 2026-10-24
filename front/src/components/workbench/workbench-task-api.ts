import { apiClient } from '@/composables/useApi';
import { useDeviceOwnedConversation } from '@/composables/useDeviceOwnedConversation';
import { parseConversationReference } from '@/runtime/device-owned-conversation-reference';
import { withTaskRequestDeadline } from './workbench-task-request';
import { readWorkbenchPolicyAuthority, sameWorkbenchPolicyAuthority, sameWorkbenchRoleSelection } from './workbench-task-authority';

export class WorkbenchTaskReadError extends Error {
  constructor(message: string, readonly invalidatesSnapshot: boolean) { super(message); }
}

export interface WorkbenchBudgetEvidenceQuery {
  parentInteractionId: string;
  kind: 'model' | 'tool';
  afterSequence?: number;
  afterCreatedAt?: string;
  afterEffectId?: string;
}

interface BudgetEvidenceRegistration {
  kind: 'model' | 'tool';
  recordId: string;
  evidenceRef: string;
}

export function readWorkbenchTasks(conversationId: string, characterId: string, signal: AbortSignal, coordinationId?: string, evidence?: WorkbenchBudgetEvidenceQuery): Promise<Record<string, unknown>> {
  return requestWorkbenchTasks(conversationId, characterId, signal, coordinationId, evidence);
}

export function registerWorkbenchBudgetEvidence(conversationId: string, characterId: string, coordinationId: string, parentInteractionId: string, registration: BudgetEvidenceRegistration, signal: AbortSignal): Promise<Record<string, unknown>> {
  return requestWorkbenchTasks(conversationId, characterId, signal, coordinationId, { parentInteractionId, kind: registration.kind }, registration);
}

async function requestWorkbenchTasks(conversationId: string, characterId: string, signal: AbortSignal, coordinationId?: string, evidence?: WorkbenchBudgetEvidenceQuery, registration?: BudgetEvidenceRegistration): Promise<Record<string, unknown>> {
  let origin: ReturnType<typeof parseConversationReference>;
  try { origin = parseConversationReference(conversationId); }
  catch { throw new WorkbenchTaskReadError('无法确认任务所属会话和角色，请重新打开对话', true); }
  if (!origin || !characterId) throw new WorkbenchTaskReadError('无法确认任务所属会话和角色，请重新打开对话', true);
  const owned = useDeviceOwnedConversation();
  try {
    await owned.refresh();
  } catch (cause: any) {
    const status = Number(cause?.response?.status);
    throw new WorkbenchTaskReadError(String(cause?.response?.data?.error || cause?.message || 'Core 授权查询失败'), [401, 403, 404, 409].includes(status));
  }
  if (signal.aborted) throw new Error('任务查询已取消');
  const coreId = owned.coreId.value;
  const initialPolicy = readWorkbenchPolicyAuthority(owned.policy.value);
  if (!initialPolicy) throw new WorkbenchTaskReadError('任务执行权限不可确认，请重新加载', true);
  const initialRoleSelection = owned.policy.value?.selectedRole;
  if (!coreId) throw new WorkbenchTaskReadError('执行 Core 尚未就绪', true);
  try {
    const path = `/api/workbench/tasks${registration ? '/budget-reconciliation/review' : evidence ? '/budget-reconciliation' : coordinationId ? '/checkpoints' : ''}`;
    const params = { conversationId: origin.id, conversationOwnerId: origin.ownerId, characterId, expectedCoreId: coreId, ...(coordinationId ? { coordinationId } : {}), ...(evidence ? registration ? { parentInteractionId: evidence.parentInteractionId } : { ...evidence, limit: 50 } : {}) };
    const response = await withTaskRequestDeadline(active => registration
      ? apiClient.post(path, registration, { params, signal: active })
      : apiClient.get(path, { params, signal: active }), signal);
    const data = response.data;
    if (signal.aborted) throw new Error('任务查询已取消');
    await owned.refresh();
    const context = data?.queryContext;
    const scope = context?.executionScope;
    const currentPolicy = owned.policy.value;
    if (!sameWorkbenchPolicyAuthority(initialPolicy, currentPolicy, scope)) throw new Error('任务响应权限版本已变化，请重新加载');
    if (!sameWorkbenchRoleSelection(initialRoleSelection, currentPolicy?.selectedRole)) throw new Error('任务角色选择已变化，请重新加载');
    if (signal.aborted || owned.coreId.value !== coreId || scope?.coreId !== coreId || scope?.roleId !== characterId || !scope?.spaceId || !scope?.resourceOwnerId || context?.conversationOrigin?.ownerId !== origin.ownerId || context?.conversationOrigin?.id !== origin.id || !context?.conversationId || data?.conversationId !== context.conversationId) {
      throw new Error('任务响应归属已变化，请重新加载');
    }
    return { ...data, conversationId };
  } catch (cause: any) {
    const message = String(cause?.response?.data?.error || cause?.message || '任务所属执行服务暂不可用');
    const status = Number(cause?.response?.status);
    const revoked = [401, 403, 404, 409].includes(status) || /归属|权限版本|身份|角色|执行 Core 已变化/.test(message);
    throw new WorkbenchTaskReadError(message, revoked);
  }
}
