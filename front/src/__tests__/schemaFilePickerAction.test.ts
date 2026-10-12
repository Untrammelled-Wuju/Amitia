import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia } from "pinia";
import { reactive } from "vue";
import { describe, expect, it } from "vitest";
import SchemaUINode from "../components/extension/SchemaUINode.vue";
import type { SchemaUINode as SchemaUINodeType } from "../components/extension/schema-ui-utils";

const filePickerNode: SchemaUINodeType = {
  id: "emote-import",
  type: "file_picker",
  props: { text: "导入表情包", accept: ".png", multiple: true },
  actions: [
    { action_id: "command", target: "tool", input: { action: "ui.import" }, statePath: "importResult" },
    { action_id: "command", target: "tool", input: { action: "ui.dashboard" }, statePath: "dashboard" },
  ],
} as SchemaUINodeType;

describe("file_picker 动作分发", () => {
  it("只把文件交给首个动作，其余动作作为串行后续动作下发", async () => {
    const formState = reactive<Record<string, unknown>>({});
    const wrapper = mount(SchemaUINode, {
      props: {
        node: filePickerNode,
        depth: 1,
        formState,
        context: { formState },
        sessionId: "session",
        extensionId: "com.amitia/emote",
        contributionId: "emote-page",
      },
      global: { plugins: [createPinia(), ElementPlus] },
    });

    try {
      const input = wrapper.find('input[type="file"]');
      expect(input.exists()).toBe(true);
      const file = new File([new Uint8Array([1, 2, 3])], "sample.png", { type: "image/png" });
      Object.defineProperty(input.element, "files", { value: [file], configurable: true });
      await input.trigger("change");
      await new Promise((resolve) => setTimeout(resolve, 60));

      const emitted = wrapper.emitted("action");
      expect(emitted).toBeTruthy();
      expect(emitted!.length).toBe(1);
      const payload = emitted![0][0] as {
        action: { input: { action: string } };
        files?: Array<{ name: string }>;
        followUpActions?: Array<{ input: { action: string } }>;
      };
      expect(payload.action.input.action).toBe("ui.import");
      expect(payload.files?.length).toBe(1);
      expect(payload.files?.[0]?.name).toBe("sample.png");
      expect(payload.followUpActions?.length).toBe(1);
      expect(payload.followUpActions?.[0]?.input.action).toBe("ui.dashboard");
    } finally {
      wrapper.unmount();
    }
  });
});
