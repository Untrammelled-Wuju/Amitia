import type { OwnedChatEvent } from './device-owned-chat';

export const workbenchTaskEventName = 'amitia:workbench-task-changed';

export interface WorkbenchTaskChange {
  coreId: string;
  characterId: string;
  conversationId: string;
  coordinationId: string;
}

export function ownedWorkbenchTaskChange(event: OwnedChatEvent): WorkbenchTaskChange | null {
  const scope = event.executionScope;
  if (event.type !== 'task' || !scope?.coreId || !scope.roleId || !event.conversationId || !event.task?.coordinationId) return null;
  return { coreId: scope.coreId, characterId: scope.roleId, conversationId: event.conversationId, coordinationId: event.task.coordinationId };
}

export function matchesWorkbenchTaskChange(change: WorkbenchTaskChange | undefined, coreId: string, characterId: string, conversationId: string): boolean {
  return !!change && change.coreId === coreId && change.characterId === characterId && change.conversationId === conversationId;
}
