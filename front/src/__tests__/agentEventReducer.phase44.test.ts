import { describe, expect, it } from "vitest";
import { AgentEventReducer, type AgentUIEvent, type RuntimeBlockSnapshot } from "../conversation/runtime/agentEventReducer";
import type { AssistantTurnData } from "../conversation/rendering/types";

const citation = { marker: "[K1]", kind: "knowledge" as const, index: 1, sourceId: "source-1" };

function historic(): AssistantTurnData {
  return {
    id: "turn-1", conversationId: "conversation-1", characterId: "character-1",
    requestId: "request-1", executionId: "execution-1", sequence: 1, status: "running",
    items: [{ id: "text-1", turnId: "turn-1", conversationId: "conversation-1",
      sequence: 1, revision: 5, type: "text", status: "completed",
      content: "引用 [K1]", messageId: "message-1", isFinal: 1,
      citationSnapshot: [citation] }],
  };
}

function active(block: RuntimeBlockSnapshot) {
  return { turnId: "turn-1", turnSequence: 1, requestId: "request-1",
    executionId: "execution-1", status: "running", blocks: [block], lastEventSequence: 0 };
}

describe("Phase 44 Snapshot and Replay Citation contract", () => {
  it("preserves frozen references when active Snapshot omits citation metadata", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic()], 10, active({ blockId: "text-1", blockSequence: 1,
      type: "text", status: "completed", revision: 5 }));
    const turn = reducer.turn("turn-1")!;
    expect(turn.conversationId).toBe("conversation-1");
    expect(turn.items[0].messageId).toBe("message-1");
    expect(turn.items[0].citationSnapshot).toEqual([citation]);
    expect(turn.items[0].content).toBe("引用 [K1]");
  });

  it("rejects stale runtime blocks without losing newer citation", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic()], 10, active({ blockId: "text-1", blockSequence: 1,
      type: "text", status: "running", revision: 4, content: "旧正文",
      payload: { citationSnapshot: [] } }));
    expect(reducer.turn("turn-1")?.items[0].revision).toBe(5);
    expect(reducer.turn("turn-1")?.items[0].content).toBe("引用 [K1]");
    expect(reducer.turn("turn-1")?.items[0].citationSnapshot).toEqual([citation]);
  });

  it("respects explicit empty content and citation array on a newer checkpoint", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic()], 10, active({ blockId: "text-1", blockSequence: 1,
      type: "text", status: "completed", revision: 6, content: "",
      payload: { citationSnapshot: [] } }));
    expect(reducer.turn("turn-1")?.items[0].content).toBe("");
    expect(reducer.turn("turn-1")?.items[0].citationSnapshot).toEqual([]);
  });

  it("replays a final reference exactly once, then stays terminal", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic()], 20, active({ blockId: "text-1", blockSequence: 1,
      type: "text", status: "streaming", revision: 5 }));
    const finish: AgentUIEvent = {
      version: 1, eventId: "event-21", eventSequence: 21,
      conversationId: "conversation-1", turnId: "turn-1", blockId: "text-1",
      revision: 6, type: "text.completed", messageId: "message-1",
      payload: { blockType: "text", content: "答案 [K1]", citationSnapshot: [citation] },
    };
    expect(reducer.apply(finish)).toBe("applied");
    expect(reducer.apply(finish)).toBe("duplicate");
    expect(reducer.apply({ ...finish, eventId: "event-22", eventSequence: 22,
      type: "turn.completed", blockId: "", payload: {} })).toBe("applied");
    const item = reducer.turn("turn-1")?.items[0];
    expect(item?.content).toBe("答案 [K1]");
    expect(item?.citationSnapshot).toMatchObject([citation]);
    expect(reducer.turn("turn-1")?.status).toBe("completed");
  });
});
