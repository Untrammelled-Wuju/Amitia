import { mount } from "@vue/test-utils";
import { createPinia } from "pinia";
import { createMemoryHistory, createRouter } from "vue-router";
import { afterEach, describe, expect, it } from "vitest";
import DesktopTitleBar from "../components/DesktopTitleBar.vue";

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
  document.documentElement.classList.remove("amitia-titlebar-shell", "amitia-desktop-shell");
});

describe("跨端窗口导航控件", () => {
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
