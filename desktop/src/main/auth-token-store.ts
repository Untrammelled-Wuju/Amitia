// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { safeStorage } from "electron";

const ACCESS_TOKEN_KEY = "amitia-access-token";
const REFRESH_TOKEN_KEY = "amitia-refresh-token";
const SESSION_META_KEY = "amitia-session-meta";

export interface AccountSessionMeta {
  sessionId: string | null;
  userId: string | null;
  username: string | null;
  role: string | null;
  accessTokenExpiresAt: string | null;
}

let memoryAccessToken: string | null = null;

export function getAccessToken(): string | null {
  if (memoryAccessToken) return memoryAccessToken;
  try {
    const raw = localStorage.getItem(ACCESS_TOKEN_KEY);
    if (raw) {
      memoryAccessToken = raw;
      return raw;
    }
  } catch {}
  return null;
}

export function setAccessToken(token: string | null): void {
  memoryAccessToken = token;
  try {
    if (token) {
      localStorage.setItem(ACCESS_TOKEN_KEY, token);
    } else {
      localStorage.removeItem(ACCESS_TOKEN_KEY);
    }
  } catch {}
}

export function getRefreshToken(): string | null {
  try {
    if (safeStorage.isEncryptionEnabled()) {
      const encrypted = localStorage.getItem(REFRESH_TOKEN_KEY);
      if (encrypted) {
        return safeStorage.decryptString(Buffer.from(encrypted, "base64"));
      }
    }
  } catch {}
  try {
    return localStorage.getItem(REFRESH_TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setRefreshToken(token: string | null): void {
  try {
    if (!token) {
      localStorage.removeItem(REFRESH_TOKEN_KEY);
      return;
    }
    if (safeStorage.isEncryptionEnabled()) {
      const encrypted = safeStorage.encryptString(token);
      localStorage.setItem(REFRESH_TOKEN_KEY, encrypted.toString("base64"));
    } else {
      localStorage.setItem(REFRESH_TOKEN_KEY, token);
    }
  } catch {}
}

export function getSessionMeta(): AccountSessionMeta {
  try {
    const raw = localStorage.getItem(SESSION_META_KEY);
    if (raw) {
      return JSON.parse(raw) as AccountSessionMeta;
    }
  } catch {}
  return { sessionId: null, userId: null, username: null, role: null, accessTokenExpiresAt: null };
}

export function setSessionMeta(meta: AccountSessionMeta): void {
  try {
    localStorage.setItem(SESSION_META_KEY, JSON.stringify(meta));
  } catch {}
}

export function setAuthToken(token: string | null): void {
  setAccessToken(token);
}

export function hasAuthToken(): boolean {
  return !!getAccessToken();
}

export function clearSession(): void {
  memoryAccessToken = null;
  try {
    localStorage.removeItem(ACCESS_TOKEN_KEY);
    localStorage.removeItem(REFRESH_TOKEN_KEY);
    localStorage.removeItem(SESSION_META_KEY);
    const legacyKey = "ai-companion-token";
    localStorage.removeItem(legacyKey);
  } catch {}
}

export function setFullSession(accessToken: string, refreshToken: string | null, meta: AccountSessionMeta | null): void {
  setAccessToken(accessToken);
  if (refreshToken) {
    setRefreshToken(refreshToken);
  }
  if (meta) {
    setSessionMeta(meta);
  }
}

