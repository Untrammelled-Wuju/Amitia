import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import AssistantTurnTimeline from "../conversation/rendering/AssistantTurnTimeline.vue";

describe("AssistantTurnTimeline", () => {
  it("renders reasoning, collapsed tool stream and text in sequence order", async () => {
    const wrapper = mount(AssistantTurnTimeline, {
      props: {
        turn: {
          id: "turn-1",
          conversationId: "conv-1",
          sequence: 1,
          status: "completed",
          items: [
            {
              id: "text-1",
              turnId: "turn-1",
              conversationId: "conv-1",
              sequence: 4,
              type: "text",
              status: "completed",
              content: "最终回复",
            },
            {
              id: "result-1",
              turnId: "turn-1",
              conversationId: "conv-1",
              sequence: 3,
              type: "tool_result",
              status: "completed",
              callId: "call-1",
              toolName: "read_file",
              resultJson: "\"ok\"",
            },
            {
              id: "call-1",
              turnId: "turn-1",
              conversationId: "conv-1",
              sequence: 2,
              type: "tool_call",
              status: "completed",
              callId: "call-1",
              toolName: "read_file",
              argumentsJson: "{\"path\":\"a.go\"}",
            },
            {
              id: "thinking-1",
              turnId: "turn-1",
              conversationId: "conv-1",
              sequence: 1,
              type: "reasoning",
              status: "completed",
              content: "分析",
              durationMs: 1200,
            },
          ],
        },
      },
    });
    const classes = wrapper
      .findAll(".amrp-thinking, .turn-tool-stream, .turn-text")
      .map((node) => node.classes().join(" "));
    expect(classes[0]).toContain("amrp-thinking");
    expect(classes[1]).toContain("turn-tool-stream");
    expect(classes[2]).toContain("turn-text");
    expect(wrapper.find(".turn-tool-line").exists()).toBe(false);
    await wrapper.find(".turn-tool-stream-head").trigger("click");
    expect(wrapper.find(".turn-tool-line").exists()).toBe(true);
    expect(wrapper.find(".turn-tool-result").exists()).toBe(false);
    expect(wrapper.find(".turn-tool-details").exists()).toBe(false);

    await wrapper.find(".turn-tool-expand").trigger("click");
    expect(wrapper.find(".turn-tool-details").exists()).toBe(true);
    expect(wrapper.text()).toContain("调用工具");
    expect(wrapper.text()).toContain("调用参数");
    expect(wrapper.text()).toContain("调用结果");
  });

  it("expands only the clicked tool stream", async () => {
    const wrapper = mount(AssistantTurnTimeline, {
      props: {
        turn: {
          id: "turn-2",
          conversationId: "conv-1",
          sequence: 2,
          status: "completed",
          items: [
            {
              id: "call-1",
              turnId: "turn-2",
              conversationId: "conv-1",
              sequence: 1,
              type: "tool_call",
              status: "completed",
              callId: "call-1",
              toolName: "read_file",
              argumentsJson: "{\"path\":\"a.go\"}",
            },
            {
              id: "result-1",
              turnId: "turn-2",
              conversationId: "conv-1",
              sequence: 2,
              type: "tool_result",
              status: "completed",
              callId: "call-1",
              toolName: "read_file",
              resultJson: "\"ok\"",
            },
            {
              id: "text-1",
              turnId: "turn-2",
              conversationId: "conv-1",
              sequence: 3,
              type: "text",
              status: "completed",
              content: "中间回复",
            },
            {
              id: "call-2",
              turnId: "turn-2",
              conversationId: "conv-1",
              sequence: 4,
              type: "tool_call",
              status: "completed",
              callId: "call-2",
              toolName: "write_file",
              argumentsJson: "{\"path\":\"b.go\"}",
            },
            {
              id: "result-2",
              turnId: "turn-2",
              conversationId: "conv-1",
              sequence: 5,
              type: "tool_result",
              status: "completed",
              callId: "call-2",
              toolName: "write_file",
              resultJson: "\"ok\"",
            },
          ],
        },
      },
    });

    const streams = wrapper.findAll(".turn-tool-stream");
    const heads = wrapper.findAll(".turn-tool-stream-head");
    expect(streams).toHaveLength(2);
    expect(heads.map((head) => head.attributes("aria-expanded"))).toEqual([
      "false",
      "false",
    ]);

    await heads[0].trigger("click");
    expect(heads.map((head) => head.attributes("aria-expanded"))).toEqual([
      "true",
      "false",
    ]);
    expect(streams[0].find(".turn-tool-stream-body").exists()).toBe(true);
    expect(streams[1].find(".turn-tool-stream-body").exists()).toBe(false);

    await heads[0].trigger("click");
    await heads[1].trigger("click");
    expect(heads.map((head) => head.attributes("aria-expanded"))).toEqual([
      "false",
      "true",
    ]);
    expect(streams[0].find(".turn-tool-stream-body").exists()).toBe(false);
    expect(streams[1].find(".turn-tool-stream-body").exists()).toBe(true);
  });

  it("counts failed tools once per call", () => {
    const wrapper = mount(AssistantTurnTimeline, {
      props: {
        turn: {
          id: "turn-3",
          conversationId: "conv-1",
          sequence: 3,
          status: "completed",
          items: [
            {
              id: "call-1",
              turnId: "turn-3",
              conversationId: "conv-1",
              sequence: 1,
              type: "tool_call",
              status: "completed",
              callId: "call-1",
              toolName: "read_file",
            },
            {
              id: "call-2",
              turnId: "turn-3",
              conversationId: "conv-1",
              sequence: 2,
              type: "tool_call",
              status: "failed",
              callId: "call-2",
              toolName: "write_file",
            },
            {
              id: "call-3",
              turnId: "turn-3",
              conversationId: "conv-1",
              sequence: 3,
              type: "tool_call",
              status: "completed",
              callId: "call-3",
              toolName: "calculate",
            },
            {
              id: "result-1",
              turnId: "turn-3",
              conversationId: "conv-1",
              sequence: 4,
              type: "tool_result",
              status: "completed",
              callId: "call-1",
              toolName: "read_file",
            },
            {
              id: "result-2",
              turnId: "turn-3",
              conversationId: "conv-1",
              sequence: 5,
              type: "tool_result",
              status: "failed",
              callId: "call-2",
              toolName: "write_file",
            },
            {
              id: "result-3",
              turnId: "turn-3",
              conversationId: "conv-1",
              sequence: 6,
              type: "tool_result",
              status: "completed",
              callId: "call-3",
              toolName: "calculate",
            },
          ],
        },
      },
    });

    expect(wrapper.find(".turn-tool-stream-summary").text()).toBe("3 个工具 · 1 个失败");
  });

  it("counts completed tools once while running", () => {
    const wrapper = mount(AssistantTurnTimeline, {
      props: {
        turn: {
          id: "turn-4",
          conversationId: "conv-1",
          sequence: 4,
          status: "running",
          items: [
            {
              id: "call-1",
              turnId: "turn-4",
              conversationId: "conv-1",
              sequence: 1,
              type: "tool_call",
              status: "completed",
              callId: "call-1",
              toolName: "read_file",
            },
            {
              id: "call-2",
              turnId: "turn-4",
              conversationId: "conv-1",
              sequence: 2,
              type: "tool_call",
              status: "running",
              callId: "call-2",
              toolName: "write_file",
            },
            {
              id: "call-3",
              turnId: "turn-4",
              conversationId: "conv-1",
              sequence: 3,
              type: "tool_call",
              status: "running",
              callId: "call-3",
              toolName: "calculate",
            },
            {
              id: "result-1",
              turnId: "turn-4",
              conversationId: "conv-1",
              sequence: 4,
              type: "tool_result",
              status: "completed",
              callId: "call-1",
              toolName: "read_file",
            },
          ],
        },
      },
    });

    expect(wrapper.find(".turn-tool-stream-summary").text()).toBe("执行中 · 1/3 完成");
  });

  it("deduplicates legacy results without call records", () => {
    const wrapper = mount(AssistantTurnTimeline, {
      props: {
        turn: {
          id: "turn-5",
          conversationId: "conv-1",
          sequence: 5,
          status: "completed",
          items: [
            {
              id: "result-1",
              turnId: "turn-5",
              conversationId: "conv-1",
              sequence: 1,
              type: "tool_result",
              status: "completed",
              callId: "call-1",
              toolName: "read_file",
            },
            {
              id: "result-1-copy",
              turnId: "turn-5",
              conversationId: "conv-1",
              sequence: 2,
              type: "tool_result",
              status: "completed",
              callId: "call-1",
              toolName: "read_file",
            },
            {
              id: "result-2",
              turnId: "turn-5",
              conversationId: "conv-1",
              sequence: 3,
              type: "tool_result",
              status: "completed",
              callId: "call-2",
              toolName: "calculate",
            },
          ],
        },
      },
    });

    expect(wrapper.find(".turn-tool-stream-summary").text()).toBe("2 个工具 · 已完成");
  });
});
