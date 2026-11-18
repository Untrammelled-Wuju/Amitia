import { mount } from "@vue/test-utils";
import { createMemoryHistory, createRouter } from "vue-router";
import { describe, expect, it } from "vitest";
import SettingsSidebar from "@/components/SettingsSidebar.vue";
import SettingsView from "@/views/settings/SettingsView.vue";

const slotStub = {
  props: ["slotId", "context", "dispatchOnly"],
  template: "<div :data-slot-id='slotId' :data-dispatch-only='dispatchOnly' :data-slot-context='JSON.stringify(context)'><slot /></div>",
};

async function routerFor(path: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: "/:pathMatch(.*)*", component: { template: "<div />" } }],
  });
  await router.push(path);
  await router.isReady();
  return router;
}

describe("settings shell extension slots", () => {
  it("mounts the settings header and section icon seats", async () => {
    const wrapper = mount(SettingsSidebar, {
      global: {
        plugins: [await routerFor("/settings/model/llm")],
        stubs: {
          ExtensionSlot: slotStub,
          RouterLink: { template: "<a><slot /></a>" },
        },
      },
    });
    expect(wrapper.find("[data-slot-id='settings.shell.header']").exists()).toBe(true);
    const icons = wrapper.findAll("[data-slot-id='settings.section.icon']");
    expect(icons.length).toBeGreaterThan(0);
    expect(icons.some((icon) => icon.attributes("data-dispatch-only") === "model")).toBe(true);
    wrapper.unmount();
  });

  it("mounts actions, close and onboarding inside the settings shell", async () => {
    const wrapper = mount(SettingsView, {
      global: {
        plugins: [await routerFor("/settings/system")],
        stubs: {
          ExtensionSlot: slotStub,
          RouterView: { template: "<div />" },
        },
      },
    });
    expect(wrapper.find("[data-slot-id='settings.shell']").exists()).toBe(true);
    expect(wrapper.find("[data-slot-id='settings.shell.action']").exists()).toBe(true);
    expect(wrapper.find("[data-slot-id='settings.shell.close']").exists()).toBe(true);
    expect(wrapper.find("[data-slot-id='settings.onboarding']").exists()).toBe(true);
    expect(wrapper.find("[data-slot-id='extension.settings.tab']").exists()).toBe(true);
    wrapper.unmount();
  });
});
