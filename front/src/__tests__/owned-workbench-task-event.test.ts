import { describe, expect, it } from 'vitest';
import { matchesWorkbenchTaskChange, ownedWorkbenchTaskChange } from '../runtime/owned-workbench-task-event';
import type { OwnedChatEvent } from '../runtime/device-owned-chat';

describe('Owned task refresh context', () => {
  it('refreshes only the same Core, role and owned conversation', () => {
    const event = { type: 'task', conversationId: 'meshconv1:owner:conversation', executionScope: { coreId: 'core', roleId: 'role' }, task: { coordinationId: 'task', status: 'running' } } as OwnedChatEvent;
    const change = ownedWorkbenchTaskChange(event)!;
    expect(matchesWorkbenchTaskChange(change, 'core', 'role', event.conversationId!)).toBe(true);
    expect(matchesWorkbenchTaskChange(change, 'other', 'role', event.conversationId!)).toBe(false);
    expect(matchesWorkbenchTaskChange(change, 'core', 'other', event.conversationId!)).toBe(false);
    expect(matchesWorkbenchTaskChange(change, 'core', 'role', 'meshconv1:other:conversation')).toBe(false);
    expect(ownedWorkbenchTaskChange({ ...event, type: 'delta' })).toBeNull();
  });
});
