import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia } from "pinia";
import { reactive, nextTick } from "vue";
import { describe, expect, it } from "vitest";
import SchemaUINode from "../components/extension/SchemaUINode.vue";
import { validateDocument, type SchemaUIDocument } from "../components/extension/schema-ui-utils";

const document = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins/emote/ui/schema/desktop-index.json"), "utf8")) as SchemaUIDocument;

describe("表情包 Schema 页面交互", () => {
  it("空选中隐藏编辑操作，选中后可编辑并携带原表情触发操作", async () => {
    expect(validateDocument(document).valid).toBe(true);
    const formState = reactive({ selected: { id: "", name: "", meaning: "", keywords: "", aiEnabled: false } });
    const item = { id: "emote-one", name: "开心", meaning: "开心的表情" };
    const wrapper = mount(SchemaUINode, {
      props: {
        node: document.children![0],
        depth: 1,
        formState,
        context: { formState, localState: { dashboard: { emotes: [item], groups: [] } } },
        sessionId: "session",
        extensionId: "com.amitia/emote",
        contributionId: "emote-page",
      },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      expect(wrapper.text()).toContain("选择一个表情查看和编辑详情");
      expect(wrapper.find('[data-node-id="emote-detail-save"]').exists()).toBe(false);
      await wrapper.find(".schema-ui-gallery__item").trigger("click");
      const selection = wrapper.emitted("action")!.at(-1)![0] as { item: unknown; action: { input: { action: string } } };
      expect(selection.item).toMatchObject({ ...item, input: { payload: { id: item.id } } });
      expect(selection.action.input.action).toBe("ui.select_emote");
      Object.assign(formState.selected, { id: item.id, name: item.name, meaning: item.meaning });
      await nextTick();
      expect(wrapper.find('[data-node-id="emote-detail-empty"]').exists()).toBe(false);
      await wrapper.find('[data-node-id="emote-detail-name"] input').setValue("新的名称");
      expect(formState.selected.name).toBe("新的名称");
      await wrapper.find('[data-node-id="emote-detail-save"]').trigger("click");
      const actions = wrapper.emitted("action")!;
      expect(actions.some(([event]) => (event as { action: { input: { action: string } } }).action.input.action === "emotes.update")).toBe(true);
      formState.selected.id = "";
      await nextTick();
      expect(wrapper.find('[data-node-id="emote-detail-delete"]').exists()).toBe(false);
      expect(wrapper.text()).toContain("选择一个表情查看和编辑详情");
    } finally {
      wrapper.unmount();
    }
  });
});
