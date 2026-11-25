import { mount } from "@vue/test-utils";
import { computed } from "vue";
import { expect, it, vi } from "vitest";

vi.mock("@/composables/useExtensionSlot", () => ({
  useExtensionSlot: () => ({ contributions: computed(() => []), scopeReady: computed(() => true), fallback: computed(() => "default"), layout: computed(() => "stack"), buildContext: () => ({}) }),
}));
vi.mock("@/stores/extensionUI", () => ({ useExtensionUIStore: () => ({ slotsById: new Map() }) }));
vi.mock("@/ui-runtime/clientPluginRuntime", () => ({ browserClientPluginRuntime: { slots: { revision: { value: 0 }, getDefinition: () => null, dispatchContributions: () => [] } } }));

import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";

it("文本代理跳过文件输入框并更新实际文本框", async () => {
  const wrapper = mount(ExtensionSlot, {
    props: { slotId: "chat.composer.root", fallback: "default" },
    slots: { default: '<input type="file"><button>上传</button><textarea></textarea>' },
  });
  const proxy = wrapper.vm as unknown as { setText(value: string): void; getText(): string; focus(): void; clear(): void };
  proxy.setText("队列消息");
  expect((wrapper.get("input").element as HTMLInputElement).value).toBe("");
  expect((wrapper.get("textarea").element as HTMLTextAreaElement).value).toBe("队列消息");
  expect(proxy.getText()).toBe("队列消息");
  proxy.clear();
  expect(proxy.getText()).toBe("");
  wrapper.unmount();
});
