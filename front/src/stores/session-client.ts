// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { apiClient } from "@/composables/useApi";

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  accessToken: string;
  accessTokenExpiresAt: string;
  refreshToken?: string;
  refreshTokenExpiresAt?: string;
  token?: string;
  session?: {
    sessionId: string;
    createdAt: string;
  };
  user?: {
    id: string;
    username: string;
    role: string;
  };
}

export interface RefreshRequest {
  refreshToken: string;
}

export interface RefreshResponse {
  accessToken: string;
  accessTokenExpiresAt: string;
  refreshToken?: string;
  refreshTokenExpiresAt?: string;
}

export interface SessionInfo {
  sessionId: string;
  current: boolean;
  status: string;
  deviceName: string;
  ipAddress: string;
  userAgent: string;
  createdAt: string;
  lastActiveAt?: string;
  lastRefreshedAt?: string;
  expiresAt?: string;
}

export async function loginUser(req: LoginRequest): Promise<LoginResponse> {
  const res = await apiClient.post<LoginResponse>("/api/public/auth/login", req);
  return res as unknown as LoginResponse;
}

export async function refreshToken(req: RefreshRequest): Promise<RefreshResponse> {
  const res = await apiClient.post<RefreshResponse>("/api/public/auth/refresh", req);
  return res as unknown as RefreshResponse;
}

export async function logoutUser(): Promise<void> {
  await apiClient.post("/api/public/auth/logout");
}

export async function listSessions(): Promise<SessionInfo[]> {
  const res = await apiClient.get<SessionInfo[]>("/api/public/auth/sessions");
  return res as unknown as SessionInfo[];
}

export async function revokeSession(sessionId: string): Promise<void> {
  await apiClient.delete(`/api/public/auth/sessions/${sessionId}`);
}

export async function revokeOtherSessions(): Promise<{ revokedCount: number }> {
  const res = await apiClient.delete<{ revokedCount: number }>("/api/public/auth/sessions");
  return res as unknown as { revokedCount: number };
}

export async function logoutAll(): Promise<void> {
  await apiClient.post("/api/public/auth/logout-all");
}
