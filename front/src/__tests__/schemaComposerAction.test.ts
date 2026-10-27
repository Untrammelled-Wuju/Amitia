import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { defineComponent } from "vue";
import SchemaComposerActionProxy from "../components/extension/SchemaComposerActionProxy.vue";
import ExtensionContributionRenderer from "../components/extension/ExtensionContributionRenderer.vue";
import type { UIContributionSummary } from "../stores/extensionUI";

vi.mock("../components/extension/SchemaUIRenderer.vue", () => ({
  __esModule: true,
  default: defineComponent({
    name: "SchemaUIRenderer",
    props: ["contribution", "context", "slotId", "hostActions"],
    template: '<div class="test-schema">{{ context.conversationId }}<button>刷新</button></div>',
  }),
}));

const contribution = {
  contributionId: "emote-composer", extensionId: "com.amitia/emote",
  title: "发送表情", icon: "emote", kind: "composer_action", generation: 1,
} as UIContributionSummary;
const wrappers: ReturnType<typeof mount>[] = [];

function create() {
  const wrapper = mount(SchemaComposerActionProxy, {
    attachTo: document.body,
    props: { contribution, context: { conversationId: "chat-a", characterId: "role-a" }, slotId: "chat.composer.action" },
    global: { plugins: [ElementPlus] },
  });
  wrappers.push(wrapper);
  return wrapper;
}

afterEach(() => {
  wrappers.splice(0).forEach(wrapper => wrapper.unmount());
  document.body.innerHTML = "";
});

describe("schema composer action panel", () => {
  it.each([
    ["composer_action", "chat.composer.action", true],
    ["schema_page", "settings.main", false],
  ])("routes %s in %s to the appropriate surface", async (kind, slotId, compact) => {
    const wrapper = mount(ExtensionContributionRenderer, {
      attachTo: document.body,
      props: { contribution: { ...contribution, kind, sandbox: "schema_renderer" }, context: { conversationId: "chat-a" }, slotId },
      global: { plugins: [ElementPlus] },
    });
    wrappers.push(wrapper);
    await vi.waitFor(() => expect(wrapper.find(compact ? ".schema-composer-action-trigger" : ".test-schema").exists()).toBe(true));
    expect(wrapper.find(".test-schema").exists()).toBe(!compact);
  });

  it("mounts the full schema only after opening in the shared composer surface", async () => {
    const wrapper = create();
    expect(document.querySelector(".test-schema")).toBeNull();
    const trigger = wrapper.get('button[aria-label="发送表情"]');
    expect(trigger.attributes("data-open")).toBe("false");
    await trigger.trigger("click");
    await vi.waitFor(() => expect(trigger.attributes("data-open")).toBe("true"));
    expect(trigger.attributes("data-open")).toBe("true");
    expect(wrapper.find(".test-schema").exists()).toBe(true);
    expect(document.querySelector('.schema-composer-action-panel[role="dialog"]')?.textContent).toContain("chat-a");
    document.querySelector<HTMLButtonElement>('button[aria-label="关闭面板"]')!.click();
    await flushPromises();
    expect(document.querySelector(".test-schema")).toBeNull();
  });

  it("closes stale panels when the conversation or contribution changes", async () => {
    const wrapper = create();
    await wrapper.get(".schema-composer-action-trigger").trigger("click");
    await vi.waitFor(() => expect(document.querySelector(".test-schema")).not.toBeNull());
    await wrapper.setProps({ context: { conversationId: "chat-b", characterId: "role-a" } });
    expect(wrapper.get(".schema-composer-action-trigger").attributes("data-open")).toBe("false");
    await wrapper.get(".schema-composer-action-trigger").trigger("click");
    await vi.waitFor(() => expect(document.querySelector(".test-schema")).not.toBeNull());
    expect(document.querySelector(".test-schema")?.textContent).toContain("chat-b");
    await wrapper.setProps({ contribution: { ...contribution, generation: 2 } });
    expect(document.querySelector(".test-schema")).toBeNull();
  });

  it("closes on Escape without submitting the composer", async () => {
    const wrapper = create();
    await wrapper.get(".schema-composer-action-trigger").trigger("click");
    await vi.waitFor(() => expect(document.querySelector(".test-schema")).not.toBeNull());
    const event = new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
    document.querySelector(".schema-composer-action-panel")!.dispatchEvent(event);
    await flushPromises();
    expect(event.defaultPrevented).toBe(true);
    expect(document.querySelector(".test-schema")).toBeNull();
  });

  it("closes on an outside pointer and reuses the composer floating surface", async () => {
    const wrapper = create();
    const trigger = wrapper.get(".schema-composer-action-trigger");
    expect(trigger.attributes()).toHaveProperty("data-composer-picker-trigger");
    await trigger.trigger("click");
    await vi.waitFor(() => expect(document.querySelector(".test-schema")).not.toBeNull());
    const panel = document.querySelector(".schema-composer-action-panel");
    expect(panel?.classList.contains("composer-picker")).toBe(true);
    document.body.dispatchEvent(
      new MouseEvent("pointerdown", { bubbles: true }),
    );
    await flushPromises();
    expect(document.querySelector(".test-schema")).toBeNull();
  });
});
