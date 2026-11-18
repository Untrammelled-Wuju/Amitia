import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { describe, expect, it } from "vitest";
import AppLayout from "../components/AppLayout.vue";

describe("application shell slots", () => {
  it("mounts quota notices above the root and bottom content after it", () => {
    const pinia = createPinia();
    setActivePinia(pinia);
    const wrapper = mount(AppLayout, {
      global: {
        plugins: [pinia],
        stubs: {
          UIPortalHost: { template: "<div />" },
          UIRightPaneHost: { template: "<div />" },
          UIProviderHost: {
            props: ["capability", "fallback"],
            template: '<div data-provider="true"><slot /></div>',
          },
          BuiltinAppShell: { template: "<div data-builtin-shell />" },
          ExtensionSlot: {
            props: ["slotId"],
            template: '<div :data-slot-id="slotId"><slot /></div>',
          },
        },
      },
    });

    const root = wrapper.get(".app-shell-target");
    const children = Array.from(root.element.children);
    const slotIndex = (slotId: string) =>
      children.findIndex(
        (child) => child.getAttribute("data-slot-id") === slotId,
      );
    const rootIndex = slotIndex("root");
    expect(slotIndex("app.shell.quota_notice")).toBeLessThan(rootIndex);
    expect(slotIndex("app.shell.bottom")).toBeGreaterThan(rootIndex);
    wrapper.unmount();
  });
});
