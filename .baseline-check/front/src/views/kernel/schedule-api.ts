import { apiClient } from "@/composables/useApi";
import type {
  ScheduleContributionDefinition,
  ScheduleDetail,
  ScheduleTriggerRecord,
  ScheduleRunRecord,
  ScheduleMisfireRecord,
  ScheduleCircuitRecord,
  ScheduleQuarantineRecord,
} from "./schedule-types";

const BASE = "/api/extensions/schedules";

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

export async function listSchedules(extensionId?: string): Promise<{ items: ScheduleDetail[]; total: number }> {
  const qs = buildQuery({ extensionId });
  const res = await apiClient.get(`${BASE}${qs}`);
  return res.data;
}

export async function getSchedule(scheduleId: string): Promise<ScheduleDetail> {
  const res = await apiClient.get(`${BASE}/${encodeURIComponent(scheduleId)}`);
  return res.data;
}

export async function installSchedule(def: ScheduleContributionDefinition): Promise<ScheduleDetail> {
  const res = await apiClient.post(BASE, def);
  return res.data;
}

export async function updateSchedule(scheduleId: string, def: ScheduleContributionDefinition): Promise<ScheduleDetail> {
  const res = await apiClient.put(`${BASE}/${encodeURIComponent(scheduleId)}`, def);
  return res.data;
}

export async function uninstallSchedule(scheduleId: string): Promise<{ scheduleId: string; status: string }> {
  const res = await apiClient.delete(`${BASE}/${encodeURIComponent(scheduleId)}`);
  return res.data;
}

export async function enableSchedule(scheduleId: string): Promise<{ scheduleId: string; status: string }> {
  const res = await apiClient.post(`${BASE}/${encodeURIComponent(scheduleId)}/enable`);
  return res.data;
}

export async function disableSchedule(scheduleId: string): Promise<{ scheduleId: string; status: string }> {
  const res = await apiClient.post(`${BASE}/${encodeURIComponent(scheduleId)}/disable`);
  return res.data;
}

export async function pauseSchedule(scheduleId: string): Promise<{ scheduleId: string; status: string }> {
  const res = await apiClient.post(`${BASE}/${encodeURIComponent(scheduleId)}/pause`);
  return res.data;
}

export async function resumeSchedule(scheduleId: string): Promise<{ scheduleId: string; status: string }> {
  const res = await apiClient.post(`${BASE}/${encodeURIComponent(scheduleId)}/resume`);
  return res.data;
}

export async function runScheduleNow(scheduleId: string): Promise<{ triggerId: string; message: string }> {
  const res = await apiClient.post(`${BASE}/${encodeURIComponent(scheduleId)}/run-now`);
  return res.data;
}

export async function skipNextRun(scheduleId: string): Promise<{ scheduleId: string; status: string }> {
  const res = await apiClient.post(`${BASE}/${encodeURIComponent(scheduleId)}/skip-next`);
  return res.data;
}

export async function recalculateSchedule(scheduleId: string): Promise<{ scheduleId: string; status: string }> {
  const res = await apiClient.post(`${BASE}/${encodeURIComponent(scheduleId)}/recalculate`);
  return res.data;
}

export async function getTriggers(scheduleId: string, limit?: number): Promise<{ items: ScheduleTriggerRecord[]; total: number }> {
  const qs = buildQuery({ limit });
  const res = await apiClient.get(`${BASE}/${encodeURIComponent(scheduleId)}/triggers${qs}`);
  return res.data;
}

export async function getRuns(scheduleId: string, limit?: number): Promise<{ items: ScheduleRunRecord[]; total: number }> {
  const qs = buildQuery({ limit });
  const res = await apiClient.get(`${BASE}/${encodeURIComponent(scheduleId)}/runs${qs}`);
  return res.data;
}

export async function getMisfires(scheduleId: string, limit?: number): Promise<{ items: ScheduleMisfireRecord[]; total: number }> {
  const qs = buildQuery({ limit });
  const res = await apiClient.get(`${BASE}/${encodeURIComponent(scheduleId)}/misfires${qs}`);
  return res.data;
}

export async function getCircuit(scheduleId: string): Promise<ScheduleCircuitRecord> {
  const res = await apiClient.get(`${BASE}/${encodeURIComponent(scheduleId)}/circuit`);
  return res.data;
}

export async function resetCircuit(scheduleId: string): Promise<{ message: string }> {
  const res = await apiClient.post(`${BASE}/${encodeURIComponent(scheduleId)}/circuit/reset`);
  return res.data;
}

export async function getQuarantines(): Promise<{ items: ScheduleQuarantineRecord[]; total: number }> {
  const res = await apiClient.get(`${BASE}/quarantines`);
  return res.data;
}
