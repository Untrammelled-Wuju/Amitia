import { describe, expect, it } from 'vitest';
import { TaskCenterReadFence, type TaskCenterReadChannel } from '../views/kernel/tasks/task-center-read-fence';

describe('M08 TaskCenter owner read epoch', () => {
  it('rejects every in-flight channel after execution owner changes', () => {
    const guard = new TaskCenterReadFence();
    const channels: TaskCenterReadChannel[] = ['definitions', 'approvals', 'approval-mutation', 'devices', 'roles', 'catalog', 'submission'];
    const tickets = channels.map(channel => guard.begin(channel));
    guard.invalidate();
    for (let i = 0; i < channels.length; i++) {
      expect(guard.current(channels[i], tickets[i])).toBe(false);
      expect(guard.current(channels[i], guard.begin(channels[i]))).toBe(true);
    }
  });

  it('drops earlier responses within a channel and preserves unrelated requests', () => {
    const guard = new TaskCenterReadFence();
    const first = guard.begin('definitions');
    const approval = guard.begin('approvals');
    const second = guard.begin('definitions');
    expect(guard.current('definitions', first)).toBe(false);
    expect(guard.current('definitions', second)).toBe(true);
    expect(guard.current('approvals', approval)).toBe(true);
  });

  it('does not reuse tickets after permission changes, even at the same sequence', () => {
    const guard = new TaskCenterReadFence();
    const prior = guard.begin('submission');
    guard.invalidate();
    const next = guard.begin('submission');
    expect(prior.sequence).toBe(next.sequence);
    expect(guard.current('submission', prior)).toBe(false);
    expect(guard.current('submission', next)).toBe(true);
  });
});

describe('M08 re-opening device task dialog', () => {
  it('invalidates stale device options and old submissions on a fresh open', () => {
    const guard = new TaskCenterReadFence();
    const device = guard.begin('devices');
    const submission = guard.begin('submission');
    const approvals = guard.begin('approvals');
    guard.begin('submission');
    guard.begin('devices');
    expect(guard.current('devices', device)).toBe(false);
    expect(guard.current('submission', submission)).toBe(false);
    expect(guard.current('approvals', approvals)).toBe(true);
  });
});
