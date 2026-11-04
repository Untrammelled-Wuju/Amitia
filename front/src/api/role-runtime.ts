import { apiClient } from "@/composables/useApi";
import type { AgentProfile, Team } from "@/api/role-types";

const BASE = "/api/roles";

function unwrap<T>(response: { data?: any }): T {
  return response.data?.data ?? response.data as T;
}

function collection<T>(response: { data?: any }, key: string): T[] {
  const value = unwrap<any>(response);
  if (Array.isArray(value)) return value;
  const rows = value?.[key];
  if (rows == null) return [];
  if (!Array.isArray(rows)) throw new Error(`角色运行接口字段 ${key} 不是数组: ${JSON.stringify(rows)}`);
  return rows;
}

export async function listRoleRuntimeProfiles(): Promise<AgentProfile[]> {
  return collection<AgentProfile>(await apiClient.get(`${BASE}`, { params: { includeDisabled: true } }), "roles");
}

export async function listRoleTeams(): Promise<Team[]> {
  return collection<Team>(await apiClient.get(`${BASE}/teams`), "teams");
}

export async function createRoleTeam(payload: Record<string, unknown>): Promise<Team> {
  return unwrap(await apiClient.post(`${BASE}/teams`, payload));
}

export async function updateRoleTeam(id: string, payload: Record<string, unknown>): Promise<Team> {
  return unwrap(await apiClient.put(`${BASE}/teams/${encodeURIComponent(id)}`, payload));
}

export async function listRoleTeamTemplates(): Promise<Record<string, any>[]> {
  return collection<Record<string, any>>(await apiClient.get(`${BASE}/teams/templates`), "teamTemplates");
}

export async function createRoleTeamFromTemplate(payload: Record<string, unknown>): Promise<Team> {
  return unwrap(await apiClient.post(`${BASE}/teams/from-template`, payload));
}

export async function getRoleTeam(id: string): Promise<Record<string, any>> {
  return unwrap(await apiClient.get(`${BASE}/teams/${encodeURIComponent(id)}`));
}

export async function addRoleToTeam(id: string, payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return unwrap(await apiClient.post(`${BASE}/teams/${encodeURIComponent(id)}/members`, payload));
}

export async function createRoleTeamContext(id: string, payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return unwrap(await apiClient.post(`${BASE}/teams/${encodeURIComponent(id)}/contexts`, payload));
}

export async function routeRoleTeamMessage(id: string, payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return unwrap(await apiClient.post(`${BASE}/teams/${encodeURIComponent(id)}/route`, payload));
}
