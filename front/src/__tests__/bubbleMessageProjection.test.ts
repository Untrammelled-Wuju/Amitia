import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { normalizeAIMessage } from "../conversation/rendering/amrp";
import { flowBubbleText, projectBubbleContent, splitBubbleText } from "../conversation/rendering/bubbleMessageProjection";
import type { AssistantTurnData } from "../conversation/rendering/types";

describe("bubble message delivery", () => {
  beforeEach(() => { vi.resetModules(); localStorage.clear(); });

  it("delivers only delimited text and flushes the tail at completion", () => {
    expect(splitBubbleText("第一段[AMI", false)).toEqual([]);
    expect(splitBubbleText("第一段[AMITIA_BR]第二段", false)).toEqual(["第一段"]);
    expect(splitBubbleText("第一段[AMITIA_BR]第二段", true)).toEqual(["第一段", "第二段"]);
    expect(splitBubbleText("[AMITIA_BR]第一段[AMITIA_BR][AMITIA_BR]", true)).toEqual(["第一段"]);
    expect(splitBubbleText("中断的尾段[AMITI", true)).toEqual(["中断的尾段"]);
  });

  it("keeps literal delimiters in fenced and inline code", () => {
    const content = "`[AMITIA_BR]`\n```js\nconst a = '[AMITIA_BR]';\n```[AMITIA_BR]结束";
    expect(splitBubbleText(content, true)).toEqual([content.slice(0, content.lastIndexOf("[AMITIA_BR]")), "结束"]);
  });

  it("keeps tool pairs together, thinking separate, stable keys and partial reasoning hidden", () => {
    const message = normalizeAIMessage({ id: "a1", role: "assistant", status: "streaming", content: "" });
    const turn: AssistantTurnData = { id: "t1", conversationId: "c1", sequence: 1, status: "running", items: [
      { id: "r1", turnId: "t1", conversationId: "c1", sequence: 1, type: "reasoning", status: "running", content: "不能逐字展示" },
      { id: "c1", turnId: "t1", conversationId: "c1", sequence: 2, type: "tool_call", status: "running", callId: "call-1", toolName: "read_file" },
      { id: "x1", turnId: "t1", conversationId: "c1", sequence: 3, type: "text", status: "running", content: "已读取[AMITIA_BR]未完成" },
      { id: "result1", turnId: "t1", conversationId: "c1", sequence: 4, type: "tool_result", status: "completed", callId: "call-1", toolName: "read_file" },
    ] };
    const projected = projectBubbleContent(message, turn);
    expect(projected.map(item => item.kind)).toEqual(["thinking", "turn", "text"]);
    expect(projected[0]).toMatchObject({ content: "", complete: false });
    expect(projected[1].kind === "turn" && projected[1].turn.items).toHaveLength(2);
    expect(projectBubbleContent(message, turn)).toEqual(projected);
    const completed = projectBubbleContent(message, { ...turn, status: "interrupted" });
    expect(completed.map(item => item.key).slice(0, 3)).toEqual(projected.map(item => item.key));
    expect(completed.at(-1)).toMatchObject({ kind: "text", content: "未完成" });
  });

  it.each(["user", "assistant"])("projects %s multimedia as independent bubbles", role => {
    const message = normalizeAIMessage({ id: "m1", role, content: "说明", status: "sent", attachments: [
      { kind: "audio", url: "/voice.wav" }, { kind: "image", url: "/image.png" },
      { kind: "video", url: "/video.mp4" }, { kind: "file", url: "/doc.pdf", name: "文件" },
    ] });
    expect(projectBubbleContent(message).map(item => item.kind === "block" ? item.block.kind : item.kind)).toEqual(["text", "audio", "image", "video", "file"]);
  });

  it("handles resource-only media and converts history separators when returning to flow", () => {
    for (const type of ["audio", "image", "video"]) {
      const normalized = normalizeAIMessage({ id: type, role: "user", msgType: type, resourceUri: "amitia://artifacts/a1", status: "sent" });
      expect(normalized.blocks[0]).toMatchObject({ kind: type, url: "amitia://artifacts/a1" });
    }
    expect(flowBubbleText("第一段[AMITIA_BR]第二段")).toBe("第一段\n\n第二段");
    expect(flowBubbleText("`[AMITIA_BR]`")).toBe("`[AMITIA_BR]`");
  });

  it("renders independent bubbles without streaming Markdown", async () => {
    const Content = (await import("../conversation/rendering/BubbleMessageContent.vue")).default;
    const wrapper = mount(Content, {
      props: { message: normalizeAIMessage({ id: "m1", role: "assistant", status: "streaming", content: "完成的消息[AMITIA_BR]隐藏的消息" }), pending: true },
      global: { stubs: { MarkdownContent: { props: ["source", "streaming"], template: '<p :data-streaming="streaming">{{ source }}</p>' } } },
    });
    expect(wrapper.findAll('[data-bubble-kind="text"]')).toHaveLength(1);
    expect(wrapper.text()).not.toContain("隐藏的消息");
    expect(wrapper.get("p").attributes("data-streaming")).toBe("false");
    await wrapper.setProps({ message: normalizeAIMessage({ id: "m1", role: "assistant", status: "sent", content: "完成的消息[AMITIA_BR]隐藏的消息" }), pending: false });
    expect(wrapper.findAll('[data-bubble-kind="text"]')).toHaveLength(2);
    expect(wrapper.text()).not.toContain("[AMITIA_BR]");
    wrapper.unmount();
  });
});
