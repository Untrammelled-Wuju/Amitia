// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { getWebDeviceAuthHeaders } from "./web-device-mesh";

export interface WebAccessStatus {
  configured: boolean;
  authenticated: boolean;
}

function normalizeBaseURL(value: string): string {
  return String(value || "").trim().replace(/\/+$/, "");
}

async function readPayload<T>(response: Response): Promise<T> {
  let payload: any = null;
  try {
    payload = await response.json();
  } catch {
    payload = null;
  }
  if (!response.ok) {
    throw new Error(String(payload?.msg || payload?.message || `请求失败 (${response.status})`));
  }
  return (payload?.data ?? payload) as T;
}

export async function getWebAccessStatus(baseURL: string): Promise<WebAccessStatus> {
  const response = await fetch(`${normalizeBaseURL(baseURL)}/api/public/web-access/status`, {
    method: "GET",
    headers: { Accept: "application/json", "X-Amitia-Client-Type": "web" },
    credentials: "include",
    cache: "no-store",
  });
  return readPayload<WebAccessStatus>(response);
}

export async function loginWebAccess(baseURL: string, password: string): Promise<void> {
  const response = await fetch(`${normalizeBaseURL(baseURL)}/api/public/web-access/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json", "X-Amitia-Client-Type": "web" },
    credentials: "include",
    body: JSON.stringify({ password }),
  });
  await readPayload(response);
}

export async function setupWebAccess(baseURL: string, password: string): Promise<void> {
  const normalized = normalizeBaseURL(baseURL);
  const auth = getWebDeviceAuthHeaders(normalized);
  if (!auth.Authorization) throw new Error("当前浏览器尚未完成设备配对");
  const response = await fetch(`${normalized}/api/web-access/setup`, {
    method: "POST",
    headers: { ...auth, "Content-Type": "application/json", Accept: "application/json" },
    credentials: "include",
    body: JSON.stringify({ password }),
  });
  await readPayload(response);
}

export async function logoutWebAccess(baseURL: string): Promise<void> {
  const response = await fetch(`${normalizeBaseURL(baseURL)}/api/public/web-access/logout`, {
    method: "POST",
    headers: { Accept: "application/json", "X-Amitia-Client-Type": "web" },
    credentials: "include",
  });
  await readPayload(response);
}
