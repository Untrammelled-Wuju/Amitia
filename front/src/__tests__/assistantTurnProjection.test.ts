import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { ref } from "vue";
import { useAssistantTurns } from "../composables/useAssistantTurns";
import AIMessageRenderer from "../conversation/rendering/AIMessageRenderer.vue";

describe("assistant turn projection", () => {
  it("keeps legacy reasoning while matching a turn", () => {
    const messages = ref<any[]>([
      {
        id: "assistant-1",
        role: "assistant",
        requestId: "request-1",
        content: "回复",
        reasoningContent: "思考内容",
        reasoningDurationMs: 1200,
        generationPending: true,
      },
    ]);
    const { turns, applyTurns } = useAssistantTurns(ref("conversation-1"), messages);
    turns.value = [
      {
        id: "turn-1",
        conversationId: "conversation-1",
        requestId: "request-1",
        responseGroupId: "request-1",
        sequence: 1,
        status: "running",
        items: [],
      },
    ];

    applyTurns();

    expect(messages.value[0].assistantTurn?.id).toBe("turn-1");
    expect(messages.value[0].reasoningContent).toBe("思考内容");
    expect(messages.value[0].reasoningDurationMs).toBe(1200);
  });

  it("renders fallback thinking and text while turn items are incomplete", () => {
    const wrapper = mount(AIMessageRenderer, {
      global: {
        stubs: {
          "el-icon": true,
        },
      },
      props: {
        message: {
          id: "assistant-1",
          role: "assistant",
          content: "最终回复",
          reasoningContent: "思考内容",
          reasoningDurationMs: 1200,
          status: "sent",
          createdAt: "2026-09-20T00:00:00.000Z",
          assistantTurn: {
            id: "turn-1",
            conversationId: "conversation-1",
            sequence: 1,
            status: "completed",
            items: [],
          },
        },
      },
    });

    expect(wrapper.find(".amrp-thinking").text()).toContain("思考完成");
    expect(wrapper.find(".turn-text").text()).toContain("最终回复");
  });

  it("keeps a running thinking placeholder before turn items arrive", () => {
    const wrapper = mount(AIMessageRenderer, {
      global: {
        stubs: {
          "el-icon": true,
        },
      },
      props: {
        message: {
          id: "assistant-1",
          role: "assistant",
          content: "",
          generationPending: true,
          status: "streaming",
          createdAt: "2026-09-20T00:00:00.000Z",
          assistantTurn: {
            id: "turn-1",
            conversationId: "conversation-1",
            sequence: 1,
            status: "running",
            items: [],
          },
        },
      },
    });

    expect(wrapper.find(".amrp-thinking").text()).toContain("思考中");
  });
});
