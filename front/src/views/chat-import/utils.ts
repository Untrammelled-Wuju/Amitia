// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
export function parseSpeakerNames(input: string): string[] {
  if (!input.trim()) return []
  return input.split(/[,;，；]/).map(s => s.trim()).filter(Boolean)
}

export function warningType(w: any): "error" | "warning" | "info" {
  const type = typeof w === "string" ? "" : (w.type || "")
  if (type === "sensitive_data" || type === "empty_content") return "error"
  if (type === "low_confidence") return "warning"
  if (type === "unknown_speaker") return "info"
  return "warning"
}

export function confidenceColor(conf: number): string {
  if (conf >= 0.8) return "#67c23a"
  if (conf >= 0.5) return "#e6a23c"
  return "#f56c6c"
}

export function fmtDate(d: string): string {
  if (!d) return ""
  try { return new Date(d).toLocaleString("zh-CN") } catch { return d }
}
