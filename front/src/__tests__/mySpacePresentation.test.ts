import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { afterEach, describe, expect, it, vi } from "vitest";
import UserSettingsView from "../views/user-settings/UserSettingsView.vue";
import SpaceIdentityView from "../views/user-settings/SpaceIdentityView.vue";
import MySpaceRuntimeView from "../views/user-settings/MySpaceRuntimeView.vue";

vi.mock("../composables/useApi", () => ({
  apiClient: {
    get: vi.fn(async (url: string) => {
      if (url.includes("/api/space/profile")) return { data: { data: { displayName: "测试用户", userLabel: "开发者", bio: "", avatar: "" } } };
      return { data: { data: { spaceId: "space_example", instanceId: "instance_example" } } };
    }),
    put: vi.fn(),
  },
}));
vi.mock("../stores/app", () => ({
  useAppStore: () => ({ setAvatar: vi.fn() }),
}));
vi.mock("../runtime/web-device-mesh", () => ({
  getWebDeviceMeshIdentity: () => ({ deviceId: "web-device", runtimeId: "web-runtime", platform: "web" }),
}));

afterEach(() => { vi.unstubAllGlobals(); });

describe("我的空间内容层级", () => {
  it("个人资料优先呈现头像与表单，高级头像地址默认收起", async () => {
    const wrapper = mount(UserSettingsView, { global: { plugins: [ElementPlus] } });
    try {
      await vi.waitFor(() => expect(wrapper.find(".profile-overview-text").text()).toContain("测试用户"));
      expect(wrapper.findAll(".profile-field-row .el-form-item")).toHaveLength(2);
      expect(wrapper.get(".profile-advanced").element.hasAttribute("open")).toBe(false);
      expect(wrapper.find(".profile-actions .el-button").exists()).toBe(true);
    } finally { wrapper.unmount(); }
  });

  it.each([true, false])("身份信息展示空间、实例和当前设备的真实标识，桌面端=%s", async (desktop) => {
    const writeClipboardText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("amitiaDesktop", desktop ? {
      getMeshIdentity: async () => ({ deviceId: "desktop-device", runtimeId: "desktop-runtime", platform: "darwin" }),
      writeClipboardText,
    } : undefined);
    const wrapper = mount(SpaceIdentityView, { global: { plugins: [ElementPlus] } });
    try {
      const device = desktop ? "desktop" : "web";
      await vi.waitFor(() => expect(wrapper.findAll("code").map((node) => node.text())).toEqual(["space_example", "instance_example", `${device}-device`, `${device}-runtime`]));
      expect(wrapper.get("h2").text()).toBe("身份信息");
      expect(wrapper.findAll("h3").map((node) => node.text())).toEqual(["Space ID", "Instance ID", "Device ID", "Runtime ID"]);
      if (desktop) {
        await wrapper.findAll(".identity-value-row .el-button")[2].trigger("click");
        expect(writeClipboardText).toHaveBeenCalledWith("desktop-device");
      }
    } finally { wrapper.unmount(); }
  });

  it("当前设备身份读取失败时保留 Space ID，不生成替代设备身份", async () => {
    vi.stubGlobal("amitiaDesktop", { getMeshIdentity: async () => null });
    const wrapper = mount(SpaceIdentityView, { global: { plugins: [ElementPlus] } });
    try {
      await vi.waitFor(() => expect(wrapper.findAll("code").map((node) => node.text())).toEqual(["space_example", "instance_example", "未读取", "未读取"]));
      expect(wrapper.findAll(".identity-value-row .el-button")[2].attributes("disabled")).toBeDefined();
    } finally { wrapper.unmount(); }
  });

  it("运行模式只保留连接与配对配置", () => {
    const wrapper = mount(MySpaceRuntimeView, {
      global: { stubs: {
        DeploymentPanel: { template: '<div data-testid="deployment">连接方式</div>' },
      } },
    });
    try {
      expect(wrapper.find('[data-testid="deployment"]').exists()).toBe(true);
      expect(wrapper.find(".runtime-advanced").exists()).toBe(false);
    } finally { wrapper.unmount(); }
  });
});
