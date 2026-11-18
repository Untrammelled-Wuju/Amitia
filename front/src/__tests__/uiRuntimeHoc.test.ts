import { createPinia } from "pinia";
import { defineComponent, h } from "vue";
import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import HocSlotLayers from "@/components/extension/HocSlotLayers.vue";
import { BrowserSlotRuntime, browserClientPluginRuntime } from "@/ui-runtime/clientPluginRuntime";
import { buildUnifiedSlotItems } from "@/ui-runtime/slotLedger";

describe("HOC slot runtime", () => {
  it("orders wildcard HOC layers without treating equal priorities as conflicts", async () => {
    const slots = new BrowserSlotRuntime();
    await slots.declare({
      slotId: "test.hoc",
      contractVersion: 1,
      supportedKinds: ["panel"],
      kind: "hoc",
    });

    (slots.registerEntry as any)("plugin", {
      name: "test.hoc",
      key: "outer",
      order: 0,
      priority: 10,
    }, {} as any);
    (slots.registerEntry as any)("plugin", {
      name: "test.hoc",
      key: "inner",
      order: 1,
      priority: 10,
    }, {} as any);

    const layers = slots.dispatchContributions("test.hoc");
    expect(layers.map((item) => item.contribution.key)).toEqual(["outer", "inner"]);
  });

  it("includes only wildcard and matching keyed HOC layers", async () => {
    const slots = new BrowserSlotRuntime();
    await slots.declare({
      slotId: "test.hoc.keyed",
      contractVersion: 1,
      supportedKinds: ["panel"],
      kind: "hoc",
    });

    (slots.registerEntry as any)("plugin", {
      name: "test.hoc.keyed",
      key: "wildcard",
      order: 0,
      priority: 0,
    }, {} as any);
    (slots.registerEntry as any)("plugin", {
      name: "test.hoc.keyed",
      key: "matching",
      entryKey: "markdown",
      order: 1,
      priority: 0,
    }, {} as any);
    (slots.registerEntry as any)("plugin", {
      name: "test.hoc.keyed",
      key: "other",
      entryKey: "terminal",
      order: 2,
      priority: 0,
    }, {} as any);

    expect(slots.dispatchContributions("test.hoc.keyed").map((item) => item.contribution.key))
      .toEqual(["wildcard"]);
    expect(slots.dispatchContributions("test.hoc.keyed", {}, "markdown").map((item) => item.contribution.key))
      .toEqual(["wildcard", "matching"]);
  });

  it("merges server and client HOC layers in order, priority, and assembly order", () => {
    const server = [{
      contributionId: "server-layer",
      extensionId: "server",
      moduleId: "server",
      kind: "panel",
      slotId: "test.hoc.merge",
      contractVersion: 1,
      generation: 1,
      title: "Server",
      ordering: 2,
      priority: 0,
      visible: true,
      effective: true,
      enabled: true,
      runtimeReady: true,
    }];
    const client = [
      {
        contributionId: "client-outer",
        pluginId: "client",
        key: "outer",
        slotId: "test.hoc.merge",
        component: {} as any,
        ordering: 0,
        priority: 20,
        sequence: 2,
      },
      {
        contributionId: "client-inner",
        pluginId: "client",
        key: "inner",
        slotId: "test.hoc.merge",
        component: {} as any,
        ordering: 1,
        priority: 0,
        sequence: 1,
      },
    ] as any[];

    expect(buildUnifiedSlotItems({ kind: "hoc" }, server, client).map((item) => item.contributionId))
      .toEqual(["client-outer", "client-inner", "server-layer"]);
  });

  it("renders HOC layers around the owner content", async () => {
    const layer = (name: string) => defineComponent({
      setup(_, { slots }) {
        return () => h("div", { "data-layer": name }, slots.default?.());
      },
    });
    const slots = browserClientPluginRuntime.slots;
    await slots.declare({
      slotId: "test.hoc.render",
      contractVersion: 1,
      supportedKinds: ["panel"],
      kind: "hoc",
    });
    const disposeOuter = (slots.registerEntry as any)("outer-plugin", {
      name: "test.hoc.render",
      key: "outer",
      order: 0,
      priority: 0,
    }, layer("outer"));
    const disposeInner = (slots.registerEntry as any)("inner-plugin", {
      name: "test.hoc.render",
      key: "inner",
      order: 1,
      priority: 0,
    }, layer("inner"));

    try {
      const items = buildUnifiedSlotItems(
        { kind: "hoc" },
        [],
        slots.dispatchContributions("test.hoc.render").map((item) => item.contribution),
      );
      const wrapper = mount(HocSlotLayers, {
        props: {
          items,
          slotId: "test.hoc.render",
          context: { conversationId: "conversation-1" },
        },
        slots: {
          default: () => h("span", { "data-owner": "content" }, "owner"),
        },
        global: {
          plugins: [createPinia()],
        },
      });

      expect(wrapper.find('[data-layer="outer"] [data-layer="inner"] [data-owner="content"]').exists()).toBe(true);
      wrapper.unmount();
    } finally {
      await disposeOuter();
      await disposeInner();
    }
  });
});
