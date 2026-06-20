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
