// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import axios, {
  type AxiosInstance,
  type AxiosResponse,
  type AxiosError,
} from "axios";
import { ElMessage, ElMessageBox } from "element-plus";
import { ERR, type ApiResponse } from "@/types";
import { getRuntimeConnection } from "@/runtime/runtime-adapter";
import { forceCleanupSession, getAccessToken } from "@/stores/refresh-coordinator";

const BASE_URL = (import.meta as any).env?.VITE_API_URL || "";

export type ErrorSeverity = "toast" | "banner" | "panel" | "fatal";

export interface RequestError {
  code: number;
  message: string;
  detail?: string;
  severity: ErrorSeverity;
  action?: { label: string; handler: () => void };
  raw?: any;
}

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
  const message =
    problem?.message ||
    problem?.msg ||
    problem?.title ||
    axiosError?.message ||
    "Network error";
  const detail = problem?.detail;

  if (!body && axiosError) {
    if (
      axiosError.code === "ECONNABORTED" ||
      axiosError.message?.includes("timeout")
    ) {
      return {
        code: ERR.TIMEOUT,
        message: "Request timed out",
        severity: "toast",
      };
    }
    if (
      axiosError.code === "ERR_NETWORK" ||
      axiosError.message?.includes("Network Error")
    ) {
      return {
        code: ERR.SERVICE_UNAVAILABLE,
        message: "核心服务连接中...",
        severity: "toast",
      };
    }
    return { code: ERR.SERVICE_UNAVAILABLE, message, severity: "toast" };
  }

  if (!body) {
    return { code: ERR.INTERNAL, message: "Unknown error", severity: "toast" };
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
      forceCleanupSession();
      const PUBLIC_PATHS = ["/login", "/setup", "/onboarding", "/privacy", "/usage-boundary"];
      if (!PUBLIC_PATHS.includes(window.location.pathname)) {
        window.location.href = "/login";
      }
      return { code, message: message || "Please login", severity: "fatal" };
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

export const request: AxiosInstance = axios.create({
  baseURL: BASE_URL,
  timeout: 30000,
});

request.interceptors.request.use(async (config) => {
  const runtime = await getRuntimeConnection();
  config.baseURL = runtime.apiBaseURL;
  const token = getAccessToken();
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

request.interceptors.response.use(
  (response: AxiosResponse) => {
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
  (error: AxiosError) => {
    if (error?.response?.status === 401) {
      forceCleanupSession();
    }
    const body = (error.response?.data as ApiResponse) || null;
    const err = classifyError(body, error);
    err.raw = body;
    displayError(err);
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
