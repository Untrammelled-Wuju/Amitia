import { apiClient } from "@/composables/useApi";
import type {
  MigrationDefinition,
  MigrationPlanInput,
  MigrationPlanOutput,
  MigrationOperation,
  RollbackPlan,
  RollbackStepRecord,
  LifecycleJournalEntry,
  RecoveryAction,
  CanaryPolicy,
  CanaryState,
  CanaryMetric,
  HealthEvaluation,
  GenerationRoute,
} from "./migration-types";

const MIGRATIONS_BASE = "/api/extensions/migrations";
const ROLLBACKS_BASE = "/api/extensions/rollbacks";
const RECOVERY_BASE = "/api/extensions/recovery";
const JOURNAL_BASE = "/api/extensions/journal";
const CANARY_BASE = "/api/extensions/canary";

function buildQuery(params: Record<string, unknown>): string {
  const sp = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") {
      sp.set(key, String(value));
    }
  }
  const qs = sp.toString();
  return qs ? `?${qs}` : "";
}

export async function listMigrations(extensionId?: string): Promise<{ items: MigrationDefinition[]; total: number }> {
  const qs = buildQuery({ extension_id: extensionId });
  const res = await apiClient.get(`${MIGRATIONS_BASE}${qs}`);
  return res.data;
}

export async function getMigration(migrationId: string): Promise<MigrationDefinition> {
  const res = await apiClient.get(`${MIGRATIONS_BASE}/${encodeURIComponent(migrationId)}`);
  return res.data;
}

export async function planMigration(input: MigrationPlanInput): Promise<MigrationPlanOutput> {
  const res = await apiClient.post(`${MIGRATIONS_BASE}/plan`, input);
  return res.data;
}

export async function executeMigration(input: MigrationPlanInput): Promise<MigrationOperation> {
  const res = await apiClient.post(`${MIGRATIONS_BASE}/execute`, input);
  return res.data;
}

export async function getMigrationOperation(operationId: string): Promise<MigrationOperation> {
  const res = await apiClient.get(`${MIGRATIONS_BASE}/operations/${encodeURIComponent(operationId)}`);
  return res.data;
}

export async function listRollbacks(extensionId?: string): Promise<{ items: RollbackPlan[]; total: number }> {
  const qs = buildQuery({ extension_id: extensionId });
  const res = await apiClient.get(`${ROLLBACKS_BASE}${qs}`);
  return res.data;
}

export async function getRollback(rollbackId: string): Promise<RollbackPlan> {
  const res = await apiClient.get(`${ROLLBACKS_BASE}/${encodeURIComponent(rollbackId)}`);
  return res.data;
}

export async function listRollbackSteps(rollbackId: string): Promise<{ items: RollbackStepRecord[]; total: number }> {
  const res = await apiClient.get(`${ROLLBACKS_BASE}/${encodeURIComponent(rollbackId)}/steps`);
  return res.data;
}

export async function executeRollback(rollbackId: string): Promise<RollbackPlan> {
  const res = await apiClient.post(`${ROLLBACKS_BASE}/${encodeURIComponent(rollbackId)}/execute`);
  return res.data;
}

export async function recoverRollback(rollbackId: string): Promise<RollbackPlan> {
  const res = await apiClient.post(`${ROLLBACKS_BASE}/${encodeURIComponent(rollbackId)}/recover`);
  return res.data;
}

export async function scanRecovery(): Promise<{ items: RecoveryAction[]; total: number }> {
  const res = await apiClient.get(`${RECOVERY_BASE}/scan`);
  return res.data;
}

export async function executeRecovery(action: RecoveryAction): Promise<{ operation_id: string; status: string }> {
  const res = await apiClient.post(`${RECOVERY_BASE}/execute`, action);
  return res.data;
}

export async function getJournalEntries(operationId: string): Promise<{ items: LifecycleJournalEntry[]; total: number }> {
  const qs = buildQuery({ operation_id: operationId });
  const res = await apiClient.get(`${JOURNAL_BASE}${qs}`);
  return res.data;
}

