// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { safeStorage } from "electron";

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
let memorySessionMeta: AccountSessionMeta | null = null;

let safeStorageAvailable: boolean | null = null;

function isSafeStorageAvailable(): boolean {
  if (safeStorageAvailable !== null) return safeStorageAvailable as boolean;
  try {
    safeStorageAvailable = (safeStorage as any).isEncryptionEnabled?.() ?? false;
  } catch {
    safeStorageAvailable = false;
  }
  return safeStorageAvailable as boolean;
}

export function getAccessToken(): string | null {
  return memoryAccessToken;
}

export function setAccessToken(token: string | null): void {
  memoryAccessToken = token;
}

export function getRefreshToken(): string | null {
  if (!isSafeStorageAvailable()) {
    return null;
  }
  try {
    const encrypted = localStorage.getItem(REFRESH_TOKEN_KEY);
    if (encrypted) {
      return safeStorage.decryptString(Buffer.from(encrypted, "base64"));
    }
  } catch {}
  return null;
}

export function setRefreshToken(token: string | null): void {
  if (!token) {
    try { localStorage.removeItem(REFRESH_TOKEN_KEY); } catch {}
    return;
  }
  if (!isSafeStorageAvailable()) {
    throw new Error("SAFE_STORAGE_UNAVAILABLE");
  }
  try {
    const encrypted = safeStorage.encryptString(token);
    localStorage.setItem(REFRESH_TOKEN_KEY, encrypted.toString("base64"));
  } catch {
    throw new Error("SAFE_STORAGE_ENCRYPT_FAILED");
  }
}

export function getSessionMeta(): AccountSessionMeta {
  if (memorySessionMeta) return memorySessionMeta;
  if (!isSafeStorageAvailable()) {
    return { sessionId: null, userId: null, username: null, role: null, accessTokenExpiresAt: null };
  }
  try {
    const raw = localStorage.getItem(SESSION_META_KEY);
    if (raw) {
      const meta = JSON.parse(raw) as AccountSessionMeta;
      memorySessionMeta = meta;
      return meta;
    }
  } catch {}
  return { sessionId: null, userId: null, username: null, role: null, accessTokenExpiresAt: null };
}

export function setSessionMeta(meta: AccountSessionMeta): void {
  memorySessionMeta = meta;
  if (!isSafeStorageAvailable()) {
    return;
  }
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
  memorySessionMeta = null;
  try {
    localStorage.removeItem(REFRESH_TOKEN_KEY);
    localStorage.removeItem(SESSION_META_KEY);
    localStorage.removeItem("ai-companion-token");
    localStorage.removeItem("amitia-access-token");
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
