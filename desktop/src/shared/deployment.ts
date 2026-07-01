import type { DeploymentModeConfig } from "./types"

export const DEFAULT_CLOUD_SERVER_URL = "https://api.amitia.cn"

export function normalizeServerURL(input: string): string {
  const url = new URL(input.trim())
  url.hash = ""
  url.search = ""
  return url.toString().replace(/\/$/, "")
}

export function validateSelfHostedURL(input: string): string {
  const normalized = normalizeServerURL(input)
  const url = new URL(normalized)
  if (url.protocol !== "https:" && url.protocol !== "http:") {
    throw new Error("自建服务器地址只允许 http 或 https")
  }
  if (!url.hostname) {
    throw new Error("自建服务器地址缺少主机名")
  }
  const blockedHosts = new Set(["0.0.0.0", "::", "[::]"])
  if (blockedHosts.has(url.hostname)) {
    throw new Error("自建服务器地址不能使用不可路由地址")
  }
  if (url.username || url.password) {
    throw new Error("自建服务器地址不能包含用户名或密码")
  }
  return normalized
}

export function validateDeploymentConfig(config: DeploymentModeConfig): DeploymentModeConfig {
  if (!config || typeof config !== "object") {
    throw new Error("部署配置必须是对象")
  }
  if (config.mode === "local") {
    return { mode: "local" }
  }
  if (config.mode === "cloud") {
    return { mode: "cloud", serverURL: DEFAULT_CLOUD_SERVER_URL }
  }
  if (config.mode === "self-hosted") {
    if (!config.serverURL) {
      throw new Error("自建模式必须填写服务器地址")
    }
    return { mode: "self-hosted", serverURL: validateSelfHostedURL(config.serverURL) }
  }
  throw new Error("未知部署模式")
}

export function configToLabel(config: DeploymentModeConfig): string {
  if (config.mode === "local") return "本地模式"
  if (config.mode === "cloud") return "云端模式"
  return "自建服务器"
}
