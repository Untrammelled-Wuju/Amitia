// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import axios, { type AxiosInstance, type AxiosRequestConfig } from "axios"
import { ref } from "vue"
import { ElMessage } from "element-plus"
import type { ApiResponse } from "@/types"
import { ERR } from "@/types"

// Import from the unified request module for error classification
import { request as unifiedRequest } from "./request"

const BASE_URL = (import.meta as any).env?.VITE_API_URL || ""
const TOKEN_KEY = "ai-companion-token"

export const apiClient: AxiosInstance = axios.create({
  baseURL: BASE_URL,
  timeout: 30000,
  headers: { "Content-Type": "application/json" },
})

// Request interceptor: attach auth token
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem(TOKEN_KEY)
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Response interceptor: delegate to unified error handling
apiClient.interceptors.response.use(
  (response) => {
    const body = response.data as ApiResponse
    if (body && typeof body.code === "number") {
      if (body.code === 200) {
        response.data = body.data ?? (body as any)
        return response
      }
      // Delegate to unified request module for error classification
      return unifiedRequest.interceptors.response.handlers?.[0]?.fulfilled?.(response) ?? response
    }
    return response
  },
  (error) => {
    // Delegate to unified request module for network errors
    return unifiedRequest.interceptors.response.handlers?.[0]?.rejected?.(error) ?? Promise.reject(error)
  }
)

export function useApi() {
  const loading = ref(false)

  async function get<T>(url: string, params?: any): Promise<T> {
    loading.value = true
    try {
      const res = await apiClient.get(url, { params })
      return res.data as T
    } finally {
      loading.value = false
    }
  }

  async function post<T>(url: string, data?: any): Promise<T> {
    loading.value = true
    try {
      const res = await apiClient.post(url, data)
      return res.data as T
    } finally {
      loading.value = false
    }
  }

  async function put<T>(url: string, data?: any): Promise<T> {
    loading.value = true
    try {
      const res = await apiClient.put(url, data)
      return res.data as T
    } finally {
      loading.value = false
    }
  }

  async function del<T>(url: string): Promise<T> {
    loading.value = true
    try {
      const res = await apiClient.delete(url)
      return res.data as T
    } finally {
      loading.value = false
    }
  }

  return { loading, get, post, put, del }
}

// Auth helpers
export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function removeToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

export function isLoggedIn(): boolean {
  return !!localStorage.getItem(TOKEN_KEY)
}
