import { beforeEach, describe, expect, it, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useWorkbenchStore } from '@/stores/workbench';

describe('conversation workbench', () => {
  beforeEach(() => {
    localStorage.clear();
    setActivePinia(createPinia());
    vi.stubGlobal('crypto', { randomUUID: (() => { let index = 0; return () => `tab-${++index}`; })() });
  });

  it('isolates tab groups between conversations', () => {
    const workbench = useWorkbenchStore();
    workbench.setConversation('conversation-a');
    workbench.openTab({ kind: 'files' });
    expect(workbench.tabs).toHaveLength(1);
    workbench.setConversation('conversation-b');
    expect(workbench.tabs).toHaveLength(0);
    workbench.openTab({ kind: 'git' });
    workbench.setConversation('conversation-a');
    expect(workbench.tabs[0].kind).toBe('files');
  });

  it('deduplicates identical resources unless explicitly forced', () => {
    const workbench = useWorkbenchStore();
    workbench.setConversation('conversation-a');
    const request = { kind: 'file' as const, resourceUri: 'amitia://workspace/@a/readme.md' };
    const one = workbench.openTab(request);
    const two = workbench.openTab(request);
    expect(one.id).toBe(two.id);
    workbench.openTab({ ...request, forceNew: true });
    expect(workbench.tabs).toHaveLength(2);
  });

  it('does not steal focus for background tabs', () => {
    const workbench = useWorkbenchStore();
    const selected = workbench.openTab({ kind: 'files' });
    workbench.openTab({ kind: 'browser', resourceUri: 'https://example.org', focus: 'background' });
    expect(workbench.activeTab?.id).toBe(selected.id);
    expect(workbench.tabs).toHaveLength(2);
  });

  it('adopts unbound draft tabs when a conversation receives its stable id', () => {
    const workbench = useWorkbenchStore();
    workbench.openTab({ kind: 'browser', resourceUri: 'https://example.org' });
    workbench.setConversation('created-conversation');
    expect(workbench.tabs).toHaveLength(1);
    expect(workbench.activeTab?.kind).toBe('browser');
    workbench.setConversation('__draft__');
    expect(workbench.tabs).toHaveLength(0);
  });

  it('persists agent commands for their own conversation without stealing the current view', () => {
    const workbench = useWorkbenchStore();
    workbench.setConversation('conversation-a');
    const command = { protocolVersion: 1 as const, conversationId: 'conversation-a', kind: 'files' as const, operationId: 'open-1' };
    expect(workbench.openCommand(command)).not.toBeNull();
    expect(workbench.openCommand(command)).toBeNull();
    expect(workbench.openCommand({ ...command, operationId: 'open-2', conversationId: 'conversation-b' })).not.toBeNull();
    expect(workbench.tabs).toHaveLength(1);
    workbench.setConversation('conversation-b');
    expect(workbench.tabs).toHaveLength(1);
    expect(workbench.tabs[0].kind).toBe('files');
    expect(workbench.openCommand({ ...command, operationId: 'open-2', conversationId: 'conversation-b' })).toBeNull();
  });

  it('keeps operation deduplication across session persistence and reload', () => {
    const workbench = useWorkbenchStore();
    workbench.setConversation('persisted');
    const command = { protocolVersion: 1 as const, conversationId: 'persisted', kind: 'browser' as const, resourceUri: 'https://example.org', forceNew: true, operationId: 'event-1', focus: 'background' as const };
    expect(workbench.openCommand(command)).not.toBeNull();
    workbench.persistSessions();
    setActivePinia(createPinia());
    const reloaded = useWorkbenchStore();
    reloaded.setConversation('persisted');
    expect(reloaded.openCommand(command)).toBeNull();
    expect(reloaded.tabs).toHaveLength(1);
    expect(reloaded.activeTab?.kind).toBe('browser');
    expect(reloaded.visible).toBe(false);
  });

  it('allows pinning and reordering, and preserves pinned tabs on closeAll', () => {
    const workbench = useWorkbenchStore();
    const first = workbench.openTab({ kind: 'files', forceNew: true });
    const second = workbench.openTab({ kind: 'git', forceNew: true });
    workbench.setPinned(first.id, true);
    workbench.moveTab(second.id, -1);
    expect(workbench.tabs[0].id).toBe(second.id);
    workbench.closeTab(first.id);
    expect(workbench.tabs).toHaveLength(2);
    workbench.closeAll();
    expect(workbench.tabs).toHaveLength(1);
    expect(workbench.tabs[0].id).toBe(first.id);
    workbench.closeAll(true);
    expect(workbench.visible).toBe(false);
  });

  it('updates browser resource in place and persists on pagehide', () => {
    const workbench = useWorkbenchStore();
    const tab = workbench.openTab({ kind: 'browser' });
    workbench.updateTab(tab.id, { resourceUri: 'https://example.com', title: 'Example' });
    expect(workbench.tabs[0].resourceUri).toBe('https://example.com');
    expect(workbench.tabs[0].title).toBe('Example');
    workbench.persistSessions();
    expect(localStorage.getItem('amitia:workbench:layout:v1')).toContain('https://example.com');
  });

  it('closes selected tab and clamps widths', () => {
    const workbench = useWorkbenchStore();
    workbench.openTab({ kind: 'files' });
    workbench.openTab({ kind: 'git' });
    workbench.closeTab(workbench.activeTab!.id);
    expect(workbench.tabs).toHaveLength(1);
    workbench.setWidth(15);
    expect(workbench.width).toBe(350);
    workbench.closeAll();
    expect(workbench.visible).toBe(false);
  });
  it('separates browser sessions by agent observation capability', () => {
    const workbench = useWorkbenchStore();
    const one = workbench.openTab({ kind: 'browser', resourceUri: 'https://example.org', browserRef: 'A'.repeat(32) });
    const two = workbench.openTab({ kind: 'browser', resourceUri: 'https://example.org', browserRef: 'B'.repeat(32) });
    expect(one.id).not.toBe(two.id);
    workbench.persistSessions();
    expect(localStorage.getItem('amitia:workbench:layout:v1')).toContain('A'.repeat(32));
  });

});
