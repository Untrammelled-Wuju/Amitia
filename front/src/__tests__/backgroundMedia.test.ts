import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import BackgroundMedia from "../components/BackgroundMedia.vue";
import { defaultBackground } from "../composables/backgroundStorage";

describe("background rendering", () => {
  afterEach(() => vi.restoreAllMocks());
  it("renders an isolated blurred image while keeping controls interactive", () => {
    const wrapper = mount(BackgroundMedia, { props: { source: "blob:photo", settings: { ...defaultBackground, opacity: 0.4, blurEnabled: true, blurRadius: 12 } }, slots: { default: '<button>内容操作</button>' } });
    const style = wrapper.get(".media-layer").attributes("style");
    expect(style).toContain("opacity: 0.4");
    expect(style).toContain("blur(12px)");
    expect(wrapper.find("img").exists()).toBe(true);
    expect(wrapper.find("button").exists()).toBe(true);
    wrapper.unmount();
  });
  it("mutes video and pauses for reduced motion, invisible content and preview", async () => {
    const play = vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue(undefined);
    const pause = vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
    const wrapper = mount(BackgroundMedia, { props: { source: "blob:clip", settings: { ...defaultBackground, kind: "video" }, animate: true } });
    await wrapper.get("video").trigger("loadeddata");
    expect(play).toHaveBeenCalled();
    expect((wrapper.get("video").element as HTMLVideoElement).muted).toBe(true);
    await wrapper.setProps({ animate: false });
    expect(pause).toHaveBeenCalled();
    play.mockClear();
    await wrapper.setProps({ animate: true, settings: { ...defaultBackground, kind: "video", opacity: 0 } });
    expect(play).not.toHaveBeenCalled();
    wrapper.unmount();
  });
  it("falls back after decoder errors and recovers after replacing the file", async () => {
    const wrapper = mount(BackgroundMedia, { props: { source: "blob:bad", settings: defaultBackground, preview: true } });
    await wrapper.get("img").trigger("error");
    expect(wrapper.get('[role="alert"]').text()).toContain("背景无法显示");
    await wrapper.setProps({ source: "blob:good" });
    expect(wrapper.find("img").exists()).toBe(true);
    wrapper.unmount();
  });
});
