import { beforeEach, describe, expect, it, vi } from 'vitest';
import { defineComponent, h, KeepAlive } from 'vue';
import { flushPromises, mount } from '@vue/test-utils';

const mocks = vi.hoisted(() => ({ post: vi.fn(), roles: { value: [{ id: 'role' }] }, get: vi.fn(), refresh: vi.fn(), coreId: { value: 'core-a' }, policy: { value: { providerEpoch: 1, modeRevision: 1, permissionRevision: 1, coordinated: true } } }));
vi.mock('@/composables/useApi', () => ({ apiClient: { get: mocks.get, post: mocks.post } }));
vi.mock('@/composables/useDeviceOwnedConversation', () => ({ useDeviceOwnedConversation: () => mocks }));
import { readWorkbenchTasks, registerWorkbenchBudgetEvidence, WorkbenchTaskReadError } from '../components/workbench/workbench-task-api';
import WorkbenchTasksView from '../components/workbench/WorkbenchTasksView.vue';

const reference = 'meshconv1:device-a:source';
function response() {
  return { data: { protocolVersion: 1, source: 'durable-checkpoint', truncated: false, liveObservationTruncated: false, summary: { total: 1, active: 1, paused: 0, succeeded: 0, failed: 0, runtimeObserved: 0, checkpointOnly: 1, needsReconciliation: 0, codingWorkersRequiringReview: 1 }, conversationId: 'continuation/hash', tasks: [{ coordinationId: 'task-a', status: 'running', updatedAt: '2026-10-15T00:00:00Z', recoveryState: 'checkpoint-only', assignmentCount: 1, completedCount: 0, failedCount: 0, assignments: [{ id: 'worker', status: 'running', role: 'coding_worker', objective: 'review source', reviewRequired: true }] }], queryContext: { executionScope: { coreId: 'core-a', spaceId: 'core-a', roleId: 'role', resourceOwnerId: 'core-a', providerEpoch: 1, modeRevision: 1, permissionRevision: 1, coordinated: true }, conversationOrigin: { ownerId: 'device-a', id: 'source' }, conversationId: 'continuation/hash' } } };
}

