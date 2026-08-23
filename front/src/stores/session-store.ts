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

let revoking = false;

export function revokeOnServer(refreshToken?: string | null): void {
  if (revoking) {
    return;
  }
  revoking = true;
  revokeRefreshToken(refreshToken).finally(() => {
    revoking = false;
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
  }) {
    state.value = {
      accessToken: data.accessToken,
      accessTokenExpiresAt: data.accessTokenExpiresAt ?? null,
      sessionId: data.sessionId ?? null,
      userId: data.userId ?? null,
      username: data.username ?? null,
      role: data.role ?? null,
    };
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
  }

  return {
    state,
    isAuthenticated,
    setSession,
    setAccessToken,
    clearSession,
  };
}
