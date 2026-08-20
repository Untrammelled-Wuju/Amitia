// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { apiClient } from "@/composables/useApi";

export async function postScan(scope: string[]) {
  const res = await apiClient.post("/api/privacy/scan", { scope });
  return res.data?.data || res.data;
}

export async function getScanResults(params: any) {
  const res = await apiClient.get("/api/privacy/scan-results", { params });
  return res.data?.data || res.data;
}

export async function postMask(ids: number[], confirmToken: string) {
  const res = await apiClient.post("/api/privacy/mask", { ids, confirmToken });
  return res.data?.data || res.data;
}

export async function getScanHistory(params: { page: number; pageSize: number }) {
  const res = await apiClient.get("/api/privacy/scan-results");
  const data = res.data?.data || res.data || {};
  const history = Array.isArray(data.history) ? data.history : [];
  const start = Math.max(0, (params.page - 1) * params.pageSize);
  return {
    items: history.slice(start, start + params.pageSize),
    total: data.historyTotal ?? history.length,
  };
}

export async function getExportScanReport(params: {
  scanId?: number | string;
  format: "csv" | "json";
}) {
  const res = await apiClient.get("/api/privacy/scan-results");
  const data = res.data?.data || res.data || {};
  const items = Array.isArray(data.items) ? data.items : [];
  let content: string;
  let type: string;
  if (params.format === "csv") {
    const quote = (value: unknown) => `"${String(value ?? "").replaceAll('"', '""')}"`;
    const rows = [
      ["id", "risk_level", "risk_type", "source_table", "snippet", "masked"],
      ...items.map((item: any) => [item.id, item.risk_level, item.risk_type, item.source_table, item.snippet, item.masked]),
    ];
    content = rows.map((row) => row.map(quote).join(",")).join("\n");
    type = "text/csv;charset=utf-8";
  } else {
    content = JSON.stringify({ exportedAt: new Date().toISOString(), items }, null, 2);
    type = "application/json;charset=utf-8";
  }
  return new Blob([content], { type });
}

export async function getDeletionStats() {
  const res = await apiClient.get("/api/privacy/deletion/stats");
  return res.data?.data || res.data || {};
}

export async function requestDeletion(payload: {
  targetId: string;
  targetType: string;
  scope: string;
  reason?: string;
}) {
  const res = await apiClient.post("/api/privacy/deletion/request", payload);
  return res.data?.data || res.data || {};
}

export async function getDeletionStatus(id: string) {
  const res = await apiClient.get(`/api/privacy/deletion/status/${encodeURIComponent(id)}`);
  return res.data?.data || res.data || {};
}

export async function runDeletionCleanup() {
  const res = await apiClient.post("/api/privacy/deletion/cleanup");
  return res.data?.data || res.data || {};
}

export async function runDeletionSecurityTests(payload: { targetId: string; targetType: string }) {
  const res = await apiClient.post("/api/privacy/deletion/security-tests", payload);
  const data = res.data?.data || res.data;
  return Array.isArray(data) ? data : [];
}
