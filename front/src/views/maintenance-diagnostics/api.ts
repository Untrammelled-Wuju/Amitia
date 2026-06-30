// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { apiClient } from "@/composables/useApi"
import type { DiagResult, StatusData, ExportRecord } from "./types"

async function apiPost(url: string, data?: any) {
  const res = await apiClient.post(url, data || {})
  return res.data?.data ?? res.data
}

async function apiGet(url: string) {
  const res = await apiClient.get(url)
  return res.data?.data ?? res.data
}

export async function runDiagnoseApi(): Promise<DiagResult> {
  const data = await apiPost("/api/maintenance/diagnose")
  const checks = data?.diagnosis?.checks || []
  const passedCount = checks.filter((c: any) => c.pass).length
  return {
    overallStatus: data?.diagnosis?.passed ? "healthy" : passedCount > 0 ? "degraded" : "unhealthy",
    items: checks.map((c: any) => ({ name: c.name, status: c.pass ? "ok" as const : "error" as const, message: c.pass ? "正常" : (c.error || "异常") })),
    summary: { ok: passedCount, warn: 0, error: checks.length - passedCount, unknown: 0 },
    timestamp: new Date().toISOString()
  }
}

export async function exportDiagnosticApi(): Promise<ExportRecord> {
  const data = await apiPost("/api/maintenance/export-diagnostic")
  return { file: data.file, timestamp: new Date().toISOString() }
}

export async function fetchStatusApi(): Promise<StatusData> {
  return await apiGet("/api/maintenance/status")
}

export async function restartBridgeApi() {
  return await apiPost("/api/maintenance/restart-bridge", { confirmToken: "restart-bridge-confirm" })
}

export async function restartQQBridgeApi() {
  return await apiPost("/api/maintenance/restart-qq-bridge", { confirmToken: "restart-qq-bridge-confirm" })
}

export async function reloadConfigApi() {
  return await apiPost("/api/maintenance/reload-config", { confirmToken: "reload-config-confirm" })
}
