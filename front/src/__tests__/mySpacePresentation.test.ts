import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { describe, expect, it, vi } from "vitest";
import UserSettingsView from "../views/user-settings/UserSettingsView.vue";
import SpaceIdentityView from "../views/user-settings/SpaceIdentityView.vue";
import MySpaceRuntimeView from "../views/user-settings/MySpaceRuntimeView.vue";
import ModeSwitchPanel from "../views/runtime-mode/components/ModeSwitchPanel.vue";

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

  it("Space 身份默认突出空间标识，实例标识可按需展开", async () => {
    const wrapper = mount(SpaceIdentityView, { global: { plugins: [ElementPlus] } });
    try {
      await vi.waitFor(() => expect(wrapper.find(".identity-main").text()).toContain("space_example"));
      expect(wrapper.get(".identity-advanced").element.hasAttribute("open")).toBe(false);
      expect(wrapper.find(".identity-value-row .el-button").exists()).toBe(true);
    } finally { wrapper.unmount(); }
  });

  it("服务端部署形态独立于设备连接目标，修改前明确告知不迁移服务", async () => {
    const wrapper = mount(ModeSwitchPanel, {
      props: { modeDeployMode: "desktop-local", switching: false },
      global: { plugins: [ElementPlus] },
    });
    try {
      expect(wrapper.get(".section-title").text()).toBe("修改当前 Core 的服务端部署形态");
      expect(wrapper.get(".mode-scope-note").text()).toContain("不会切换这台设备连接的服务器");
      expect(wrapper.findAll(".mo-label").map(item => item.text())).toEqual(["本机 Core 部署", "Core 私有云部署"]);
      await wrapper.findAll(".mode-option")[1].trigger("click");
      expect(wrapper.get(".impact-box").text()).toContain("不会自动迁移至云服务器");
      expect(wrapper.get(".impact-actions .el-button").text()).toContain("确认修改 Core 为私有云部署配置");
      await wrapper.get(".impact-actions .el-button").trigger("click");
      expect(wrapper.emitted("confirmSwitch")?.[0]).toEqual(["cloud-web"]);
    } finally { wrapper.unmount(); }
  });

  it("运行模式保留常用连接配置并收纳高级 Core 参数", () => {
    const wrapper = mount(MySpaceRuntimeView, {
      global: { stubs: {
        DeploymentPanel: { template: '<div data-testid="deployment">连接方式</div>' },
        RuntimeModeView: { template: '<div data-testid="runtime-detail">Core 参数</div>' },
      } },
    });
    try {
      expect(wrapper.find('[data-testid="deployment"]').exists()).toBe(true);
      expect(wrapper.get(".runtime-advanced").element.hasAttribute("open")).toBe(false);
      expect(wrapper.get(".runtime-advanced summary").text()).toContain("Core 服务端设置");
      expect(wrapper.find('[data-testid="runtime-detail"]').exists()).toBe(true);
    } finally { wrapper.unmount(); }
  });
});
