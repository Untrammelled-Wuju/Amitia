import { mount } from "@vue/test-utils";
import { defineComponent, h, markRaw, ref } from "vue";
import { describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ fallbackSetText: vi.fn(), boundarySetText: vi.fn(), fallbackFocus: vi.fn(), boundaryFocus: vi.fn() }));

vi.mock("vue-router", () => ({ useRoute: () => ({ params: {}, query: {} }), useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/stores/extensionUI", () => ({ useExtensionUIStore: () => ({ snapshot: {}, getResolvedProvider: () => null, getProviders: () => [], slotsById: new Map() }) }));
vi.mock("@/components/extension/ExtensionSlot.vue", () => ({
  default: defineComponent({
    setup(_props, { expose, slots }) {
      expose({ setText: mocks.boundarySetText, focus: mocks.boundaryFocus, getText: () => "boundary" });
      return () => h("div", slots.default?.());
    },
  }),
}));

import UIProviderHost from "@/components/ui-runtime/UIProviderHost.vue";

describe("输入框代理", () => {
  it("优先调用输入组件且不重复调用边界，并返回当前文本", () => {
    const text = ref("");
    const fallback = markRaw(defineComponent({
      setup(_props, { expose }) {
        expose({
          setText(value: string) { mocks.fallbackSetText(value); text.value = value; },
          focus: mocks.fallbackFocus,
          getText: () => text.value,
        });
        return () => h("div", [h("input", { type: "file" }), h("textarea", { value: text.value })]);
      },
    }));
    const wrapper = mount(UIProviderHost, { props: { capability: "conversation.composer", fallback } });
    const proxy = wrapper.vm as unknown as { setText(value: string): void; focus(): void; getText(): string };
    proxy.setText("待编辑消息");
    proxy.focus();
    expect(proxy.getText()).toBe("待编辑消息");
    expect(mocks.fallbackSetText).toHaveBeenCalledOnce();
    expect(mocks.fallbackFocus).toHaveBeenCalledOnce();
    expect(mocks.boundarySetText).not.toHaveBeenCalled();
    expect(mocks.boundaryFocus).not.toHaveBeenCalled();
    wrapper.unmount();
  });
});
