import { afterEach, expect, it, vi } from "vitest";
import { loadLastKnownGoodSnapshot, saveLastKnownGoodSnapshot } from "@/ui-runtime/snapshotCache";
import type { UIContributionSnapshot } from "@/stores/extensionUI";

afterEach(() => { vi.unstubAllGlobals(); localStorage.clear(); });

it("保存新快照后只清理同设备的本机端口快照，保留其他设备和服务", async () => {
  const values = new Map<string, unknown>();
  const db = {
    close: vi.fn(),
    transaction() {
      const tx: Record<string, any> = {};
      tx.objectStore = () => ({
        put(value: unknown, key: string) { values.set(key, value); queueMicrotask(() => tx.oncomplete()); },
        get(key: string) { const request = { result: values.get(key) }; queueMicrotask(() => tx.oncomplete()); return request; },
      });
      return tx;
    },
  };
  vi.stubGlobal("indexedDB", { open() { const request: Record<string, any> = { result: db }; queueMicrotask(() => request.onsuccess()); return request; } });
  vi.stubGlobal("amitiaDesktop", { writeClipboardText: vi.fn() });
  const makeKey = (namespace: string, device: string) => `amitia.ui.snapshot.lkg.v1:${encodeURIComponent(namespace)}:macos:${device}`;
  const stale = makeKey("http://127.0.0.1:1234|device=device-a", "device-a");
  const other = makeKey("http://127.0.0.1:1234|device=device-b", "device-b");
  const cloud = makeKey("https://example.com|device=device-a", "device-a");
  for (const key of [stale, other, cloud]) localStorage.setItem(key, "cached");
  const snapshot = { generatedAt: "now", slots: [] } as unknown as UIContributionSnapshot;
  await saveLastKnownGoodSnapshot("macos", "device-a", snapshot, "desktop-local:device-a");
  expect(await loadLastKnownGoodSnapshot("macos", "device-a", "desktop-local:device-a")).toEqual(snapshot);
  expect(localStorage.getItem(stale)).toBeNull();
  expect(localStorage.getItem(other)).toBe("cached");
  expect(localStorage.getItem(cloud)).toBe("cached");
});

it("新快照保存失败时保留原有缓存并传播原始错误", async () => {
  const failure = new Error("indexeddb failed");
  vi.stubGlobal("indexedDB", { open() { const request: Record<string, any> = { error: failure }; queueMicrotask(() => request.onerror()); return request; } });
  vi.stubGlobal("amitiaDesktop", {});
  const existing = `amitia.ui.snapshot.lkg.v1:${encodeURIComponent("http://127.0.0.1:1234|device=device-a")}:macos:device-a`;
  localStorage.setItem(existing, "cached");
  await expect(saveLastKnownGoodSnapshot("macos", "device-a", {} as UIContributionSnapshot, "desktop-local:device-a")).rejects.toBe(failure);
  expect(localStorage.getItem(existing)).toBe("cached");
});
