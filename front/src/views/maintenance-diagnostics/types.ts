// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
export interface DiagItem {
  name: string
  status: "ok" | "warn" | "error" | "unknown"
  message: string
  details?: string
  suggestion?: string
}

export interface DiagResult {
  timestamp: string
  overallStatus: "healthy" | "degraded" | "unhealthy"
  items: DiagItem[]
  summary: { ok: number; warn: number; error: number; unknown: number }
}

export interface StatusData {
  status: string
  issues: Array<{ type: string; msg: string }>
  lastCheck: string
}

export interface ExportRecord {
  file: string
  timestamp: string
}
