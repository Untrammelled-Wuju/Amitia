import { beforeEach, describe, expect, it, vi } from "vitest";

const refreshToken = vi.fn();

vi.mock("../stores/session-client", () => ({
  refreshToken: (...args: unknown[]) => refreshToken(...args),
}));

import {
  ensureValidToken,
  restoreSessionOnStartup,
  stopRefreshCoordinator,
} from "../stores/refresh-coordinator";
import { useSessionStore } from "../stores/session-store";

const SESSION_KEY = "amitia-session-persist";

function setSession(session: Record<string, unknown>) {
  localStorage.setItem(SESSION_KEY, JSON.stringify(session));
}

describe("refresh coordinator", () => {
  beforeEach(() => {
    localStorage.clear();
    refreshToken.mockReset();
    stopRefreshCoordinator();
    useSessionStore().clearSession();
  });

  it("restores an unexpired session without rotating the refresh token", async () => {
    setSession({
      accessToken: "access",
      accessTokenExpiresAt: new Date(Date.now() + 10 * 60 * 1000).toISOString(),
      refreshToken: "amt_rt_valid",
      sessionId: "sess_1",
      userId: "1",
      username: "admin",
      role: "admin",
    });

    await expect(restoreSessionOnStartup()).resolves.toBe(true);
    expect(refreshToken).not.toHaveBeenCalled();
    expect(JSON.parse(localStorage.getItem(SESSION_KEY)!).refreshToken).toBe("amt_rt_valid");
  });

  it("rotates an expired refresh token exactly once", async () => {
    setSession({
      accessToken: "expired",
      accessTokenExpiresAt: new Date(Date.now() - 1000).toISOString(),
      refreshToken: "amt_rt_old",
    });
    refreshToken.mockImplementation(async () => ({
      accessToken: "renewed",
      accessTokenExpiresAt: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
      refreshToken: "amt_rt_new",
      refreshTokenExpiresAt: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(),
    }));

    const [first, second] = await Promise.all([
      ensureValidToken(),
      ensureValidToken(),
    ]);

    expect(first).toBe("renewed");
    expect(second).toBe("renewed");
    expect(refreshToken).toHaveBeenCalledTimes(1);
    expect(localStorage.getItem("amitia-refresh-token-persist")).toBe("amt_rt_new");
  });
});
