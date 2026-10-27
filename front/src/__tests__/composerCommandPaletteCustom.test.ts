import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { afterEach, describe, expect, it } from "vitest";
import ComposerCommandPalette from "../components/ComposerCommandPalette.vue";

const wrappers: ReturnType<typeof mount>[] = [];

function create() {
  const wrapper = mount(ComposerCommandPalette, {
    attachTo: document.body,
    props: {
      mode: "custom",
      title: "表情",
      dismissible: true,
      instanceId: "emote",
    },
    slots: {
      toolbar: '<div class="test-toolbar">分组</div>',
      default: '<div class="test-content">表情图片</div>',
      footer: '<div class="test-footer">底部</div>',
    },
    global: {
      plugins: [ElementPlus],
    },
  });
  wrappers.push(wrapper);
  return wrapper;
}

afterEach(() => {
  wrappers.splice(0).forEach((wrapper) => wrapper.unmount());
  document.body.innerHTML = "";
});

describe("ComposerCommandPalette custom mode", () => {
  it("reuses the composer surface and exposes named slots", async () => {
    const wrapper = create();
    await flushPromises();
    expect(wrapper.classes()).toContain("composer-picker");
    expect(wrapper.get(".picker-heading").text()).toContain("表情");
    expect(wrapper.get(".test-toolbar").text()).toBe("分组");
    expect(wrapper.get(".test-content").text()).toBe("表情图片");
    expect(wrapper.get(".test-footer").text()).toBe("底部");
    expect(wrapper.find(".picker-option").exists()).toBe(false);
    expect(document.activeElement).toBe(wrapper.element);
  });

  it("dismisses on Escape, outside pointer and another composer panel", async () => {
    const wrapper = create();
    await flushPromises();
    wrapper.element.dispatchEvent(
      new KeyboardEvent("keydown", {
        key: "Escape",
        bubbles: true,
        cancelable: true,
      }),
    );
    expect(wrapper.emitted("dismiss")).toHaveLength(1);

    document.body.dispatchEvent(
      new MouseEvent("pointerdown", { bubbles: true }),
    );
    expect(wrapper.emitted("dismiss")).toHaveLength(2);

    window.dispatchEvent(
      new CustomEvent("amitia:composer-floating-open", {
        detail: { instanceId: "other" },
      }),
    );
    expect(wrapper.emitted("dismiss")).toHaveLength(3);
  });
});
