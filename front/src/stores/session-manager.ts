// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { useSessionStore } from "./session-store";
import { loginUser, logoutUser, logoutAll } from "./session-client";
import { initRefreshCoordinator, stopRefreshCoordinator } from "./refresh-coordinator";

export function useSessionManager() {
  const { state, isAuthenticated, setSession, clearSession } = useSessionStore();

  async function login(username: string, password: string) {
    const result = await loginUser({ username, password });
    setSession({
      accessToken: result.accessToken,
      accessTokenExpiresAt: result.accessTokenExpiresAt,
      sessionId: result.session?.sessionId,
      userId: result.user?.id,
      username: result.user?.username,
      role: result.user?.role,
    });
    if (result.accessTokenExpiresAt) {
      initRefreshCoordinator(result.accessTokenExpiresAt);
    }
    return result;
  }

  async function logout() {
    try {
      await logoutUser();
    } catch {
    } finally {
      stopRefreshCoordinator();
      clearSession();
    }
  }

  async function logoutEverywhere() {
    try {
      await logoutAll();
    } catch {
    } finally {
      stopRefreshCoordinator();
      clearSession();
    }
  }

  return {
    state,
    isAuthenticated,
    login,
    logout,
    logoutEverywhere,
  };
}
