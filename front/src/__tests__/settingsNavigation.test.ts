import { mount } from "@vue/test-utils";
import { defineComponent } from "vue";
import { createMemoryHistory, createRouter } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import SettingsView from "../views/settings/SettingsView.vue";
import SettingsWorkspace from "../components/SettingsWorkspace.vue";
import AccountWorkspace from "../components/AccountWorkspace.vue";
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
      { path: "/user-settings", component: defineComponent({ template: '<div>个人资料内容</div>' }) },
      { path: "/devices", component: defineComponent({ template: '<div>设备内容</div>' }) },
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
  it("保留全部入口及插件插槽，并识别模型子页面", async () => {
    const { wrapper } = await renderSettings("/settings/model/providers");
    try {
      expect(wrapper.findAll("nav section")).toHaveLength(6);
      const links = wrapper.findAll("nav a");
      expect(links).toHaveLength(15);
      expect(new Set(links.map(link => link.attributes("href"))).size).toBe(15);
      expect(wrapper.find('a[href="/settings/model"]').classes()).toContain("settings-navigation-item-active");
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

  it("三个独立页面返回原会话且保留查询参数", async () => {
    const { wrapper, router } = await renderSettings("/chat?conversationId=test&workspace=local");
    try {
      for (const path of ["/settings/system", "/user-settings", "/devices"]) {
        await router.push(path);
        expect(wrapper.find("[data-main-shell]").exists()).toBe(false);
        expect(wrapper.get('.workspace-back').attributes('href')).toBe('/chat?conversationId=test&workspace=local');
        expect(wrapper.find(".settings-navigation").exists()).toBe(path.startsWith("/settings"));
        expect(wrapper.find(".secondary-sidebar").exists()).toBe(true);
        if (!path.startsWith("/settings")) {
          const links = wrapper.findAll('.account-navigation a');
          expect(links).toHaveLength(3);
          expect(links.every(link => link.attributes('href')?.startsWith(`${path}#`))).toBe(true);
        }
      }
      await wrapper.get('.workspace-back').trigger('click');
      await vi.waitFor(() => expect(router.currentRoute.value.fullPath).toBe('/chat?conversationId=test&workspace=local'));
      expect(wrapper.find("[data-main-shell]").exists()).toBe(true);
      expect(wrapper.find('.secondary-sidebar').exists()).toBe(false);
    } finally { wrapper.unmount(); }
  });

  it("直接打开设置返回聊天，重新进入设置记住新的来源", async () => {
    const { wrapper, router } = await renderSettings("/devices");
    try {
      expect(wrapper.get('.workspace-back').attributes('href')).toBe('/chat');
      await router.push('/character?tab=list');
      await router.push('/user-settings');
      expect(wrapper.get('.workspace-back').attributes('href')).toBe('/character?tab=list');
    } finally { wrapper.unmount(); }
  });

  it("独立设备侧栏定位本页分区，不显示设置入口", async () => {
    const { wrapper, router } = await renderSettings('/devices');
    try {
      expect(wrapper.findAll('.account-navigation a').map(link => link.text())).toEqual(['设备配对', '当前设备', '已绑定设备']);
      await wrapper.get('a[href="/devices#bound-devices"]').trigger('click');
      await vi.waitFor(() => expect(router.currentRoute.value.hash).toBe('#bound-devices'));
      expect(wrapper.get('a[href="/devices#bound-devices"]').attributes('aria-current')).toBe('location');
      expect(wrapper.get('.workspace-back').attributes('href')).toBe('/chat');
      expect(wrapper.find('a[href="/settings/system"]').exists()).toBe(false);
    } finally { wrapper.unmount(); }
  });
});
