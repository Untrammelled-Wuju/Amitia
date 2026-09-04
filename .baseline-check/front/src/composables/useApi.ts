// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import axios, { type AxiosInstance, type AxiosRequestConfig } from "axios";
import type { AxiosError } from "axios";
import { ref } from "vue";
import type { ApiResponse } from "@/types";
import {
  getRuntimeConnection,
  getDeploymentConfig,
  getBackendAuthHeaders,
  isDeviceLocalApiPath,
  LOCAL_DEVICE_RUNTIME_BASE_URL,
} from "@/runtime/runtime-adapter";
import { getDeviceTimezone } from "@/utils/requestEnvelope";
import { resolveUIHostDeviceId } from "@/ui-runtime/deviceIdentity";
import { classifyError, displayError } from "./request";
import { ensureValidToken, initRefreshCoordinator, stopRefreshCoordinator, forceCleanupSession } from "@/stores/refresh-coordinator";

const BASE_URL = (import.meta as any).env?.VITE_API_URL || "";

function isGameCenterApiPath(path: string): boolean {
  const normalized = String(path || "").split("?", 1)[0];
  return normalized === "/api/game-center" || normalized.startsWith("/api/game-center/");
}

const PUBLIC_AUTH_PATHS = new Set([
  "/api/public/auth/status",
  "/api/public/auth/setup",
  "/api/public/auth/login",
  "/api/public/auth/refresh",
  "/api/public/auth/logout/revoke",
  "/api/public/onboarding/status",
  "/api/public/onboarding/complete",
  "/api/public/runtime/capabilities",
]);

export const apiClient: AxiosInstance = axios.create({
  baseURL: BASE_URL,
  timeout: 30000,
});

apiClient.interceptors.request.use(async (config) => {
  const runtime = await getRuntimeConnection();
  const deployment = await getDeploymentConfig();
  const requestPath = String(config.url ?? "");
  config.headers = config.headers ?? {};
  const managementTarget = String(
    (config.headers as any)?.["X-Amitia-Management-Target"] ??
      (config.headers as any)?.get?.("X-Amitia-Management-Target") ??
      "",
  ).toLowerCase();
  const gamePackageLocal =
    Boolean(window.amitiaDesktop) &&
    managementTarget === "game-center" &&
    (requestPath === "/api/extensions/packages" ||
      requestPath.startsWith("/api/extensions/packages/") ||
      requestPath === "/api/extensions/kernel/extensions/uninstall" ||
      requestPath.startsWith("/api/extensions/kernel/extensions/uninstall/"));
  const deviceLocal =
    Boolean(window.amitiaDesktop) && (isDeviceLocalApiPath(requestPath) || gamePackageLocal);
  config.baseURL = deviceLocal ? LOCAL_DEVICE_RUNTIME_BASE_URL : runtime.apiBaseURL;
  delete (config.headers as any)["X-Amitia-Management-Target"];
  (config.headers as any)?.delete?.("X-Amitia-Management-Target");

  if (PUBLIC_AUTH_PATHS.has(requestPath)) {
    delete config.headers.Authorization;
    delete config.headers["X-Amitia-Desktop-Session"];
    delete config.headers["X-Amitia-Desktop-Instance"];
    config.headers["X-Amitia-Client-Type"] = "desktop";
    (config as AxiosRequestConfig & { __amitiaPublicAuth?: boolean; __amitiaDeviceLocal?: boolean }).__amitiaPublicAuth = true;
    return config;
  }

  if (deviceLocal) {
    (config as AxiosRequestConfig & { __amitiaDeviceLocal?: boolean }).__amitiaDeviceLocal = true;
  }

  if ((deployment.mode === "local" || deviceLocal) && window.amitiaDesktop) {
    const desktopHeaders = await getBackendAuthHeaders();
    for (const [key, value] of Object.entries(desktopHeaders)) {
      config.headers[key] = value;
    }
  }

  config.headers["X-Amitia-Client-Type"] = "desktop";

  const deviceId = await resolveUIHostDeviceId();
  if (deviceId) {
    config.headers["X-Amitia-Device-ID"] = deviceId;
  }

  if (deviceId && deployment.mode === "cloud" && isGameCenterApiPath(requestPath)) {
    // Cloud Core is the control plane; the GameHost remains on this concrete
    // device. Sending an explicit target prevents the cloud from guessing
    // among multiple online devices.
    config.headers["X-Amitia-Target-Device-ID"] = deviceId;
  }

  if (deviceLocal) {
    // Desktop-pet package/runtime authority is always the local device agent,
    // including cloud deployments. Do not leak the cloud bearer token to the
    // loopback runtime.
    delete config.headers.Authorization;
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
    const requestConfig = error?.config as
      | (AxiosRequestConfig & {
          __amitiaPublicAuth?: boolean;
          __amitiaDeviceLocal?: boolean;
        })
      | undefined;
    if (
      error?.response?.status === 401 &&
      !requestConfig?.__amitiaPublicAuth &&
      !requestConfig?.__amitiaDeviceLocal
    ) {
      forceCleanupSession();
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

  async function get<T>(url: string, params?: any, config?: AxiosRequestConfig): Promise<T> {
    loading.value = true;
    try {
      const res = await apiClient.get(url, { ...(config ?? {}), params });
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

export { ensureValidToken, initRefreshCoordinator, stopRefreshCoordinator, forceCleanupSession };
