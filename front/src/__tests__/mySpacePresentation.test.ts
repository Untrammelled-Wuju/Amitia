import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { describe, expect, it, vi } from "vitest";
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
