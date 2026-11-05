import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia } from "pinia";
import { describe, expect, it } from "vitest";
import SchemaUINode from "../components/extension/SchemaUINode.vue";

describe("Schema gallery item state", () => {
  it("blocks disabled items and keeps supported items actionable", async () => {
    const wrapper = mount(SchemaUINode, {
      props: {
        node: {
          id: "market-listings",
          type: "gallery",
          props: {
            items: [
              { id: "desktop", title: "Desktop", actionDisabled: true },
              { id: "mobile", title: "Mobile", actionDisabled: false },
            ],
            titleField: "title",
            disabledField: "actionDisabled",
            showSubtitle: false,
          },
          actions: [{ action_id: "command", target: "tool" }],
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
      const items = wrapper.findAll(".schema-ui-gallery__item");
      expect(items[0]?.attributes("disabled")).toBeDefined();
      expect(items[1]?.attributes("disabled")).toBeUndefined();
      await items[0]!.trigger("click");
      expect(wrapper.emitted("action")).toBeUndefined();
      await items[1]!.trigger("click");
      expect(wrapper.emitted("action")).toHaveLength(1);
    } finally {
      wrapper.unmount();
    }
  });
});
