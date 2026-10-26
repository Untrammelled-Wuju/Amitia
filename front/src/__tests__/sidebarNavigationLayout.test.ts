import { mount } from "@vue/test-utils";
import { createPinia } from "pinia";
import { createMemoryHistory, createRouter } from "vue-router";
import ElementPlus from "element-plus";
import { describe, expect, it, vi } from "vitest";
import SideNav from "../components/SideNav.vue";

vi.mock("../composables/useBrandLogo", () => ({
  useBrandLogo: () => ({ logoUrl: "/test-logo.svg" }),
}));
vi.mock("../runtime/runtime-capabilities", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../runtime/runtime-capabilities")>();
  return {
    ...actual,
    isRuntimeCapabilityAvailable: (capability: string) => capability === "gameMode",
  };
});

describe("图标导航栏布局", () => {
  it("使用统一导航注册表渲染全部一级图标入口", async () => {
    const pinia = createPinia();
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: "/:pathMatch(.*)*", component: { template: "<div />" } }],
    });
    await router.push("/continuity");
    await router.isReady();
    const wrapper = mount(SideNav, {
      global: { plugins: [pinia, router, ElementPlus] },
    });
    try {
      expect(wrapper.get(".side-nav").classes()).not.toContain("is-collapsed");
      expect(wrapper.findAll(".el-menu-item").map(item => item.text())).toEqual([
        "主页",
        "角色",
        "记忆",
        "主动任务",
        "持续事项",
        "游戏模式",
        "创意工坊",
        "扩展中心",
      ]);
      expect(wrapper.find(".user-profile").exists()).toBe(true);
    } finally {
      wrapper.unmount();
    }
  });

  it("悬浮一级图标时显示名称和简短介绍", async () => {
    const pinia = createPinia();
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: "/:pathMatch(.*)*", component: { template: "<div />" } }],
    });
    await router.push("/chat");
    await router.isReady();
    const wrapper = mount(SideNav, {
      attachTo: document.body,
      global: { plugins: [pinia, router, ElementPlus] },
    });
    try {
      const memoryItem = wrapper.findAll(".el-menu-item").find(item => item.text().includes("记忆"));
      expect(memoryItem).toBeTruthy();
      await memoryItem!.trigger("mouseenter");
      await vi.waitFor(() => {
        const tooltip = document.body.querySelector("#side-nav-tooltip");
        expect(tooltip?.textContent).toContain("记忆");
        expect(tooltip?.textContent).toContain("知识库、用户画像、记忆图谱、情景和时间线");
      });
    } finally {
      wrapper.unmount();
      document.body.querySelector("#side-nav-tooltip")?.remove();
    }
  });
});
