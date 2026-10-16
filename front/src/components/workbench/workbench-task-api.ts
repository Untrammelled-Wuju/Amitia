import { apiClient } from '@/composables/useApi';
import { useDeviceOwnedConversation } from '@/composables/useDeviceOwnedConversation';
import { parseConversationReference } from '@/runtime/device-owned-conversation-reference';
import { withTaskRequestDeadline } from './workbench-task-request';

export async function readWorkbenchTasks(conversationId: string, characterId: string, signal: AbortSignal, coordinationId?: string): Promise<Record<string, unknown>> {
  const origin = parseConversationReference(conversationId);
  if (!origin || !characterId) throw new Error('无法确认任务所属会话和角色，请重新打开对话');
  const owned = useDeviceOwnedConversation();
  await owned.refresh();
  if (signal.aborted) throw new Error('任务查询已取消');
  const coreId = owned.coreId.value;
  const policy = JSON.stringify(owned.policy.value);
  if (!coreId) throw new Error('执行 Core 尚未就绪');
  try {
    const response = await withTaskRequestDeadline(active => apiClient.get(`/api/workbench/tasks${coordinationId ? '/checkpoints' : ''}`, {
      params: { conversationId: origin.id, conversationOwnerId: origin.ownerId, characterId, expectedCoreId: coreId, ...(coordinationId ? { coordinationId } : {}) }, signal: active,
    }), signal);
    const data = response.data;
    const context = data?.queryContext;
    const scope = context?.executionScope;
    const currentPolicy = owned.policy.value;
    if (!currentPolicy || scope?.providerEpoch !== currentPolicy.providerEpoch || scope?.modeRevision !== currentPolicy.modeRevision || scope?.permissionRevision !== currentPolicy.permissionRevision || scope?.coordinated !== currentPolicy.coordinated) throw new Error('任务响应权限版本已变化，请重新加载');
    if (signal.aborted || owned.coreId.value !== coreId || JSON.stringify(owned.policy.value) !== policy || scope?.coreId !== coreId || scope?.roleId !== characterId || !scope?.spaceId || !scope?.resourceOwnerId || context?.conversationOrigin?.ownerId !== origin.ownerId || context?.conversationOrigin?.id !== origin.id || !context?.conversationId || data?.conversationId !== context.conversationId) {
      throw new Error('任务响应归属已变化，请重新加载');
    }
    return { ...data, conversationId };
  } catch (cause: any) {
    throw new Error(cause?.response?.data?.error || cause?.message || '任务所属执行服务暂不可用');
  }
}
