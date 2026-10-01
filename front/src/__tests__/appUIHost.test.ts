import { mount } from "@vue/test-utils";
import { defineComponent, nextTick, ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import { useAppUIHost } from "../composables/useAppUIHost";

const host = vi.hoisted(() => ({
  connect: vi.fn(),
  disconnect: vi.fn(),
  disposeListener: vi.fn(),
  invalidateSnapshot: vi.fn(),
}));
vi.mock("../composables/useUIHostSSE", () => ({ useUIHostSSE: () => host }));
vi.mock("../stores/extensionUI", () => ({
  useExtensionUIStore: () => ({
    setupExtensionChangeListener: () => host.disposeListener,
    invalidateSnapshot: host.invalidateSnapshot,
  }),
}));
vi.mock("vue-router", () => ({ useRouter: () => ({ push: vi.fn() }) }));

describe("应用级 UI Host 生命周期", () => {
  it("切换页面壳层保持连接，公共页面断开，应用卸载清理订阅", async () => {
    vi.clearAllMocks();
    const isPublic = ref(false);
    const settings = ref(false);
    const wrapper = mount(defineComponent({
      setup() { useAppUIHost(isPublic); return { settings }; },
      template: '<div><div v-if="settings">设置壳层</div><div v-else>主壳层</div></div>',
    }));
    try {
      expect(host.connect).toHaveBeenCalledTimes(1);
      settings.value = true;
      await nextTick();
      settings.value = false;
      await nextTick();
      expect(host.connect).toHaveBeenCalledTimes(1);
      expect(host.disconnect).not.toHaveBeenCalled();
      expect(host.invalidateSnapshot).not.toHaveBeenCalled();
      isPublic.value = true;
      await nextTick();
      expect(host.disconnect).toHaveBeenCalledTimes(1);
      isPublic.value = false;
      await nextTick();
      expect(host.connect).toHaveBeenCalledTimes(2);
    } finally { wrapper.unmount(); }
    expect(host.disposeListener).toHaveBeenCalledTimes(1);
    expect(host.invalidateSnapshot).toHaveBeenCalledTimes(1);
  });
});
