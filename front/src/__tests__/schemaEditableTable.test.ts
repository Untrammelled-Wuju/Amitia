import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia } from "pinia";
import { reactive } from "vue";
import { describe, expect, it } from "vitest";
import SchemaUINode from "../components/extension/SchemaUINode.vue";
import type { SchemaUINode as SchemaUINodeType } from "../components/extension/schema-ui-utils";

describe("editable_table", () => {
  it("编辑表单中的动态行，并把角色列渲染为选择器", async () => {
    const rows = reactive([
      { speaker: "用户", role: "user", content: "你好" },
      { speaker: "AI", role: "assistant", content: "你好呀" },
    ]);
    const formState = reactive<Record<string, unknown>>({ rows });
    const node: SchemaUINodeType = {
      id: "messages",
      type: "editable_table",
      bindings: [{ path: "rows", source: "form" }],
      props: {
        columns: [
          { prop: "speaker", label: "发言者", width: 100 },
          {
            prop: "role",
            label: "角色",
            width: 100,
            type: "select",
            options: [
              { label: "用户", value: "user" },
              { label: "AI", value: "assistant" },
            ],
          },
          { prop: "content", label: "内容" },
        ],
      },
    } as SchemaUINodeType;
    const wrapper = mount(SchemaUINode, {
      props: {
        node,
        depth: 1,
        formState,
        context: { formState },
        sessionId: "session",
        extensionId: "com.amitia/chat-import",
        contributionId: "chat-import-page",
      },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      await flushPromises();
      expect(wrapper.findAll(".el-select")).toHaveLength(2);
      const inputs = wrapper.findAll(".el-input input");
      expect(inputs.length).toBeGreaterThanOrEqual(4);
      await inputs[0].setValue("用户 A");
      expect(rows[0].speaker).toBe("用户 A");
    } finally {
      wrapper.unmount();
    }
  });
});
