import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia } from "pinia";
import { describe, expect, it } from "vitest";
import SchemaUINode from "../components/extension/SchemaUINode.vue";

describe("Schema permission summary", () => {
  it("renders permission names and reasons without an image placeholder", () => {
    const wrapper = mount(SchemaUINode, {
      props: {
        node: {
          id: "permissions",
          type: "permission_summary",
          props: {
            title: "插件所需权限",
            permissions: [
              { name: "storage.state.read", reason: "读取插件配置" },
              { name: "service.network.request", reason: "访问外部服务" },
              "legacy.permission",
            ],
          },
        },
        depth: 1,
        formState: {},
        context: {},
        sessionId: "session",
        extensionId: "com.amitia/plugin-market",
        contributionId: "plugin-market-desktop-page",
      },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      expect(wrapper.text()).toContain("插件所需权限");
      expect(wrapper.text()).toContain("storage.state.read");
      expect(wrapper.text()).toContain("读取插件配置");
      expect(wrapper.text()).toContain("service.network.request");
      expect(wrapper.text()).toContain("访问外部服务");
      expect(wrapper.text()).toContain("legacy.permission");
      expect(wrapper.text()).not.toContain("无预览");
    } finally {
      wrapper.unmount();
    }
  });
});
