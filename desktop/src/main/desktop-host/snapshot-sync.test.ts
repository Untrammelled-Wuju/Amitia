import { expect, it, vi } from "vitest";
import type { BrowserWindow } from "electron";
import type { BusinessCoreClient } from "../business-core-client";
import type { DesktopHostManager } from "./desktop-host";
import { DesktopSnapshotSync } from "./snapshot-sync";

it("loads once and coalesces event-driven sync requests without a polling timer", async () => {
  let generation = 0;
  const fetch = vi.fn((path: string) => {
    if (path.endsWith("/snapshot/build")) {
      generation += 1;
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ generation, hash: String(generation) }) });
    }
    return Promise.resolve({ ok: true, status: 200 });
  });
  const applySnapshot = vi.fn(() => true);
  const mainWindow = {
    isDestroyed: () => false,
    webContents: { isLoading: () => false, once: vi.fn() },
  } as unknown as BrowserWindow;
  const host = { getCurrentSnapshot: () => null, applySnapshot } as unknown as DesktopHostManager;
  const businessCore = { fetch } as unknown as BusinessCoreClient;
  const sync = new DesktopSnapshotSync(
    mainWindow,
    host,
    businessCore,
  );
  const setInterval = vi.spyOn(globalThis, "setInterval");

  sync.start();
  await vi.waitFor(() => expect(applySnapshot).toHaveBeenCalledTimes(1));
  expect(setInterval).not.toHaveBeenCalled();

  const coalescedFetch = vi.fn(fetch.getMockImplementation());
  const coalescedApply = vi.fn(() => true);
  const coalescingSync = new DesktopSnapshotSync(
    mainWindow,
    { getCurrentSnapshot: () => null, applySnapshot: coalescedApply } as unknown as DesktopHostManager,
    { fetch: coalescedFetch } as unknown as BusinessCoreClient,
  );
  await Promise.all([coalescingSync.syncNow(), coalescingSync.syncNow()]);

  expect(coalescedFetch.mock.calls.map(([path]) => path)).toEqual([
    "/api/extensions/desktop/snapshot/build",
    "/api/extensions/desktop/snapshot/apply-result",
    "/api/extensions/desktop/snapshot/build",
    "/api/extensions/desktop/snapshot/apply-result",
  ]);
  expect(coalescedApply).toHaveBeenCalledTimes(2);
  setInterval.mockRestore();
});
