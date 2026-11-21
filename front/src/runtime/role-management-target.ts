import { ref } from "vue";

export type RoleManagementTarget = "local" | "center";

export const ROLE_MANAGEMENT_TARGET_HEADER = "X-Amitia-Role-Target";

export const roleManagementTarget = ref<RoleManagementTarget | null>(null);

export const mcpManagementTarget = ref<RoleManagementTarget | null>(null);

export const skillManagementTarget = ref<RoleManagementTarget | null>(null);

export const taskManagementTarget = ref<RoleManagementTarget | null>(null);

export function roleTargetLabel(target: RoleManagementTarget): string {
  return target === "center" ? "统筹中心" : "本机";
}

export function withRoleTarget(
  headers: Record<string, string> | undefined,
  target: RoleManagementTarget | null,
): Record<string, string> | undefined {
  if (!target) return headers;
  return { ...(headers || {}), [ROLE_MANAGEMENT_TARGET_HEADER]: target };
}
