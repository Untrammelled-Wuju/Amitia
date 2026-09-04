import { apiClient } from "@/composables/useApi";

const BASE = "/api/extensions/hooks";

export interface HookPointSummary {
  id: string;
  contractVersion: number;
  description: string;
  riskLevel: string;
  thirdPartyAllowed: boolean;
  supportedPhases: string[];
  maxHandlers: number;
  defaultTimeout: number;
  maxTimeout: number;
}

export interface HookFailurePolicy {
  onRuntimeError: string;
  onTimeout: string;
  onInvalidResult: string;
  onPermissionDenied: string;
  disableAfterConsecutiveFailures: number;
}

export interface PermissionRequirement {
  permissionID: string;
  reason: string;
  required: boolean;
}

export interface RuntimeBinding {
  runtimeType: string;
  moduleID: string;
  entry: string;
}

export interface ScopeRule {
  scopeType: string;
  scopeID: string;
  reason: string;
}

export interface HookContributionSummary {
  contributionId: string;
  extensionId: string;
  hookPointId: string;
  phase: string;
  priority: number;
  enabled: boolean;
  circuitState: string;
  effectiveState: string;
  before: string[];
  after: string[];
  timeout: number;
  failurePolicy: HookFailurePolicy | null;
  mutationClaims: string[];
  permissionRequirements: PermissionRequirement[];
  scopeRule: ScopeRule;
  runtimeBinding: RuntimeBinding;
  definitionHash: string;
}

export interface HookCircuitSummary {
  contributionId: string;
  state: string;
  consecutiveFails: number;
  totalFails: number;
  totalSuccess: number;
  lastFailCode: string;
  openedAt: string;
}

export async function listHookPoints(): Promise<{ points: HookPointSummary[]; total: number }> {
  const res = await apiClient.get(`${BASE}/points`);
  return res.data;
}

export async function listContributions(
  extensionId?: string,
): Promise<{ contributions: HookContributionSummary[]; total: number }> {
  const res = await apiClient.get(`${BASE}/contributions`, {
    params: extensionId ? { extensionId } : {},
  });
  return res.data;
}

export async function listContributionsByPoint(
  pointId: string,
): Promise<{ contributions: HookContributionSummary[]; total: number }> {
  const res = await apiClient.get(`${BASE}/points/${encodeURIComponent(pointId)}/contributions`);
  return res.data;
}

export async function getContribution(id: string): Promise<HookContributionSummary> {
  const res = await apiClient.get(`${BASE}/contributions/${encodeURIComponent(id)}`);
  return res.data;
}

export async function enableContribution(id: string): Promise<{ contributionId: string; enabled: boolean }> {
  const res = await apiClient.post(`${BASE}/contributions/${encodeURIComponent(id)}/enable`);
  return res.data;
}

export async function disableContribution(id: string): Promise<{ contributionId: string; enabled: boolean }> {
  const res = await apiClient.post(`${BASE}/contributions/${encodeURIComponent(id)}/disable`);
  return res.data;
}

export async function getCircuitStats(id: string): Promise<HookCircuitSummary> {
  const res = await apiClient.get(`${BASE}/contributions/${encodeURIComponent(id)}/circuit`);
  return res.data;
}

export async function resetCircuit(id: string): Promise<{ contributionId: string; reset: boolean }> {
  const res = await apiClient.post(`${BASE}/contributions/${encodeURIComponent(id)}/circuit/reset`);
  return res.data;
}
