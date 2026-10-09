import { afterEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { defineComponent } from "vue";
import { deviceManagementFingerprint, deviceManagementRequestConfig, useDeviceManagementIntent } from "../composables/useDeviceManagementIntent";

const mocks = vi.hoisted(() => ({ get: vi.fn(), connection: vi.fn() }));
vi.mock("../composables/useApi", () => ({ useApi: () => ({ get: mocks.get }) }));
vi.mock("../runtime/runtime-adapter", () => ({ getRuntimeConnection: mocks.connection }));
const state = () => ({ coreId: "core-b", canAdminister: false, coordinationAvailable: true,
  policy: { deviceId: "phone-a", coordinated: false, administrator: false, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } });
afterEach(() => vi.resetAllMocks());

describe("原设备管理权限", () => {
  it("普通设备自授权仍携带原Core和三种权限版本", () => {
    const controller = new AbortController();
    const intent = { state: state(), apiBaseURL: "https://core-b", controller };
    expect(deviceManagementRequestConfig(intent).headers).toEqual({
      "X-Amitia-Expected-Core-ID": "core-b", "X-Amitia-Expected-Configuration-Policy": "1:1:1",
    });
    controller.abort();
    expect(() => deviceManagementRequestConfig(intent)).toThrow();
    expect(() => deviceManagementFingerprint({ ...state(), policy: { ...state().policy, permissionRevision: 0 } })).toThrow();
  });
  it.each(["providerEpoch", "modeRevision", "permissionRevision"])("拒绝%s变更后旧页面操作", async (field) => {
    let current = state();
    mocks.get.mockImplementation(async () => structuredClone(current));
    mocks.connection.mockResolvedValue({ apiBaseURL: "https://core-b" });
    let management!: ReturnType<typeof useDeviceManagementIntent>;
    const wrapper = mount(defineComponent({ setup() { management = useDeviceManagementIntent(); return () => null; } }));
    const intent = await management.capture();
    current = { ...current, policy: { ...current.policy, [field]: 3 } };
    await expect(management.validate(intent)).rejects.toThrow();
    expect(intent.controller.signal.aborted).toBe(true);
    wrapper.unmount();
  });
  it("自授权只接受精确增量确认并在Core切换时取消", async () => {
    let current = state();
    mocks.get.mockImplementation(async () => structuredClone(current));
    mocks.connection.mockResolvedValue({ apiBaseURL: "https://core-b" });
    let management!: ReturnType<typeof useDeviceManagementIntent>;
    const wrapper = mount(defineComponent({ setup() { management = useDeviceManagementIntent(); return () => null; } }));
    const intent = await management.capture();
    current.policy.permissionRevision = 2;
    await expect(management.validate(intent, 1)).resolves.toEqual(current);
    window.dispatchEvent(new Event("amitia:runtime-connection-changed"));
    expect(intent.controller.signal.aborted).toBe(true);
    await expect(management.validate(intent, 1)).rejects.toThrow();
    wrapper.unmount();
  });
});
