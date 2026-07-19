import type { RuntimeConnection, DeploymentModeConfig } from "./runtime-types"

let cachedConnection: RuntimeConnection | null = null
let cachedConfig: DeploymentModeConfig | null = null

const TOKEN_KEY = "ai-companion-token"

export async function getRuntimeConnection(): Promise<RuntimeConnection> {
  if (cachedConnection) return cachedConnection

  const api = window.amitiaDesktop
  if (!api) {
    const base = (import.meta as any).env?.VITE_API_URL || window.location.origin
    cachedConnection = {
      apiBaseURL: base,
      websocketBaseURL: base.replace(/^http/, "ws"),
    }
    return cachedConnection
  }

  const config = await api.getDeploymentConfig()
  cachedConfig = config

  if (config.mode === "cloud" && config.serverURL) {
    cachedConnection = {
      apiBaseURL: config.serverURL,
      websocketBaseURL: config.serverURL.replace(/^http/, "ws"),
    }
    return cachedConnection
  }

  cachedConnection = {
    apiBaseURL: "http://127.0.0.1:18899",
    websocketBaseURL: "ws://127.0.0.1:18899",
  }
  return cachedConnection
}

export async function getApiBaseURL(): Promise<string> {
  const conn = await getRuntimeConnection()
  return conn.apiBaseURL
}

export async function getQQApiBaseURL(): Promise<string> {
  const base = await getApiBaseURL()
  if (base === "http://127.0.0.1:18899") {
    return "http://127.0.0.1:9877"
  }
  return base
}

export async function resolveApiUrl(path: string): Promise<string> {
  const base = await getApiBaseURL()
  return base + path
}

export async function resolveWebSocketUrl(path: string): Promise<string> {
  const conn = await getRuntimeConnection()
  return conn.websocketBaseURL + path
}

export async function createAuthorizedRequestInit(init?: RequestInit): Promise<RequestInit> {
  const token = localStorage.getItem(TOKEN_KEY)
  const headers: Record<string, string> = {}
  if (token) {
    headers["Authorization"] = `Bearer ${token}`
  }
  return {
    ...init,
    headers: {
      ...headers,
      ...(init?.headers as Record<string, string> || {}),
    },
  }
}

export async function getDeploymentConfig(): Promise<DeploymentModeConfig> {
  if (cachedConfig) return cachedConfig

  const api = window.amitiaDesktop
  if (!api) {
    cachedConfig = { mode: "local" }
    return cachedConfig
  }

  cachedConfig = await api.getDeploymentConfig()
  return cachedConfig
}

export async function saveDeploymentConfig(config: DeploymentModeConfig): Promise<DeploymentModeConfig> {
  cachedConnection = null
  cachedConfig = null

  const api = window.amitiaDesktop
  if (!api) {
    throw new Error("当前环境不支持保存部署配置")
  }

  cachedConfig = await api.saveDeploymentConfig(config)
  return cachedConfig
}

export function clearRuntimeCache(): void {
  cachedConnection = null
  cachedConfig = null
}

export function resetRuntimeConnectionCache(): void {
  cachedConnection = null
  cachedConfig = null
}
