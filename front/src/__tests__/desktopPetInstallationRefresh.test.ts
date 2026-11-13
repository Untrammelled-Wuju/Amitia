import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDesktopPetInstallations } from "@/composables/useDesktopPetInstallations";

const { get } = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock("@/composables/useApi", () => ({ useApi: () => ({ get, post: vi.fn() }) }));

const pending = { items: [{ id: "pet", status: "uninstalling" }], total: 1 };
const complete = { items: [{ id: "pet", status: "uninstalled" }], total: 1 };

describe("桌宠卸载状态后台刷新", () => {
  beforeEach(() => get.mockReset());

  it("后台查询失败时保留已有状态和完整错误，下一次成功可更新状态", async () => {
    const failure = new Error("SQLite read timeout", { cause: new Error("database is locked") });
    get.mockResolvedValueOnce(pending).mockRejectedValueOnce(failure).mockResolvedValueOnce(complete);
    const view = useDesktopPetInstallations();
    const { signal } = new AbortController();

    await view.listInstallations();
    await expect(view.listInstallations(signal)).rejects.toBe(failure);
    expect(view.installations.value).toEqual(pending.items);
    expect(view.loading.value).toBe(false);
    await view.listInstallations(signal);
    expect(view.installations.value).toEqual(complete.items);
  });

  it("较旧响应和离开页面后抵达的响应不会覆盖最新状态", async () => {
    let finish!: (result: typeof pending) => void;
    get.mockImplementationOnce(() => new Promise(resolve => { finish = resolve; })).mockResolvedValueOnce(complete);
    const view = useDesktopPetInstallations();
    const controller = new AbortController();

    const older = view.listInstallations(controller.signal);
    await view.listInstallations();
    finish(pending);
    await older;
    expect(view.installations.value).toEqual(complete.items);

    get.mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
    const cancelled = view.listInstallations(controller.signal);
    controller.abort();
    finish(pending);
    await cancelled;
    expect(get).toHaveBeenLastCalledWith("/api/desktop-pets/installations", undefined, { signal: controller.signal });
    expect(view.installations.value).toEqual(complete.items);
  });
});
