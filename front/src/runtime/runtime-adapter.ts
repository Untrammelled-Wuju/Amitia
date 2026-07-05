import { isDesktopShell } from "./runtime-capabilities"
import type { DeploymentModeConfig, RuntimeConnection } from "./runtime-types"

let runtimeConnectionPromise: Promise<RuntimeConnection> | null = null

function inferWebSocketBaseURL(apiBaseURL: string): string {
  if (!apiBaseURL) {
    if (typeof window === "undefined") return ""
    return window.location.origin.replace(/^http:/, "ws:").replace(/^https:/, "wss:")
  }
  return apiBaseURL.replace(/^http:/, "ws:").replace(/^https:/, "wss:")
}

function createBrowserConnection(): RuntimeConnection {
  const apiBaseURL = (import.meta as any).env?.VITE_API_URL || ""
  return {
    mode: "browser",
    apiBaseURL,
    websocketBaseURL: inferWebSocketBaseURL(apiBaseURL),
  }
}

async function createDesktopConnection(): Promise<RuntimeConnection> {
  const config = await window.amitiaDesktop!.getDeploymentConfig()
  if (config.mode === "cloud") {
    const apiBaseURL = config.serverURL || "https://api.amitia.cn"
    return {
      mode: "desktop-cloud",
      apiBaseURL,
      websocketBaseURL: inferWebSocketBaseURL(apiBaseURL),
    }
  }
  if (config.mode === "self-hosted") {
    const apiBaseURL = config.serverURL || ""
    return {
      mode: "desktop-self-hosted",
      apiBaseURL,
      websocketBaseURL: inferWebSocketBaseURL(apiBaseURL),
    }
  }
  return {
    mode: "desktop-local",
    apiBaseURL: "http://127.0.0.1:18080",
    websocketBaseURL: "ws://127.0.0.1:18080",
  }
}

export async function getRuntimeConnection(): Promise<RuntimeConnection> {
  if (!runtimeConnectionPromise) {
    runtimeConnectionPromise = isDesktopShell() ? createDesktopConnection() : Promise.resolve(createBrowserConnection())
  }
  return runtimeConnectionPromise
}

export function resetRuntimeConnectionCache(): void {
  runtimeConnectionPromise = null
}

function joinUrl(baseURL: string, path: string): string {
  if (!baseURL) return path
  if (/^https?:\/\//.test(path) || /^wss?:\/\//.test(path)) return path
  return `${baseURL.replace(/\/$/, "")}/${path.replace(/^\//, "")}`
}

export async function getApiBaseURL(): Promise<string> {
  const runtime = await getRuntimeConnection()
  return runtime.apiBaseURL
}

export async function resolveApiUrl(path: string): Promise<string> {
  const runtime = await getRuntimeConnection()
  return joinUrl(runtime.apiBaseURL, path)
}

export async function resolveWebSocketUrl(path: string): Promise<string> {
  const runtime = await getRuntimeConnection()
  return joinUrl(runtime.websocketBaseURL, path)
}

export async function resolveServerBaseURL(): Promise<string> {
  const runtime = await getRuntimeConnection()
  return runtime.apiBaseURL || window.location.origin
}

export async function createAuthorizedRequestInit(init: RequestInit = {}): Promise<RequestInit> {
  const token = localStorage.getItem("ai-companion-token")
  const headers = new Headers(init.headers || {})
  if (token) headers.set("Authorization", `Bearer ${token}`)
  return { ...init, headers }
}

export async function authorizedFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const url = await resolveApiUrl(path)
  const requestInit = await createAuthorizedRequestInit(init)
  return fetch(url, requestInit)
}

export async function getQQApiBaseURL(): Promise<string> {
  return resolveApiUrl("/api/qq")
}

export async function getDeploymentConfig(): Promise<DeploymentModeConfig | null> {
  if (!isDesktopShell()) return null
  return window.amitiaDesktop!.getDeploymentConfig()
}
