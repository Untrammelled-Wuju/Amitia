import { mount } from "@vue/test-utils";
import { nextTick } from "vue";
import { afterEach, describe, expect, it, vi } from "vitest";
import ForeignComponentHost from "@/components/extension/ForeignComponentHost.vue";
import {
  FOREIGN_COMPONENT_REGISTRY_KEY,
  ForeignComponentRegistry,
  type ForeignComponentInstance,
} from "@/ui-runtime/foreignComponentRuntime";
import { buildUIPluginContext } from "@/ui-runtime/uiContext";

function context() {
  return buildUIPluginContext({
    slotId: "ui.portal.overlay",
    contractVersion: 1,
    route: "/chat",
    platform: "web",
    theme: { mode: "light", density: "comfortable" },
    locale: "zh-CN",
    data: { conversationId: "conversation-1" },
    host: "web",
    os: "unknown",
    surface: { role: "overlay", width: 800, height: 600, breakpoint: "md" },
    capabilities: [],
    permissions: [],
    scope: "root",
  });
}

describe("ForeignComponentHost", () => {
  let dispose: (() => void) | undefined;

  afterEach(() => {
    dispose?.();
    dispose = undefined;
  });

  it("mounts, updates and unmounts an external component adapter", async () => {
    const registry = new ForeignComponentRegistry();
    const update = vi.fn();
    const unmount = vi.fn();
    const instance: ForeignComponentInstance = {
      update,
      unmount,
    };
    const mountAdapter = vi.fn((input) => {
      input.element.textContent = String(input.props.label ?? "");
      input.emit({ type: "ready" });
      return instance;
    });
    dispose = registry.register({ id: "test-adapter", mount: mountAdapter });

    const wrapper = mount(ForeignComponentHost, {
      props: {
        adapter: "test-adapter",
        context: context(),
        componentProps: { label: "first" },
      },
      global: {
        provide: {
          [FOREIGN_COMPONENT_REGISTRY_KEY as symbol]: registry,
        },
      },
    });

    await vi.waitFor(() => expect(mountAdapter).toHaveBeenCalledOnce());
    wrapper.setProps({ componentProps: { label: "second" } });
    await nextTick();
    await vi.waitFor(() => expect(update).toHaveBeenCalled());
    expect(wrapper.emitted("component-event")).toEqual([[{ type: "ready" }]]);

    wrapper.unmount();
    await vi.waitFor(() => expect(unmount).toHaveBeenCalledOnce());
  });

  it("returns the original missing-adapter error", async () => {
    const wrapper = mount(ForeignComponentHost, {
      props: {
        adapter: "missing",
        context: context(),
      },
    });
    await vi.waitFor(() => expect(wrapper.emitted("error")?.[0]?.[0]).toMatchObject({
      message: "foreign component adapter missing is not registered",
    }));
    wrapper.unmount();
  });
});
