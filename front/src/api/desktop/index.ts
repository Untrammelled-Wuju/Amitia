import { apiClient } from "@/composables/useApi";
import type {
  ResolvedContribution,
  ConflictRecord,
  DesktopSnapshot,
  DesktopContract,
  DesktopPermissionDef,
  ResourceOwner,
} from "./types";

export type * from "./types";

const DESKTOP_BASE = "/api/extensions/desktop";

export async function listDesktopContributions(extensionId: string): Promise<ResolvedContribution[]> {
  const res = await apiClient.get(`${DESKTOP_BASE}/contributions`, { params: { extensionId } });
  const data = res.data;
  if (Array.isArray(data)) return data;
  if (Array.isArray(data?.items)) return data.items;
  return [];
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
