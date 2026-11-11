import { flushPromises, shallowMount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ElMessage } from "element-plus";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), confirm: vi.fn(), prompt: vi.fn(), offer: vi.fn(), provision: vi.fn(), saveDeployment: vi.fn(), pairingStatus: vi.fn(), initialize: vi.fn(), meshIdentity: vi.fn(), meshStatus: vi.fn(), invalidate: undefined as (() => void) | undefined, lanChanged: undefined as (() => void) | undefined, lanFailed: undefined as ((error: unknown) => void) | undefined, state: {} as any, devices: [] as any[] }));
vi.mock("@/composables/useApi", () => ({ useApi: () => ({ get: mocks.get, post: mocks.post, put: mocks.put }) }));
vi.mock("element-plus", () => ({ ElMessage: { success: vi.fn(), warning: vi.fn(), error: vi.fn() }, ElMessageBox: { confirm: mocks.confirm, prompt: mocks.prompt } }));
vi.mock("@/composables/useDeviceManagementIntent", () => ({
  deviceManagementRequestConfig: () => ({}),
  useDeviceManagementIntent: (invalidate: () => void) => (mocks.invalidate = invalidate, {
    release: vi.fn(), validate: vi.fn(),
    capture: async () => ({ controller: new AbortController(), state: mocks.state }),
  }),
}));
vi.mock("@/runtime/runtime-adapter", () => ({ getDeploymentConfig: async () => ({ mode: "local" }), getRuntimeConnection: async () => ({ apiBaseURL: "http://local.test" }), createCurrentDevicePairingOffer: mocks.offer, provisionCurrentDeviceMesh: mocks.provision, saveDeploymentConfig: mocks.saveDeployment, getCurrentDevicePairingStatus: mocks.pairingStatus, resolveApiUrl: async (path: string) => path, isDeviceLocalApiPath: () => false, isDeviceRoleManagementPath: () => false, getBackendAuthHeaders: async () => ({}) }));
vi.mock("@/runtime/runtime-capabilities", () => ({ initializeRuntimeCapabilities: mocks.initialize }));
vi.mock("@/runtime/pairing-lan-monitor", () => ({ startPairingLANMonitor: vi.fn(async (_controller: AbortController, changed: () => void, failed: (error: unknown) => void) => { mocks.lanChanged = changed; mocks.lanFailed = failed; }) }));
vi.mock("@/components/DevicePairScanner.vue", () => ({ default: { template: "<div />" } }));
vi.mock("@/components/DeviceCapabilityGrants.vue", () => ({ default: { template: "<div />" } }));
import DevicePairingPanel from "@/components/DevicePairingPanel.vue";
import BoundDevicesPanel from "@/components/BoundDevicesPanel.vue";

function mountBoundDevices() {
  return shallowMount(BoundDevicesPanel, { global: {
    stubs: {
      "el-button": true,
      "el-tag": true,
      "el-alert": true,
      "el-card": true,
      "el-empty": true,
      "el-dialog": { props: ["modelValue", "title"], template: '<div data-dialog :data-title="title"><slot /></div>' },
    },
    directives: { loading: () => {} },
  } });
}

function mountPairing(mode: "local" | "cloud" = "local") {
  return shallowMount(DevicePairingPanel, {
    props: { serverURL: "https://draft.core", mode },
    global: { stubs: { "el-button": true, "el-alert": true, "el-input": true } },
  });
}

