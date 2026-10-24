import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import ExtensionPageHeader from "../views/extensions/components/ExtensionPageHeader.vue";

describe("二级页面主工作区标题", () => {
  it("默认不显示标题、描述和面包屑", () => {
    const wrapper = mount(ExtensionPageHeader, {
      props: {
        title: "角色管理",
        description: "创建、编辑、导入与导出角色",
      },
      global: { stubs: { RouterLink: true } },
    });

    expect(wrapper.find(".heading-group").exists()).toBe(false);
    expect(wrapper.text()).not.toContain("角色管理");
    expect(wrapper.text()).not.toContain("创建、编辑、导入与导出角色");
  });

  it("保留页面专属操作", () => {
    const wrapper = mount(ExtensionPageHeader, {
      props: { title: "扩展包" },
      slots: {
        actions: "<button class=\"page-action\">刷新</button>",
      },
      global: { stubs: { RouterLink: true } },
    });

    expect(wrapper.find(".heading-group").exists()).toBe(false);
    expect(wrapper.get(".page-action").text()).toBe("刷新");
  });
});
