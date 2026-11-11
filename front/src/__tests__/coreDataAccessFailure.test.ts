import { describe, expect, it, vi } from "vitest";
import { defineComponent } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import { useCoreDataAccess } from "../composables/useCoreDataAccess";

const mocks = vi.hoisted(() => ({ get: vi.fn(), deployment: vi.fn() }));
vi.mock("../composables/useApi", () => ({ apiClient: { get: mocks.get } }));
vi.mock("../runtime/runtime-adapter", () => ({ getDeploymentConfig: mocks.deployment }));

describe("数据权限检查错误", () => {
  it("禁止写入并保留和传播原始错误", async () => {
    const original = new Error("数据权限服务不可用", { cause: new Error("底层连接失败") });
    mocks.deployment.mockResolvedValue({ mode: "cloud" });
    mocks.get.mockRejectedValue(original);
    let access!: ReturnType<typeof useCoreDataAccess>;
    const wrapper = mount(defineComponent({ setup() { access = useCoreDataAccess(); return () => null; } }));
    await flushPromises();
    expect(access.access.value.write).toBe(false);
    expect(access.error.value).toContain("数据权限服务不可用");
    expect(access.error.value).toContain("底层连接失败");
    await expect(access.requireWrite()).rejects.toBe(original);
    wrapper.unmount();
  });

  it("过期的权限响应不能覆盖较新的拒绝结果", async () => {
    mocks.deployment.mockResolvedValue({ mode: "cloud" });
    const writable = { coreId: "core-a", coordinationAvailable: true, policy: { permissionRevision: 1 }, coreDataAccess: { read: true, write: true, reason: "" } };
    const readOnly = { ...writable, coreDataAccess: { read: true, write: false, reason: "无写入权限" } };
    mocks.get.mockResolvedValue({ data: { data: writable } });
    let access!: ReturnType<typeof useCoreDataAccess>;
    const wrapper = mount(defineComponent({ setup() { access = useCoreDataAccess(); return () => null; } }));
    await flushPromises();
    let resolveOld!: (value: unknown) => void;
    mocks.get.mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve; }));
    const old = access.refresh();
    await flushPromises();
    mocks.get.mockResolvedValueOnce({ data: { data: readOnly } });
    await access.refresh();
    resolveOld({ data: { data: writable } });
    await expect(old).rejects.toThrow("权限检查已更新");
    expect(access.access.value.write).toBe(false);
    expect(access.access.value.reason).toBe("无写入权限");
    wrapper.unmount();
  });
});
