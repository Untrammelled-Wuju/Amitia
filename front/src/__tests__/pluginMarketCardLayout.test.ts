import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia } from "pinia";
import { nextTick, reactive } from "vue";
import { describe, expect, it } from "vitest";
import SchemaUINode from "../components/extension/SchemaUINode.vue";
import { validateDocument, type SchemaUIDocument, type SchemaUINode as SchemaNode } from "../components/extension/schema-ui-utils";

const document = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-code/plugin-market/ui/schema/desktop.json"), "utf8")) as SchemaUIDocument;

function findNode(nodes: SchemaNode[], id: string): SchemaNode | undefined {
  for (const node of nodes) {
    if (node.id === id) return node;
    const found = findNode(node.children ?? [], id);
    if (found) return found;
  }
  return undefined;
}

describe("插件市场卡片布局", () => {
  it("使用固定工具栏和多操作插件卡片", async () => {
    expect(validateDocument(document).valid).toBe(true);
    const toolbar = findNode(document.children ?? [], "plugin-market-toolbar");
    const list = findNode(document.children ?? [], "plugin-market-list");
    const allCategories = findNode(document.children ?? [], "plugin-market-all-categories");
    const detail = findNode(document.children ?? [], "plugin-market-detail-dialog");
    const review = findNode(document.children ?? [], "plugin-market-review");
    const permissions = findNode(review?.children ?? [], "plugin-market-permissions");
    const installAction = (list?.props?.itemActions as Array<Record<string, unknown>>).find((action) => action.id === "install");
    const uninstallAction = (list?.props?.itemActions as Array<Record<string, unknown>>).find((action) => action.id === "uninstall");
    const confirmInstall = findNode(review?.children ?? [], "plugin-market-install");
    const confirmAction = (confirmInstall?.actions as Array<Record<string, unknown>> | undefined)?.[0];
    expect(toolbar?.props?.sticky).toBe(true);
    expect(list?.props?.variant).toBe("cards");
    expect((list?.props?.itemActions as unknown[]).length).toBe(3);
    expect(allCategories?.props?.text).toBe("全部分类");
    expect(detail?.props?.presentation).toBe("dialog");
    expect(review?.props?.presentation).toBe("dialog");
    expect(confirmInstall?.visibleWhen).toBeUndefined();
    expect(confirmInstall?.disabledWhen).toEqual([
      expect.objectContaining({ field: "formState.installConfirmed", operator: "==", value: false }),
    ]);
    expect(permissions?.type).toBe("permission_summary");
    expect(permissions?.bindings).toEqual([
      expect.objectContaining({ path: "permissions", source: "state", sourcePath: "review.permissions" }),
    ]);
    expect(installAction?.visibleWhen).toEqual([
      expect.objectContaining({ field: "installable", operator: "==", value: true }),
    ]);
    expect(installAction?.followUpActions).toEqual([
      expect.objectContaining({ action_id: "reset-confirmation", target: "local", formPath: "installConfirmed", value: false }),
      expect.objectContaining({ action_id: "open-review", target: "local", statePath: "review.open", value: true }),
    ]);
    expect(uninstallAction).toEqual(expect.objectContaining({
      label: "卸载",
      action_id: "command",
      loadingText: "卸载中",
      successMessage: "卸载完成",
      target: "tool",
      input: expect.objectContaining({ action: "uninstall", extensionId: "$item.extensionId", confirmed: true }),
    }));
    expect(confirmAction?.followUpActions).toEqual([
      expect.objectContaining({ action_id: "close-review-after-install", target: "local", statePath: "review.open", value: false }),
      expect.objectContaining({ action_id: "command", target: "tool", statePath: "market" }),
    ]);

    const formState = reactive({});
    const context = {
      localState: {
        market: {
          listings: [
            {
              extensionId: "com.amitia/example",
              displayName: "示例插件",
              summary: "用于展示卡片",
              actionLabel: "已安装",
              actionDisabled: true,
              installed: true,
              installable: false,
              platformSupported: true,
              platformSummary: "支持 Windows、macOS",
              supportedPlatforms: ["windows", "macos"],
              publisher: "Amitia",
            },
          ],
        },
      },
    };
    const wrapper = mount(SchemaUINode, {
      props: {
        node: list!,
        depth: 1,
        formState,
        context,
        sessionId: "session",
        extensionId: "com.amitia/plugin-market",
        contributionId: "plugin-market-desktop-page",
      },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      const card = wrapper.get(".schema-ui-gallery__item--card");
      expect(card.text()).toContain("示例插件");
      expect(card.text()).not.toContain("已安装");
      expect(card.findAll(".schema-ui-gallery__platform-tag").map((tag) => tag.text())).toEqual(["Windows", "macOS"]);
      expect(card.findAll(".schema-ui-gallery__image")).toHaveLength(0);
      const buttons = card.findAll(".schema-ui-gallery__card-actions button");
      expect(buttons).toHaveLength(2);
      expect(buttons[0].text()).toBe("卸载");
      expect(buttons[1].text()).toBe("详情");
      await buttons[1].trigger("click");
      expect(wrapper.emitted("action")!.at(-1)![0]).toMatchObject({
        action: { action_id: "open-details", target: "local" },
        item: { extensionId: "com.amitia/example" },
      });
    } finally {
      wrapper.unmount();
    }
  });

  it("已安装且有更新时同时显示更新和卸载", async () => {
    const list = findNode(document.children ?? [], "plugin-market-list");
    const wrapper = mount(SchemaUINode, {
      props: {
        node: list!,
        depth: 1,
        formState: reactive({}),
        context: {
          localState: {
            market: {
              listings: [{
                extensionId: "com.amitia/updateable",
                displayName: "可更新插件",
                summary: "存在可安装的新版本",
                actionLabel: "更新",
                actionDisabled: false,
                installed: true,
                installable: true,
                updateAvailable: true,
                platformSupported: true,
                supportedPlatforms: ["windows"],
                publisher: "Amitia",
              }],
            },
          },
        },
        sessionId: "session",
        extensionId: "com.amitia/plugin-market",
        contributionId: "plugin-market-desktop-page",
      },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      const card = wrapper.get(".schema-ui-gallery__item--card");
      expect(card.text()).not.toContain("已安装");
      expect(card.findAll(".schema-ui-gallery__card-actions button").map((button) => button.text())).toEqual(["更新", "卸载", "详情"]);
    } finally {
      wrapper.unmount();
    }
  });

  it("搜索与分类工具栏标记为固定区域", () => {
    const toolbar = findNode(document.children ?? [], "plugin-market-toolbar");
    const wrapper = mount(SchemaUINode, {
      props: {
        node: toolbar!,
        depth: 1,
        formState: reactive({ pluginQuery: "" }),
        context: { localState: { market: { categories: [] } } },
        sessionId: "session",
        extensionId: "com.amitia/plugin-market",
        contributionId: "plugin-market-desktop-page",
      },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      expect(wrapper.get('[data-node-id="plugin-market-toolbar"]').classes()).toContain("schema-ui-node--sticky");
    } finally {
      wrapper.unmount();
    }
  });

  it("确认安装按钮始终显示并在接受选项前保持禁用", async () => {
    const review = findNode(document.children ?? [], "plugin-market-review");
    const button = findNode(review?.children ?? [], "plugin-market-install");
    const formState = reactive({ installConfirmed: false });
    const wrapper = mount(SchemaUINode, {
      props: {
        node: button!,
        depth: 1,
        formState,
        context: {
          formState,
          localState: {
            review: {
              operationLabel: "确认安装",
            },
          },
        },
        sessionId: "session",
        extensionId: "com.amitia/plugin-market",
        contributionId: "plugin-market-desktop-page",
      },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      const control = wrapper.get("button");
      expect(control.text()).toBe("确认安装");
      expect(control.attributes("disabled")).toBeDefined();
      formState.installConfirmed = true;
      await nextTick();
      expect(control.attributes("disabled")).toBeUndefined();
    } finally {
      wrapper.unmount();
    }
  });
});
