// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only


function envStr(key: string, fallback: string): string {
  return process.env[key] || fallback
}

export const sidecarConfig = {
  mergeWindowMs: parseInt(process.env.MERGE_WINDOW_MS || "6000", 10),

  host: envStr("SIDECAR_HOST", "127.0.0.1"),
  port: parseInt(envStr("SIDECAR_PORT", "9876"), 10),

  // Core URL for forwarding incoming messages
  coreUrl: envStr("CORE_URL", "http://127.0.0.1:8899"),

  // Bridge API token for auth
  bridgeApiToken: (() => { const t = process.env.BRIDGE_API_TOKEN; if (!t || t === "change-me-bridge-token") { console.error("[Sidecar] 安全警告: BRIDGE_API_TOKEN 未设置或仍为默认值"); } return t || "" })(),
}
