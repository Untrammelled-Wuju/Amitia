// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
function envStr(key: string, fallback: string): string {
  return process.env[key] || fallback
}

export const qqSidecarConfig = {
  mergeWindowMs: parseInt(process.env.MERGE_WINDOW_MS || "6000", 10),
  host: envStr("QQ_SIDECAR_HOST", "127.0.0.1"),
  port: parseInt(envStr("QQ_SIDECAR_PORT", "19877"), 10),
  coreUrl: envStr("AMITIA_CORE_URL", envStr("CORE_URL", "http://127.0.0.1:18899")),
  serviceAuthToken: envStr("AMITIA_SERVICE_AUTH_TOKEN", envStr("BRIDGE_API_TOKEN", "")),
  localToken: envStr("AMITIA_LOCAL_TOKEN", ""),
  extensionId: envStr("AMITIA_EXTENSION_ID", "com.amitia/channel-qq"),
  moduleId: envStr("AMITIA_MODULE_ID", "qq-channel-service"),
  qqbot: {
    appId: envStr("QQBOT_APP_ID", ""),
    token: envStr("QQBOT_TOKEN", ""),
    sandbox: envStr("QQBOT_SANDBOX", "false") === "true",
  },
}

export function coreRequestHeaders(): Record<string, string> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "X-Amitia-Extension-ID": qqSidecarConfig.extensionId,
    "X-Amitia-Module-ID": qqSidecarConfig.moduleId,
  }
  if (qqSidecarConfig.serviceAuthToken) {
    headers.Authorization = "Bearer " + qqSidecarConfig.serviceAuthToken
  }
  if (qqSidecarConfig.localToken) {
    headers["X-Amitia-Local-Token"] = qqSidecarConfig.localToken
  }
  return headers
}
