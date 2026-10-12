import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import ExtensionRenderState from "../components/extension/ExtensionRenderState.vue";

describe("扩展界面加载状态", () => {
  it("加载时只有覆盖动画，切换为空状态后移除覆盖层", async () => {
    const wrapper = mount(ExtensionRenderState, { props: { state: "loading" } });
    try {
      expect(wrapper.find(".extension-loading-overlay__spinner").exists()).toBe(true);
      expect(wrapper.text()).toBe("");
      expect(wrapper.attributes("aria-busy")).toBe("true");
      expect(wrapper.attributes("aria-label")).toBe("扩展界面加载中");
      await wrapper.setProps({ state: "empty", detail: "暂无数据" });
      expect(wrapper.find(".extension-loading-overlay").exists()).toBe(false);
      expect(wrapper.text()).toContain("暂无数据");
    } finally {
      wrapper.unmount();
    }
  });

  it("失败时保留错误信息与重试入口", async () => {
    const wrapper = mount(ExtensionRenderState, { props: { state: "error", detail: "加载失败" } });
    try {
      expect(wrapper.find(".extension-loading-overlay").exists()).toBe(false);
      expect(wrapper.text()).toContain("加载失败");
      await wrapper.find("button").trigger("click");
      expect(wrapper.emitted("retry")).toHaveLength(1);
    } finally {
      wrapper.unmount();
    }
  });
});
