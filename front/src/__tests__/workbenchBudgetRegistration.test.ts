import { beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';

const api = vi.hoisted(() => ({ load: vi.fn(), register: vi.fn(), success: vi.fn() }));
vi.mock('../components/workbench/workbench-task-reconciliation', () => ({ loadBudgetEvidence: api.load }));
vi.mock('../components/workbench/workbench-task-api', () => ({ registerWorkbenchBudgetEvidence: api.register, WorkbenchTaskReadError: class extends Error {} }));
vi.mock('element-plus', () => ({ ElMessage: { success: api.success } }));
import WorkbenchBudgetEvidencePanel from '../components/workbench/WorkbenchBudgetEvidencePanel.vue';

const props = { conversationId: 'meshconv1:owner:conversation', characterId: 'role', coordinationId: 'coord', parentInteractionId: 'parent', updatedAt: '2026-10-24T00:00:00Z', authority: 'current-owner', stale: false };
const entry = { id: 'model-record', status: 'interrupted', startedAt: props.updatedAt, reservedTokens: 128, reservedCostMicroUSD: 50, usageReported: false, budgetOverrun: false, toolName: '', reviewEvidenceRef: '', reviewActorId: '', reviewRecordedAt: '' };
const receipt = { recorded: true, effect: 'evidence-only', budgetReleased: false, executionConfirmed: false, coordinationId: 'coord', parentInteractionId: 'parent', kind: 'model', recordId: 'model-record', evidenceRef: 'case:12345678' };

describe('Owner budget evidence registration', () => {
  beforeEach(() => { vi.clearAllMocks(); api.load.mockResolvedValue({ records: [entry], canReview: true }); api.register.mockResolvedValue(receipt); });
  it('submits the selected ledger record and reloads after an exact immutable receipt', async () => {
    const wrapper = mount(WorkbenchBudgetEvidencePanel, { props });
    try {
      await wrapper.findAll('button')[0].trigger('click'); await flushPromises();
      await wrapper.findAll('button').find(button => button.text() === '登记核对证据')!.trigger('click');
      await wrapper.get('input').setValue('case:12345678');
      await wrapper.get('form').trigger('submit'); await flushPromises();
      expect(api.register).toHaveBeenCalledWith(props.conversationId, 'role', 'coord', 'parent', { kind: 'model', recordId: 'model-record', evidenceRef: 'case:12345678' }, expect.any(AbortSignal));
      expect(api.success).toHaveBeenCalledOnce();
      expect(api.load).toHaveBeenCalledTimes(2);
    } finally { wrapper.unmount(); }
  });
  it('does not present registration to a reader without owner capability', async () => {
    api.load.mockResolvedValue({ records: [entry], canReview: false });
    const wrapper = mount(WorkbenchBudgetEvidencePanel, { props });
    try {
      await wrapper.findAll('button')[0].trigger('click'); await flushPromises();
      expect(wrapper.text()).not.toContain('登记核对证据');
      expect(wrapper.find('form').exists()).toBe(false);
      expect(api.register).not.toHaveBeenCalled();
    } finally { wrapper.unmount(); }
  });
  it('discards a late registration result after authority changes', async () => {
    let complete!: (value: unknown) => void;
    api.register.mockReturnValue(new Promise(resolve => { complete = resolve; }));
    const wrapper = mount(WorkbenchBudgetEvidencePanel, { props });
    try {
      await wrapper.findAll('button')[0].trigger('click'); await flushPromises();
      await wrapper.findAll('button').find(button => button.text() === '登记核对证据')!.trigger('click');
      await wrapper.get('input').setValue('case:12345678'); await wrapper.get('form').trigger('submit');
      await wrapper.setProps({ authority: 'revoked-owner' });
      complete(receipt); await flushPromises();
      expect(api.success).not.toHaveBeenCalled();
      expect(wrapper.find('input').exists()).toBe(false);
    } finally { wrapper.unmount(); }
  });
});
