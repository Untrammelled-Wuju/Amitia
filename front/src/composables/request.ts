// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import axios, {
  type AxiosInstance,
  type AxiosRequestConfig,
  type AxiosResponse,
  type AxiosError,
} from "axios";
import { ElMessage, ElMessageBox } from "element-plus";
import { operationRequestTimeout, readOperationTimeoutHeaders } from "@/runtime/operation-timeout";
import { ERR, type ApiResponse } from "@/types";
import {
  getRuntimeConnection,
  getDeploymentConfig,
  getBackendAuthHeaders,
  isDeviceLocalApiPath,
  LOCAL_DEVICE_RUNTIME_BASE_URL,
} from "@/runtime/runtime-adapter";
import { getDeviceTimezone } from "@/utils/requestEnvelope";
import { resolveUIHostDeviceId } from "@/ui-runtime/deviceIdentity";
import { originalErrorDetail, originalErrorMessage } from "@/runtime/original-error";

const BASE_URL = (import.meta as any).env?.VITE_API_URL || "";

function notifyWebAccessExpired(error: any): boolean {
  if (typeof window === "undefined" || window.amitiaDesktop || Number(error?.response?.status) !== 401) return false;
  const body = error?.response?.data as any;
  const message = String(body?.msg || body?.message || "");
  if (!message.includes("Web 登录已失效")) return false;
  window.dispatchEvent(new CustomEvent("amitia:web-access-expired"));
  return true;
}

export type ErrorSeverity = "toast" | "banner" | "panel" | "fatal";

export interface RequestError {
  code: number;
  message: string;
  detail?: string;
  severity: ErrorSeverity;
  action?: { label: string; handler: () => void };
  raw?: any;
}

const recentDisplayedErrors = new Map<string, number>();

export function classifyError(
  body: ApiResponse | null,
  axiosError: AxiosError | null,
): RequestError {
  const problem = body as any;
  const code =
    typeof problem?.code === "number"
      ? problem.code
      : typeof problem?.status === "number"
        ? problem.status
        : 0;
  const message = originalErrorMessage(body ?? axiosError, "Unknown error");
  const detail = originalErrorDetail(body ?? axiosError);

  if (!body && axiosError) {
    const code = axiosError.code === "ECONNABORTED" || axiosError.message?.includes("timeout")
      ? ERR.TIMEOUT
      : ERR.SERVICE_UNAVAILABLE;
    return { code, message, detail, severity: "toast", raw: axiosError };
  }

  if (!body) {
    return { code: ERR.INTERNAL, message, detail, severity: "toast" };
  }

  if (
    code === 401 ||
    code === 403 ||
    code === 700 ||
    code === 701 ||
    code === 702 ||
    code === ERR.TOKEN_EXPIRED ||
    code === ERR.TOKEN_INVALID
  ) {
    if (
      code === 401 ||
      code === 700 ||
      code === 701 ||
      code === ERR.TOKEN_EXPIRED ||
      code === ERR.TOKEN_INVALID
    ) {
      return {
        code,
        message: message || "设备凭证无效或已过期，请重新配对当前设备",
        detail,
        severity: "toast",
      };
    }
    return { code, message, detail, severity: "toast" };
  }

  if (code >= 500 && code < 600) {
    return { code, message, detail, severity: "panel" };
  }

  if (code >= 400 && code < 500) {
    return { code, message, detail, severity: "toast" };
  }

  if (code >= 600 && code < 700) {
    return { code, message, detail, severity: "banner" };
  }

  return { code, message, detail, severity: "toast" };
}

let onStartCoreRequest: (() => void) | null = null;
let onErrorPanel: ((err: RequestError) => void) | null = null;
let onErrorBanner: ((err: RequestError) => void) | null = null;

export function setStartCoreHandler(fn: () => void) {
  onStartCoreRequest = fn;
}
export function setErrorPanelHandler(fn: (err: RequestError) => void) {
  onErrorPanel = fn;
}
export function setErrorBannerHandler(fn: (err: RequestError) => void) {
  onErrorBanner = fn;
}

export function displayError(err: RequestError) {
  const key = `${err.code}:${err.message}`;
  const now = Date.now();
  const previous = recentDisplayedErrors.get(key) || 0;
  const dedupeWindow = err.code === 401 || err.code === 403 || err.code === 503 ? 15_000 : 2_000;
  if (now - previous < dedupeWindow) return;
  recentDisplayedErrors.set(key, now);
  if (recentDisplayedErrors.size > 64) {
    for (const [storedKey, storedAt] of recentDisplayedErrors) {
      if (now - storedAt > 60_000) recentDisplayedErrors.delete(storedKey);
    }
  }
  switch (err.severity) {
    case "toast":
      if (err.action) {
        ElMessage({ message: err.message, type: "warning", duration: 4000 });
      } else {
        ElMessage.warning(err.message);
      }
      break;
    case "banner":
      if (onErrorBanner) onErrorBanner(err);
      else ElMessage.warning(err.message);
      break;
    case "panel":
      if (onErrorPanel) onErrorPanel(err);
      else
        ElMessageBox.alert(err.detail || err.message, err.message, {
          type: "error",
        });
      break;
    case "fatal":
      break;
  }
}

