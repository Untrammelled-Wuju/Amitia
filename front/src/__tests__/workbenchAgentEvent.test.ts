import { describe, expect, it } from 'vitest';
import { workbenchCommandFromEvent, resolveWorkbenchBrowserTarget } from '@/runtime/workbench-agent-event';
import type { AgentUIEvent } from '@/conversation/runtime/agentEventReducer';

function event(payload: Record<string, unknown>, conversationId = 'c1'): AgentUIEvent {
  return { version: 1, eventSequence: 1, eventId: 'event-1', conversationId, type: 'workbench.open', payload };
}

describe('AgentUIEvent Workbench Bridge', () => {
  it('routes a background event to its source conversation', () => {
    const command = workbenchCommandFromEvent(event({ kind: 'files' }, 'c2'), 'c1');
    expect(command?.conversationId).toBe('c2');
    expect(command?.focus).toBe('background');
    expect(workbenchCommandFromEvent(event({ kind: 'files' }, '../other'), 'c1')).toBeNull();
  });

  it('rejects unsupported kinds and dangerous browser URLs', () => {
    expect(workbenchCommandFromEvent(event({ kind: 'unknown' }), 'c1')).toBeNull();
    expect(workbenchCommandFromEvent(event({ kind: 'browser', resourceUri: 'javascript:alert(1)' }), 'c1')).toBeNull();
    expect(workbenchCommandFromEvent(event({ kind: 'browser', resourceUri: 'file:///etc/passwd' }), 'c1')).toBeNull();
    expect(workbenchCommandFromEvent(event({ kind: 'browser', resourceUri: 'https://user:pass@example.com' }), 'c1')).toBeNull();
    expect(workbenchCommandFromEvent(event({ kind: 'diff', resourceUri: 'some/path' }), 'c1')).toBeNull();
  });

  it('returns background focus and stable operationId by default', () => {
    const command = workbenchCommandFromEvent(event({ kind: 'file', resourceUri: 'amitia://workspace/@a/a.ts', workspaceUri: 'amitia://workspace/@a' }), 'c1');
    expect(command?.focus).toBe('background');
    expect(command?.operationId).toBe('event-1');
    expect(command?.conversationId).toBe('c1');
  });

  it('accepts an explicit activate request', () => {
    expect(workbenchCommandFromEvent(event({ kind: 'browser', resourceUri: 'https://example.com', focus: 'activate' }), 'c1')?.focus).toBe('activate');
  });
  it('chooses cloud observation target only for an authenticated known origin', () => {
    const command = workbenchCommandFromEvent(event({ kind: 'browser', resourceUri: 'https://example.org', browserRef: 'A'.repeat(32), originDeviceId: 'device-a' }), 'c1')!;
    expect(resolveWorkbenchBrowserTarget(command, 'device-a', 'device-b', 'cloud')?.targetDeviceId).toBe('device-a');
    expect(resolveWorkbenchBrowserTarget(command, 'device-a', 'device-a', 'cloud')?.targetDeviceId).toBe('');
    expect(resolveWorkbenchBrowserTarget(command, 'device-a', 'device-b', 'local')?.targetDeviceId).toBe('');
    expect(resolveWorkbenchBrowserTarget(command, '../device', 'device-b', 'cloud')).toBeNull();
    expect(workbenchCommandFromEvent(event({ kind: 'browser', resourceUri: 'https://example.org', originDeviceId: '../bad' }), 'c1')).toBeNull();
  });
  it('preserves scoped read-only agent browser reference', () => {
    const ref = 'A'.repeat(32);
    const command = workbenchCommandFromEvent(event({ kind: 'browser', resourceUri: 'https://example.org', browserRef: ref, focus: 'background' }), 'c1');
    expect(command?.browserRef).toBe(ref);
    expect(command?.focus).toBe('background');
    expect(workbenchCommandFromEvent(event({ kind: 'browser', resourceUri: 'https://example.org', browserRef: 'bad-token' }), 'c1')).toBeNull();
  });

});
