// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import axios, { type AxiosInstance, type AxiosResponse, type AxiosError } from "axios"
import { ElMessage, ElMessageBox } from "element-plus"
import { ERR, type ApiResponse } from "@/types"

const BASE_URL = (import.meta as any).env?.VITE_API_URL || ""
const TOKEN_KEY = "ai-companion-token"

// ============================================================
// Error severity
// ============================================================
export type ErrorSeverity = "toast" | "banner" | "panel" | "fatal"

export interface RequestError {
  code: number
  message: string
  detail?: string
  severity: ErrorSeverity
  action?: { label: string; handler: () => void }
  raw?: any
}

// ============================================================
// Error classification
// ============================================================

function classifyError(body: ApiResponse | null, axiosError: AxiosError | null): RequestError {
  const code = body?.code ?? 0
  const message = body?.message || axiosError?.message || "Network error"
  const detail = body?.detail

  // Network / server not reachable
  if (!body && axiosError) {
    if (axiosError.code === "ECONNABORTED" || axiosError.message?.includes("timeout")) {
      return { code: ERR.TIMEOUT, message: "Request timed out", severity: "toast" }
    }
    if (axiosError.code === "ERR_NETWORK" || axiosError.message?.includes("Network Error")) {
      return {
        code: ERR.SERVICE_UNAVAILABLE,
        message: "Core service not running",
        detail: "Cannot connect to backend. Please start the core service.",
        severity: "panel",
        action: { label: "Try starting core service", handler: () => onStartCoreRequest?.() },
      }
    }
    return { code: ERR.SERVICE_UNAVAILABLE, message, severity: "toast" }
  }

  if (!body) {
    return { code: ERR.INTERNAL, message: "Unknown error", severity: "toast" }
  }

  // Auth errors: 401, 403, 700-702
  if (code === 401 || code === 403 || code === ERR.TOKEN_EXPIRED || code === ERR.TOKEN_INVALID || code === 702) {
    if (code === 401 || code === ERR.TOKEN_EXPIRED || code === ERR.TOKEN_INVALID) {
      localStorage.removeItem(TOKEN_KEY)
      if (window.location.pathname !== "/login") {
        window.location.href = "/login"
      }
      return { code, message: message || "Please login", severity: "fatal" }
    }
    return { code, message, detail, severity: "toast" }
  }

  // 5xx server errors
  if (code >= 500 && code < 600) {
    return { code, message, detail, severity: "panel" }
  }

  // 4xx client errors
  if (code >= 400 && code < 500) {
    return { code, message, detail, severity: "toast" }
  }

  // Business errors (600-699)
  if (code >= 600 && code < 700) {
    return { code, message, detail, severity: "banner" }
  }

  // Default
  return { code, message, detail, severity: "toast" }
}

// ============================================================
// Error callbacks (set by app entry)
// ============================================================

let onStartCoreRequest: (() => void) | null = null
let onErrorPanel: ((err: RequestError) => void) | null = null
let onErrorBanner: ((err: RequestError) => void) | null = null

export function setStartCoreHandler(fn: () => void) { onStartCoreRequest = fn }
export function setErrorPanelHandler(fn: (err: RequestError) => void) { onErrorPanel = fn }
export function setErrorBannerHandler(fn: (err: RequestError) => void) { onErrorBanner = fn }

// ============================================================
// Display error based on severity
// ============================================================

function displayError(err: RequestError) {
  switch (err.severity) {
    case "toast":
      if (err.action) {
        ElMessage({ message: err.message, type: "warning", duration: 4000 })
      } else {
        ElMessage.warning(err.message)
      }
      break
    case "banner":
      if (onErrorBanner) onErrorBanner(err)
      else ElMessage.warning(err.message)
      break
    case "panel":
      if (onErrorPanel) onErrorPanel(err)
      else ElMessageBox.alert(err.detail || err.message, err.message, { type: "error" })
      break
    case "fatal":
      // Already handled (redirect to login)
      break
  }
}

// ============================================================
// Axios instance
// ============================================================

export const request: AxiosInstance = axios.create({
  baseURL: BASE_URL,
  timeout: 30000,
  headers: { "Content-Type": "application/json" },
})

// Request interceptor: attach token
request.interceptors.request.use((config) => {
  const token = localStorage.getItem(TOKEN_KEY)
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// Response interceptor: unwrap and handle errors
request.interceptors.response.use(
  (response: AxiosResponse) => {
    const body = response.data as ApiResponse
    if (body && typeof body.code === "number") {
      if (body.code === 200) {
        return body.data ?? body
      }
      const err = classifyError(body, null)
      displayError(err)
      return Promise.reject(err)
    }
    return response.data
  },
  (error: AxiosError) => {
    const body = (error.response?.data as ApiResponse) || null
    const err = classifyError(body, error)
    displayError(err)
    return Promise.reject(err)
  }
)

// ============================================================
// Convenience methods
// ============================================================

export async function get<T>(url: string, params?: any): Promise<T> {
  const res = await request.get(url, { params })
  return res as unknown as T
}

export async function post<T>(url: string, data?: any): Promise<T> {
  const res = await request.post(url, data)
  return res as unknown as T
}

export async function put<T>(url: string, data?: any): Promise<T> {
  const res = await request.put(url, data)
  return res as unknown as T
}

export async function del<T>(url: string): Promise<T> {
  const res = await request.delete(url)
  return res as unknown as T
}

// Auth helpers
export function getToken(): string | null { return localStorage.getItem(TOKEN_KEY) }
export function setToken(token: string): void { localStorage.setItem(TOKEN_KEY, token) }
export function removeToken(): void { localStorage.removeItem(TOKEN_KEY) }
export function isLoggedIn(): boolean { return !!localStorage.getItem(TOKEN_KEY) }
