import { apiClient } from "@/composables/useApi";

const BASE = "/api/extensions/dev-mode";

export interface DevWorkspace {
  workspaceId: string;
  extensionId: string;
  path: string;
  manifestPath: string;
  status: string;
  watchEnabled: boolean;
  autoReload: boolean;
  devTrust: boolean;
  currentRevision: string;
  createdAt: string;
  updatedAt: string;
  lastReloadAt: string;
  failureCount: number;
  lastError: string;
}

export interface DevWorkspaceDetail extends Omit<DevWorkspace, 'currentRevision'> {
  currentRevision?: DevRevision;
}

export interface DevRevision {
  revisionId: string;
  workspaceId: string;
  manifestHash: string;
  sourceHash: string;
  builtAt: string;
  buildDurationMs: number;
  artifactPath: string;
  errors: DevBuildError[];
  warnings: string[];
  status: string;
  errorCount: number;
}

export interface DevBuildError {
  file: string;
  line: number;
  column: number;
  message: string;
  code: string;
}

export interface DevReloadEvent {
  workspaceId: string;
  revisionId: string;
  stage: string;
  reason: string;
  startedAt: string;
  completedAt: string;
  success: boolean;
  error: string;
}

export interface DevSession {
  sessionId: string;
  workspaceId: string;
  deviceId: string;
  userAgent: string;
  startedAt: string;
  expiresAt: string;
  revoked: boolean;
  scopes: string[];
}

export interface RegisterWorkspaceBody {
  extensionId: string;
  path: string;
  manifestPath: string;
  watchEnabled: boolean;
  autoReload: boolean;
}

export interface BuildBody {
  sourceMap?: boolean;
  outDir?: string;
}

export interface OpenSessionBody {
  deviceId: string;
  userAgent: string;
  scopes?: string[];
}

export async function listWorkspaces(): Promise<{ workspaces: DevWorkspace[]; total: number }> {
  const res = await apiClient.get(`${BASE}/workspaces`);
  return res.data;
}

export async function getWorkspace(id: string): Promise<DevWorkspaceDetail> {
  const res = await apiClient.get(`${BASE}/workspaces/${encodeURIComponent(id)}`);
  return res.data;
}

export async function registerWorkspace(body: RegisterWorkspaceBody): Promise<DevWorkspace> {
  const res = await apiClient.post(`${BASE}/workspaces`, body);
  return res.data;
}

export async function removeWorkspace(id: string): Promise<{ workspaceId: string; removed: boolean }> {
  const res = await apiClient.delete(`${BASE}/workspaces/${encodeURIComponent(id)}`);
  return res.data;
}

export async function grantTrust(id: string): Promise<{ workspaceId: string; devTrust: boolean; workspace: DevWorkspace }> {
  const res = await apiClient.post(`${BASE}/workspaces/${encodeURIComponent(id)}/trust`);
  return res.data;
}

export async function revokeTrust(id: string): Promise<{ workspaceId: string; devTrust: boolean; workspace: DevWorkspace }> {
  const res = await apiClient.delete(`${BASE}/workspaces/${encodeURIComponent(id)}/trust`);
  return res.data;
}

export async function buildWorkspace(id: string, body: BuildBody): Promise<{ revision: DevRevision }> {
  const res = await apiClient.post(`${BASE}/workspaces/${encodeURIComponent(id)}/build`, body);
  return res.data;
}

export async function reloadWorkspace(id: string, reason: string): Promise<{ event: DevReloadEvent }> {
  const res = await apiClient.post(`${BASE}/workspaces/${encodeURIComponent(id)}/reload`, { reason });
  return res.data;
}

export async function startWatch(id: string): Promise<{ workspaceId: string; watching: boolean }> {
  const res = await apiClient.post(`${BASE}/workspaces/${encodeURIComponent(id)}/watch/start`);
  return res.data;
}

export async function stopWatch(id: string): Promise<{ workspaceId: string; watching: boolean }> {
  const res = await apiClient.post(`${BASE}/workspaces/${encodeURIComponent(id)}/watch/stop`);
  return res.data;
}

export async function openSession(id: string, body: OpenSessionBody): Promise<DevSession> {
  const res = await apiClient.post(`${BASE}/workspaces/${encodeURIComponent(id)}/sessions`, body);
  return res.data;
}

export async function closeSession(id: string): Promise<{ workspaceId: string; sessionId: string; revoked: boolean }> {
  const res = await apiClient.delete(`${BASE}/workspaces/${encodeURIComponent(id)}/sessions`);
  return res.data;
}

export async function listRevisions(id: string): Promise<{ revisions: DevRevision[]; total: number }> {
  const res = await apiClient.get(`${BASE}/workspaces/${encodeURIComponent(id)}/revisions`);
  return res.data;
}
