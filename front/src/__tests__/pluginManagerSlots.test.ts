import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia } from "pinia";
import { createMemoryHistory, createRouter } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import PackageManagerView from "@/views/extensions/packages/PackageManagerView.vue";

vi.mock("@/views/extensions/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/views/extensions/api")>();
  return {
    ...actual,
    getKernelStatus: vi.fn().mockResolvedValue({ ready: true, count: 1 }),
    fetchExtensionCenterDetail: vi.fn().mockResolvedValue({
      displayName: "Fixture",
      extensionId: "com.example/fixture",
      version: "1.0.0",
      enabled: true,
      effectiveState: "active",
      permissions: [],
      modules: [{
        moduleId: "root",
        kind: "javascript",
        runtime: "node",
        entry: "index.js",
        status: "active",
      }],
      contributions: [],
    }),
    fetchExtensionCenterView: vi.fn().mockResolvedValue({
      installed: [{
        displayName: "Fixture",
        extensionId: "com.example/fixture",
        version: "1.0.0",
        status: "installed",
        enabled: true,
        systemManaged: false,
      }],
      updates: [],
      needsAction: [],
      localImports: [],
      dev: [],
    }),
  };
});

vi.mock("@/views/kernel/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/views/kernel/api")>();
  return {
    ...actual,
    getKernelStatus: vi.fn().mockResolvedValue({ ready: true, count: 1 }),
  };
});

describe("plugin manager extension slots", () => {
  it("mounts tabs, official items and keyed activation guidance", async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: "/:pathMatch(.*)*", component: { template: "<div />" } }],
    });
    const pinia = createPinia();
    await router.push("/extensions/packages");
    await router.isReady();
    const wrapper = mount(PackageManagerView, {
      global: {
        plugins: [pinia, ElementPlus, router],
        stubs: {
          teleport: true,
          ElDropdown: { template: "<div><slot /></div>" },
          ElDropdownMenu: { template: "<div><slot /></div>" },
          ElDropdownItem: { template: "<div><slot /></div>" },
          ExtensionPageHeader: { template: "<header><slot name='actions' /></header>" },
          ExtensionSlot: {
            props: ["slotId", "dispatchKey", "context"],
            template: "<div :data-slot-id='slotId' :data-dispatch-key='dispatchKey' :data-slot-context='JSON.stringify(context)'><slot /></div>",
          },
        },
      },
    });
    await flushPromises();
    expect(wrapper.find("[data-slot-id='extension.plugin.tab']").exists()).toBe(true);
    expect(wrapper.find("[data-slot-id='extension.plugin.item']").exists()).toBe(true);
    expect(wrapper.find("[data-slot-id='extension.center.header.action']").exists()).toBe(true);
    expect(wrapper.find("[data-slot-id='extension.center.card.badge']").exists()).toBe(true);
    const activation = wrapper.get("[data-slot-id='extension.plugin.bundle.activation']");
    expect(activation.attributes("data-dispatch-key")).toBe("com.example/fixture");
    expect(JSON.parse(activation.attributes("data-slot-context") || "{}")).toMatchObject({
      view: "activation",
      enabled: true,
    });
    const detailButton = wrapper.findAll("button").find((button) => button.text() === "详情");
    expect(detailButton).toBeDefined();
    await detailButton!.trigger("click");
    await flushPromises();
    expect(wrapper.find("[data-slot-id='extension.detail.action']").exists()).toBe(true);
    expect(wrapper.find("[data-slot-id='extension.detail.tab']").exists()).toBe(true);
    const rowConfig = wrapper.get("[data-slot-id='extension.plugin.row.config']");
    expect(rowConfig.attributes("data-dispatch-key")).toBe("com.example/fixture#root");
    wrapper.unmount();
  });
});
