// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
function envStr(key: string, fallback: string): string {
  return process.env[key] || fallback
}

export const qqSidecarConfig = {
  mergeWindowMs: parseInt(process.env.MERGE_WINDOW_MS || "6000", 10),

  host: envStr("QQ_SIDECAR_HOST", "127.0.0.1"),
  port: parseInt(envStr("QQ_SIDECAR_PORT", "9877"), 10),

  coreUrl: envStr("CORE_URL", "http://127.0.0.1:8899"),

  bridgeApiToken: envStr("BRIDGE_API_TOKEN", ""),

  qqbot: {
    appId: envStr("QQBOT_APP_ID", ""),
    token: envStr("QQBOT_TOKEN", ""),
    sandbox: envStr("QQBOT_SANDBOX", "false") === "true",
  },
}
