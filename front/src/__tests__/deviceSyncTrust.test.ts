import { flushPromises, shallowMount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), confirm: vi.fn(), prompt: vi.fn(), offer: vi.fn(), provision: vi.fn(), saveDeployment: vi.fn(), pairingStatus: vi.fn(), initialize: vi.fn(), state: {} as any, devices: [] as any[] }));
vi.mock("@/composables/useApi", () => ({ useApi: () => ({ get: mocks.get, post: mocks.post }) }));
vi.mock("element-plus", () => ({ ElMessage: { success: vi.fn(), warning: vi.fn(), error: vi.fn() }, ElMessageBox: { confirm: mocks.confirm, prompt: mocks.prompt } }));
vi.mock("@/composables/useDeviceManagementIntent", () => ({
  deviceManagementRequestConfig: () => ({}),
  useDeviceManagementIntent: () => ({
    release: vi.fn(), validate: vi.fn(),
    capture: async () => ({ controller: new AbortController(), state: mocks.state }),
  }),
}));
vi.mock("@/runtime/runtime-adapter", () => ({ getDeploymentConfig: async () => ({ mode: "local" }), getRuntimeConnection: async () => ({ apiBaseURL: "http://local.test" }), createCurrentDevicePairingOffer: mocks.offer, provisionCurrentDeviceMesh: mocks.provision, saveDeploymentConfig: mocks.saveDeployment, getCurrentDevicePairingStatus: mocks.pairingStatus }));
vi.mock("@/runtime/runtime-capabilities", () => ({ initializeRuntimeCapabilities: mocks.initialize }));
vi.mock("@/components/DevicePairScanner.vue", () => ({ default: { template: "<div />" } }));
vi.mock("@/components/DeviceCapabilityGrants.vue", () => ({ default: { template: "<div />" } }));
vi.mock("@/components/DeviceCoreCall.vue", () => ({ default: { template: "<div />" } }));
import DevicePairingPanel from "@/components/DevicePairingPanel.vue";
import BoundDevicesPanel from "@/components/BoundDevicesPanel.vue";

function mountBoundDevices() {
  return shallowMount(BoundDevicesPanel, { global: {
    stubs: { "el-button": true, "el-tag": true, "el-alert": true, "el-card": true, "el-empty": true },
    directives: { loading: () => {} },
  } });
}

function mountPairing() {
  return shallowMount(DevicePairingPanel, {
    props: { serverURL: "https://draft.core" },
    global: { stubs: { "el-button": true, "el-alert": true, "el-input": true } },
  });
}

