// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, computed } from "vue";
import { revokeRefreshToken } from "./session-client";

export interface SessionState {
  accessToken: string | null;
  accessTokenExpiresAt: string | null;
  sessionId: string | null;
  userId: string | null;
  username: string | null;
  role: string | null;
}

const state = ref<SessionState>({
  accessToken: null,
  accessTokenExpiresAt: null,
  sessionId: null,
  userId: null,
  username: null,
  role: null,
});

const TOKEN_KEY = "ai-companion-token";
let revoking = false;

export function getStoredToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setStoredToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearStoredToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}

export function revokeOnServer(): void {
  if (revoking) {
    return;
  }
  revoking = true
  revokeRefreshToken().finally(() => {
    revoking = false
  });
}

export function useSessionStore() {
  const isAuthenticated = computed(() => !!state.value.accessToken);

  function setSession(data: {
    accessToken: string;
    accessTokenExpiresAt?: string;
    sessionId?: string;
    userId?: string;
    username?: string;
    role?: string;
    refreshToken?: string;
    clientType?: string;
  }) {
    state.value = {
      accessToken: data.accessToken,
      accessTokenExpiresAt: data.accessTokenExpiresAt ?? null,
      sessionId: data.sessionId ?? null,
      userId: data.userId ?? null,
      username: data.username ?? null,
      role: data.role ?? null,
    };
    if (data.refreshToken && data.clientType !== "web") {
      setStoredToken(data.refreshToken);
    }
  }

  function setAccessToken(token: string, expiresAt?: string) {
    state.value.accessToken = token;
    state.value.accessTokenExpiresAt = expiresAt ?? null;
  }

  function clearSession() {
    state.value = {
      accessToken: null,
      accessTokenExpiresAt: null,
      sessionId: null,
      userId: null,
      username: null,
      role: null,
    };
    clearStoredToken();
    revokeOnServer();
  }

  return {
    state,
    isAuthenticated,
    setSession,
    setAccessToken,
    clearSession,
  };
}