export type RetriableDesktopRequestConfig = AxiosRequestConfig & {
  __amitiaDesktopAuthRetried?: boolean;
  __amitiaDeviceLocal?: boolean;
};

export function isDesktopSessionAuthFailure(error: any, config?: RetriableDesktopRequestConfig): boolean {
  if (typeof window === "undefined" || !window.amitiaDesktop || config?.__amitiaDesktopAuthRetried) return false;
  const body = error?.response?.data as ApiResponse | undefined;
  const message = String(body?.message || (body as any)?.msg || "");
  const headers = config?.headers as any;
  const sessionHeader = String(headers?.get ? headers.get("X-Amitia-Desktop-Session") : headers?.["X-Amitia-Desktop-Session"] || "");
  return Number(error?.response?.status) === 401 && Boolean(sessionHeader) && /凭据无效|invalid session|missing session|invalid desktop/i.test(message);
}

export async function applyDesktopSessionRetry(config: RetriableDesktopRequestConfig): Promise<void> {
  config.__amitiaDesktopAuthRetried = true;
  await window.amitiaDesktop?.invalidateBackendSession?.();
  const authHeaders = await getBackendAuthHeaders(config.__amitiaDeviceLocal ? "local" : "business");
  const headers = config.headers as any;
  for (const [key, value] of Object.entries(authHeaders)) {
    if (!value) continue;
    if (headers?.set) headers.set(key, value);
    else headers[key] = value;
  }
}

export const request: AxiosInstance = axios.create({
  baseURL: BASE_URL,
  timeout: 30000,
});

function isGameCenterApiPath(path: string): boolean {
  const normalized = String(path || "").split("?", 1)[0];
  return normalized === "/api/game-center" || normalized.startsWith("/api/game-center/");
}

request.interceptors.request.use(async (config) => {
  const runtime = await getRuntimeConnection();
  const deployment = await getDeploymentConfig();
  config.withCredentials = !window.amitiaDesktop;
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
    Boolean(window.amitiaDesktop) &&
    (isDeviceLocalApiPath(requestPath) || gamePackageLocal);

  config.baseURL = deviceLocal ? LOCAL_DEVICE_RUNTIME_BASE_URL : runtime.apiBaseURL;
  config.timeout = operationRequestTimeout(config.baseURL || "");
  delete (config.headers as any)["X-Amitia-Management-Target"];
  (config.headers as any)?.delete?.("X-Amitia-Management-Target");
  (config as any).__amitiaDeviceLocal = deviceLocal;

  const publicPath = requestPath.split("?", 1)[0].startsWith("/api/public/");
  if (!publicPath) {
    const authHeaders = await getBackendAuthHeaders(deviceLocal ? "local" : "business");
    for (const [key, value] of Object.entries(authHeaders)) {
      if (value) (config.headers as any)[key] = value;
    }
  }

  (config.headers as any)["X-Amitia-Client-Type"] = window.amitiaDesktop ? "desktop" : "web";
  const deviceId = await resolveUIHostDeviceId();
  if (deviceId) {
    (config.headers as any)["X-Amitia-Device-ID"] = deviceId;
    if (deployment.mode === "cloud" && isGameCenterApiPath(requestPath)) {
      (config.headers as any)["X-Amitia-Target-Device-ID"] = deviceId;
    }
  }


  const timezone = getDeviceTimezone();
  if (timezone) (config.headers as any)["X-Device-Timezone"] = timezone;
  return config;
});

request.interceptors.response.use(
  (response: AxiosResponse) => {
    readOperationTimeoutHeaders(response.config.baseURL || "", response.headers);
    const body = response.data as ApiResponse;
    if (body && typeof body.code === "number") {
      if (body.code === 200) {
        return body.data ?? body;
      }
      const err = classifyError(body, null);
      displayError(err);
      return Promise.reject(err);
    }
    return response.data;
  },
  async (error: AxiosError) => {
    const requestConfig = error.config as RetriableDesktopRequestConfig | undefined;
    if (isDesktopSessionAuthFailure(error, requestConfig) && requestConfig) {
      await applyDesktopSessionRetry(requestConfig);
      return request.request(requestConfig);
    }
    const body = (error.response?.data as ApiResponse) || null;
    const webAccessExpired = notifyWebAccessExpired(error);
    const err = classifyError(body, error);
    err.raw = body;
    if (!webAccessExpired) displayError(err);
    return Promise.reject(err);
  },
);

export async function get<T>(url: string, params?: any): Promise<T> {
  const res = await request.get(url, { params });
  return res as unknown as T;
}

export async function post<T>(url: string, data?: any): Promise<T> {
  const res = await request.post(url, data);
  return res as unknown as T;
}

export async function put<T>(url: string, data?: any): Promise<T> {
  const res = await request.put(url, data);
  return res as unknown as T;
}

export async function del<T>(url: string): Promise<T> {
  const res = await request.delete(url);
  return res as unknown as T;
}
