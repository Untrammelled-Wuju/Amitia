import { mount } from "@vue/test-utils";
import { defineComponent, nextTick } from "vue";
import { createPinia, setActivePinia } from "pinia";
import { describe, expect, it, beforeEach } from "vitest";
import QueuedMessages from "../components/QueuedMessages.vue";
import AppContextMenu from "../components/context-menu/AppContextMenu.vue";
import { contextMenuState, closeContextMenu } from "../components/context-menu/contextMenuStore";

describe("排队消息操作", () => {
  beforeEach(() => closeContextMenu());

  it("整行可拖拽调整顺序，并保留引导与删除", async () => {
    const wrapper = mount(QueuedMessages, { props: {
      items: [
        { requestId: "a", message: "第一条", position: 1, status: "queued" },
        { requestId: "b", message: "第二条", position: 2, status: "queued" },
      ], busy: false, canSteer: true, error: "",
    } });
    const rows = wrapper.findAll(".queue-item");
    expect(rows[1].attributes("draggable")).toBe("true");
    await rows[1].trigger("dragstart");
    await rows[0].trigger("drop");
    expect(wrapper.emitted("reorder")?.[0]).toEqual([["b", "a"]]);
    await rows[0].findAll("button")[0].trigger("click");
    await rows[1].findAll("button")[2].trigger("click");
    expect(wrapper.emitted("steer")?.[0]).toEqual(["a"]);
    expect(wrapper.emitted("remove")?.[0]).toEqual(["b"]);
  });

  it("不再渲染上下移动按钮，改为 Alt+方向键排序", async () => {
    const wrapper = mount(QueuedMessages, { props: {
      items: [
        { requestId: "a", message: "第一条", position: 1, status: "queued" },
        { requestId: "b", message: "第二条", position: 2, status: "queued" },
      ], busy: false, canSteer: true, error: "",
    } });
    const labels = wrapper.findAll(".queue-actions button").map((button) => button.attributes("aria-label") || button.text());
    expect(labels).not.toContain("上移排队消息");
    expect(labels).not.toContain("下移排队消息");
    const rows = wrapper.findAll(".queue-item");
    await rows[0].trigger("keydown", { key: "ArrowDown", altKey: true });
    expect(wrapper.emitted("reorder")?.[0]).toEqual([["b", "a"]]);
  });

  it("保留附件、引用和完整错误", async () => {
    const wrapper = mount(QueuedMessages, { props: {
      items: [
        { requestId: "a", message: "第一条", position: 1, status: "queued" },
        { requestId: "b", message: "第二条", position: 2, status: "uncertain", error: "底层错误\n原因链", attachments: [{ artifactId: "art", ownerDeviceId: "device", storageDeviceId: "device", kind: "file", name: "说明.pdf", mimeType: "application/pdf", sha256: "abc", sizeBytes: 3 }], quote: { ownerId: "device", characterId: "role", conversationId: "chat", messageId: "source", expectedRevision: 2 } },
      ], busy: false, canSteer: true, error: "",
    } });
    const rows = wrapper.findAll(".queue-item");
    expect(rows[1].attributes("tabindex")).toBe("0");
    expect(rows[1].text()).toContain("状态不确定");
    expect(rows[1].text()).toContain("说明.pdf");
    expect(rows[1].text()).toContain("引用 source");
    expect(rows[1].text()).toContain("底层错误");
  });

  it("⋯ 打开的真实菜单不会被同一次点击立即关闭", async () => {
    const Host = defineComponent({
      components: { QueuedMessages, AppContextMenu },
      template: `<div><QueuedMessages :items="[{ requestId: 'a', message: '菜单消息', position: 1, status: 'queued' }]" :busy="false" :can-steer="true" error="" /><AppContextMenu /></div>`,
    });
    const pinia = createPinia();
    setActivePinia(pinia);
    const host = mount(Host, { attachTo: document.body, global: { plugins: [pinia] } });
    await host.find("[aria-label='排队消息操作']").trigger("click");
    await nextTick();
    expect(document.querySelector(".app-context-menu")).not.toBeNull();
    expect([...document.querySelectorAll(".context-menu-label")].map((node) => node.textContent?.trim())).toEqual(["编辑消息", "在侧边工作区中打开", "关闭排队"]);
    host.unmount();
    closeContextMenu();
  });

  it("⋯ 菜单提供编辑、侧边工作区与关闭排队", async () => {
    const wrapper = mount(QueuedMessages, { props: {
      items: [
        { requestId: "a", message: "待编辑的消息", position: 1, status: "queued" },
      ], busy: false, canSteer: true, error: "",
    } });
    const more = wrapper.find("[aria-label='排队消息操作']");
    await more.trigger("click");
    expect(contextMenuState.visible).toBe(true);
    expect(contextMenuState.items.map((item) => item.label)).toEqual(["编辑消息", "在侧边工作区中打开", "关闭排队"]);
    await contextMenuState.items[0].action?.();
    await contextMenuState.items[1].action?.();
    await contextMenuState.items[2].action?.();
    expect(wrapper.emitted("edit")?.[0]).toEqual(["a"]);
    expect(wrapper.emitted("openSide")?.[0]).toEqual(["a"]);
    expect(wrapper.emitted("remove")?.[0]).toEqual(["a"]);
  });

  it("编辑中的排队消息有明确标识，非排队状态不可编辑", async () => {
    const wrapper = mount(QueuedMessages, { props: {
      items: [
        { requestId: "a", message: "待编辑的消息", position: 1, status: "running" },
      ], busy: false, canSteer: false, error: "", editingId: "a",
    } });
    expect(wrapper.find(".queue-item").classes()).toContain("is-editing");
    await wrapper.find("[aria-label='排队消息操作']").trigger("click");
    const edit = contextMenuState.items.find((item) => item.label === "编辑消息");
    expect(edit?.disabled).toBe(true);
  });

  it("单行队列条点击展开详情，失败项默认展开", async () => {
    const wrapper = mount(QueuedMessages, { props: {
      items: [
        { requestId: "a", message: "等待中的消息", position: 1, status: "queued", attachments: [{ artifactId: "art", ownerDeviceId: "device", storageDeviceId: "device", kind: "file", name: "说明.pdf", mimeType: "application/pdf", sha256: "abc", sizeBytes: 3 }] },
        { requestId: "b", message: "失败的消息", position: 2, status: "failed", error: "底层错误\n原因链" },
      ], busy: false, canSteer: true, error: "",
    } });
    const rows = wrapper.findAll(".queue-item");
    expect(rows[0].classes()).not.toContain("is-expanded");
    expect(rows[0].find(".queue-expand").attributes("aria-expanded")).toBe("false");
    await rows[0].find(".queue-expand").trigger("click");
    expect(rows[0].classes()).toContain("is-expanded");
    expect(rows[0].find(".queue-expand").attributes("aria-expanded")).toBe("true");
    expect(rows[0].find(".queue-body").text()).toContain("说明.pdf");
    await rows[0].find(".queue-expand").trigger("click");
    expect(rows[0].classes()).not.toContain("is-expanded");
    expect(rows[1].classes()).toContain("is-expanded");
    expect(rows[1].find(".queue-body").text()).toContain("底层错误");
  });
});
