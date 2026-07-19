import type { DeploymentModeConfig } from "./types"

export function validateDeploymentConfig(raw: unknown): DeploymentModeConfig {
  if (!raw || typeof raw !== "object") {
    return { mode: "local" }
  }

  const obj = raw as Record<string, unknown>
  const mode = obj.mode

  if (mode === "cloud") {
    const serverURL = obj.serverURL
    if (typeof serverURL !== "string" || serverURL.trim().length === 0) {
      return { mode: "local" }
    }
    let url = serverURL.trim().replace(/\/+$/, "")
    if (!/^https?:\/\//i.test(url)) {
      url = "http://" + url
    }
    return { mode: "cloud", serverURL: url }
  }

  return { mode: "local" }
}

export function configToLabel(config: DeploymentModeConfig): string {
  if (config.mode === "cloud") {
    return config.serverURL ? `云端 (${config.serverURL})` : "云端"
  }
  return "本地"
}
