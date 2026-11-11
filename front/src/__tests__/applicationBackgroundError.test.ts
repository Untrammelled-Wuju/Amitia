import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import ApplicationBackground from "../components/ApplicationBackground.vue";

const mock = vi.hoisted(() => ({ notification: vi.fn(), init: vi.fn() }));
vi.mock("element-plus", () => ({ ElNotification: mock.notification }));
vi.mock("../composables/useBackgroundPreference", async () => {
  const { ref } = await import("vue");
  return { useBackgroundPreference: () => ({ settings: ref({ enabled: false, mode: "default" }), source: ref(""), init: mock.init }) };
});
vi.mock("../composables/useTheme", async () => {
  const { ref } = await import("vue");
  return { useTheme: () => ({ state: ref({ reduceAnimation: false, dynamicEffect: true }) }) };
});

describe("application background error", () => {
  it("初始化失败时通知包含可查看的原始错误", async () => {
    mock.init.mockRejectedValueOnce(new Error("background-original-stack"));
    const wrapper = mount(ApplicationBackground, { props: { visible: true } });
    try {
      await flushPromises();
      expect(mock.notification).toHaveBeenCalledOnce();
      expect(mock.notification.mock.calls[0][0].message.children).toContain("background-original-stack");
    } finally { wrapper.unmount(); }
  });
});
