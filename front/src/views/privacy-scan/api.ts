// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { apiClient } from "@/composables/useApi";

function unwrap(response: any): any {
  return response?.data?.data ?? response?.data ?? response ?? {};
}

function buildFacets(items: any[]) {
  const riskCounts = new Map<string, number>();
  const sourceCounts = new Map<string, number>();
  for (const item of items) {
    const riskType = String(item?.risk_type ?? item?.pattern ?? "").trim();
    const sourceTable = String(item?.source_table ?? item?.sourceTable ?? "").trim();
    if (riskType) riskCounts.set(riskType, (riskCounts.get(riskType) ?? 0) + 1);
    if (sourceTable) sourceCounts.set(sourceTable, (sourceCounts.get(sourceTable) ?? 0) + 1);
  }
  return {
    riskTypes: Array.from(riskCounts, ([risk_type, cnt]) => ({ risk_type, cnt })),
    sourceTables: Array.from(sourceCounts, ([source_table, cnt]) => ({ source_table, cnt })),
  };
}

export async function postScan(scope: string[]) {
  const res = await apiClient.post("/api/privacy/scan", { scope });
  return unwrap(res);
}

export async function getScanResult(scanId: number | string) {
  const res = await apiClient.get(
    `/api/privacy/scan-results/${encodeURIComponent(String(scanId))}`,
  );
  const data = unwrap(res);
  return data?.result ?? null;
}

export async function getScanResults(params: any, scanId?: number | string) {
  let allItems: any[] = [];
  if (scanId != null && String(scanId).trim()) {
    const result = await getScanResult(scanId);
    allItems = Array.isArray(result?.findings) ? result.findings : [];
  } else {
    const res = await apiClient.get("/api/privacy/scan-results");
    const data = unwrap(res);
    allItems = Array.isArray(data?.items) ? data.items : [];
  }

  const facets = buildFacets(allItems);
  let filtered = allItems;
  if (params?.riskLevel) {
    filtered = filtered.filter((item) => {
      const level = String(item?.risk_level ?? item?.severity ?? "");
      return params.riskLevel === "high"
        ? level === "high" || level === "critical"
        : level === params.riskLevel;
    });
  }
  if (params?.riskType) {
    filtered = filtered.filter(
      (item) => String(item?.risk_type ?? item?.pattern ?? "") === params.riskType,
    );
  }
  if (params?.sourceTable) {
    filtered = filtered.filter(
      (item) => String(item?.source_table ?? item?.sourceTable ?? "") === params.sourceTable,
    );
  }

  const page = Math.max(1, Number(params?.page ?? 1));
  const pageSize = Math.max(1, Number(params?.pageSize ?? 50));
  const start = (page - 1) * pageSize;
  return {
    items: filtered.slice(start, start + pageSize),
    total: filtered.length,
    ...facets,
  };
}

export interface PrivacyMaskItem {
  id: string | number;
  sourceTable: string;
}

export async function postMask(items: PrivacyMaskItem[], confirmToken: string) {
  const res = await apiClient.post("/api/privacy/mask", {
    items: items.map((item) => ({
      id: item.id,
      sourceTable: item.sourceTable,
    })),
    confirmToken,
  });
  return unwrap(res);
}

export async function getScanHistory(params: { page: number; pageSize: number }) {
  const res = await apiClient.get("/api/privacy/scan-results");
  const data = unwrap(res);
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
  let items: any[] = [];
  if (params.scanId != null && String(params.scanId).trim()) {
    const result = await getScanResult(params.scanId);
    items = Array.isArray(result?.findings) ? result.findings : [];
  } else {
    const res = await apiClient.get("/api/privacy/scan-results");
    const data = unwrap(res);
    items = Array.isArray(data.items) ? data.items : [];
  }

  let content: string;
  let type: string;
  if (params.format === "csv") {
    const quote = (value: unknown) =>
      `"${String(value ?? "").replaceAll('"', '""')}"`;
    const rows = [
      ["id", "risk_level", "risk_type", "source_table", "snippet", "masked"],
      ...items.map((item: any) => [
        item.id,
        item.risk_level,
        item.risk_type,
        item.source_table,
        item.snippet,
        item.masked,
      ]),
    ];
    content = rows.map((row) => row.map(quote).join(",")).join("\n");
    type = "text/csv;charset=utf-8";
  } else {
    content = JSON.stringify(
      { exportedAt: new Date().toISOString(), scanId: params.scanId, items },
      null,
      2,
    );
    type = "application/json;charset=utf-8";
  }
  return new Blob([content], { type });
}

export async function getDeletionStats() {
  const res = await apiClient.get("/api/privacy/deletion/stats");
  return unwrap(res);
}

export async function requestDeletion(payload: {
  targetId: string;
  targetType: string;
  scope: string;
  reason?: string;
}) {
  const res = await apiClient.post("/api/privacy/deletion/request", payload);
  return unwrap(res);
}

export async function getDeletionStatus(id: string) {
  const res = await apiClient.get(
    `/api/privacy/deletion/status/${encodeURIComponent(id)}`,
  );
  return unwrap(res);
}

export async function runDeletionCleanup() {
  const res = await apiClient.post("/api/privacy/deletion/cleanup");
  return unwrap(res);
}

export async function runDeletionSecurityTests(payload: {
  targetId: string;
  targetType: string;
}) {
  const res = await apiClient.post(
    "/api/privacy/deletion/security-tests",
    payload,
  );
  const data = unwrap(res);
  return Array.isArray(data) ? data : [];
}
