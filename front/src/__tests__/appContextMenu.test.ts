import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { afterEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";

vi.mock("@/runtime/runtime-capabilities", () => ({
  isDesktopShell: () => true,
}));

import AppContextMenu from "@/components/context-menu/AppContextMenu.vue";
import {
  closeContextMenu,
  contextMenuState,
  openContextMenu,
} from "@/components/context-menu/contextMenuStore";

function pressKey(key: string) {
  const target = document.activeElement instanceof HTMLElement ? document.activeElement : document.body;
  const event = new KeyboardEvent("keydown", {
    key,
    bubbles: true,
    cancelable: true,
  });
  target.dispatchEvent(event);
  return event;
}

describe("桌面端通用右键菜单", () => {
  afterEach(() => {
    closeContextMenu();
    document.body.innerHTML = "";
  });

  it("在非消息区域拦截原生菜单并仅提供主题切换", async () => {
    const wrapper = mount(AppContextMenu, {
      global: { plugins: [ElementPlus] },
    });
    const target = document.createElement("div");
    target.textContent = "通用页面内容";
    document.body.appendChild(target);

    const event = new MouseEvent("contextmenu", {
      bubbles: true,
      cancelable: true,
      clientX: 120,
      clientY: 80,
    });
    target.dispatchEvent(event);
    await nextTick();

    expect(event.defaultPrevented).toBe(true);
    expect(contextMenuState.visible).toBe(true);
    const menu = document.body.querySelector("[data-amitia-context-menu]");
    expect(menu).not.toBeNull();
    expect(menu?.textContent).toContain("切换为浅色模式");
    expect(menu?.textContent).not.toContain("复制当前页面地址");
    expect(menu?.textContent).not.toContain("后退");
    expect(menu?.textContent).not.toContain("前进");
    expect(menu?.textContent).not.toContain("刷新");
    expect(menu?.textContent).not.toContain("打开对话");
    expect(menu?.textContent).not.toContain("归档");

    wrapper.unmount();
  });

  it("聚焦首个可用项并支持单键直接执行", async () => {
    const wrapper = mount(AppContextMenu, {
      global: { plugins: [ElementPlus] },
    });
    const disabledAction = vi.fn();
    const openAction = vi.fn();
    openContextMenu({
      x: 80,
      y: 60,
      items: [
        {
          id: "disabled",
          label: "不可用操作",
          accessKey: "D",
          disabled: true,
          action: disabledAction,
        },
        {
          id: "open",
          label: "打开",
          accessKey: "O",
          action: openAction,
        },
      ],
    });
    await nextTick();
    await nextTick();

    expect(document.activeElement?.textContent).toContain("打开");

    const disabledEvent = pressKey("d");
    expect(disabledEvent.defaultPrevented).toBe(true);
    expect(disabledAction).not.toHaveBeenCalled();
    expect(contextMenuState.visible).toBe(true);

    const openEvent = pressKey("o");
    expect(openEvent.defaultPrevented).toBe(true);
    expect(openAction).toHaveBeenCalledOnce();
    expect(contextMenuState.visible).toBe(false);

    wrapper.unmount();
  });

  it("支持方向键、首尾键和回车执行", async () => {
    const wrapper = mount(AppContextMenu, {
      global: { plugins: [ElementPlus] },
    });
    const firstAction = vi.fn();
    const lastAction = vi.fn();
    openContextMenu({
      x: 80,
      y: 60,
      items: [
        { id: "first", label: "第一项", accessKey: "F", action: firstAction },
        { id: "disabled", label: "禁用项", accessKey: "X", disabled: true },
        { id: "last", label: "最后一项", accessKey: "L", action: lastAction },
      ],
    });
    await nextTick();
    await nextTick();

    pressKey("ArrowDown");
    expect(document.activeElement?.textContent).toContain("最后一项");
    pressKey("Home");
    expect(document.activeElement?.textContent).toContain("第一项");
    pressKey("End");
    expect(document.activeElement?.textContent).toContain("最后一项");
    pressKey("Enter");
    expect(lastAction).toHaveBeenCalledOnce();
    expect(firstAction).not.toHaveBeenCalled();
    expect(contextMenuState.visible).toBe(false);

    wrapper.unmount();
  });

  it("支持子菜单展开、方向导航、返回与分层退出", async () => {
    const wrapper = mount(AppContextMenu, {
      global: { plugins: [ElementPlus] },
    });
    openContextMenu({
      x: 80,
      y: 60,
      items: [
        {
          id: "move",
          label: "移动到项目",
          accessKey: "V",
          children: [
            { id: "child-first", label: "项目一", accessKey: "A", action: vi.fn() },
            { id: "child-disabled", label: "项目二", accessKey: "B", disabled: true },
            { id: "child-last", label: "项目三", accessKey: "C", action: vi.fn() },
          ],
        },
      ],
    });
    await nextTick();
    await nextTick();

    pressKey("ArrowRight");
    await nextTick();
    await nextTick();
    expect(document.body.querySelector(".context-submenu")).not.toBeNull();
    expect(document.activeElement?.textContent).toContain("项目一");

    pressKey("ArrowDown");
    expect(document.activeElement?.textContent).toContain("项目三");
    pressKey("ArrowLeft");
    await nextTick();
    expect(document.body.querySelector(".context-submenu")).toBeNull();
    expect(document.activeElement?.textContent).toContain("移动到项目");

    pressKey("ArrowRight");
    await nextTick();
    await nextTick();
    pressKey("Escape");
    await nextTick();
    expect(contextMenuState.visible).toBe(true);
    expect(document.body.querySelector(".context-submenu")).toBeNull();

    pressKey("Escape");
    expect(contextMenuState.visible).toBe(false);

    wrapper.unmount();
  });
});
