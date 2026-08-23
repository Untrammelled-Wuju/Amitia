// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { useSessionStore, revokeOnServer } from "./session-store";
import { refreshToken } from "./session-client";

const REFRESH_TOKEN_KEY = "amitia-refresh-token-persist";

let refreshPromise: Promise<string> | null = null;
let refreshTimer: ReturnType<typeof setTimeout> | null = null;

function clearRefreshTimer(): void {
  if (refreshTimer) {
    clearTimeout(refreshTimer);
    refreshTimer = null;
  }
}

export function saveRefreshToken(token: string): void {
  try {
    localStorage.setItem(REFRESH_TOKEN_KEY, token);
  } catch {}
}

export function loadRefreshToken(): string | null {
  try {
    return localStorage.getItem(REFRESH_TOKEN_KEY);
  } catch {
    return null;
  }
}

export function clearPersistedSession(): void {
  try {
    localStorage.removeItem(REFRESH_TOKEN_KEY);
  } catch {}
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
    console.log("[performRefresh] reusing existing refresh promise");
    return refreshPromise;
  }
  refreshPromise = (async () => {
    try {
      const storedRefreshToken = loadRefreshToken();
      console.log("[performRefresh] storedRefreshToken:", storedRefreshToken ? storedRefreshToken.substring(0, 8) + "..." : "null");
      if (!storedRefreshToken) {
        throw new Error("NO_REFRESH_TOKEN");
      }
      const result = await refreshToken({ refreshToken: storedRefreshToken });
      console.log("[performRefresh] new accessToken:", result.accessToken ? result.accessToken.substring(0, 8) + "..." : "null");
      console.log("[performRefresh] new refreshToken:", result.refreshToken ? result.refreshToken.substring(0, 8) + "..." : "null");
      setAccessToken(result.accessToken, result.accessTokenExpiresAt);
      if (result.refreshToken) {
        saveRefreshToken(result.refreshToken);
      }
      if (result.refreshTokenExpiresAt) {
        scheduleRefresh(result.refreshTokenExpiresAt);
      }
      return result.accessToken;
    } catch (err) {
      console.error("[performRefresh] failed:", err);
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
  clearPersistedSession();
  revokeOnServer();
}

export async function ensureValidToken(): Promise<string | null> {
  const { state, clearSession } = useSessionStore();
  const token = state.value.accessToken;
  const expiresAt = state.value.accessTokenExpiresAt;
  if (token && expiresAt) {
    const expiry = new Date(expiresAt).getTime();
    if (Date.now() < expiry - 30000) {
      return token;
    }
  }
  try {
    const newToken = await performRefresh();
    return newToken;
  } catch {
    clearSession();
    return null;
  }
}

export async function restoreSessionOnStartup(): Promise<boolean> {
  const { setSession } = useSessionStore();
  const storedRefreshToken = loadRefreshToken();
  console.log("[restoreSessionOnStartup] storedRefreshToken:", storedRefreshToken ? storedRefreshToken.substring(0, 8) + "..." : "null");
  if (!storedRefreshToken) {
    return false;
  }
  try {
    const result = await refreshToken({ refreshToken: storedRefreshToken });
    console.log("[restoreSessionOnStartup] refresh success, accessToken:", result.accessToken ? result.accessToken.substring(0, 8) + "..." : "null");
    console.log("[restoreSessionOnStartup] new refreshToken:", result.refreshToken ? result.refreshToken.substring(0, 8) + "..." : "null");
    setSession({
      accessToken: result.accessToken,
      accessTokenExpiresAt: result.accessTokenExpiresAt,
      sessionId: null,
      userId: null,
      username: null,
      role: null,
    });
    if (result.refreshToken) {
      saveRefreshToken(result.refreshToken);
      console.log("[restoreSessionOnStartup] saved new refreshToken to localStorage");
    } else {
      console.warn("[restoreSessionOnStartup] no new refreshToken in response!");
    }
    if (result.refreshTokenExpiresAt) {
      scheduleRefresh(result.refreshTokenExpiresAt);
    }
    const verifyToken = loadRefreshToken();
    console.log("[restoreSessionOnStartup] verify localStorage token:", verifyToken ? verifyToken.substring(0, 8) + "..." : "null");
    return true;
  } catch (err) {
    console.error("[restoreSessionOnStartup] refresh failed:", err);
    clearPersistedSession();
    return false;
  }
}
