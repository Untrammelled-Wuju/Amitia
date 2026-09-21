import { describe, expect, it } from "vitest";
import { AgentEventReducer, type AgentUIEvent } from "../conversation/runtime/agentEventReducer";
import type { AssistantTurnData, AssistantTurnItem } from "../conversation/rendering/types";
import fixture01 from "../../../contracts/agent-runtime/v1/fixtures/01_simple_text.json";
import fixture02 from "../../../contracts/agent-runtime/v1/fixtures/02_reasoning_text.json";
import fixture03 from "../../../contracts/agent-runtime/v1/fixtures/03_tool.json";
import fixture04 from "../../../contracts/agent-runtime/v1/fixtures/04_parallel_tools.json";
import fixture05 from "../../../contracts/agent-runtime/v1/fixtures/05_tool_failure.json";
import fixture06 from "../../../contracts/agent-runtime/v1/fixtures/06_interrupt.json";
import fixture07 from "../../../contracts/agent-runtime/v1/fixtures/07_reconnect.json";
import fixture08 from "../../../contracts/agent-runtime/v1/fixtures/08_approval.json";
import fixture09 from "../../../contracts/agent-runtime/v1/fixtures/09_retry.json";
import fixture10 from "../../../contracts/agent-runtime/v1/fixtures/10_history_replay.json";
import fixture11 from "../../../contracts/agent-runtime/v1/fixtures/11_duplicate_event.json";
import fixture12 from "../../../contracts/agent-runtime/v1/fixtures/12_out_of_order_event.json";

type FixtureBlock = {
  blockId: string;
  blockSequence: number;
  type: string;
  status: string;
  content?: string;
  callId?: string;
  arguments?: string;
  result?: string;
};

type FixtureTurn = {
  turnId: string;
  turnSequence: number;
  executionId: string;
  status: string;
  blocks: FixtureBlock[];
};

type Fixture = {
  name: string;
  version: number;
  initialState: { lastEventSequence: number; turns: FixtureTurn[] };
  events: AgentUIEvent[];
  expectedState: { lastEventSequence: number; activeTurnId: string; turns: FixtureTurn[] };
  expectedApplyResults?: string[];
};

const fixtures = [
  fixture01, fixture02, fixture03, fixture04, fixture05, fixture06,
  fixture07, fixture08, fixture09, fixture10, fixture11, fixture12,
] as unknown as Fixture[];

function initialItem(turnId: string, block: FixtureBlock): AssistantTurnItem {
  return {
    id: block.blockId,
    turnId,
    conversationId: "conversation-1",
    sequence: block.blockSequence,
    type: block.type,
    status: block.status,
    content: block.content || "",
    callId: block.callId,
    argumentsJson: block.arguments || "",
    resultJson: block.result || "",
  };
}

function initialTurn(turn: FixtureTurn): AssistantTurnData {
  return {
    id: turn.turnId,
    conversationId: "conversation-1",
    executionId: turn.executionId,
    sequence: turn.turnSequence,
    status: turn.status,
    items: turn.blocks.map((block) => initialItem(turn.turnId, block)),
  };
}

function normalizedBlock(item: AssistantTurnItem): FixtureBlock {
  const result: FixtureBlock = {
    blockId: item.id,
    blockSequence: Number(item.sequence || 0),
    type: String(item.type || ""),
    status: String(item.status || ""),
  };
  if (item.content) result.content = item.content;
  if (item.callId) result.callId = item.callId;
  if (item.argumentsJson) result.arguments = item.argumentsJson;
  if (item.resultJson) result.result = item.resultJson;
  return result;
}

function normalizedTurn(turn: AssistantTurnData): FixtureTurn {
  return {
    turnId: turn.id,
    turnSequence: Number(turn.sequence || 0),
    executionId: String(turn.executionId || ""),
    status: String(turn.status || ""),
    blocks: (turn.items || []).map(normalizedBlock),
  };
}

describe("Agent Runtime v1 golden fixtures", () => {
  for (const fixture of fixtures) {
    it(fixture.name, () => {
      expect(fixture.version).toBe(1);
      const reducer = new AgentEventReducer();
      reducer.reset(
        fixture.initialState.turns.map(initialTurn),
        fixture.initialState.lastEventSequence,
        null,
      );
      const results = fixture.events.map((event) => reducer.apply(event));
      if (fixture.expectedApplyResults) expect(results).toEqual(fixture.expectedApplyResults);
      expect({
        lastEventSequence: reducer.lastEventSequence,
        activeTurnId: reducer.activeTurnId,
        turns: reducer.turns.map(normalizedTurn),
      }).toEqual(fixture.expectedState);
    });
  }
});
