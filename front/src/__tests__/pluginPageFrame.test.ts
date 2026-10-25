import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import ExtensionContributionRenderer from "../components/extension/ExtensionContributionRenderer.vue";
import type { UIContributionSummary } from "../stores/extensionUI";

vi.mock("../components/extension/SchemaUIRenderer.vue", () => ({
  __esModule: true,
  default: { name: "SchemaUIRenderer", template: "<div>Schema 内容</div>" },
}));
function contribution(kind: string): UIContributionSummary {
  return {
    contributionId: "emote-page", extensionId: "com.amitia/emote", moduleId: "emote-ui",
    kind, title: "表情包管理", slotId: "provider.page.provider", contractVersion: 1,
    generation: 1, ordering: 0, visible: true, effective: true, enabled: true,
    runtimeReady: true, sandbox: kind === "web_page" ? "client_module" : "schema_renderer",
    schemaPath: "desktop/index.schema.json", entryPath: "desktop/index.html",
  };
}

describe("插件页面公共适配", () => {
  it("Schema 页面通过 Provider 直出时仍有标题和页面容器", async () => {
    const kind = "schema_page";
    const wrapper = mount(ExtensionContributionRenderer, {
      props: { contribution: { ...contribution(kind), extensionId: "com.amitia/example" }, context: {}, slotId: "provider.page.provider" },
    });
    try {
      expect(wrapper.find(".extension-loading-overlay").exists()).toBe(true);
      expect(wrapper.text()).not.toContain("加载扩展界面");
      await flushPromises();
      expect(wrapper.find(".extension-loading-overlay").exists()).toBe(false);
      expect(wrapper.find(".plugin-page-frame").exists()).toBe(true);
      expect(wrapper.find("h2").text()).toBe("表情包管理");
      expect(wrapper.findAll("h2")).toHaveLength(1);
      expect(wrapper.find(".plugin-page-frame__content").text()).toContain("内容");
      expect(wrapper.find(".extension-surface").exists()).toBe(false);
    } finally {
      wrapper.unmount();
    }
  });

  it("聊天内嵌面板不会增加页面标题或页面边距层", async () => {
    const wrapper = mount(ExtensionContributionRenderer, {
      props: { contribution: contribution("panel"), context: { surface: { role: "sidebar" } }, slotId: "chat.sidebar" },
    });
    try {
      await flushPromises();
      expect(wrapper.find(".plugin-page-frame").exists()).toBe(false);
      expect(wrapper.find(".extension-surface--sidebar").exists()).toBe(true);
    } finally {
      wrapper.unmount();
    }
  });

  it("表情包主页面保留内容容器并移除标题占位", async () => {
    const wrapper = mount(ExtensionContributionRenderer, {
      props: { contribution: contribution("schema_page"), context: {}, slotId: "provider.page.provider" },
    });
    try {
      await flushPromises();
      expect(wrapper.find(".plugin-page-frame").exists()).toBe(true);
      expect(wrapper.find(".plugin-page-frame__header").exists()).toBe(false);
      expect(wrapper.find(".plugin-page-frame__content").text()).toContain("内容");
    } finally {
      wrapper.unmount();
    }
  });

});
