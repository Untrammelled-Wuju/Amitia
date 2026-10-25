import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { mount } from "@vue/test-utils";
import ElementPlus, { ElTabs } from "element-plus";
import { createPinia } from "pinia";
import { reactive, nextTick } from "vue";
import { describe, expect, it } from "vitest";
import SchemaUINode from "../components/extension/SchemaUINode.vue";
import { validateDocument, type SchemaUIDocument } from "../components/extension/schema-ui-utils";

const document = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-code/emote/ui/schema/desktop-index.json"), "utf8")) as SchemaUIDocument;
const sidebarDocument = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-code/emote/ui/schema/desktop-sidebar.json"), "utf8")) as SchemaUIDocument;

describe("表情包 Schema 页面交互", () => {
  it("无标签参数时默认激活表情导航，切换标签后同步选中", async () => {
    const wrapper = mount(SchemaUINode, {
      props: {
        node: sidebarDocument.children![0], depth: 1, formState: {},
        context: { route: "/emotes", defaultNavigationRoute: "/emotes?tab=emotes" },
        sessionId: "session", extensionId: "com.amitia/emote", contributionId: "emote-workspace-sidebar",
      },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      expect(wrapper.findAll('[aria-current="page"]').map((link) => link.text())).toEqual(["表情"]);
      await wrapper.setProps({ context: { route: "/emotes?tab=groups", defaultNavigationRoute: "/emotes?tab=emotes" } });
      expect(wrapper.findAll('[aria-current="page"]').map((link) => link.text())).toEqual(["分组"]);
      await wrapper.setProps({ context: { route: "/emotes?tab=settings", defaultNavigationRoute: "/emotes?tab=emotes" } });
      expect(wrapper.findAll('[aria-current="page"]').map((link) => link.text())).toEqual(["AI 设置"]);
    } finally {
      wrapper.unmount();
    }
  });

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

  it("根据路由查询参数切换主页面标签", async () => {
    expect(validateDocument(sidebarDocument).valid).toBe(true);
    const formState = reactive({ selected: { id: "", name: "", meaning: "", keywords: "", aiEnabled: false } });
    const wrapper = mount(SchemaUINode, {
      props: {
        node: document.children![0],
        depth: 1,
        formState,
        context: { formState, routeQuery: { tab: "groups" } },
        sessionId: "session",
        extensionId: "com.amitia/emote",
        contributionId: "emote-page",
      },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      expect(wrapper.findComponent(ElTabs).props("modelValue")).toBe("groups");
      await wrapper.setProps({ context: { formState, routeQuery: { tab: "settings" } } });
      expect(wrapper.findComponent(ElTabs).props("modelValue")).toBe("settings");
    } finally {
      wrapper.unmount();
    }
  });
});
