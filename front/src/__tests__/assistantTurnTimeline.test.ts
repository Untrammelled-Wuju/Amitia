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
    expect(wrapper.find(".turn-tool-result").exists()).toBe(true);
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
});
