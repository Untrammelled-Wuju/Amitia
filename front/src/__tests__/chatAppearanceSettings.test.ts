import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { nextTick } from "vue";

describe("chat message appearance", () => {
  beforeEach(() => { vi.resetModules(); localStorage.clear(); vi.restoreAllMocks(); });

  it("defaults to flow and persists one shared selection", async () => {
    const module = await import("../composables/useChatAppearancePreference");
    const first = module.useChatAppearancePreference();
    const second = module.useChatAppearancePreference();
    expect(first.messageStyle.value).toBe("flow");
    first.setMessageStyle("bubble");
    expect(second.messageStyle.value).toBe("bubble");
    expect(localStorage.getItem(module.chatMessageStyleStorageKey)).toBe("bubble");
    vi.resetModules();
    expect((await import("../composables/useChatAppearancePreference")).useChatAppearancePreference().messageStyle.value).toBe("bubble");
  });

  it("retains the active style if saving fails and rejects unknown stored values", async () => {
    localStorage.setItem("amitia.chat.message-style.desktop.v1", "invalid");
    const { useChatAppearancePreference } = await import("../composables/useChatAppearancePreference");
    const preference = useChatAppearancePreference();
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("full"); });
    expect(() => preference.setMessageStyle("bubble")).toThrow();
    expect(preference.messageStyle.value).toBe("flow");
  });

  it("switches the actual renderer live and retains content and actions", async () => {
    const { useChatAppearancePreference } = await import("../composables/useChatAppearancePreference");
    const Renderer = (await import("../conversation/rendering/AIMessageRenderer.vue")).default;
    const wrapper = mount(Renderer, {
      props: { message: { id: "a1", role: "assistant", content: "你好", status: "sent" }, showAvatar: false, showHeader: false },
      global: { stubs: { MarkdownContent: { props: ["source"], template: '<p>{{ source }}</p>' }, ElIcon: { template: '<span><slot/></span>' } } },
    });
    expect(wrapper.classes()).not.toContain("amrp-bubbles");
    useChatAppearancePreference().setMessageStyle("bubble");
    await nextTick();
    expect(wrapper.classes()).toContain("amrp-bubbles");
    expect(wrapper.find(".amrp-avatar").exists()).toBe(true);
    expect(wrapper.text()).toContain("你好");
    await wrapper.get('[aria-label="回复"]').trigger("click");
    expect(wrapper.emitted("reply")).toHaveLength(1);
    useChatAppearancePreference().setMessageStyle("flow");
    await nextTick();
    expect(wrapper.classes()).not.toContain("amrp-bubbles");
    expect(wrapper.find(".amrp-avatar").exists()).toBe(false);
    wrapper.unmount();
  });
});
