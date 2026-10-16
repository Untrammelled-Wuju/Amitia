import { beforeEach, describe, expect, it, vi } from 'vitest';
import { defineComponent, h, KeepAlive } from 'vue';
import { flushPromises, mount } from '@vue/test-utils';

const mocks = vi.hoisted(() => ({ get: vi.fn(), refresh: vi.fn(), coreId: { value: 'core-a' }, policy: { value: { providerEpoch: 1, modeRevision: 1, permissionRevision: 1, coordinated: true } } }));
vi.mock('@/composables/useApi', () => ({ apiClient: { get: mocks.get } }));
vi.mock('@/composables/useDeviceOwnedConversation', () => ({ useDeviceOwnedConversation: () => mocks }));
import { readWorkbenchTasks } from '../components/workbench/workbench-task-api';
import WorkbenchTasksView from '../components/workbench/WorkbenchTasksView.vue';

const reference = 'meshconv1:device-a:source';
function response() {
  return { data: { protocolVersion: 1, source: 'durable-checkpoint', truncated: false, liveObservationTruncated: false, summary: { total: 1, active: 1, paused: 0, succeeded: 0, failed: 0, runtimeObserved: 0, checkpointOnly: 1, needsReconciliation: 0, codingWorkersRequiringReview: 1 }, conversationId: 'continuation/hash', tasks: [{ coordinationId: 'task-a', status: 'running', updatedAt: '2026-10-15T00:00:00Z', recoveryState: 'checkpoint-only', assignmentCount: 1, completedCount: 0, failedCount: 0, assignments: [{ id: 'worker', status: 'running', role: 'coding_worker', objective: 'review source', reviewRequired: true }] }], queryContext: { executionScope: { coreId: 'core-a', spaceId: 'core-a', roleId: 'role', resourceOwnerId: 'core-a', providerEpoch: 1, modeRevision: 1, permissionRevision: 1, coordinated: true }, conversationOrigin: { ownerId: 'device-a', id: 'source' }, conversationId: 'continuation/hash' } } };
}

describe('Owned workbench task API', () => {
  beforeEach(() => { vi.clearAllMocks(); mocks.coreId.value = 'core-a'; mocks.refresh.mockResolvedValue(true); mocks.get.mockResolvedValue(response()); });
  it('sends resource identity and preserves the UI reference after validating continuation', async () => {
    const result = await readWorkbenchTasks(reference, 'role', new AbortController().signal);
    expect(mocks.get.mock.calls[0][1].params).toEqual({ conversationId: 'source', conversationOwnerId: 'device-a', characterId: 'role', expectedCoreId: 'core-a' });
    expect(result.conversationId).toBe(reference);
    expect(result.tasks).toHaveLength(1);
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
      expect(wrapper.text()).not.toContain('当前对话尚无可读取');
    } finally { wrapper.unmount(); }
  });
});
