export interface GoalEditThread {
  id: string;
  revision: number;
  title: string;
  goal?: string;
  nextAction?: string;
  status: string;
}

export interface GoalEditScope {
  coreId: string;
  roleId: string;
  resourceOwnerId: string;
  providerEpoch: number;
  modeRevision: number;
  permissionRevision: number;
  roleRevision?: number;
  targetPermissionRevision?: number;
  targetProviderEpoch?: number;
}

export interface GoalEditDocument {
  thread: GoalEditThread;
  ownerId: string;
  executionScope?: GoalEditScope;
  readOnly?: boolean;
  lease?: { state: string };
}

export interface GoalEditForm {
  title: string;
  goal: string;
  nextAction: string;
}

export function goalEditAuthorityKey(document: GoalEditDocument | undefined): string {
  const scope = document?.executionScope;
  const thread = document?.thread;
  if (!scope || !thread || !thread.id || !Number.isSafeInteger(thread.revision) || thread.revision < 1 ||
    !scope.coreId || !scope.roleId || !scope.resourceOwnerId || document.ownerId !== scope.resourceOwnerId) return '';
  return JSON.stringify([
    thread.id, thread.revision, thread.status, thread.title, thread.goal || '', thread.nextAction || '',
    document.ownerId, scope.coreId, scope.roleId, scope.resourceOwnerId, scope.providerEpoch,
    scope.modeRevision, scope.permissionRevision, scope.roleRevision, scope.targetPermissionRevision,
    scope.targetProviderEpoch,
  ]);
}

export function canEditGoalDocument(document: GoalEditDocument | undefined, currentCore: string,
  authorizedRoles: readonly { id: string; revision?: number }[], source: string): boolean {
  const key = goalEditAuthorityKey(document);
  if (!key || !document?.executionScope || document.readOnly || source !== 'current' ||
    document.thread.status === 'completed' || document.thread.status === 'cancelled' ||
    document.lease?.state === 'running' || document.lease?.state === 'unknown') return false;
  const scope = document.executionScope;
  return scope.coreId === currentCore && authorizedRoles.some(role => role.id === scope.roleId &&
    (role.revision === undefined || scope.roleRevision === undefined || role.revision === scope.roleRevision));
}

export function prepareGoalEdit(form: GoalEditForm, document: GoalEditDocument): GoalEditForm {
  const title = form.title.trim();
  const goal = form.goal.trim();
  const nextAction = form.nextAction.trim();
  const utf8Length = (value: string) => new TextEncoder().encode(value).length;
  if (!title || utf8Length(title) > 4096) throw new Error('事项标题不能为空或超过 4096 字节');
  if (utf8Length(goal) > 128 * 1024) throw new Error('目标内容超过 128 KiB');
  if (utf8Length(nextAction) > 8192) throw new Error('下一步内容超过 8192 字节');
  if (title === document.thread.title && goal === (document.thread.goal || '') &&
    nextAction === (document.thread.nextAction || '')) throw new Error('事项内容没有变化');
  return { title, goal, nextAction };
}
