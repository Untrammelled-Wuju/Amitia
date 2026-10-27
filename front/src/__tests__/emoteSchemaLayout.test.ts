import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { mount } from "@vue/test-utils";
import ElementPlus, { ElMessage } from "element-plus";
import { createPinia } from "pinia";
import { reactive, nextTick } from "vue";
import { describe, expect, it, vi } from "vitest";
import SchemaUINode from "../components/extension/SchemaUINode.vue";
import { validateDocument, type SchemaUIDocument } from "../components/extension/schema-ui-utils";

const document = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-code/emote/ui/schema/desktop-index.json"), "utf8")) as SchemaUIDocument;
const sidebarDocument = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-code/emote/ui/schema/desktop-sidebar.json"), "utf8")) as SchemaUIDocument;
const composerDocument = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-code/emote/ui/schema/desktop-composer.json"), "utf8")) as SchemaUIDocument;

describe("表情包 Schema 页面交互", () => {
  it("composer 首行显示分组，下方只平铺表情图片", async () => {
    expect(validateDocument(composerDocument).valid).toBe(true);
    const formState = reactive({ groupId: "__ungrouped__" });
    const context = {
      formState,
      localState: {
        composer: {
          groups: [
            { id: "__ungrouped__", name: "未分组", cover: "/ungrouped.png" },
            { id: "__recent__", name: "最近使用", cover: "/recent.png" },
            { id: "g1", name: "常用", cover: "/group.png" },
          ],
          emotes: [
            { id: "e1", name: "开心", meaning: "开心", thumbnailUrl: "/e1.png" },
            { id: "e2", name: "微笑", meaning: "友好", thumbnailUrl: "/e2.png" },
          ],
        },
      },
    };
    const wrapper = mount(SchemaUINode, {
      props: {
        node: composerDocument.children![0],
        depth: 1,
        formState,
        context,
        sessionId: "s",
        extensionId: "com.amitia/emote",
        contributionId: "emote-composer",
      },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      const groups = wrapper.get('[data-node-id="emote-composer-groups"]');
      expect(groups.classes()).toContain("schema-ui-gallery--groups");
      expect(groups.text()).toContain("未分组");
      expect(groups.text()).toContain("最近使用");
      await groups.findAll(".schema-ui-gallery__item")[2].trigger("click");
      expect(wrapper.emitted("action")!.at(-1)![0]).toMatchObject({
        action: { input: { action: "ui.composer" } },
        item: { input: { payload: { groupId: "g1" } } },
      });

      const stickers = wrapper.get('[data-node-id="emote-composer-gallery"]');
      expect(stickers.classes()).toContain("schema-ui-gallery--stickers");
      expect(stickers.findAll(".schema-ui-gallery__image")).toHaveLength(2);
      expect(stickers.findAll(".schema-ui-gallery__title")).toHaveLength(0);
      expect(stickers.findAll(".schema-ui-gallery__subtitle")).toHaveLength(0);
      await stickers.findAll(".schema-ui-gallery__item")[0].trigger("click");
      expect(wrapper.emitted("action")!.at(-1)![0]).toMatchObject({
        action: { input: { action: "emotes.send" } },
      });
    } finally {
      wrapper.unmount();
    }
  });

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
    const formState = reactive({ browser: { groupId: "", groupName: "", query: "" }, picker: { open: false, emoteIds: [] }, selected: { id: "", name: "", meaning: "", keywords: "", aiEnabled: false } });
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
      expect(wrapper.find('[data-node-id="emote-detail"]').exists()).toBe(false);
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
      expect(wrapper.find('[data-node-id="emote-detail"]').exists()).toBe(false);
    } finally {
      wrapper.unmount();
    }
  });

  it("点击分组后只展示该组图片并支持搜索和空状态", async () => {
    const formState = reactive({ browser: { groupId: "", groupName: "", query: "" }, picker: { open: false }, selected: { id: "" } });
    const context = {
      formState, routeQuery: { tab: "groups" },
      localState: { dashboard: {
        groups: [{ id: "g1", name: "常用", covers: ["/a.png"], countLabel: "1 张图片" }],
        emotes: [{ id: "a", name: "开心", groupIds: ["g1"], thumbnailUrl: "/a.png" }, { id: "b", name: "其他", groupIds: ["g2"] }],
      } },
    };
    const wrapper = mount(SchemaUINode, {
      props: { node: document.children![0], depth: 1, formState, context, sessionId: "s", extensionId: "com.amitia/emote", contributionId: "emote-page" },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      await wrapper.find(".schema-ui-gallery__item").trigger("click");
      expect(wrapper.emitted("action")!.at(-1)![0]).toMatchObject({ action: { input: { action: "ui.select_group" } }, item: { input: { payload: { id: "g1" } } } });
      Object.assign(formState.browser, { groupId: "g1", groupName: "常用" });
      await nextTick();
      expect(wrapper.findAll(".schema-ui-gallery__item").map((item) => item.text())).toEqual(["开心"]);
      expect(wrapper.find('[data-node-id="emote-group-back"]').exists()).toBe(true);
      formState.browser.query = "没有";
      await nextTick();
      expect(wrapper.findAll(".schema-ui-gallery__item")).toHaveLength(0);
      expect(wrapper.text()).toContain("没有符合条件的表情");
      formState.browser.groupId = "";
      await nextTick();
      expect(wrapper.find('[data-node-id="emote-groups-gallery"]').exists()).toBe(true);
    } finally {
      wrapper.unmount();
    }
  });

  it("新建分组打开弹窗，提交时校验名称并顺序刷新", async () => {
    const warning = vi.spyOn(ElMessage, "warning").mockImplementation(() => ({ close() {} }));
    const formState = reactive({ browser: { groupId: "" }, selected: { id: "" }, group: { name: "" }, groupDialog: { open: false } });
    const wrapper = mount(SchemaUINode, {
      props: { node: document.children![0], depth: 1, formState, context: { formState, routeQuery: { tab: "groups" } }, sessionId: "s", extensionId: "com.amitia/emote", contributionId: "emote-page" },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      await wrapper.find('[data-node-id="emote-group-create"]').trigger("click");
      expect(wrapper.emitted("action")![0][0]).toMatchObject({ action: { input: { action: "ui.group_dialog", payload: { open: true } } } });
      formState.groupDialog.open = true;
      await nextTick();
      await wrapper.find('[data-node-id="emote-group-confirm-create"]').trigger("click");
      expect(warning).toHaveBeenCalledWith("请先输入分组名称");
      expect(wrapper.emitted("action")).toHaveLength(1);
      await wrapper.find('[data-node-id="emote-group-name"] input').setValue("新分组");
      await wrapper.find('[data-node-id="emote-group-confirm-create"]').trigger("click");
      expect(formState.group.name).toBe("新分组");
      expect(wrapper.emitted("action")).toHaveLength(2);
      expect(wrapper.emitted("action")![1][0]).toMatchObject({ action: { input: { action: "groups.create" } }, followUpActions: [{ input: { action: "ui.dashboard" } }] });
    } finally {
      wrapper.unmount();
      warning.mockRestore();
    }
  });

  it("根据路由查询参数切换主页面标签", async () => {
    expect(validateDocument(sidebarDocument).valid).toBe(true);
    const formState = reactive({ browser: { groupId: "", groupName: "", query: "" }, picker: { open: false, emoteIds: [] }, selected: { id: "", name: "", meaning: "", keywords: "", aiEnabled: false } });
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
      expect(wrapper.find('[data-node-id="emote-groups-gallery"]').exists()).toBe(true);
      expect(wrapper.find(".el-tabs").exists()).toBe(false);
      await wrapper.setProps({ context: { formState, routeQuery: { tab: "settings" } } });
      expect(wrapper.find('[data-node-id="emote-settings-save"]').exists()).toBe(true);
      expect(wrapper.find('[data-node-id="emote-import"]').exists()).toBe(false);
      await wrapper.setProps({ context: { formState, routeQuery: {} } });
      expect(wrapper.find('[data-node-id="emote-gallery"]').exists()).toBe(true);
    } finally {
      wrapper.unmount();
    }
  });
});
