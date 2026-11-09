import { mount } from "@vue/test-utils";
import { createPinia } from "pinia";
import { createMemoryHistory, createRouter } from "vue-router";
import ElementPlus from "element-plus";
import { computed, ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import SideNav from "../components/SideNav.vue";

vi.mock("../composables/useBrandLogo", () => ({
  useBrandLogo: () => ({ logoUrl: "/test-logo.svg" }),
}));

vi.mock("../ui-runtime/navigationRegistry", () => ({
  useUINavigationRegistry: () => {
    const items = ref([
      { id: "home", route: "/chat", label: "主页", placement: "primary", icon: "span", group: "home", description: "主页说明" },
      { id: "memory", route: "/memory", label: "记忆", placement: "primary", icon: "span", group: "memory", description: "记忆说明" },
      { id: "settings", route: "/settings", label: "设置", placement: "profile", icon: "span", group: "settings", description: "设置说明" },
      { id: "logs", route: "/logs", label: "归档对话", placement: "profile", icon: "span", group: "logs", description: "归档说明" },
      { id: "storage", route: "/storage", label: "存储维护", placement: "settings", icon: "span", group: "settings", groupLabel: "设置", description: "存储说明" },
      { id: "runtime-debug", route: "/runtime-debug", label: "运行时调试", placement: "settings", icon: "span", group: "settings", groupLabel: "设置", description: "调试说明" },
      { id: "decision-viz", route: "/decision-viz", label: "BDI 决策可视化", placement: "settings", icon: "span", group: "settings", groupLabel: "设置", description: "决策说明" },
      { id: "long-running", route: "/long-running", label: "长期运行维护", placement: "settings", icon: "span", group: "settings", groupLabel: "设置", description: "维护说明" },
      { id: "workspace", route: "/workspaces", label: "工作区", placement: "workspace", icon: "span", group: "workspace", description: "工作区说明" },
    ]);
    return {
      groups: ref([]),
      items,
      primaryItems: computed(() => items.value.filter((item) => item.placement === "primary")),
    };
  },
  isUINavigationItemActive: (path: string, item: { route: string }) => path === item.route,
}));

describe("桌面图标导航栏", () => {
  it("只渲染一级入口，不生成悬浮二级菜单", async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: "/:pathMatch(.*)*", component: { template: "<div />" } }],
    });
    await router.push("/chat");
    await router.isReady();
    const wrapper = mount(SideNav, {
      attachTo: document.body,
      global: { plugins: [createPinia(), router, ElementPlus] },
    });
    try {
      expect(wrapper.findAll(".el-sub-menu")).toHaveLength(0);
      expect(wrapper.findAll(".el-menu-item").map((item) => item.text())).toEqual(["主页", "记忆"]);
      await wrapper.findAll(".el-menu-item")[1].trigger("mouseenter");
      expect(document.body.querySelector(".el-menu--popup")).toBeNull();
      await wrapper.get(".user-profile").trigger("click");
      expect(wrapper.findAll(".profile-menu__item").map((item) => item.text())).toContain("设置");
      expect(wrapper.findAll(".profile-menu__item").map((item) => item.text())).toContain("归档对话");
      await wrapper.findAll(".el-menu-item")[1].trigger("click");
      await vi.waitFor(() => expect(router.currentRoute.value.path).toBe("/memory"));
    } finally {
      wrapper.unmount();
      document.body.querySelector("#side-nav-tooltip")?.remove();
    }
  });
});
