import { afterEach, describe, expect, it, vi } from "vitest";
import { CharacterWatcher } from "../character-watcher";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("CharacterWatcher", () => {
  it("authenticates and reconciles the active character on the first observation", async () => {
    const fetchMock = vi.fn(async (_url: string, init?: RequestInit) => {
      const headers = new Headers(init?.headers);
      expect(headers.get("X-Amitia-Desktop-Session")).toBe("session-1");
      return jsonResponse({ code: 200, data: { characterId: "character-a" } });
    });
    vi.stubGlobal("fetch", fetchMock);

    const onChanged = vi.fn(async () => undefined);
    const watcher = new CharacterWatcher({
      coreBaseURL: "http://127.0.0.1:18899/",
      pollIntervalMs: 60_000,
      authHeadersProvider: async () => ({
        "X-Amitia-Desktop-Session": "session-1",
      }),
      onActiveCharacterChanged: onChanged,
    });

    await watcher.start();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://127.0.0.1:18899/api/companion/role-profile",
    );
    expect(onChanged).toHaveBeenCalledTimes(1);
    expect(onChanged).toHaveBeenCalledWith("character-a");
    watcher.stop();
  });

  it("keeps retrying when initial reconciliation fails and only commits after success", async () => {
    vi.useFakeTimers();
    vi.spyOn(console, "warn").mockImplementation(() => undefined);
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse({ code: 200, data: { characterId: "character-b" } }),
      ),
    );

    const onChanged = vi
      .fn<(_: string) => Promise<void>>()
      .mockRejectedValueOnce(new Error("switch failed"))
      .mockResolvedValue(undefined);
    const watcher = new CharacterWatcher({
      pollIntervalMs: 100,
      requestTimeoutMs: 1_000,
      onActiveCharacterChanged: onChanged,
    });

    await watcher.start();
    expect(onChanged).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(100);
    expect(onChanged).toHaveBeenCalledTimes(2);
    expect(onChanged).toHaveBeenLastCalledWith("character-b");

    await vi.advanceTimersByTimeAsync(100);
    expect(onChanged).toHaveBeenCalledTimes(2);
    watcher.stop();
  });
});
