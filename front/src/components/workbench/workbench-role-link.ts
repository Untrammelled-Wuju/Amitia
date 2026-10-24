export interface WorkbenchRoleIdentifier { id: string }
export interface WorkbenchCheckpointIdentity { coordinationId: string; updatedAt: string }

export function authorizedWorkbenchRole(roleId: unknown, roles: readonly WorkbenchRoleIdentifier[], coreId: unknown): boolean {
  return typeof roleId === 'string' && /^[A-Za-z0-9_.:-]{1,160}$/.test(roleId) &&
    typeof coreId === 'string' && coreId.length > 0 &&
    roles.some(role => role.id === roleId);
}

export function unchangedWorkbenchCheckpoint(
  beforeAuthority: string,
  afterAuthority: string,
  beforeRoleId: string,
  afterRoleId: string,
  coordinationId: string,
  updatedAt: string,
  stale: boolean,
  tasks: readonly WorkbenchCheckpointIdentity[],
): boolean {
  return !stale && !!beforeAuthority && beforeAuthority === afterAuthority &&
    !!beforeRoleId && beforeRoleId === afterRoleId &&
    tasks.some(task => task.coordinationId === coordinationId && task.updatedAt === updatedAt);
}
