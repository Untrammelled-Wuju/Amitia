import { createPinia } from "pinia";
import { defineComponent, h } from "vue";
import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import ClientSlotContributionHost from "@/components/extension/ClientSlotContributionHost.vue";
import { useClientUIContext } from "@/ui-runtime/clientSlotContext";
import { browserClientPluginRuntime } from "@/ui-runtime/clientPluginRuntime";
import { closeContextMenu, contextMenuState } from "@/components/context-menu/contextMenuStore";
import {
  buildUIPluginContext,
  createUIPluginContextSource,
  selectUIPluginContext,
  type UIPluginContextBuildInput,
} from "@/ui-runtime/uiContext";
import {
  UI_SURFACE_DEFINITIONS,
  canonicalUISurfaceId,
  normalizeUIRoute,
  resolveUISurfaceDefinition,
} from "@/ui-runtime/uiSurfaceCatalog";

function contextInput(overrides: Partial<UIPluginContextBuildInput> = {}): UIPluginContextBuildInput {
  return {
    slotId: "chat.message.action",
    contractVersion: 1,
    route: "/characters/character-1?tab=memory",
    platform: "web",
    theme: { mode: "dark", density: "compact" },
    locale: "zh-CN",
    data: {
      conversation: { id: "conversation-1" },
      characterId: "character-1",
      messageId: "message-1",
      capabilities: ["chat.image", "", "chat.image", "chat.audio"],
      permissions: ["media.image.read", "media.image.read"],
    },
    host: "web",
    os: "unknown",
    surface: { role: "message", width: 640, height: 480, breakpoint: "md" },
    capabilities: ["chat.image", "", "chat.image", "chat.audio"],
    permissions: ["media.image.read", "media.image.read"],
    scope: "session",
    ...overrides,
  };
}

describe("UI plugin context", () => {
  it("builds a stable identity and surface context from slot data", () => {
    const context = buildUIPluginContext(contextInput());

    expect(context.schemaVersion).toBe(1);
    expect(context.route).toBe("/characters/character-1");
    expect(context.surfaceId).toBe("surface.character.detail");
    expect(context.sessionId).toBe("conversation-1");
    expect(context.conversationId).toBe("conversation-1");
    expect(context.characterId).toBe("character-1");
    expect(context.messageId).toBe("message-1");
    expect(context.capabilities).toEqual(["chat.image", "chat.audio"]);
    expect(context.permissions).toEqual(["media.image.read"]);
  });

  it("notifies selectors only when the selected value changes", () => {
    const source = createUIPluginContextSource(buildUIPluginContext(contextInput()));
    const selected = selectUIPluginContext(source, (context) => context.surfaceId);
    const listener = vi.fn();
    const dispose = selected.subscribe(listener);
    const initial = selected.getSnapshot();

    source.setContext(buildUIPluginContext(contextInput({ slotId: "chat.message.badge" })));
    expect(selected.getSnapshot()).toBe(initial);
    expect(listener).not.toHaveBeenCalled();

    source.setContext(buildUIPluginContext(contextInput({ route: "/settings/model" })));
    expect(selected.getSnapshot()).toBe("surface.settings.section");
    expect(listener).toHaveBeenCalledOnce();

    dispose();
    source.dispose();
  });

  it("resolves surface definitions in declared priority order", () => {
    expect(normalizeUIRoute("characters/character-1/?tab=profile")).toBe("/characters/character-1");
    expect(canonicalUISurfaceId("/characters")).toBe("surface.character.list");
    expect(canonicalUISurfaceId("/characters/character-1")).toBe("surface.character.detail");
    expect(canonicalUISurfaceId("/settings/devices")).toBe("surface.settings.devices");
    expect(canonicalUISurfaceId("/settings/runtime")).toBe("surface.settings.section");
    expect(canonicalUISurfaceId("/unknown/page")).toBe("surface.page");
    expect(resolveUISurfaceDefinition("/unknown/page")).toBe(UI_SURFACE_DEFINITIONS[UI_SURFACE_DEFINITIONS.length - 1]);
    expect(new Set(UI_SURFACE_DEFINITIONS.map((definition) => definition.id)).size).toBe(UI_SURFACE_DEFINITIONS.length);
  });

  it("provides the stable UI context to registered client slot components", async () => {
    const slots = browserClientPluginRuntime.slots;
    await slots.declare({
      slotId: "test.context.slot",
      contractVersion: 1,
      supportedKinds: ["panel"],
      kind: "single",
    });
    const Probe = defineComponent({
      setup() {
        const context = useClientUIContext();
        return () => h("span", {
          "data-surface": context.surfaceId,
          "data-session": context.sessionId,
          "data-route": context.route,
        });
      },
    });
    const dispose = (slots.registerEntry as any)("context-plugin", {
      name: "test.context.slot",
      key: "probe",
    }, Probe);

    try {
      const contribution = slots.listContributions("test.context.slot")[0]!;
      const context = buildUIPluginContext(contextInput());
      const wrapper = mount(ClientSlotContributionHost, {
        props: {
          contribution,
          context: context as unknown as Record<string, unknown>,
        },
        global: {
          plugins: [createPinia()],
        },
      });

      expect(wrapper.get("span").attributes()).toMatchObject({
        "data-surface": "surface.character.detail",
        "data-session": "conversation-1",
        "data-route": "/characters/character-1",
      });
      wrapper.unmount();
    } finally {
      await dispose();
    }
  });

  it("provides file and URL context-menu bindings to client slot components", async () => {
    const slots = browserClientPluginRuntime.slots;
    await slots.declare({
      slotId: "test.context-menu.slot",
      contractVersion: 1,
      supportedKinds: ["panel"],
      kind: "single",
    });
    const Probe = defineComponent({
      props: {
        useContextMenu: { type: Function, required: true },
      },
      setup(props) {
        const bindings = (props.useContextMenu as () => {
          getTriggerProps: (request: unknown) => Record<string, unknown>;
        })().getTriggerProps({
          kind: "file",
          value: {
            sessionId: "conversation-1",
            path: "/workspace/notes.md",
          },
        });
        return () => h("button", bindings, "文件");
      },
    });
    const dispose = (slots.registerEntry as any)("context-menu-plugin", {
      name: "test.context-menu.slot",
      key: "probe",
    }, Probe);

    try {
      const contribution = slots.listContributions("test.context-menu.slot")[0]!;
      const wrapper = mount(ClientSlotContributionHost, {
        props: {
          contribution,
          context: buildUIPluginContext(contextInput()) as unknown as Record<string, unknown>,
        },
        global: {
          plugins: [createPinia()],
        },
      });
      await wrapper.get("button").trigger("contextmenu", {
        clientX: 120,
        clientY: 80,
      });
      expect(contextMenuState.visible).toBe(true);
      expect(contextMenuState.title).toBe("文件操作");
      expect(contextMenuState.items.map((item) => item.label)).toEqual([
        "复制文件路径",
        "在工作台打开文件",
      ]);
      wrapper.unmount();
    } finally {
      closeContextMenu();
      await dispose();
    }
  });
});
