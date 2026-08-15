// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import axios, { type AxiosInstance, type AxiosRequestConfig } from "axios";
import type { AxiosError } from "axios";
import { ref } from "vue";
import type { ApiResponse } from "@/types";
import { getRuntimeConnection, getDeploymentConfig, getBackendAuthHeaders } from "@/runtime/runtime-adapter";
import { getDeviceTimezone } from "@/utils/requestEnvelope";
import { classifyError, displayError } from "./request";
import { getStoredToken, setStoredToken, clearStoredToken } from "@/stores/session-store";
import { ensureValidToken, initRefreshCoordinator, stopRefreshCoordinator } from "@/stores/refresh-coordinator";

const BASE_URL = (import.meta as any).env?.VITE_API_URL || "";

const PUBLIC_AUTH_PATHS = new Set([
  "/api/public/auth/status",
  "/api/public/auth/setup",
  "/api/public/auth/login",
  "/api/public/auth/refresh",
  "/api/public/onboarding/status",
]);

export const apiClient: AxiosInstance = axios.create({
  baseURL: BASE_URL,
  timeout: 30000,
});

apiClient.interceptors.request.use(async (config) => {
  const runtime = await getRuntimeConnection();
  const deployment = await getDeploymentConfig();
  config.baseURL = runtime.apiBaseURL;

  config.headers = config.headers ?? {};

  const requestPath = String(config.url ?? "");
  if (PUBLIC_AUTH_PATHS.has(requestPath)) {
    delete config.headers.Authorization;
    delete config.headers["X-Amitia-Desktop-Session"];
    delete config.headers["X-Amitia-Desktop-Instance"];
    return config;
  }

  if (deployment.mode === "local" && window.amitiaDesktop) {
    const desktopHeaders = await getBackendAuthHeaders();
    for (const [key, value] of Object.entries(desktopHeaders)) {
      config.headers[key] = value;
    }
    const token = getStoredToken();
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
  } else {
    const token = await ensureValidToken();
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
  }

  const deviceTimezone = getDeviceTimezone();
  if (deviceTimezone) {
    config.headers["X-Device-Timezone"] = deviceTimezone;
  }

  return config;
});

// Response interceptor: unwrap and handle errors
apiClient.interceptors.response.use(
  (response) => {
    const body = response.data as ApiResponse;
    if (body && typeof body.code === "number") {
      if (body.code === 200) {
        response.data = body.data ?? (body as any);
        return response;
      }
      const err = classifyError(body, null);
      displayError(err);
      return Promise.reject(err);
    }
    return response;
  },
  (error) => {
    if (error && typeof error === "object" && "severity" in error) {
      return Promise.reject(error);
    }
    const body = (error.response?.data as ApiResponse) || null;
    const err = classifyError(body, error as AxiosError);
    err.raw = body;
    displayError(err);
    return Promise.reject(err);
  },
);

export function useApi() {
  const loading = ref(false);

  async function get<T>(url: string, params?: any): Promise<T> {
    loading.value = true;
    try {
      const res = await apiClient.get(url, { params });
      return res.data as T;
    } finally {
      loading.value = false;
    }
  }

  async function post<T>(
    url: string,
    data?: any,
    config?: AxiosRequestConfig,
  ): Promise<T> {
    loading.value = true;
    try {
      const res = await apiClient.post(url, data, config);
      return res.data as T;
    } finally {
      loading.value = false;
    }
  }

  async function postUpload<T>(url: string, file: File, fieldName: string = "card"): Promise<T> {
    loading.value = true;
    try {
      const formData = new FormData();
      formData.append(fieldName, file);
      const res = await apiClient.post(url, formData, {
        headers: { "Content-Type": "multipart/form-data" },
      });
      return res.data as T;
    } finally {
      loading.value = false;
    }
  }

  async function put<T>(url: string, data?: any): Promise<T> {
    loading.value = true;
    try {
      const res = await apiClient.put(url, data);
      return res.data as T;
    } finally {
      loading.value = false;
    }
  }

  async function del<T>(url: string): Promise<T> {
    loading.value = true;
    try {
      const res = await apiClient.delete(url);
      return res.data as T;
    } finally {
      loading.value = false;
    }
  }

  return { loading, get, post, postUpload, put, del };
}

// Auth helpers (legacy compatibility - prefer session-manager)
export function getToken(): string | null {
  return getStoredToken();
}

export function setToken(token: string): void {
  setStoredToken(token);
}

export function removeToken(): void {
  clearStoredToken();
  stopRefreshCoordinator();
}

export function isLoggedIn(): boolean {
  return !!getStoredToken();
}
