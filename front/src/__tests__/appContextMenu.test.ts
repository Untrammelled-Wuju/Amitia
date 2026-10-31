import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { afterEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";

vi.mock("@/runtime/runtime-capabilities", () => ({
  isDesktopShell: () => true,
}));

import AppContextMenu from "@/components/context-menu/AppContextMenu.vue";
import { closeContextMenu, contextMenuState } from "@/components/context-menu/contextMenuStore";

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
});
