import { flushPromises, shallowMount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), confirm: vi.fn(), offer: vi.fn(), state: {} as any, devices: [] as any[] }));
vi.mock("@/composables/useApi", () => ({ useApi: () => ({ get: mocks.get, post: mocks.post }) }));
vi.mock("element-plus", () => ({ ElMessage: { success: vi.fn(), warning: vi.fn(), error: vi.fn() }, ElMessageBox: { confirm: mocks.confirm } }));
vi.mock("@/composables/useDeviceManagementIntent", () => ({
  deviceManagementRequestConfig: () => ({}),
  useDeviceManagementIntent: () => ({
    release: vi.fn(), validate: vi.fn(),
    capture: async () => ({ controller: new AbortController(), state: mocks.state }),
  }),
}));
vi.mock("@/runtime/runtime-adapter", () => ({ getDeploymentConfig: async () => ({ mode: "local" }), getRuntimeConnection: async () => ({ apiBaseURL: "http://local.test" }), createCurrentDevicePairingOffer: mocks.offer }));
vi.mock("@/components/DevicePairScanner.vue", () => ({ default: { template: "<div />" } }));
vi.mock("@/components/DeviceCapabilityGrants.vue", () => ({ default: { template: "<div />" } }));
vi.mock("@/components/DeviceCoreCall.vue", () => ({ default: { template: "<div />" } }));
import DevicesView from "@/views/devices/DevicesView.vue";

function mountDevices() {
  return shallowMount(DevicesView, { global: {
    stubs: { "el-button": true, "el-tag": true, "el-alert": true, "el-card": true, "el-input": true, "el-switch": true, "el-empty": true },
    directives: { loading: () => {} },
  } });
}

describe("设备同步信任状态", () => {
  beforeEach(() => {
    window.amitiaDesktop = { getMeshIdentity: async () => ({ deviceId: "trusted" }), getMeshStatus: async () => ({}) } as any;
    mocks.devices = ["trusted", "revoked", "pending"].map(trustState => ({ deviceId: trustState, trustState, platform: "windows" }));
    mocks.state = { policy: { deviceId: "trusted" }, canAdminister: false };
    mocks.post.mockReset().mockImplementation(async () => { mocks.devices = mocks.devices.map(device => device.deviceId === "trusted" ? { ...device, trustState: "trusted" } : device); return { ok: true }; });
    mocks.confirm.mockReset().mockResolvedValue("confirm");
    mocks.offer.mockReset().mockResolvedValue({ qrPayload: "amitia://pair?offer=test", qrImage: "test-image" });
    mocks.get.mockReset().mockImplementation(async (path: string) => path.endsWith("/devices") ? { devices: mocks.devices } : { lastApplied: 3 });
  });
  it("仅查询可信设备，撤销和待配对设备显示明确状态", async () => {
    const wrapper = mountDevices();
    await flushPromises();
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status")).map(call => call[1].deviceId)).toEqual(["trusted"]);
    expect((wrapper.vm as any).syncLabel("revoked")).toBe("已撤销，不再同步");
    expect((wrapper.vm as any).syncLabel("pending")).toBe("未受信任，暂不可同步");
    await (wrapper.vm as any).loadSync("revoked");
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status"))).toHaveLength(1);
    wrapper.unmount();
  });
  it("刷新后撤销状态覆盖已有同步结果", async () => {
    const wrapper = mountDevices();
    await flushPromises();
    mocks.devices = [{ deviceId: "trusted", trustState: "revoked" }];
    await (wrapper.vm as any).refresh();
    expect((wrapper.vm as any).syncLabel("trusted")).toBe("已撤销，不再同步");
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status"))).toHaveLength(1);
    wrapper.unmount();
  });
  it("普通设备不查询其他可信设备的同步状态", async () => {
    mocks.devices.push({ deviceId: "other-device", trustState: "trusted" });
    const wrapper = mountDevices();
    await flushPromises();
    await (wrapper.vm as any).loadSync("other-device");
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status")).map(call => call[1].deviceId)).toEqual(["trusted"]);
    wrapper.unmount();
  });
  it("本机所有者确认恢复后才生成配对二维码", async () => {
    mocks.state = { coreId: "core", coreConsoleDeviceId: "trusted", policy: {}, canAdminister: true };
    mocks.devices[0].trustState = "revoked";
    const wrapper = mountDevices();
    await flushPromises();
    await (wrapper.vm as any).generatePairingOffer();
    expect(mocks.confirm).toHaveBeenCalledOnce();
    expect(mocks.post).toHaveBeenCalledWith("/api/device-mesh/v1/pairing/recover-local-device", { coreId: "core", deviceId: "trusted" }, {});
    expect(mocks.offer).toHaveBeenCalledOnce();
    expect(mocks.post.mock.invocationCallOrder[0]).toBeLessThan(mocks.offer.mock.invocationCallOrder[0]);
    expect(mocks.devices.find(device => device.deviceId === "revoked")?.trustState).toBe("revoked");
    wrapper.unmount();
  });
  it("取消恢复不会修改信任或生成二维码", async () => {
    mocks.state = { coreId: "core", coreConsoleDeviceId: "trusted", policy: {}, canAdminister: true };
    mocks.devices[0].trustState = "revoked";
    mocks.confirm.mockRejectedValue("cancel");
    const wrapper = mountDevices();
    await flushPromises();
    await (wrapper.vm as any).generatePairingOffer();
    expect(mocks.post).not.toHaveBeenCalled();
    expect(mocks.offer).not.toHaveBeenCalled();
    wrapper.unmount();
  });
});