describe("设备同步信任状态", () => {
  beforeEach(() => {
    mocks.meshIdentity.mockReset().mockResolvedValue({ deviceId: "trusted" });
    mocks.meshStatus.mockReset().mockResolvedValue({});
    window.amitiaDesktop = { getMeshIdentity: mocks.meshIdentity, getMeshStatus: mocks.meshStatus } as any;
    mocks.devices = ["trusted", "revoked", "pending"].map(trustState => ({ deviceId: trustState, trustState, platform: "windows" }));
    mocks.state = { policy: { deviceId: "trusted" }, canAdminister: false };
    mocks.post.mockReset().mockImplementation(async () => { mocks.devices = mocks.devices.map(device => device.deviceId === "trusted" ? { ...device, trustState: "trusted" } : device); return { ok: true }; });
    mocks.put.mockReset().mockResolvedValue({ ok: true });
    mocks.confirm.mockReset().mockResolvedValue("confirm");
    mocks.offer.mockReset().mockResolvedValue({ qrPayload: "amitia://pair?offer=test", qrImage: "test-image", expiresAt: new Date(Date.now() + 1800000).toISOString() });
    mocks.prompt.mockReset().mockResolvedValue({ value: "manual-offer" });
    mocks.provision.mockReset().mockResolvedValue(undefined);
    mocks.saveDeployment.mockReset().mockResolvedValue(undefined);
    mocks.pairingStatus.mockReset().mockResolvedValue({ firstDeviceSetupRequired: false });
    mocks.initialize.mockReset().mockResolvedValue(undefined);
    mocks.get.mockReset().mockImplementation(async (path: string) => path.endsWith("/devices") ? { devices: mocks.devices } : { lastApplied: 3 });
  });
  it("设备身份不可用时说明原因且不发起配对", async () => {
    mocks.meshIdentity.mockRejectedValueOnce(new Error("身份读取失败"));
    vi.mocked(ElMessage.warning).mockClear();
    const wrapper = mountPairing("cloud");
    await flushPromises();
    const controls = wrapper.findAll('.pairing-control[role="button"]');
    expect(controls).toHaveLength(3);
    expect(wrapper.get("details").text()).toContain("身份读取失败");
    await controls[0].trigger("click");
    expect(ElMessage.warning).toHaveBeenCalledWith("设备身份尚未就绪，请检查页面中的错误信息。");
    expect(mocks.provision).not.toHaveBeenCalled();
    wrapper.unmount();
  });
  it("本机中心只生成二维码，子设备只扫码或选择图片", async () => {
    const center = mountPairing("local");
    await flushPromises();
    expect(center.find('[data-amitia-ui="device-pairing.generate"]').exists()).toBe(true);
    expect(center.find('[data-amitia-ui="device-pairing.manual"]').exists()).toBe(true);
    expect(center.find('[data-amitia-ui="device-pairing.scan"]').exists()).toBe(false);
    expect(center.find('[data-amitia-ui="device-pairing.image"]').exists()).toBe(false);
    center.unmount();

    const child = mountPairing("cloud");
    await flushPromises();
    expect(child.find('[data-amitia-ui="device-pairing.generate"]').exists()).toBe(false);
    expect(child.find('[data-amitia-ui="device-pairing.scan"]').exists()).toBe(true);
    expect(child.find('[data-amitia-ui="device-pairing.image"]').exists()).toBe(true);
    expect(child.find('[data-amitia-ui="device-pairing.manual"]').exists()).toBe(true);
    child.unmount();
  });
  it("仅查询可信设备，撤销和待配对设备显示明确状态", async () => {
    mocks.meshIdentity.mockResolvedValue({ deviceId: "device-host" });
    const wrapper = mountBoundDevices();
    await flushPromises();
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status")).map(call => call[1].deviceId)).toEqual(["trusted"]);
    expect((wrapper.vm as any).syncLabel("revoked")).toBe("已撤销，不再同步");
    expect((wrapper.vm as any).syncLabel("pending")).toBe("未受信任，暂不可同步");
    await (wrapper.vm as any).loadSync("revoked");
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status"))).toHaveLength(1);
    wrapper.unmount();
  });
  it("中心设备不显示自身也不查询自身同步状态", async () => {
    mocks.meshIdentity.mockResolvedValue({ deviceId: "trusted" });
    mocks.state = { policy: { deviceId: "trusted" }, canAdminister: true };
    mocks.devices = [
      { deviceId: "trusted", trustState: "trusted", platform: "android" },
      { deviceId: "phone-a", trustState: "trusted", platform: "android" },
    ];
    const wrapper = mountBoundDevices();
    await flushPromises();
    expect((wrapper.vm as any).boundDevices.map((device: any) => device.deviceId)).toEqual(["phone-a"]);
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status")).map(call => call[1].deviceId)).toEqual(["phone-a"]);
    expect(mocks.meshStatus).not.toHaveBeenCalled();
    wrapper.unmount();
  });
  it("刷新后撤销状态覆盖已有同步结果", async () => {
    mocks.meshIdentity.mockResolvedValue({ deviceId: "device-host" });
    const wrapper = mountBoundDevices();
    await flushPromises();
    mocks.devices = [{ deviceId: "trusted", trustState: "revoked" }];
    await (wrapper.vm as any).refresh();
    expect((wrapper.vm as any).syncLabel("trusted")).toBe("已撤销，不再同步");
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status"))).toHaveLength(1);
    wrapper.unmount();
  });
  it("普通设备不查询其他可信设备的同步状态", async () => {
    mocks.meshIdentity.mockResolvedValue({ deviceId: "device-host" });
    mocks.devices.push({ deviceId: "other-device", trustState: "trusted" });
    const wrapper = mountBoundDevices();
    await flushPromises();
    await (wrapper.vm as any).loadSync("other-device");
    expect(mocks.get.mock.calls.filter(call => call[0].endsWith("/sync/status")).map(call => call[1].deviceId)).toEqual(["trusted"]);
    wrapper.unmount();
  });
  it("子设备请求在单个审批弹窗内以列表更新", async () => {
    mocks.state = { coreId: "core", coreConsoleDeviceId: "trusted", policy: { deviceId: "trusted" }, canAdminister: true };
    mocks.get.mockImplementation(async (path: string) => {
      if (path.endsWith("/pairing/approvals")) {
        return {
          requests: [
            { requestId: "request-a", deviceId: "phone-a", platform: "android", label: "", revision: 1 },
            { requestId: "request-b", deviceId: "phone-b", platform: "ios", label: "", revision: 1 },
          ],
        };
      }
      if (path.endsWith("/devices")) return { devices: mocks.devices };
      return { lastApplied: 3 };
    });
    const wrapper = mountBoundDevices();
    await flushPromises();
    const dialogs = wrapper.findAll("[data-dialog]");
    expect(dialogs).toHaveLength(1);
    expect(dialogs[0].attributes("data-title")).toBe("子设备配对请求");
    expect(dialogs[0].text()).toContain("phone-a");
    expect(dialogs[0].text()).toContain("phone-b");
    wrapper.unmount();
  });
  it("批准子设备后立即重新读取已绑定设备列表", async () => {
    mocks.state = { coreId: "core", coreConsoleDeviceId: "trusted", policy: { deviceId: "trusted" }, canAdminister: true };
    let approved = false;
    mocks.get.mockImplementation(async (path: string) => {
      if (path.endsWith("/pairing/approvals")) {
        return approved
          ? { requests: [] }
          : { requests: [{ requestId: "request-a", deviceId: "phone-a", platform: "android", label: "", revision: 1 }] };
      }
      if (path.endsWith("/devices")) {
        return { devices: approved ? [...mocks.devices, { deviceId: "phone-a", trustState: "trusted", platform: "android" }] : mocks.devices };
      }
      return { lastApplied: 3 };
    });
    mocks.put.mockImplementation(async () => { approved = true; return { ok: true }; });
    const wrapper = mountBoundDevices();
    await flushPromises();
    await (wrapper.vm as any).decidePairing({ requestId: "request-a", deviceId: "phone-a", platform: "android", label: "", revision: 1 }, true);
    expect(mocks.put).toHaveBeenCalledOnce();
    expect((wrapper.vm as any).boundDevices.some((device: any) => device.deviceId === "phone-a")).toBe(true);
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
  it("连接失效时清除二维码并提示重新生成", async () => {
    const wrapper = mountPairing();
    await flushPromises();
    await (wrapper.vm as any).generatePairingOffer();
    expect((wrapper.vm as any).pairingOffer).toBeTruthy();
    mocks.invalidate?.();
    await flushPromises();
    expect((wrapper.vm as any).pairingOffer).toBe("");
    expect((wrapper.vm as any).offerNotice).toContain("重新生成");
    wrapper.unmount();
  });
  it("局域网地址变化或监测失败时清除二维码并显示原始错误", async () => {
    const wrapper = mountPairing();
    await flushPromises();
    await (wrapper.vm as any).generatePairingOffer();
    mocks.lanChanged?.();
    expect((wrapper.vm as any).pairingOffer).toBe("");
    expect((wrapper.vm as any).offerNotice).toContain("局域网地址已变化");
    await (wrapper.vm as any).generatePairingOffer();
    mocks.lanFailed?.(new Error("stream failure detail"));
    expect((wrapper.vm as any).pairingOffer).toBe("");
    expect((wrapper.vm as any).offerNotice).toContain("stream failure detail");
    wrapper.unmount();
  });
  it("生成期间连接失效时不展示迟到的二维码", async () => {
    let resolveOffer!: (value: unknown) => void;
    mocks.offer.mockReturnValue(new Promise(resolve => { resolveOffer = resolve; }));
    const wrapper = mountPairing();
    await flushPromises();
    const pending = (wrapper.vm as any).generatePairingOffer();
    await flushPromises();
    mocks.invalidate?.();
    resolveOffer({ qrPayload: "amitia://pair?offer=old", expiresAt: new Date(Date.now() + 60000).toISOString() });
    await pending;
    expect((wrapper.vm as any).pairingOffer).toBe("");
    expect((wrapper.vm as any).offerNotice).toContain("重新生成");
    wrapper.unmount();
  });
  it("二维码到期后清除配对信息", async () => {
    vi.useFakeTimers();
    mocks.offer.mockResolvedValue({ qrPayload: "amitia://pair?offer=test", expiresAt: new Date(Date.now() + 1000).toISOString() });
    const wrapper = mountPairing();
    await (wrapper.vm as any).generatePairingOffer();
    await vi.advanceTimersByTimeAsync(1000);
    expect((wrapper.vm as any).pairingOffer).toBe("");
    expect((wrapper.vm as any).offerNotice).toContain("已过期");
    wrapper.unmount();
    vi.useRealTimers();
  });
  it("手动配对使用云端表单地址，完成配对后才保存运行模式", async () => {
    const wrapper = mountPairing("cloud");
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
    const wrapper = mountPairing("cloud");
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
    const wrapper = mountPairing("cloud");
    await flushPromises();
    await (wrapper.vm as any).joinCurrentDevice();
    expect(mocks.saveDeployment).not.toHaveBeenCalled();
    expect(wrapper.emitted("paired")).toBeUndefined();
    wrapper.unmount();
  });
});
