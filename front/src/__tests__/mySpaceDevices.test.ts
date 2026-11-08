import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import MySpaceDevicesView from "../views/user-settings/MySpaceDevicesView.vue";

const mocks = vi.hoisted(() => ({ getDeploymentConfig: vi.fn() }));

vi.mock("../runtime/runtime-adapter", () => ({
  getDeploymentConfig: mocks.getDeploymentConfig,
}));

vi.mock("../components/DevicePairingPanel.vue", () => ({
  default: {
    name: "DevicePairingPanel",
    props: ["mode", "serverURL"],
    emits: ["paired"],
    template: '<div data-device-pairing :data-mode="mode" :data-server="serverURL" />',
  },
}));

vi.mock("../components/BoundDevicesPanel.vue", () => ({
  default: { name: "BoundDevicesPanel", template: '<div data-bound-devices />' },
}));

describe("我的空间已绑定设备", () => {
  const global = { stubs: { "el-skeleton": true } };

  it("本机中心传入本地模式", async () => {
    mocks.getDeploymentConfig.mockResolvedValue({ mode: "local" });
    const wrapper = mount(MySpaceDevicesView, { global });
    await flushPromises();
    expect(wrapper.get("[data-device-pairing]").attributes("data-mode")).toBe("local");
    expect(wrapper.find("[data-bound-devices]").exists()).toBe(true);
    wrapper.unmount();
  });

  it("远程子设备传入云端模式和中心地址", async () => {
    mocks.getDeploymentConfig.mockResolvedValue({ mode: "cloud", serverURL: "https://core.example" });
    const wrapper = mount(MySpaceDevicesView, { global });
    await flushPromises();
    expect(wrapper.get("[data-device-pairing]").attributes("data-mode")).toBe("cloud");
    expect(wrapper.get("[data-device-pairing]").attributes("data-server")).toBe("https://core.example");
    wrapper.unmount();
  });
});
