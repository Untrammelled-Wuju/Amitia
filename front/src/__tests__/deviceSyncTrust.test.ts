import { flushPromises, shallowMount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ get: vi.fn(), devices: [] as any[] }));
vi.mock("@/composables/useApi", () => ({ useApi: () => ({ get: mocks.get }) }));
vi.mock("@/composables/useDeviceManagementIntent", () => ({
  deviceManagementRequestConfig: () => ({}),
  useDeviceManagementIntent: () => ({
    release: vi.fn(), validate: vi.fn(),
    capture: async () => ({ controller: new AbortController(), state: { policy: {}, canAdminister: false } }),
  }),
}));
vi.mock("@/runtime/runtime-adapter", () => ({ getDeploymentConfig: async () => ({ mode: "local" }) }));
vi.mock("@/components/DevicePairScanner.vue", () => ({ default: { template: "<div />" } }));
vi.mock("@/components/DeviceCapabilityGrants.vue", () => ({ default: { template: "<div />" } }));
vi.mock("@/components/DeviceCoreCall.vue", () => ({ default: { template: "<div />" } }));
import DevicesView from "@/views/devices/DevicesView.vue";

describe("设备同步信任状态", () => {
  beforeEach(() => {
    window.amitiaDesktop = { getMeshIdentity: async () => ({ deviceId: "trusted" }), getMeshStatus: async () => ({}) } as any;
    mocks.devices = ["trusted", "revoked", "pending"].map(trustState => ({ deviceId: trustState, trustState, platform: "windows" }));
    mocks.get.mockReset().mockImplementation(async (path: string) => path.endsWith("/devices") ? { devices: mocks.devices } : { lastApplied: 3 });
  });
  it("仅查询可信设备，撤销和待配对设备显示明确状态", async () => {
    const wrapper = shallowMount(DevicesView);
    await flushPromises();
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status")).map(call => call[1].deviceId)).toEqual(["trusted"]);
    expect((wrapper.vm as any).syncLabel("revoked")).toBe("已撤销，不再同步");
    expect((wrapper.vm as any).syncLabel("pending")).toBe("未受信任，暂不可同步");
    await (wrapper.vm as any).loadSync("revoked");
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status"))).toHaveLength(1);
    wrapper.unmount();
  });
  it("刷新后撤销状态覆盖已有同步结果", async () => {
    const wrapper = shallowMount(DevicesView);
    await flushPromises();
    mocks.devices = [{ deviceId: "trusted", trustState: "revoked" }];
    await (wrapper.vm as any).refresh();
    expect((wrapper.vm as any).syncLabel("trusted")).toBe("已撤销，不再同步");
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status"))).toHaveLength(1);
    wrapper.unmount();
  });
});
