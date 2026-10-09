import { mount } from "@vue/test-utils";
import { defineComponent } from "vue";
import { createMemoryHistory, createRouter } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import SettingsView from "../views/settings/SettingsView.vue";
import SettingsWorkspace from "../components/SettingsWorkspace.vue";
import AccountWorkspace from "../components/AccountWorkspace.vue";
import MySpaceView from "../views/user-settings/MySpaceView.vue";
import { useSecondaryWorkspace } from "../composables/useSecondaryWorkspace";

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
        { path: "identity", component: defineComponent({ template: '<div data-space-section="identity">Space 身份内容</div>' }) },
        { path: "runtime", component: defineComponent({ template: '<div data-space-section="runtime">运行模式内容</div>' }) },
        { path: "devices", component: defineComponent({ template: '<div data-space-section="devices">设备配对内容</div>' }) },
      ] },
      { path: "/chat", component: defineComponent({ template: '<div>会话内容</div>' }) },
      { path: "/character", component: defineComponent({ template: '<div>角色内容</div>' }) },
    ],
  });
  await router.push(path);
  await router.isReady();
  const wrapper = mount(defineComponent({
    components: { SettingsWorkspace, AccountWorkspace },
    setup: useSecondaryWorkspace,
    template: `<component :is="secondaryPage.kind === 'settings' ? 'SettingsWorkspace' : 'AccountWorkspace'" v-if="secondaryPage" :return-to="returnTo" :title="secondaryPage.title"><router-view /></component><div v-else data-main-shell><nav>新对话 角色列表</nav><router-view /></div>`,
  }), { global: { plugins: [router] } });
  return { wrapper, router };
}

describe("桌面设置分类导航", () => {
  it("普通设置使用精简背景，切换关于和日志页面时恢复原样式", async () => {
    const { wrapper, router } = await renderSettings("/settings/system");
    try {
      expect(wrapper.get(".secondary-workspace").classes()).toContain("settings-refined");
      for (const path of ["/settings/about", "/settings/system-logs", "/settings/prompt-trace"]) {
        await router.push(path);
        expect(wrapper.get(".secondary-workspace").classes()).not.toContain("settings-refined");
      }
      await router.push("/settings/model/providers");
      expect(wrapper.get(".secondary-workspace").classes()).toContain("settings-refined");
    } finally { wrapper.unmount(); }
  });
  it("保留全部入口及插件插槽，并识别模型子页面", async () => {
    const { wrapper } = await renderSettings("/settings/model/providers");
    try {
      expect(wrapper.findAll("nav section")).toHaveLength(7);
      const links = wrapper.findAll("nav a");
      expect(links).toHaveLength(20);
      expect(new Set(links.map(link => link.attributes("href"))).size).toBe(20);
      expect(wrapper.find('a[href="/settings/deployment"]').exists()).toBe(false);
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

  it("设置和我的空间返回原会话且保留查询参数", async () => {
    const { wrapper, router } = await renderSettings("/chat?conversationId=test&workspace=local");
    try {
      for (const path of ["/settings/system", "/my-space"]) {
        await router.push(path);
        expect(wrapper.find("[data-main-shell]").exists()).toBe(false);
        expect(wrapper.get('.workspace-back').attributes('href')).toBe('/chat?conversationId=test&workspace=local');
        expect(wrapper.get('.workspace-back').attributes('aria-label')).toBe('返回主界面');
        expect(wrapper.get('.workspace-back').text()).toBe('');
        expect(wrapper.get('.secondary-header').element.firstElementChild).toBe(wrapper.get('.workspace-back').element);
        expect(wrapper.find(".settings-navigation").exists()).toBe(path.startsWith("/settings"));
        expect(wrapper.find(".secondary-sidebar").exists()).toBe(true);
        if (!path.startsWith("/settings")) {
          const links = wrapper.findAll('.account-navigation a');
          expect(links).toHaveLength(4);
          expect(links.every(link => link.attributes('href')?.startsWith(`${path}/`))).toBe(true);
        }
      }
      await wrapper.get('.workspace-back').trigger('click');
      await vi.waitFor(() => expect(router.currentRoute.value.fullPath).toBe('/chat?conversationId=test&workspace=local'));
      expect(wrapper.find("[data-main-shell]").exists()).toBe(true);
      expect(wrapper.find('.secondary-sidebar').exists()).toBe(false);
    } finally { wrapper.unmount(); }
  });

  it("直接打开设置返回聊天，重新进入设置记住新的来源", async () => {
    const { wrapper, router } = await renderSettings("/my-space");
    try {
      expect(wrapper.get('.workspace-back').attributes('href')).toBe('/chat');
      await router.push('/character?tab=list');
      await router.push('/my-space');
      expect(wrapper.get('.workspace-back').attributes('href')).toBe('/character?tab=list');
    } finally { wrapper.unmount(); }
  });

  it("我的空间统一侧栏覆盖资料、运行模式和设备管理", async () => {
    const { wrapper, router } = await renderSettings('/my-space');
    try {
      expect(wrapper.findAll('.account-navigation a').map(link => link.text())).toEqual(['资料与头像', 'Space 身份', '运行模式', '设备配对']);
      expect(wrapper.find('[data-space-section="profile"]').exists()).toBe(true);
      for (const section of ['identity', 'runtime', 'devices', 'profile']) {
        await wrapper.get(`a[href="/my-space/${section}"]`).trigger('click');
        await vi.waitFor(() => expect(router.currentRoute.value.path).toBe(`/my-space/${section}`));
        expect(wrapper.find(`[data-space-section="${section}"]`).exists()).toBe(true);
        expect(wrapper.findAll('[data-space-section]')).toHaveLength(1);
        expect(wrapper.get(`a[href="/my-space/${section}"]`).attributes('aria-current')).toBe('page');
      }
      expect(wrapper.get('.workspace-back').attributes('href')).toBe('/chat');
      expect(wrapper.find('a[href="/settings/system"]').exists()).toBe(false);
    } finally { wrapper.unmount(); }
  });
});