export async function listCanaryPolicies(extensionId?: string): Promise<{ items: CanaryPolicy[]; total: number }> {
  const qs = buildQuery({ extension_id: extensionId });
  const res = await apiClient.get(`${CANARY_BASE}/policies${qs}`);
  return res.data;
}

export async function getCanaryPolicy(policyId: string): Promise<CanaryPolicy> {
  const res = await apiClient.get(`${CANARY_BASE}/policies/${encodeURIComponent(policyId)}`);
  return res.data;
}

export async function createCanaryPolicy(policy: Partial<CanaryPolicy>): Promise<CanaryPolicy> {
  const res = await apiClient.post(`${CANARY_BASE}/policies`, policy);
  return res.data;
}

export async function listCanaryStates(extensionId?: string): Promise<{ items: CanaryState[]; total: number }> {
  const qs = buildQuery({ extension_id: extensionId });
  const res = await apiClient.get(`${CANARY_BASE}/states${qs}`);
  return res.data;
}

export async function getCanaryState(canaryId: string): Promise<CanaryState> {
  const res = await apiClient.get(`${CANARY_BASE}/states/${encodeURIComponent(canaryId)}`);
  return res.data;
}

export async function createCanaryState(state: Partial<CanaryState>, policy: Partial<CanaryPolicy>): Promise<CanaryState> {
  const res = await apiClient.post(`${CANARY_BASE}/states`, { state, policy });
  return res.data;
}

export async function advanceCanary(canaryId: string): Promise<CanaryState> {
  const res = await apiClient.post(`${CANARY_BASE}/states/${encodeURIComponent(canaryId)}/advance`);
  return res.data;
}

export async function pauseCanary(canaryId: string): Promise<CanaryState> {
  const res = await apiClient.post(`${CANARY_BASE}/states/${encodeURIComponent(canaryId)}/pause`);
  return res.data;
}

export async function resumeCanary(canaryId: string): Promise<CanaryState> {
  const res = await apiClient.post(`${CANARY_BASE}/states/${encodeURIComponent(canaryId)}/resume`);
  return res.data;
}

export async function abortCanary(canaryId: string, reason: string): Promise<CanaryState> {
  const res = await apiClient.post(`${CANARY_BASE}/states/${encodeURIComponent(canaryId)}/abort`, { reason });
  return res.data;
}

export async function commitCanary(canaryId: string): Promise<CanaryState> {
  const res = await apiClient.post(`${CANARY_BASE}/states/${encodeURIComponent(canaryId)}/commit`);
  return res.data;
}

export async function listCanaryMetrics(
  extensionId: string,
  generation?: number,
  startTime?: string,
  endTime?: string,
): Promise<{ items: CanaryMetric[]; total: number }> {
  const qs = buildQuery({ extension_id: extensionId, generation, start_time: startTime, end_time: endTime });
  const res = await apiClient.get(`${CANARY_BASE}/metrics${qs}`);
  return res.data;
}

export async function recordCanaryMetric(metric: Partial<CanaryMetric>): Promise<CanaryMetric> {
  const res = await apiClient.post(`${CANARY_BASE}/metrics`, metric);
  return res.data;
}

export async function getHealthEvaluation(
  extensionId: string,
  generation: number,
  baselineWindow?: string,
): Promise<HealthEvaluation> {
  const qs = buildQuery({ extension_id: extensionId, generation, baseline_window: baselineWindow });
  const res = await apiClient.get(`${CANARY_BASE}/health${qs}`);
  return res.data;
}

export async function getGenerationRoute(
  extensionId: string,
  cohortType: string,
  cohortId: string,
): Promise<GenerationRoute> {
  const qs = buildQuery({ extension_id: extensionId, cohort_type: cohortType, cohort_id: cohortId });
  const res = await apiClient.get(`${CANARY_BASE}/routes${qs}`);
  return res.data;
}
