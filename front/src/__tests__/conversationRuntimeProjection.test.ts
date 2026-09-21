import { describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import type { AgentUIEvent } from "@/conversation/runtime/agentEventReducer";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock("@/composables/useApi", () => ({
  useApi: () => ({ get: mocks.get }),
}));

vi.mock("@/runtime/runtime-adapter", () => ({
  resolveApiUrl: vi.fn(async (value: string) => value),
}));

vi.mock("@/runtime/request-auth", () => ({
  createAuthenticatedFetchInit: vi.fn(async () => ({})),
}));

vi.mock("@/runtime/desktop-pet-chat-state", () => ({
  notifyDesktopPetChatState: vi.fn(),
}));

import { useConversationRuntime } from "@/composables/useConversationRuntime";

function event(
  eventSequence: number,
  type: string,
  overrides: Partial<AgentUIEvent> = {},
): AgentUIEvent {
  return {
    version: 1,
    eventId: `event-${eventSequence}`,
    eventSequence,
    conversationId: "conversation-1",
    requestId: "request-1",
    executionId: "execution-1",
    turnId: "turn-1",
    turnSequence: 1,
    type,
    createdAt: `2026-09-21T00:00:0${eventSequence}Z`,
    ...overrides,
  };
}

describe("conversation runtime projection", () => {
  it("keeps the live message identity stable until persistence replaces it", async () => {
    const conversationId = ref("conversation-1");
    const persistedMessages = ref<any[]>([
      {
        id: "user-1",
        role: "user",
        content: "hello",
        conversationId: "conversation-1",
        createdAt: "2026-09-21T00:00:00Z",
      },
    ]);
    const sending = ref(false);
    const runtime = useConversationRuntime(
      conversationId,
      persistedMessages,
      sending,
      () => undefined,
    );
    const messages = runtime.messages;
    const firstUser = messages.value.find((message) => message.role === "user");
    runtime.beginPendingAssistant("request-1", "character-1");
    const pendingAssistant = messages.value.find((message) => message.uiKey === "request:request-1");
    expect(pendingAssistant?.assistantTurn?.status).toBe("queued");
    expect(pendingAssistant?.assistantTurn?.items).toHaveLength(0);
    expect(pendingAssistant?.characterId).toBe("character-1");

    await runtime.applyEvent(event(1, "turn.queued", { status: "queued" }));
    await runtime.applyEvent(event(2, "turn.started", { status: "running" }));
    await runtime.applyEvent(event(3, "text.started", {
      blockId: "block-1",
      blockSequence: 1,
      revision: 1,
      status: "running",
      payload: { blockType: "text" },
    }));
    await runtime.applyEvent(event(4, "text.delta", {
      blockId: "block-1",
      blockSequence: 1,
      revision: 2,
      status: "running",
      payload: { blockType: "text", delta: "first" },
    }));
    await new Promise((resolve) => setTimeout(resolve, 40));

    const firstLive = messages.value.find((message) => message.uiKey === "request:request-1");
    expect(firstLive).toBeTruthy();

    await runtime.applyEvent(event(5, "text.delta", {
      blockId: "block-1",
      blockSequence: 1,
      revision: 3,
      status: "running",
      payload: { blockType: "text", delta: " second" },
    }));
    await new Promise((resolve) => setTimeout(resolve, 40));

    const secondLive = messages.value.find((message) => message.uiKey === "request:request-1");
    expect(secondLive).toBeTruthy();
    expect(secondLive.content).toBe("first second");
    expect(secondLive).not.toBe(firstLive);
    expect(messages.value.find((message) => message.role === "user")).toBe(firstUser);

    let resolveSnapshot: (value: any) => void = () => undefined;
    mocks.get.mockImplementationOnce(
      () => new Promise((resolve) => {
        resolveSnapshot = resolve;
      }),
    );

    await runtime.applyEvent(event(6, "text.completed", {
      blockId: "block-1",
      blockSequence: 1,
      messageId: "assistant-1",
      messageSequence: 2,
      revision: 4,
      status: "completed",
      payload: { blockType: "text", content: "first second" },
    }));
    const terminal = runtime.applyEvent(event(7, "turn.completed", {
      messageId: "assistant-1",
      messageSequence: 2,
      status: "completed",
      payload: { messageIds: ["assistant-1"] },
    }));
    await new Promise((resolve) => setTimeout(resolve, 40));

    expect(messages.value.filter((message) => message.role === "assistant")).toHaveLength(1);
    expect(messages.value.find((message) => message.role === "assistant")?.uiKey).toBe("request:request-1");

    resolveSnapshot({
      version: 1,
      revision: 1,
      lastEventSequence: 7,
      conversation: { id: "conversation-1" },
      messages: [
        {
          id: "user-1",
          role: "user",
          content: "hello",
          conversationId: "conversation-1",
          createdAt: "2026-09-21T00:00:00Z",
        },
        {
          id: "assistant-1",
          role: "assistant",
          content: "first second",
          conversationId: "conversation-1",
          createdAt: "2026-09-21T00:00:07Z",
        },
      ],
      turns: [
        {
          id: "turn-1",
          conversationId: "conversation-1",
          requestId: "request-1",
          executionId: "execution-1",
          sequence: 1,
          status: "completed",
          createdAt: "2026-09-21T00:00:00Z",
          updatedAt: "2026-09-21T00:00:07Z",
          items: [
            {
              id: "block-1",
              turnId: "turn-1",
              conversationId: "conversation-1",
              sequence: 1,
              type: "text",
              status: "completed",
              revision: 4,
              content: "first second",
              messageId: "assistant-1",
              isFinal: 1,
            },
          ],
        },
      ],
      activeTurn: null,
      approvals: [],
      messageHistory: { hasMore: false, nextBefore: 0 },
      turnHistory: { hasMore: false, nextBefore: 1 },
    });
    await terminal;
    await new Promise((resolve) => setTimeout(resolve, 40));

    const assistantMessages = messages.value.filter((message) => message.role === "assistant");
    expect(assistantMessages).toHaveLength(1);
    expect(assistantMessages[0].id).toBe("assistant-1");
    expect(assistantMessages[0].uiKey).toBe("request:request-1");
  });

  it("removes the pending assistant when sending fails", () => {
    const conversationId = ref("conversation-1");
    const persistedMessages = ref<any[]>([]);
    const sending = ref(false);
    const runtime = useConversationRuntime(
      conversationId,
      persistedMessages,
      sending,
      () => undefined,
    );

    runtime.beginPendingAssistant("request-failed");
    expect(runtime.messages.value.some((message) => message.uiKey === "request:request-failed")).toBe(true);

    runtime.failPendingAssistant("request-failed");
    expect(runtime.messages.value.some((message) => message.uiKey === "request:request-failed")).toBe(false);
  });
});
