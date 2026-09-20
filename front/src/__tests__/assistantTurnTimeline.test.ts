import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import AssistantTurnTimeline from "../conversation/rendering/AssistantTurnTimeline.vue";

describe("AssistantTurnTimeline", () => {
  it("renders thinking, collapsed tool stream and text in sequence order", async () => {
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
              type: "thinking",
              status: "completed",
              content: "分析",
              durationMs: 1200,
            },
          ],
        },
      },
    });
    const classes = wrapper
      .findAll(".turn-thinking, .turn-tool-stream, .turn-text")
      .map((node) => node.classes().join(" "));
    expect(classes[0]).toContain("turn-thinking");
    expect(classes[1]).toContain("turn-tool-stream");
    expect(classes[2]).toContain("turn-text");
    expect(wrapper.find(".turn-tool-line").exists()).toBe(false);
    await wrapper.find(".turn-tool-stream-head").trigger("click");
    expect(wrapper.find(".turn-tool-line").exists()).toBe(true);
    expect(wrapper.find(".turn-tool-result").exists()).toBe(true);
  });
});
