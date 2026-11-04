import { apiClient } from "@/composables/useApi";

const BASE = "/api/agent-platform";
const ROLE_BASE = "/api/roles";
const PLATFORM_SESSION_KEY = "amitia.agent-platform.session";

export interface PlatformSessionSnapshot {
  token: string;
  user: Record<string, any>;
  memberships: Record<string, any>[];
  expiresAt: string;
}

export function platformSession(): PlatformSessionSnapshot | null {
  if (typeof window === "undefined") return null;
  try {
    const value = window.localStorage.getItem(PLATFORM_SESSION_KEY);
    return value ? JSON.parse(value) : null;
  } catch {
    return null;
  }
}

function storePlatformSession(value: PlatformSessionSnapshot | null) {
  if (typeof window === "undefined") return;
  if (value) window.localStorage.setItem(PLATFORM_SESSION_KEY, JSON.stringify(value));
  else window.localStorage.removeItem(PLATFORM_SESSION_KEY);
}

apiClient.interceptors.request.use((config) => {
  const path = String(config.url || "");
  if (!path.startsWith(BASE) && !path.startsWith(ROLE_BASE)) return config;
  const session = platformSession();
  if (session?.token) {
    config.headers.set("X-Amitia-Platform-Session", session.token);
  }
  return config;
});

export async function loginPlatformUser(email: string, password: string): Promise<PlatformSessionSnapshot> {
  const response = await apiClient.post(`${BASE}/auth/login`, { email, password });
  const payload = response.data as Record<string, any>;
  const session = {
    token: String(payload.token || ""),
    user: payload.user || {},
    memberships: payload.membership || [],
    expiresAt: String(payload.expiresAt || ""),
  };
  if (!session.token) throw new Error("平台登录未返回会话令牌");
  storePlatformSession(session);
  return session;
}

export async function logoutPlatformUser(): Promise<void> {
  try {
    await apiClient.post(`${BASE}/auth/logout`, {});
  } finally {
    storePlatformSession(null);
  }
}

export interface AgentProfile {
  id: string;
  spaceId: string;
  characterId: string;
  name: string;
  description: string;
  archetype: string;
  status: string;
  ownerId: string;
  teamId: string;
  currentRevisionId: string;
  currentReleaseId: string;
  revision: number;
  createdAt: string;
  updatedAt: string;
}

export interface Team {
  id: string;
  name: string;
  description: string;
  leadAgentId: string;
  workspaceId: string;
  workspaceRootUri: string;
  sharedMemory: boolean;
  sharedWorkspace: boolean;
  status: string;
  revision: number;
}

export interface ActivityEvent {
  id: string;
  actorId: string;
  actorType: string;
  agentId: string;
  teamId: string;
  kind: string;
  state: string;
  title: string;
  detail: string;
  waitingReason: string;
  costMicroUsd: number;
  durationMs: number;
  occurredAt: string;
}

function data<T>(response: { data: T }): T {
  return response.data;
}

function collection(value: unknown, field: string): unknown[] {
  if (value == null) return [];
  if (!Array.isArray(value)) {
    throw new Error(`Agent Platform 字段 ${field} 需要数组，实际收到 ${JSON.stringify(value)}`);
  }
  return value;
}

function collectionData<T>(response: { data: { data: unknown } }, field: string): T[] {
  return collection(data(response), field) as T[];
}

export async function listAgents(): Promise<AgentProfile[]> {
  return collectionData<AgentProfile>(await apiClient.get(ROLE_BASE, { params: { includeDisabled: true } }), "agents");
}

export async function listTeams(): Promise<Team[]> {
  return collectionData<Team>(await apiClient.get(`${ROLE_BASE}/teams`), "teams");
}

export async function listTeamTemplates(): Promise<Record<string, any>[]> {
  return collectionData<Record<string, any>>(await apiClient.get(`${ROLE_BASE}/teams/templates`), "teamTemplates");
}

export async function createTeamFromTemplate(payload: Record<string, unknown>): Promise<Team> {
  return data(await apiClient.post(`${ROLE_BASE}/teams/from-template`, payload));
}

export async function routeTeamMessage(teamId: string, payload: Record<string, unknown>): Promise<Record<string, any>> {
  return data(await apiClient.post(`${ROLE_BASE}/teams/${encodeURIComponent(teamId)}/route`, payload));
}

export async function createTeam(payload: Record<string, unknown>): Promise<Team> {
  return data(await apiClient.post(`${ROLE_BASE}/teams`, payload));
}

export async function getTeam(id: string): Promise<Record<string, unknown>> {
  return data(await apiClient.get(`${ROLE_BASE}/teams/${encodeURIComponent(id)}`));
}

export async function addTeamMember(teamId: string, payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${ROLE_BASE}/teams/${encodeURIComponent(teamId)}/members`, payload));
}

export async function createTeamContext(teamId: string, payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${ROLE_BASE}/teams/${encodeURIComponent(teamId)}/contexts`, payload));
}

export async function listEvaluationSuites(agentId = ""): Promise<Record<string, unknown>[]> {
  return collectionData<Record<string, unknown>>(await apiClient.get(`${ROLE_BASE}/evaluations/suites`, { params: { agentId } }), "evaluationSuites");
}

export async function createEvaluationSuite(payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${ROLE_BASE}/evaluations/suites`, payload));
}

export async function createEvaluationCase(suiteId: string, payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${ROLE_BASE}/evaluations/suites/${encodeURIComponent(suiteId)}/cases`, payload));
}

export async function runEvaluation(suiteId: string, payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${ROLE_BASE}/evaluations/suites/${encodeURIComponent(suiteId)}/run`, payload));
}

export async function listA2ACards(): Promise<Record<string, any>[]> {
  return collectionData<Record<string, any>>(await apiClient.get(`${ROLE_BASE}/a2a/cards`), "a2aCards");
}

export async function registerA2ACard(payload: Record<string, unknown>): Promise<Record<string, any>> {
  return data(await apiClient.post(`${ROLE_BASE}/a2a/cards`, payload));
}

export async function listA2ATasks(agentId = ""): Promise<Record<string, any>[]> {
  return collectionData<Record<string, any>>(await apiClient.get(`${ROLE_BASE}/a2a/tasks`, { params: { agentId } }), "a2aTasks");
}

export async function submitA2ATask(payload: Record<string, unknown>): Promise<Record<string, any>> {
  return data(await apiClient.post(`${ROLE_BASE}/a2a/tasks`, payload));
}
