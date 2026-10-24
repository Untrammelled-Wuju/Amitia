import { mount } from "@vue/test-utils";
import { computed, defineComponent } from "vue";
import { createMemoryHistory, createRouter, useRoute } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import SettingsView from "../views/settings/SettingsView.vue";
import SettingsSidebar from "../components/SettingsSidebar.vue";
import AccountSidebar from "../components/AccountSidebar.vue";
import MySpaceView from "../views/user-settings/MySpaceView.vue";

vi.mock("../components/extension/ExtensionSlot.vue", async () => {
  const { defineComponent } = await import("vue");
  return { default: defineComponent({
    props: ["slotId", "context"],
    template: '<div :data-slot="slotId" :data-route="context.route" />',
  }) };
});

async function renderSettings(path: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: "/settings", component: SettingsView, children: [{
      path: ":section/:pathMatch(.*)*",
      component: defineComponent({ template: '<div>设置内容</div>' }),
    }] },
      { path: "/my-space", component: MySpaceView, redirect: "/my-space/profile", children: [
        { path: "profile", component: defineComponent({ template: '<div data-space-section="profile">个人资料内容</div>' }) },
        { path: "identity", component: defineComponent({ template: '<div data-space-section="identity">身份信息内容</div>' }) },
        { path: "runtime", component: defineComponent({ template: '<div data-space-section="runtime">运行模式内容</div>' }) },
      ] },
      { path: "/chat", component: defineComponent({ template: '<div>会话内容</div>' }) },
      { path: "/character", component: defineComponent({ template: '<div>角色内容</div>' }) },
    ],
  });
  await router.push(path);
  await router.isReady();
  const wrapper = mount(defineComponent({
    components: { SettingsSidebar, AccountSidebar },
    setup() {
      const currentRoute = useRoute();
      const isSettings = computed(() => currentRoute.path === "/settings" || currentRoute.path.startsWith("/settings/"));
      const isAccount = computed(() => currentRoute.path === "/my-space" || currentRoute.path.startsWith("/my-space/"));
      return { isAccount, isSettings };
    },
    template: `<div data-main-shell class="app-main"><AccountSidebar v-if="isAccount" /><SettingsSidebar v-else-if="isSettings" /><main><router-view /></main></div>`,
  }), { global: { plugins: [router] } });
  return { wrapper, router };
}

describe("桌面设置分类导航", () => {
  it("普通设置使用精简背景，切换关于和日志页面时恢复原样式", async () => {
    const { wrapper, router } = await renderSettings("/settings/system");
    try {
      expect(wrapper.get(".settings-page").classes()).toContain("settings-refined");
      expect(wrapper.find(".settings-sidebar").exists()).toBe(true);
      expect(wrapper.find(".secondary-workspace").exists()).toBe(false);
      for (const path of ["/settings/about", "/settings/system-logs", "/settings/prompt-trace"]) {
        await router.push(path);
        expect(wrapper.get(".settings-page").classes()).not.toContain("settings-refined");
      }
      await router.push("/settings/model/providers");
      expect(wrapper.get(".settings-page").classes()).toContain("settings-refined");
    } finally { wrapper.unmount(); }
  });
  it("保留全部入口及插件插槽，并识别模型子页面", async () => {
    const { wrapper } = await renderSettings("/settings/model/providers");
    try {
      expect(wrapper.findAll("nav section")).toHaveLength(7);
      const links = wrapper.findAll("nav a");
      expect(links).toHaveLength(21);
      expect(new Set(links.map(link => link.attributes("href"))).size).toBe(21);
      expect(wrapper.find('a[href="/settings/deployment"]').exists()).toBe(false);
      expect(wrapper.find('a[href="/settings/feedback"]').exists()).toBe(true);
      const privacyGroup = wrapper.findAll("nav section").find(section => section.text().startsWith("隐私"));
      expect(privacyGroup?.findAll("a").map(link => link.text())).toEqual(["隐私说明", "使用边界", "隐私扫描"]);
      expect(links.slice(0, 4).map(link => link.text())).toEqual(["通用", "外观", "通知", "时间与地区"]);
      expect(wrapper.find('a[href="/settings/data-management"]').exists()).toBe(true);
      expect(wrapper.find('a[href="/settings/model"]').attributes('aria-current')).toBe('page');
      expect((wrapper.get("select").element as HTMLSelectElement).value).toBe("/settings/model");
      expect(wrapper.findAll("[data-slot]").map(slot => slot.attributes("data-slot"))).toEqual([
        "system.status.item", "system.settings.section", "extension.settings.section", "extension.settings.page",
      ]);
    } finally { wrapper.unmount(); }
  });

  it("紧凑导航跳转后同步内容路由和插件上下文", async () => {
    const { wrapper, router } = await renderSettings("/settings/system");
    try {
      await wrapper.get("select").setValue("/settings/maintenance");
      await vi.waitFor(() => expect(router.currentRoute.value.path).toBe("/settings/maintenance"));
      expect(wrapper.get('[data-slot="extension.settings.page"]').attributes("data-route")).toBe("/settings/maintenance");
    } finally { wrapper.unmount(); }
  });

  it("设置复用主工作区第二栏，不再进入独立工作区", async () => {
    const { wrapper, router } = await renderSettings("/chat?conversationId=test&workspace=local");
    try {
      await router.push("/settings/system");
      expect(wrapper.get("[data-main-shell]").classes()).toContain("app-main");
      expect(wrapper.find(".settings-sidebar").exists()).toBe(true);
      expect(wrapper.find(".secondary-workspace").exists()).toBe(false);
      expect(wrapper.find(".workspace-back").exists()).toBe(false);
    } finally { wrapper.unmount(); }
  });

  it("我的空间直接使用主工作区，不再进入独立返回页", async () => {
    const { wrapper, router } = await renderSettings("/my-space");
    try {
      expect(router.currentRoute.value.path).toBe("/my-space/profile");
      expect(wrapper.get("[data-main-shell]").classes()).toContain("app-main");
      expect(wrapper.find(".account-sidebar").exists()).toBe(true);
      expect(wrapper.find(".secondary-workspace").exists()).toBe(false);
      expect(wrapper.find(".workspace-back").exists()).toBe(false);
    } finally { wrapper.unmount(); }
  });

  it("我的空间第二栏只保留资料、身份信息和运行模式", async () => {
    const { wrapper, router } = await renderSettings('/my-space');
    try {
      expect(wrapper.find(".settings-sidebar").exists()).toBe(false);
      expect(wrapper.findAll('.account-navigation a').map(link => link.text())).toEqual(['资料与头像', '身份信息', '运行模式']);
      expect(wrapper.find('[data-space-section="profile"]').exists()).toBe(true);
      for (const section of ['identity', 'runtime', 'profile']) {
        await wrapper.get(`a[href="/my-space/${section}"]`).trigger('click');
        await vi.waitFor(() => expect(router.currentRoute.value.path).toBe(`/my-space/${section}`));
        expect(wrapper.find(`[data-space-section="${section}"]`).exists()).toBe(true);
        expect(wrapper.findAll('[data-space-section]')).toHaveLength(1);
        expect(wrapper.get(`a[href="/my-space/${section}"]`).attributes('aria-current')).toBe('page');
      }
      expect(wrapper.find('a[href="/settings/system"]').exists()).toBe(false);
    } finally { wrapper.unmount(); }
  });
});