describe('Owned workbench task API', () => {
  beforeEach(() => { vi.clearAllMocks(); mocks.coreId.value = 'core-a'; mocks.policy.value = { providerEpoch: 1, modeRevision: 1, permissionRevision: 1, coordinated: true, selectedRole: 'role' } as any; mocks.refresh.mockResolvedValue(true); mocks.get.mockResolvedValue(response()); });
  it('sends resource identity and preserves the UI reference after validating continuation', async () => {
    const result = await readWorkbenchTasks(reference, 'role', new AbortController().signal);
    expect(mocks.get.mock.calls[0][1].params).toEqual({ conversationId: 'source', conversationOwnerId: 'device-a', characterId: 'role', expectedCoreId: 'core-a' });
    expect(result.conversationId).toBe(reference);
    expect(result.tasks).toHaveLength(1);
  });
  it('accepts a real policy containing selectedRole without claiming its extra fields are scope drift', async () => {
    mocks.policy.value = { providerEpoch: 1, modeRevision: 1, permissionRevision: 1, coordinated: true, selectedRole: 'role', extra: 'new' } as any;
    await expect(readWorkbenchTasks(reference, 'role', new AbortController().signal)).resolves.toMatchObject({ conversationId: reference });
  });
  it('rejects role switches during an in-flight task read even if permission revisions do not change', async () => {
    mocks.get.mockImplementationOnce(async () => { (mocks.policy.value as any).selectedRole = 'other'; return response(); });
    const error = await readWorkbenchTasks(reference, 'role', new AbortController().signal).catch(e => e);
    expect(error).toBeInstanceOf(WorkbenchTaskReadError);
    expect(error.invalidatesSnapshot).toBe(true);
    expect(error.message).toContain('角色选择');
  });
  it('uses the same contract for checkpoint history', async () => {
    await readWorkbenchTasks(reference, 'role', new AbortController().signal, 'task-a');
    expect(mocks.get.mock.calls[0][0]).toBe('/api/workbench/tasks/checkpoints');
    expect(mocks.get.mock.calls[0][1].params.coordinationId).toBe('task-a');
  });
  it('rejects a different owner, role, Core and malformed reference', async () => {
    for (const mutate of [(r: any) => r.data.queryContext.conversationOrigin.ownerId = 'other', (r: any) => r.data.queryContext.executionScope.roleId = 'other', (r: any) => r.data.queryContext.executionScope.coreId = 'other']) {
      const value = response(); mutate(value); mocks.get.mockResolvedValueOnce(value);
      await expect(readWorkbenchTasks(reference, 'role', new AbortController().signal)).rejects.toThrow('归属');
    }
    await expect(readWorkbenchTasks('source', 'role', new AbortController().signal)).rejects.toThrow('会话');
    await expect(readWorkbenchTasks('meshconv1:%ZZ:source', 'role', new AbortController().signal)).rejects.toMatchObject({ invalidatesSnapshot: true });
  });
  it('discards cancelled and late Core responses and preserves server errors', async () => {
    mocks.get.mockImplementationOnce(async () => { mocks.coreId.value = 'core-b'; return response(); });
    await expect(readWorkbenchTasks(reference, 'role', new AbortController().signal)).rejects.toThrow('归属');
    const abort = new AbortController(); abort.abort();
    await expect(readWorkbenchTasks(reference, 'role', abort.signal)).rejects.toThrow('取消');
    mocks.coreId.value = 'core-a';
    mocks.get.mockRejectedValueOnce({ response: { data: { error: '目标设备离线' } } });
    await expect(readWorkbenchTasks(reference, 'role', new AbortController().signal)).rejects.toThrow('目标设备离线');
  });
  it('posts evidence with the existing Owned conversation authority and rejects revoked receipts', async () => {
    mocks.post.mockResolvedValue(response());
    const signal = new AbortController().signal;
    const registration = { kind: 'model' as const, recordId: 'record', evidenceRef: 'case:provider-1234' };
    await registerWorkbenchBudgetEvidence(reference, 'role', 'coord', 'parent', registration, signal);
    expect(mocks.post.mock.calls[0][0]).toBe('/api/workbench/tasks/budget-reconciliation/review');
    expect(mocks.post.mock.calls[0][1]).toEqual(registration);
    expect(mocks.post.mock.calls[0][2].params).toMatchObject({ conversationId: 'source', conversationOwnerId: 'device-a', coordinationId: 'coord', parentInteractionId: 'parent', expectedCoreId: 'core-a' });
    mocks.post.mockImplementationOnce(async () => { mocks.coreId.value = 'other'; return response(); });
    await expect(registerWorkbenchBudgetEvidence(reference, 'role', 'coord', 'parent', registration, signal)).rejects.toMatchObject({ invalidatesSnapshot: true });
  });
  it('renders the Owned task and shows a connection error instead of an empty task list', async () => {
    const harness = defineComponent({ setup: () => () => h(KeepAlive, () => h(WorkbenchTasksView, { conversationId: reference, characterId: 'role' })) });
    const wrapper = mount(harness);
    try {
      await flushPromises();
      expect(wrapper.findAll('article')).toHaveLength(1);
      expect(wrapper.text()).toContain('task-a');
      mocks.get.mockRejectedValueOnce({ response: { data: { error: '目标设备离线' } } });
      await wrapper.get('header button').trigger('click');
      await flushPromises();
      expect(wrapper.get('[role="alert"]').text()).toBe('目标设备离线');
      expect(wrapper.text()).toContain('最后成功验证的只读快照');
      expect(wrapper.findAll('article')).toHaveLength(1);
      mocks.get.mockResolvedValueOnce(response());
      await wrapper.get('header button').trigger('click');
      await flushPromises();
      expect(wrapper.text()).not.toContain('最后成功验证的只读快照');
      mocks.get.mockRejectedValueOnce({ response: { status: 403, data: { error: '授权失效' } } });
      await wrapper.get('header button').trigger('click');
      await flushPromises();
      expect(wrapper.findAll('article')).toHaveLength(0);
      expect(wrapper.text()).not.toContain('最后成功验证的只读快照');
      expect(wrapper.text()).not.toContain('当前对话尚无可读取');
    } finally { wrapper.unmount(); }
  });
});
