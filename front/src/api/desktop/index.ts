import { apiClient } from "@/composables/useApi";
import type {
  ResolvedContribution,
  ConflictRecord,
  DesktopSnapshot,
  DesktopContract,
  DesktopPermissionDef,
  ResourceOwner,
  ExtensionUpdateMeta,
  UpdateOperationInfo,
  UpdateOperationStepInfo,
} from "./types";

export type * from "./types";

const DESKTOP_BASE = "/api/extensions/desktop";
const UPDATES_OPS_BASE = "/api/extensions/updates/operations";

export async function listDesktopContributions(extensionId: string): Promise<ResolvedContribution[]> {
  const res = await apiClient.get(`/api/extensions/${encodeURIComponent(extensionId)}/desktop`);
  return res.data;
}

export async function getDesktopContribution(contributionId: string): Promise<ResolvedContribution> {
  const res = await apiClient.get(`${DESKTOP_BASE}/contributions/${encodeURIComponent(contributionId)}`);
  return res.data;
}

export async function enableDesktopContribution(contributionId: string): Promise<{ contributionId: string; status: string }> {
  const res = await apiClient.post(`${DESKTOP_BASE}/contributions/${encodeURIComponent(contributionId)}/enable`);
  return res.data;
}

export async function disableDesktopContribution(contributionId: string): Promise<{ contributionId: string; status: string }> {
  const res = await apiClient.post(`${DESKTOP_BASE}/contributions/${encodeURIComponent(contributionId)}/disable`);
  return res.data;
}

export async function rebindShortcut(contributionId: string, accelerator: string): Promise<{ contributionId: string; accelerator: string }> {
  const res = await apiClient.post(`${DESKTOP_BASE}/shortcuts/${encodeURIComponent(contributionId)}/rebind`, { accelerator });
  return res.data;
}

export async function listConflicts(): Promise<ConflictRecord[]> {
  const res = await apiClient.get(`${DESKTOP_BASE}/conflicts`);
  return res.data;
}

export async function resolveConflict(conflictId: string, resolution: string): Promise<{ conflictId: string; resolved: boolean }> {
  const res = await apiClient.post(`${DESKTOP_BASE}/conflicts/${encodeURIComponent(conflictId)}/resolve`, { resolution });
  return res.data;
}

export async function getDesktopSnapshot(): Promise<DesktopSnapshot> {
  const res = await apiClient.get(`${DESKTOP_BASE}/snapshot`);
  return res.data;
}

export async function listContracts(): Promise<DesktopContract[]> {
  const res = await apiClient.get(`${DESKTOP_BASE}/contracts`);
  return res.data;
}

export async function listDesktopPermissions(): Promise<DesktopPermissionDef[]> {
  const res = await apiClient.get(`${DESKTOP_BASE}/permissions`);
  return res.data;
}

export async function listDesktopResources(): Promise<ResourceOwner[]> {
  const res = await apiClient.get(`${DESKTOP_BASE}/resources`);
  return res.data;
}

export async function checkUpdates(extensionId: string): Promise<{ available: boolean; update?: ExtensionUpdateMeta }> {
  const res = await apiClient.post(`/api/extensions/${encodeURIComponent(extensionId)}/updates/check`);
  return res.data;
}

export async function downloadUpdate(extensionId: string, version: string): Promise<UpdateOperationInfo> {
  const res = await apiClient.post(`/api/extensions/${encodeURIComponent(extensionId)}/updates/download`, { version });
  return res.data;
}

export async function installUpdate(extensionId: string, operationId: string): Promise<UpdateOperationInfo> {
  const res = await apiClient.post(`/api/extensions/${encodeURIComponent(extensionId)}/updates/install`, { operationId });
  return res.data;
}

export async function cancelUpdate(extensionId: string, operationId: string): Promise<UpdateOperationInfo> {
  const res = await apiClient.post(`/api/extensions/${encodeURIComponent(extensionId)}/updates/cancel`, { operationId });
  return res.data;
}

export async function retryUpdate(extensionId: string, operationId: string): Promise<UpdateOperationInfo> {
  const res = await apiClient.post(`/api/extensions/${encodeURIComponent(extensionId)}/updates/retry`, { operationId });
  return res.data;
}

export async function rollbackUpdate(extensionId: string, operationId: string): Promise<UpdateOperationInfo> {
  const res = await apiClient.post(`/api/extensions/${encodeURIComponent(extensionId)}/updates/rollback`, { operationId });
  return res.data;
}

export async function getUpdateOperation(operationId: string): Promise<UpdateOperationInfo> {
  const res = await apiClient.get(`${UPDATES_OPS_BASE}/${encodeURIComponent(operationId)}`);
  return res.data;
}

export async function getUpdateOperationSteps(operationId: string): Promise<UpdateOperationStepInfo[]> {
  const res = await apiClient.get(`${UPDATES_OPS_BASE}/${encodeURIComponent(operationId)}/steps`);
  return res.data;
}
