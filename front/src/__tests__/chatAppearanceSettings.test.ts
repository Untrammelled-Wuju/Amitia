import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { nextTick } from "vue";

describe("chat message appearance", () => {
  beforeEach(() => { vi.resetModules(); localStorage.clear(); vi.restoreAllMocks(); });

  it("restores avatar shapes, retains custom roundness and keeps state on failed saves", async () => {
    const module = await import("../composables/useChatAppearancePreference");
    const preference = module.useChatAppearancePreference();
    expect(preference.aiAvatarShape.value).toBe("rounded");
    preference.setAiAvatarShape("custom", 38);
    expect(preference.aiAvatarRadius.value).toBe("19%");
    preference.setAiAvatarShape("circle");
    expect(preference.aiAvatarRadius.value).toBe("50%");
    vi.resetModules();
    const restored = (await import("../composables/useChatAppearancePreference")).useChatAppearancePreference();
    expect(restored.aiAvatarShape.value).toBe("circle");
    restored.setAiAvatarShape("custom");
    expect(restored.aiAvatarRoundness.value).toBe(38);
    restored.setAiAvatarShape("custom", 200);
    expect(restored.aiAvatarRadius.value).toBe("50%");
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("full"); });
    expect(() => restored.setAiAvatarShape("rounded")).toThrow();
    expect(restored.aiAvatarShape.value).toBe("custom");
  });

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

  it("defaults AI identity on and persists independent toggles without changing failed saves", async () => {
    const module = await import("../composables/useChatAppearancePreference");
    const preference = module.useChatAppearancePreference();
    expect(preference.aiAvatarEnabled.value).toBe(true);
    expect(preference.aiNameEnabled.value).toBe(true);
    preference.setAiAvatarEnabled(false);
    expect(preference.aiNameEnabled.value).toBe(true);
    preference.setAiNameEnabled(false);
    vi.resetModules();
    const restored = (await import("../composables/useChatAppearancePreference")).useChatAppearancePreference();
    expect(restored.aiAvatarEnabled.value).toBe(false);
    expect(restored.aiNameEnabled.value).toBe(false);
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("full"); });
    expect(() => restored.setAiAvatarEnabled(true)).toThrow();
    expect(() => restored.setAiNameEnabled(true)).toThrow();
    expect(restored.aiAvatarEnabled.value).toBe(false);
    expect(restored.aiNameEnabled.value).toBe(false);
  });

  it("shows independent AI identities above every split bubble and respects flow grouping", async () => {
    const { useChatAppearancePreference } = await import("../composables/useChatAppearancePreference");
    const Renderer = (await import("../conversation/rendering/AIMessageRenderer.vue")).default;
    const preference = useChatAppearancePreference();
    preference.setMessageStyle("bubble");
    const wrapper = mount(Renderer, {
      props: { message: { id: "a", role: "assistant", content: "第一段[AMITIA_BR]第二段", status: "sent" }, charName: "测试角色", showAvatar: false, showHeader: false },
      global: { stubs: { MarkdownContent: true, ElIcon: true } },
    });
    expect(wrapper.findAll(".bubble-entry")).toHaveLength(2);
    expect(wrapper.get(".amrp-message").classes()).toContain("amrp-message--single-column");
    for (const entry of wrapper.findAll(".bubble-entry")) {
      expect(entry.find(".bubble-avatar").exists()).toBe(true);
      expect(entry.find(".bubble-entry-body .amrp-avatar").exists()).toBe(false);
      expect(entry.find(".bubble-entry-body .bubble-identity").exists()).toBe(true);
      expect(entry.get(".amrp-name").text()).toBe("测试角色");
    }
    preference.setAiAvatarEnabled(false);
    await nextTick();
    expect(wrapper.find(".amrp-avatar").exists()).toBe(false);
    expect(wrapper.findAll(".amrp-name")).toHaveLength(2);
    preference.setAiNameEnabled(false);
    await nextTick();
    expect(wrapper.find(".bubble-identity").exists()).toBe(false);
    preference.setAiAvatarEnabled(true);
    await nextTick();
    expect(wrapper.findAll(".amrp-avatar")).toHaveLength(2);
    expect(wrapper.find(".amrp-name").exists()).toBe(false);
    preference.setMessageStyle("flow");
    await nextTick();
    expect(wrapper.find(".amrp-avatar").exists()).toBe(false);
    expect(wrapper.find(".amrp-avatar-spacer").exists()).toBe(true);
    expect(wrapper.get(".amrp-message").classes()).not.toContain("amrp-message--single-column");
    await wrapper.setProps({ showAvatar: true, showHeader: true });
    expect(wrapper.find(".amrp-avatar").exists()).toBe(true);
    expect(wrapper.find(".amrp-name").exists()).toBe(false);
    preference.setAiAvatarEnabled(false);
    await nextTick();
    expect(wrapper.find(".amrp-avatar-spacer").exists()).toBe(false);
    expect(wrapper.get(".amrp-message").classes()).toContain("amrp-message--single-column");
    wrapper.unmount();
  });

  it("persists glass independently and keeps the previous value if saving fails", async () => {
    const module = await import("../composables/useChatAppearancePreference");
    const preference = module.useChatAppearancePreference();
    expect(preference.userMessageGlass.value).toBe(false);
    preference.setUserMessageGlass(true);
    expect(preference.messageStyle.value).toBe("flow");
    vi.resetModules();
    const restored = (await import("../composables/useChatAppearancePreference")).useChatAppearancePreference();
    expect(restored.userMessageGlass.value).toBe(true);
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("full"); });
    expect(() => restored.setUserMessageGlass(false)).toThrow();
    expect(restored.userMessageGlass.value).toBe(true);
  });

  it("migrates legacy glass and switches mutually exclusive materials without changing chat style", async () => {
    localStorage.setItem("amitia.chat.user-message-glass.desktop.v1", "true");
    const module = await import("../composables/useChatAppearancePreference");
    const preference = module.useChatAppearancePreference();
    expect(preference.userMessageGlass.value).toBe(true);
    preference.setUserMessageWaterGlass(true);
    expect(preference.userMessageGlass.value).toBe(false);
    expect(preference.userMessageWaterGlass.value).toBe(true);
    expect(preference.messageStyle.value).toBe("flow");
    vi.resetModules();
    const restored = (await import("../composables/useChatAppearancePreference")).useChatAppearancePreference();
    expect(restored.userMessageWaterGlass.value).toBe(true);
    restored.setUserMessageGlass(true);
    expect(restored.userMessageWaterGlass.value).toBe(false);
    restored.setUserMessageGlass(false);
    vi.resetModules();
    const solid = (await import("../composables/useChatAppearancePreference")).useChatAppearancePreference();
    expect(solid.userMessageGlass.value).toBe(false);
    expect(solid.userMessageWaterGlass.value).toBe(false);
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("full"); });
    expect(() => solid.setUserMessageWaterGlass(true)).toThrow();
    expect(solid.userMessageWaterGlass.value).toBe(false);
  });

  it("applies glass live only to user messages in both styles", async () => {
    const { useChatAppearancePreference } = await import("../composables/useChatAppearancePreference");
    const Renderer = (await import("../conversation/rendering/AIMessageRenderer.vue")).default;
    const preference = useChatAppearancePreference();
    const wrapper = mount(Renderer, {
      props: { message: { id: "user", role: "user", content: "用户消息", status: "sent" }, showAvatar: false, showHeader: false },
      global: { stubs: { MarkdownContent: true, ElIcon: true } },
    });
    preference.setUserMessageGlass(true);
    await nextTick();
    expect(wrapper.classes()).toContain("amrp-user-glass");
    preference.setUserMessageWaterGlass(true);
    await nextTick();
    expect(wrapper.classes()).toContain("amrp-user-water");
    expect(wrapper.classes()).not.toContain("amrp-user-glass");
    preference.setUserMessageGlass(true);
    await nextTick();
    preference.setMessageStyle("bubble");
    await nextTick();
    expect(wrapper.find(".bubble-content--user .message-piece").exists()).toBe(true);
    expect(wrapper.classes()).toContain("amrp-user-glass");
    preference.setUserMessageWaterGlass(true);
    await nextTick();
    expect(wrapper.classes()).toContain("amrp-user-water");
    expect(wrapper.classes()).not.toContain("amrp-user-glass");
    await wrapper.setProps({ message: { id: "assistant", role: "assistant", content: "回复", status: "sent" } });
    expect(wrapper.classes()).not.toContain("amrp-user-glass");
    expect(wrapper.classes()).not.toContain("amrp-user-water");
    await wrapper.setProps({ message: { id: "user", role: "user", content: "用户消息", status: "sent" } });
    preference.setUserMessageWaterGlass(false);
    await nextTick();
    expect(wrapper.classes()).not.toContain("amrp-user-glass");
    expect(wrapper.classes()).not.toContain("amrp-user-water");
    wrapper.unmount();
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
