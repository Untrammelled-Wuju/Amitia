// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { useSessionStore, clearStoredToken, revokeOnServer } from "./session-store";
import { refreshToken } from "./session-client";

let refreshPromise: Promise<string> | null = null;
let refreshTimer: ReturnType<typeof setTimeout> | null = null;

function clearRefreshTimer(): void {
  if (refreshTimer) {
    clearTimeout(refreshTimer);
    refreshTimer = null;
  }
}

function scheduleRefresh(expiresAt: string) {
  clearRefreshTimer();
  const expiry = new Date(expiresAt).getTime();
  const now = Date.now();
  const refreshIn = Math.max(expiry - now - 60000, 5000);
  refreshTimer = setTimeout(() => {
    performRefresh();
  }, refreshIn);
}

async function performRefresh(): Promise<string> {
  const { setAccessToken, clearSession } = useSessionStore();
  if (refreshPromise) {
    return refreshPromise;
  }
  refreshPromise = (async () => {
    try {
      const result = await refreshToken({ refreshToken: "" });
      setAccessToken(result.accessToken, result.accessTokenExpiresAt);
      if (result.refreshTokenExpiresAt) {
        scheduleRefresh(result.refreshTokenExpiresAt);
      }
      return result.accessToken;
    } catch (err) {
      clearSession();
      throw err;
    } finally {
      refreshPromise = null;
    }
  })();
  return refreshPromise;
}

export function initRefreshCoordinator(expiresAt?: string) {
  if (expiresAt) {
    scheduleRefresh(expiresAt);
  }
}

export function stopRefreshCoordinator() {
  clearRefreshTimer();
  refreshPromise = null;
}

export function getAccessToken(): string | null {
  const { state } = useSessionStore();
  return state.value.accessToken;
}

export function forceCleanupSession(): void {
  clearRefreshTimer();
  refreshPromise = null;
  clearStoredToken();
  revokeOnServer();
}

export async function ensureValidToken(): Promise<string | null> {
  const { state, clearSession } = useSessionStore();
  const token = state.value.accessToken;
  const expiresAt = state.value.accessTokenExpiresAt;
  if (!token) return null;
  if (!expiresAt) return token;
  const expiry = new Date(expiresAt).getTime();
  if (Date.now() >= expiry - 30000) {
    try {
      const newToken = await performRefresh();
      return newToken;
    } catch {
      clearSession();
      return null;
    }
  }
  return token;
}
