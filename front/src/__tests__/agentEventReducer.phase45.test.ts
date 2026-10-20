import { describe, expect, it } from "vitest";
import { AgentEventReducer, type AgentUIEvent } from "../conversation/runtime/agentEventReducer";
import type { AssistantTurnData } from "../conversation/rendering/types";

const citation = { marker: "[K1]", kind: "knowledge" as const, index: 1, sourceId: "source-a" };

function historic(status = "running", revision = 5): AssistantTurnData {
  return {
    id: "turn-a", conversationId: "conversation-a", sequence: 1, status,
    executionId: "execution-a", requestId: "request-a", completedAt: status === "running" ? "" : "2026-10-16T10:00:00Z",
    items: [{ id: "text-a", turnId: "turn-a", conversationId: "conversation-a", sequence: 1,
      type: "text", status: "completed", revision, messageId: "message-a", isFinal: 1,
      content: "可靠引用 [K1]", citationSnapshot: [citation] }],
  };
}

function event(type: string, sequence: number, blockId = ""): AgentUIEvent {
  return { version: 1, eventId: `e-${sequence}`, eventSequence: sequence,
    conversationId: "conversation-a", turnId: "turn-a", blockId, type,
    payload: {}, createdAt: `2026-10-16T10:00:${String(sequence).padStart(2, "0")}Z` };
}

describe("Phase 45 terminal and historical projection invariants", () => {
  it("rejects late turn.started and approval events after completion", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic("completed")], 5);
    expect(reducer.apply(event("turn.started", 6))).toBe("stale");
    expect(reducer.apply(event("approval.approved", 7))).toBe("stale");
    expect(reducer.turn("turn-a")?.status).toBe("completed");
    expect(reducer.activeTurnId).toBe("");
  });

  it("keeps the first terminal status across conflicting terminal events", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic("failed")], 5);
    expect(reducer.apply(event("turn.completed", 6))).toBe("stale");
    expect(reducer.turn("turn-a")?.status).toBe("failed");
  });

  it("does not append stale deltas after cancellation", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic("interrupted")], 5);
    const delayed = { ...event("text.delta", 6, "text-a"), revision: 6, payload: { delta: "不应该追加" } };
    expect(reducer.apply(delayed)).toBe("stale");
    expect(reducer.turn("turn-a")?.items[0].content).toBe("可靠引用 [K1]");
  });

  it("rejects late text.completed after interruption without exposing a cancelled response", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic("interrupted")], 5);
    expect(reducer.apply({ ...event("text.completed", 6, "text-a"), revision: 6,
      payload: { content: "已取消后的回复", citationSnapshot: [citation] } })).toBe("stale");
    expect(reducer.turn("turn-a")?.items[0].content).toBe("可靠引用 [K1]");
  });

  it("can still accept a later authoritative text.completed citation snapshot", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic("completed")], 5);
    const done = { ...event("text.completed", 6, "text-a"), revision: 6,
      messageId: "message-a", payload: { content: "更新的引用 [K1]", citationSnapshot: [citation] } };
    expect(reducer.apply(done)).toBe("applied");
    expect(reducer.turn("turn-a")?.status).toBe("completed");
    expect(reducer.turn("turn-a")?.items[0].citationSnapshot).toEqual([citation]);
  });

  it("applies terminal historical status even when its item revisions are older", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic("running", 8)], 5);
    reducer.mergeTurns([historic("completed", 5)]);
    expect(reducer.turn("turn-a")?.status).toBe("completed");
    expect(reducer.turn("turn-a")?.items[0].revision).toBe(8);
    expect(reducer.turn("turn-a")?.items[0].citationSnapshot).toEqual([citation]);
  });

  it("clears the active turn when canonical history closes it", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic("running", 8)], 5);
    expect(reducer.apply(event("turn.started", 6))).toBe("applied");
    expect(reducer.activeTurnId).toBe("turn-a");
    reducer.mergeTurns([historic("completed", 5)]);
    expect(reducer.turn("turn-a")?.status).toBe("completed");
    expect(reducer.activeTurnId).toBe("");
  });

  it("does not regress terminal state after merging a newer running DTO", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic("interrupted", 5)], 5);
    reducer.mergeTurns([historic("running", 9)]);
    expect(reducer.turn("turn-a")?.status).toBe("interrupted");
    expect(reducer.turn("turn-a")?.items[0].revision).toBe(9);
  });

  it("preserves newer item content across history merges with newer total revision", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic("running", 9)], 5);
    const incoming = historic("completed", 4);
    incoming.items.push({ id: "tool-a", turnId: "turn-a", conversationId: "conversation-a", sequence: 2,
      revision: 20, type: "tool_call", status: "completed" });
    reducer.mergeTurns([incoming]);
    expect(reducer.turn("turn-a")?.items.find((item) => item.id === "text-a")?.revision).toBe(9);
    expect(reducer.turn("turn-a")?.items.find((item) => item.id === "tool-a")?.revision).toBe(20);
    expect(reducer.turn("turn-a")?.status).toBe("completed");
  });

  it("does not resurrect a completed historical turn on active snapshot reset", () => {
    const reducer = new AgentEventReducer();
    reducer.reset([historic("completed")], 5, { turnId: "turn-a", status: "running",
      lastEventSequence: 5, blocks: [{ blockId: "text-a", blockSequence: 1, revision: 4, status: "running", type: "text" }] });
    expect(reducer.turn("turn-a")?.status).toBe("completed");
    expect(reducer.activeTurnId).toBe("");
  });
});
