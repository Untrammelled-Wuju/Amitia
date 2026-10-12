import { mount } from "@vue/test-utils";
import ElementPlus, { ElTabs } from "element-plus";
import { createPinia } from "pinia";
import { afterEach, describe, expect, it, vi } from "vitest";
import SchemaUINode from "../components/extension/SchemaUINode.vue";
import type { SchemaUINode as SchemaUINodeType } from "../components/extension/schema-ui-utils";

function createTabsNode(variant?: unknown): SchemaUINodeType {
  return {
    id: "tabs",
    type: "tabs",
    props: variant === undefined ? {} : { variant },
    children: [
      {
        id: "tab",
        type: "tab_item",
        props: { label: "标签", name: "tab" },
      },
    ],
  };
}

function mountTabs(variant?: unknown) {
  return mount(SchemaUINode, {
    props: {
      node: createTabsNode(variant),
      depth: 1,
      formState: {},
      context: {},
      sessionId: "session",
      extensionId: "extension",
      contributionId: "contribution",
    },
    global: {
      plugins: [createPinia(), ElementPlus],
    },
  });
}

describe("SchemaUINode tabs type", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("使用 Element Plus 的空字符串表示默认线条样式", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const wrapper = mountTabs();

    try {
      expect(wrapper.findComponent(ElTabs).props("type")).toBe("");
      expect(warn).not.toHaveBeenCalled();
    } finally {
      wrapper.unmount();
    }
  });

  it("兼容 line 别名并过滤非法样式", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const lineWrapper = mountTabs("line");
    const invalidWrapper = mountTabs("invalid");
    const cardWrapper = mountTabs("card");

    try {
      expect(lineWrapper.findComponent(ElTabs).props("type")).toBe("");
      expect(invalidWrapper.findComponent(ElTabs).props("type")).toBe("");
      expect(cardWrapper.findComponent(ElTabs).props("type")).toBe("card");
      expect(warn).not.toHaveBeenCalled();
    } finally {
      lineWrapper.unmount();
      invalidWrapper.unmount();
      cardWrapper.unmount();
    }
  });
});
