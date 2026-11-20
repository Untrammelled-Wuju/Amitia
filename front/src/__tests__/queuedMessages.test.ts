import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import QueuedMessages from "../components/QueuedMessages.vue";

describe("排队消息操作", () => {
  it("拖拽调整顺序并提供引导与删除操作", async () => {
    const wrapper = mount(QueuedMessages, { props: {
      items: [
        { requestId: "a", message: "第一条", position: 1, status: "queued" },
        { requestId: "b", message: "第二条", position: 2, status: "queued" },
      ], busy: false, canSteer: true, error: "",
    } });
    const rows = wrapper.findAll(".queue-item");
    await rows[1].trigger("dragstart");
    await rows[0].trigger("drop");
    expect(wrapper.emitted("reorder")?.[0]).toEqual([["b", "a"]]);
    await rows[0].findAll("button")[2].trigger("click");
    await rows[1].findAll("button")[3].trigger("click");
    expect(wrapper.emitted("steer")?.[0]).toEqual(["a"]);
    expect(wrapper.emitted("remove")?.[0]).toEqual(["b"]);
  });

  it("提供键盘排序并保留附件、引用和完整错误", async () => {
    const wrapper = mount(QueuedMessages, { props: {
      items: [
        { requestId: "a", message: "第一条", position: 1, status: "queued" },
        { requestId: "b", message: "第二条", position: 2, status: "uncertain", error: "底层错误\n原因链", attachments: [{ kind: "file", name: "说明.pdf", mimeType: "application/pdf" }], quote: { ownerId: "device", characterId: "role", conversationId: "chat", messageId: "source", expectedRevision: 2 } },
      ], busy: false, canSteer: true, error: "",
    } });
    const rows = wrapper.findAll(".queue-item");
    expect(rows[1].attributes("tabindex")).toBe("0");
    expect(rows[1].text()).toContain("状态不确定");
    expect(rows[1].text()).toContain("说明.pdf");
    expect(rows[1].text()).toContain("引用 source");
    expect(rows[1].text()).toContain("底层错误");
    await rows[1].findAll("button")[0].trigger("click");
    expect(wrapper.emitted("reorder")?.[0]).toEqual([["b", "a"]]);
  });
});