describe("设备同步信任状态", () => {
  beforeEach(() => {
    window.amitiaDesktop = { getMeshIdentity: async () => ({ deviceId: "trusted" }), getMeshStatus: async () => ({}) } as any;
    mocks.devices = ["trusted", "revoked", "pending"].map(trustState => ({ deviceId: trustState, trustState, platform: "windows" }));
    mocks.state = { policy: { deviceId: "trusted" }, canAdminister: false };
    mocks.post.mockReset().mockImplementation(async () => { mocks.devices = mocks.devices.map(device => device.deviceId === "trusted" ? { ...device, trustState: "trusted" } : device); return { ok: true }; });
    mocks.confirm.mockReset().mockResolvedValue("confirm");
    mocks.offer.mockReset().mockResolvedValue({ qrPayload: "amitia://pair?offer=test", qrImage: "test-image" });
    mocks.prompt.mockReset().mockResolvedValue({ value: "manual-offer" });
    mocks.provision.mockReset().mockResolvedValue(undefined);
    mocks.saveDeployment.mockReset().mockResolvedValue(undefined);
    mocks.pairingStatus.mockReset().mockResolvedValue({ firstDeviceSetupRequired: false });
    mocks.initialize.mockReset().mockResolvedValue(undefined);
    mocks.get.mockReset().mockImplementation(async (path: string) => path.endsWith("/devices") ? { devices: mocks.devices } : { lastApplied: 3 });
  });
  it("仅查询可信设备，撤销和待配对设备显示明确状态", async () => {
    const wrapper = mountBoundDevices();
    await flushPromises();
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status")).map(call => call[1].deviceId)).toEqual(["trusted"]);
    expect((wrapper.vm as any).syncLabel("revoked")).toBe("已撤销，不再同步");
    expect((wrapper.vm as any).syncLabel("pending")).toBe("未受信任，暂不可同步");
    await (wrapper.vm as any).loadSync("revoked");
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status"))).toHaveLength(1);
    wrapper.unmount();
  });
  it("刷新后撤销状态覆盖已有同步结果", async () => {
    const wrapper = mountBoundDevices();
    await flushPromises();
    mocks.devices = [{ deviceId: "trusted", trustState: "revoked" }];
    await (wrapper.vm as any).refresh();
    expect((wrapper.vm as any).syncLabel("trusted")).toBe("已撤销，不再同步");
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status"))).toHaveLength(1);
    wrapper.unmount();
  });
  it("普通设备不查询其他可信设备的同步状态", async () => {
    mocks.devices.push({ deviceId: "other-device", trustState: "trusted" });
    const wrapper = mountBoundDevices();
    await flushPromises();
    await (wrapper.vm as any).loadSync("other-device");
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status")).map(call => call[1].deviceId)).toEqual(["trusted"]);
    wrapper.unmount();
  });
  it("本机所有者确认恢复后才生成配对二维码", async () => {
    mocks.state = { coreId: "core", coreConsoleDeviceId: "trusted", policy: {}, canAdminister: true };
    mocks.devices[0].trustState = "revoked";
    const wrapper = mountPairing();
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
    const wrapper = mountPairing();
    await flushPromises();
    await (wrapper.vm as any).generatePairingOffer();
    expect(mocks.post).not.toHaveBeenCalled();
    expect(mocks.offer).not.toHaveBeenCalled();
    wrapper.unmount();
  });
  it("手动配对使用云端表单地址，完成配对后才保存运行模式", async () => {
    const wrapper = mountPairing();
    await flushPromises();
    await (wrapper.vm as any).joinCurrentDevice();
    expect(mocks.pairingStatus).toHaveBeenCalledWith("https://draft.core");
    expect(mocks.provision).toHaveBeenCalledWith("https://draft.core", { offerToken: "manual-offer" });
    expect(mocks.saveDeployment).toHaveBeenCalledWith({ mode: "cloud", serverURL: "https://draft.core" });
    expect(mocks.provision.mock.invocationCallOrder[0]).toBeLessThan(mocks.saveDeployment.mock.invocationCallOrder[0]);
    expect(wrapper.emitted("paired")).toEqual([["https://draft.core"]]);
    wrapper.unmount();
  });
  it("扫描局域网二维码保留身份校验，成功后更新云端连接地址", async () => {
    const wrapper = mountPairing();
    await flushPromises();
    const fingerprint = "a".repeat(64);
    await (wrapper.vm as any).joinCurrentDevice(`amitia://pair?endpoint=${encodeURIComponent("https://scanned.core")}&offer=scan-offer&core=scan-core&fingerprint=${fingerprint}`);
    expect(mocks.confirm).toHaveBeenCalledOnce();
    expect(mocks.provision).toHaveBeenCalledWith("https://scanned.core", { offerToken: "scan-offer", coreId: "scan-core", fingerprint });
    expect(mocks.saveDeployment).toHaveBeenCalledWith({ mode: "cloud", serverURL: "https://scanned.core" });
    wrapper.unmount();
  });
  it("配对失败时不保存运行模式，不发布完成事件", async () => {
    mocks.provision.mockRejectedValueOnce(new Error("配对失败"));
    const wrapper = mountPairing();
    await flushPromises();
    await (wrapper.vm as any).joinCurrentDevice();
    expect(mocks.saveDeployment).not.toHaveBeenCalled();
    expect(wrapper.emitted("paired")).toBeUndefined();
    wrapper.unmount();
  });
});
