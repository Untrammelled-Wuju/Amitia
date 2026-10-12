import type { OwnedExecutionScope } from "./device-owned-chat";

export const GLOBAL_MEMORY_ROLE = "@global-memory";
export function memoryRecordScope(scope: OwnedExecutionScope, partition: string, kind = "memory"): OwnedExecutionScope {
  if (!["memory", "fact", "vector", "graph", "episodic", "continuity", "profile"].includes(kind)) return { ...scope };
  const result = { ...scope, memoryRoleId: partition };
  if (kind === "continuity") result.continuityRoleId = partition;
  delete result.memoryViewAll;
  return result;
}
export function memoryStorageRole(scope: OwnedExecutionScope, kind: string) {
  if (kind === "continuity" && scope.continuityRoleId) return scope.continuityRoleId;
  if (!scope.memoryRoleId) return scope.roleId;
  if (kind === "profile") return GLOBAL_MEMORY_ROLE;
  return ["memory", "fact", "vector", "graph", "episodic", "continuity"].includes(kind) ? scope.memoryRoleId : scope.roleId;
}
export const memoryViewForScope = (scope: OwnedExecutionScope) => scope.memoryViewAll ? "all" : scope.memoryRoleId === GLOBAL_MEMORY_ROLE ? "global" : scope.memoryRoleId ? `role:${scope.memoryRoleId}` : "";
export const continuityViewForScope = (scope: OwnedExecutionScope) => scope.continuityRoleId === GLOBAL_MEMORY_ROLE ? "global" : scope.continuityRoleId ? `role:${scope.continuityRoleId}` : memoryViewForScope(scope);
