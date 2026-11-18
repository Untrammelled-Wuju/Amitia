import { mount } from "@vue/test-utils";
import { nextTick } from "vue";
import { afterEach, describe, expect, it } from "vitest";
import UIRightPaneHost from "@/components/extension/UIRightPaneHost.vue";
import { rightPaneRuntime } from "@/ui-runtime/rightPaneRuntime";

describe("right pane host", () => {
  afterEach(() => rightPaneRuntime.close());

  it("renders the active resource pane and closes it", async () => {
    rightPaneRuntime.open("dsh-resource://plan/session/call", { title: "Plan" });
    const wrapper = mount(UIRightPaneHost, {
      global: {
        stubs: {
          ExtensionSlot: {
            props: ["slotId"],
            template: "<div :data-slot-id='slotId'><slot /></div>",
          },
        },
      },
    });
    expect(wrapper.find("[data-amitia-ui='right-pane']").exists()).toBe(true);
    expect(wrapper.text()).toContain("Plan");
    await wrapper.get("button[aria-label='关闭右侧面板']").trigger("click");
    await nextTick();
    expect(wrapper.find("[data-amitia-ui='right-pane']").exists()).toBe(false);
  });

  it("mounts files, guide and tab action surfaces", async () => {
    rightPaneRuntime.open("dsh-resource://file/session/conversation-1/notes.md", {
      title: "notes.md",
      sessionId: "conversation-1",
      params: { absolutePath: "/workspace/notes.md", tabKind: "files" },
    });
    const filesWrapper = mount(UIRightPaneHost, {
      global: {
        stubs: {
          ExtensionSlot: {
            props: ["slotId"],
            template: "<div :data-slot-id='slotId'><slot /></div>",
          },
        },
      },
    });
    expect(filesWrapper.find("[data-slot-id='ui.sidebar.right.pane.files.actions']").exists()).toBe(true);
    filesWrapper.unmount();

    rightPaneRuntime.open("dsh-resource://guide/root", {
      title: "Guide",
      sessionId: "conversation-1",
      params: {
        tabKind: "guide",
        guideEntries: [
          { entryId: "terminal", kind: "terminal", title: "Terminal" },
          { entryId: "notes", kind: "notes", title: "Notes", description: "Pinned notes" },
        ],
      },
    });
    const guideWrapper = mount(UIRightPaneHost, {
      global: {
        stubs: {
          ExtensionSlot: {
            props: ["slotId", "dispatchKey"],
            template: "<div :data-slot-id='slotId' :data-dispatch-key='dispatchKey'><slot /></div>",
          },
        },
      },
    });
    expect(guideWrapper.find("[data-slot-id='ui.sidebar.right.pane.guide']").exists()).toBe(true);
    expect(guideWrapper.findAll("[data-slot-id='ui.sidebar.right.pane.guide.entry']").map((item) => item.attributes("data-dispatch-key"))).toEqual(["terminal", "notes"]);
    await guideWrapper.get("button[aria-label='打开标签页操作菜单']").trigger("click");
    expect(guideWrapper.find("[data-slot-id='ui.sidebar.right.pane.guide.actions']").exists()).toBe(true);
    expect(guideWrapper.find("[data-slot-id='ui.sidebar.right.pane.tab.menu.item']").exists()).toBe(true);
    guideWrapper.unmount();
  });
});
