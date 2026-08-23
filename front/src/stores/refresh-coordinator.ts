// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { useSessionStore, revokeOnServer } from "./session-store";
import { refreshToken } from "./session-client";

const REFRESH_TOKEN_KEY = "amitia-refresh-token-persist";
const SESSION_KEY = "amitia-session-persist";
const REFRESH_LOCK_NAME = "amitia-refresh-token";

let refreshPromise: Promise<string> | null = null;
let refreshTimer: ReturnType<typeof setTimeout> | null = null;
let sessionEpoch = 0;

interface PersistedSession {
  accessToken: string | null;
  accessTokenExpiresAt: string | null;
  refreshToken: string | null;
  sessionId: string | null;
  userId: string | null;
  username: string | null;
  role: string | null;
}

function normalizePersistedSession(value: unknown): PersistedSession | null {
  if (!value || typeof value !== "object") return null;
  const input = value as Partial<PersistedSession>;
  const text = (key: keyof PersistedSession) =>
    typeof input[key] === "string" && input[key] ? (input[key] as string) : null;
  return {
    accessToken: text("accessToken"),
    accessTokenExpiresAt: text("accessTokenExpiresAt"),
    refreshToken: text("refreshToken"),
    sessionId: text("sessionId"),
    userId: text("userId"),
    username: text("username"),
    role: text("role"),
  };
}

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

function loadPersistedSession(): PersistedSession {
  let persisted: PersistedSession | null = null;
  try {
    persisted = normalizePersistedSession(
      JSON.parse(localStorage.getItem(SESSION_KEY) || "null"),
    );
  } catch {
    persisted = null;
  }
  if (!persisted) {
    persisted = {
      accessToken: null,
      accessTokenExpiresAt: null,
      refreshToken: loadRefreshToken(),
      sessionId: null,
      userId: null,
      username: null,
      role: null,
    };
  }
  if (!persisted.refreshToken) {
    persisted.refreshToken = loadRefreshToken();
  }
  return persisted;
}

function savePersistedSession(session: PersistedSession): void {
  try {
    localStorage.setItem(SESSION_KEY, JSON.stringify(session));
    if (session.refreshToken) {
      localStorage.setItem(REFRESH_TOKEN_KEY, session.refreshToken);
    }
  } catch {}
}

export function saveAuthenticatedSession(data: {
  accessToken: string;
  accessTokenExpiresAt?: string | null;
  refreshToken?: string | null;
  sessionId?: string | null;
  userId?: string | null;
  username?: string | null;
  role?: string | null;
}): void {
  const current = loadPersistedSession();
  savePersistedSession({
    accessToken: data.accessToken,
    accessTokenExpiresAt: data.accessTokenExpiresAt ?? null,
    refreshToken: data.refreshToken || current.refreshToken,
    sessionId: data.sessionId ?? current.sessionId,
    userId: data.userId ?? current.userId,
    username: data.username ?? current.username,
    role: data.role ?? current.role,
  });
}

export function saveCurrentUser(data: {
  userId?: string | number | null;
  username?: string | null;
  role?: string | null;
}): void {
  const current = loadPersistedSession();
  savePersistedSession({
    ...current,
    userId: data.userId == null ? current.userId : String(data.userId),
    username: data.username ?? current.username,
    role: data.role ?? current.role,
  });
}

