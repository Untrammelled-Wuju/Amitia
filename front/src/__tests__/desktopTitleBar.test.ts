import { mount } from "@vue/test-utils";
import { createPinia } from "pinia";
import { createMemoryHistory, createRouter } from "vue-router";
import { afterEach, describe, expect, it } from "vitest";
import DesktopTitleBar from "../components/DesktopTitleBar.vue";
import indexHtmlSource from "../../index.html?raw";
import appSource from "../App.vue?raw";
import backgroundSource from "../components/ApplicationBackground.vue?raw";
import appShellSource from "../components/BuiltinAppShell.vue?raw";
import titleBarSource from "../components/DesktopTitleBar.vue?raw";

function installDesktopApi(platform: string) {
  (window as any).amitiaDesktop = {};
  (window as any).electronWindowApi = {
    platform,
    getWindowState: async () => ({ fullscreen: false, maximized: false }),
    onWindowStateChanged: () => () => {},
    minimize: async () => {},
    toggleMaximize: async () => false,
    close: async () => {},
    syncCaptionColors: async () => {},
  };
}

async function renderTitleBar(path = "/chat") {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: "/:pathMatch(.*)*", component: { template: "<div />" } }],
  });
  await router.push(path);
  await router.isReady();
  return mount(DesktopTitleBar, {
    global: { plugins: [createPinia(), router] },
  });
}

afterEach(() => {
  delete (window as any).amitiaDesktop;
  delete (window as any).electronWindowApi;
  document.querySelectorAll(".el-overlay, .search-overlay").forEach((element) => element.remove());
  document.documentElement.classList.remove("amitia-window-overlay");
  document.documentElement.classList.remove("amitia-titlebar-shell", "amitia-desktop-shell");
});

describe("跨端窗口导航控件", () => {
  it("桌面标题栏作为常规布局行，不再通过视口高度补偿", () => {
    expect(appSource).toContain("grid-template-rows: var(--desktop-titlebar-height, 40px) minmax(0, 1fr)");
    expect(appSource).not.toContain("html.amitia-titlebar-shell body {");
    expect(appSource).not.toContain("height: calc(100vh - var(--desktop-titlebar-height");
    expect(titleBarSource).not.toMatch(/#WindowControlButtons\s*\{[^}]*position: fixed/);
    expect(titleBarSource).not.toMatch(/#WindowControlButtons\s*\{[^}]*z-index:/);
    expect(titleBarSource).not.toContain("-webkit-backdrop-filter: blur");
    expect(titleBarSource).not.toContain("filter: brightness");
    expect(appShellSource).toContain("height: 100%;");
    expect(backgroundSource).toContain("position: absolute !important");
    expect(appSource).toContain("html.amitia-titlebar-shell .el-overlay {\n  top: 0 !important;");
    expect(appSource).toContain("z-index: 20000 !important;");
    expect(titleBarSource).toContain("class=\"window-drag-region\"");
    expect(titleBarSource).toContain("'is-overlay-blocked': windowLayerBlocked");
    expect(titleBarSource).not.toContain(":global(");
    expect(appSource).not.toContain("height: calc(100% - var(--desktop-titlebar-height");
    expect(indexHtmlSource).toContain('<div id="amitia-overlay-root" aria-live="polite"></div>');
    expect(appSource).not.toContain('id="amitia-overlay-root"');
  });

  it("弹窗出现时标题栏退出原生拖拽区域并停止接收点击", async () => {
    const wrapper = await renderTitleBar();
    try {
      const overlay = document.createElement("div");
      overlay.className = "el-overlay";
      overlay.style.opacity = "0";
      document.body.append(overlay);
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(wrapper.classes()).toContain("is-overlay-blocked");
      expect(wrapper.attributes("inert")).toBeDefined();
      overlay.remove();
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(wrapper.classes()).not.toContain("is-overlay-blocked");
      expect(wrapper.attributes("inert")).toBeUndefined();
    } finally {
      wrapper.unmount();
    }
  });

  it("Web 保留侧栏开关和前进后退，隐藏窗口操作", async () => {
    const wrapper = await renderTitleBar();
    try {
      expect(wrapper.classes()).toContain("is-web");
      expect(wrapper.find(".sidebar-toggle-btn").exists()).toBe(true);
      expect(wrapper.findAll(".nav-icon-btn")).toHaveLength(2);
      expect(wrapper.find(".window-actions").exists()).toBe(false);
      expect(wrapper.classes()).toContain("is-chat-page");
      expect(wrapper.find("#window-chat-slot").exists()).toBe(true);
    } finally {
      wrapper.unmount();
    }
  });

  it("Linux 桌面显示自定义窗口操作", async () => {
    installDesktopApi("linux");
    const wrapper = await renderTitleBar();
    try {
      expect(wrapper.classes()).not.toContain("is-web");
      expect(wrapper.findAll(".window-actions .icon")).toHaveLength(3);
    } finally {
      wrapper.unmount();
    }
  });

  it("macOS 使用系统交通灯而不是重复绘制窗口按钮", async () => {
    installDesktopApi("darwin");
    const wrapper = await renderTitleBar();
    try {
      expect(wrapper.classes()).toContain("is-macos");
      expect(wrapper.find(".window-actions").exists()).toBe(false);
    } finally {
      wrapper.unmount();
    }
  });

  it("非聊天页面不启用聊天标题与操作插槽", async () => {
    const wrapper = await renderTitleBar("/memory");
    try {
      expect(wrapper.classes()).not.toContain("is-chat-page");
      expect(wrapper.find("#window-chat-slot").exists()).toBe(true);
    } finally {
      wrapper.unmount();
    }
  });
});