export function clearPersistedSession(): void {
  try {
    localStorage.removeItem(SESSION_KEY);
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

function hasUsableAccessToken(session: PersistedSession): boolean {
  if (!session.accessToken || !session.accessTokenExpiresAt) return false;
  return Date.now() < new Date(session.accessTokenExpiresAt).getTime() - 30000;
}

async function withRefreshLock<T>(operation: () => Promise<T>): Promise<T> {
  if (!navigator.locks?.request) {
    return operation();
  }
  return navigator.locks.request(REFRESH_LOCK_NAME, operation);
}

function isFatalRefreshError(err: unknown): boolean {
  const status = (err as any)?.response?.status;
  const code = (err as any)?.code;
  const rawCode = (err as any)?.raw?.code;
  return status === 401 || code === 401 || rawCode === 401;
}

async function performRefresh(): Promise<string> {
  const { setAccessToken } = useSessionStore();
  if (refreshPromise) {
    return refreshPromise;
  }
  const epoch = sessionEpoch;
  refreshPromise = (async () => {
    try {
      return await withRefreshLock(async () => {
        if (epoch !== sessionEpoch) {
          throw new Error("SESSION_CHANGED");
        }
        const current = loadPersistedSession();
        if (hasUsableAccessToken(current)) {
          setAccessToken(current.accessToken!, current.accessTokenExpiresAt!);
          scheduleRefresh(current.accessTokenExpiresAt!);
          return current.accessToken!;
        }
        const storedRefreshToken = current.refreshToken;
        if (!storedRefreshToken) {
          throw new Error("NO_REFRESH_TOKEN");
        }
        const result = await refreshToken({ refreshToken: storedRefreshToken });
        if (epoch !== sessionEpoch || loadRefreshToken() !== storedRefreshToken) {
          throw new Error("SESSION_CHANGED");
        }
        saveAuthenticatedSession({
          accessToken: result.accessToken,
          accessTokenExpiresAt: result.accessTokenExpiresAt,
          refreshToken: result.refreshToken,
        });
        setAccessToken(result.accessToken, result.accessTokenExpiresAt);
        scheduleRefresh(result.accessTokenExpiresAt);
        return result.accessToken;
      });
    } catch (err) {
      if (isFatalRefreshError(err)) {
        sessionEpoch += 1;
        stopRefreshCoordinator();
        clearPersistedSession();
        useSessionStore().clearSession();
      }
      throw err;
    } finally {
      if (epoch === sessionEpoch) {
        refreshPromise = null;
      }
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
  sessionEpoch += 1;
  refreshPromise = null;
}

export function getAccessToken(): string | null {
  const { state } = useSessionStore();
  return state.value.accessToken;
}

export function forceCleanupSession(): void {
  const storedRefreshToken = loadRefreshToken();
  stopRefreshCoordinator();
  clearPersistedSession();
  useSessionStore().clearSession();
  revokeOnServer(storedRefreshToken);
}

export async function ensureValidToken(): Promise<string | null> {
  const { state, clearSession } = useSessionStore();
  const token = state.value.accessToken;
  const expiresAt = state.value.accessTokenExpiresAt;
  if (token && expiresAt) {
    const expiry = new Date(expiresAt).getTime();
    const now = Date.now();
    if (now < expiry - 30000) {
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
  const { setSession, clearSession } = useSessionStore();
  const persisted = loadPersistedSession();
  if (!persisted.accessToken && !persisted.refreshToken) {
    return false;
  }
  if (hasUsableAccessToken(persisted)) {
    setSession({
      accessToken: persisted.accessToken!,
      accessTokenExpiresAt: persisted.accessTokenExpiresAt!,
      sessionId: persisted.sessionId,
      userId: persisted.userId,
      username: persisted.username,
      role: persisted.role,
    });
    scheduleRefresh(persisted.accessTokenExpiresAt!);
    return true;
  }
  if (!persisted.refreshToken) {
    clearPersistedSession();
    clearSession();
    return false;
  }
  try {
    const accessToken = await performRefresh();
    const restored = loadPersistedSession();
    setSession({
      accessToken,
      accessTokenExpiresAt: restored.accessTokenExpiresAt,
      sessionId: restored.sessionId,
      userId: restored.userId,
      username: restored.username,
      role: restored.role,
    });
    return true;
  } catch (err) {
    if (isFatalRefreshError(err)) {
      clearPersistedSession();
      clearSession();
    }
    return false;
  }
}
