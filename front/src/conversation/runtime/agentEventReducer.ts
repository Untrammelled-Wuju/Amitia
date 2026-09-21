import type { AssistantTurnData, AssistantTurnItem } from "@/conversation/rendering/types";

export interface AgentUIEvent {
  version: number;
  eventId: string;
  eventSequence: number;
  conversationId: string;
  requestId?: string;
  executionId?: string;
  turnId?: string;
  parentTurnId?: string;
  parentBlockId?: string;
  agentId?: string;
  turnSequence?: number;
  blockId?: string;
  blockSequence?: number;
  messageId?: string;
  messageSequence?: number;
  callId?: string;
  revision?: number;
  type: string;
  status?: string;
  payload?: Record<string, any>;
  createdAt?: string;
}

export interface RuntimeBlockSnapshot {
  blockId: string;
  blockSequence: number;
  type: string;
  status: string;
  callId?: string;
  revision?: number;
  content?: string;
  payload?: Record<string, any>;
}

export interface RuntimeTurnSnapshot {
  turnId: string;
  turnSequence?: number;
  requestId?: string;
  executionId?: string;
  parentTurnId?: string;
  parentBlockId?: string;
  agentId?: string;
  status: string;
  lastEventSequence: number;
  blocks?: RuntimeBlockSnapshot[];
  startedAt?: string;
  lastModifiedAt?: string;
}

export type AgentEventApplyResult = "applied" | "duplicate" | "stale" | "gap" | "unsupported";

const terminalTurnStates = new Set(["completed", "failed", "interrupted"]);

export function normalizedTurnStatus(status: unknown): string {
  return String(status || "running").trim().toLowerCase();
}

export function isTerminalTurn(status: unknown): boolean {
  return terminalTurnStates.has(normalizedTurnStatus(status));
}

function itemTypeFromBlock(type: string): string {
  if (type === "tool") return "tool_call";
  return type || "text";
}

function eventBlockType(event: AgentUIEvent): string {
  const payloadType = String(event.payload?.blockType || "").trim();
  if (payloadType) return payloadType;
  if (event.type.startsWith("reasoning.")) return "reasoning";
  if (event.type.startsWith("text.")) return "text";
  if (event.type.startsWith("tool.")) return "tool_call";
  return String(event.type || "").split(".", 1)[0] || "text";
}

function initialTurnStatus(event: AgentUIEvent): string {
  switch (event.type) {
    case "turn.queued": return "queued";
    case "turn.started": return "running";
    case "turn.cancelling": return "cancelling";
    case "turn.completed": return "completed";
    case "turn.failed": return "failed";
    case "turn.interrupted": return "interrupted";
    case "approval.requested": return "waiting_approval";
    default: return "running";
  }
}

function blockStatus(type: string, eventStatus: string | undefined, current: string): string {
  if (type === "tool.arguments.completed") return String(eventStatus || current || "running");
  if (type.endsWith(".failed")) return "failed";
  if (type.endsWith(".interrupted")) return "interrupted";
  if (type.endsWith(".completed")) return "completed";
  if (type === "tool.running") return "running";
  return String(eventStatus || current || "running");
}

function turnFreshness(turn: AssistantTurnData): number {
  return Number(turn.sequence || 0) * 1_000_000 + (turn.items || []).reduce((sum, item) => sum + Number(item.revision || 0), 0);
}

function cloneItem(item: AssistantTurnItem): AssistantTurnItem {
  return { ...item };
}

export function cloneTurn(turn: AssistantTurnData): AssistantTurnData {
  return { ...turn, items: (turn.items || []).map(cloneItem) };
}

export class AgentEventReducer {
  private readonly byTurnId = new Map<string, AssistantTurnData>();
  private readonly seenEventIds = new Set<string>();
  private readonly seenEventOrder: string[] = [];
  private lastSequenceValue = 0;
  private activeTurnIdValue = "";
  private activeExecutionIdValue = "";

  get lastEventSequence(): number { return this.lastSequenceValue; }
  get activeTurnId(): string { return this.activeTurnIdValue; }
  get activeExecutionId(): string { return this.activeExecutionIdValue; }
  get turns(): AssistantTurnData[] {
    return [...this.byTurnId.values()]
      .map(cloneTurn)
      .sort((a, b) => Number(a.sequence || 0) - Number(b.sequence || 0));
  }

  turn(id: string): AssistantTurnData | undefined {
    const value = this.byTurnId.get(String(id || "").trim());
    return value ? cloneTurn(value) : undefined;
  }

  mergeTurns(turns: AssistantTurnData[]): void {
    for (const raw of turns || []) {
      const incoming = cloneTurn(raw);
      incoming.status = normalizedTurnStatus(incoming.status);
      incoming.items.sort((a, b) => Number(a.sequence || 0) - Number(b.sequence || 0));
      const current = this.byTurnId.get(incoming.id);
      if (!current || turnFreshness(incoming) >= turnFreshness(current)) {
        this.byTurnId.set(incoming.id, incoming);
      }
    }
  }

  reset(turns: AssistantTurnData[], lastEventSequence: number, activeTurn?: RuntimeTurnSnapshot | null): void {
    this.byTurnId.clear();
    for (const raw of turns || []) {
      const turn = cloneTurn(raw);
      turn.status = normalizedTurnStatus(turn.status);
      turn.items.sort((a, b) => Number(a.sequence || 0) - Number(b.sequence || 0));
      this.byTurnId.set(turn.id, turn);
    }
    this.seenEventIds.clear();
    this.seenEventOrder.length = 0;
    this.lastSequenceValue = Number(lastEventSequence || 0);
    this.activeTurnIdValue = "";
    this.activeExecutionIdValue = "";

    if (activeTurn?.turnId) {
      const existing = this.byTurnId.get(activeTurn.turnId);
      const turn = this.turnFromRuntime(activeTurn, existing);
      this.byTurnId.set(turn.id, turn);
      this.activeTurnIdValue = turn.id;
      this.activeExecutionIdValue = String(turn.executionId || "");
      return;
    }
    for (const turn of this.turns.reverse()) {
      if (!isTerminalTurn(turn.status)) {
        this.activeTurnIdValue = turn.id;
        this.activeExecutionIdValue = String(turn.executionId || "");
        return;
      }
    }
  }

  apply(event: AgentUIEvent): AgentEventApplyResult {
    if (!event || Number(event.version) !== 1) return "unsupported";
    if (event.eventId && this.seenEventIds.has(event.eventId)) return "duplicate";
    const sequence = Number(event.eventSequence || 0);
    if (sequence > 0) {
      if (this.lastSequenceValue > 0 && sequence > this.lastSequenceValue + 1 && event.payload?.recoveryCheckpoint !== true) {
        return "gap";
      }
      if (sequence <= this.lastSequenceValue) {
        this.remember(event.eventId);
        return "stale";
      }
    }
    this.remember(event.eventId);
    if (sequence > this.lastSequenceValue) this.lastSequenceValue = sequence;

    const turnId = String(event.turnId || "").trim();
    if (!turnId) return "applied";
    let turn = this.byTurnId.get(turnId) || this.newTurn(event);
    const type = String(event.type || "").trim();

    if (["turn.queued", "turn.started", "turn.cancelling", "turn.steered", "approval.requested", "approval.approved", "approval.denied", "approval.expired"].includes(type)) {
      let nextStatus = normalizedTurnStatus(event.status || turn.status);
      if (type === "turn.queued") nextStatus = "queued";
      if (type === "turn.started") nextStatus = "running";
      if (type === "turn.cancelling") nextStatus = "cancelling";
      if (type === "approval.requested") nextStatus = "waiting_approval";
      if (type === "approval.approved" || type === "approval.denied" || type === "approval.expired") nextStatus = "running";
      turn = {
        ...turn,
        requestId: event.requestId || turn.requestId,
        executionId: event.executionId || turn.executionId,
        sequence: Number(event.turnSequence || turn.sequence || 0),
        status: nextStatus,
        updatedAt: event.createdAt || turn.updatedAt,
      };
      this.byTurnId.set(turnId, turn);
      if (!isTerminalTurn(nextStatus)) {
        this.activeTurnIdValue = turnId;
        this.activeExecutionIdValue = String(turn.executionId || "");
      }
      return "applied";
    }

    if (["turn.completed", "turn.failed", "turn.interrupted"].includes(type)) {
      const terminalStatus = type === "turn.completed" ? "completed" : type === "turn.failed" ? "failed" : "interrupted";
      const items = (turn.items || []).map((item) => {
        const copy = cloneItem(item);
        if (terminalStatus !== "completed" && !["completed", "failed", "interrupted"].includes(String(copy.status || "").toLowerCase())) {
          copy.status = terminalStatus;
        }
        return copy;
      });
      if (terminalStatus === "failed" && !items.some((item) => item.type === "error")) {
        items.push({
          id: `turn-error:${turnId}`,
          turnId,
          conversationId: turn.conversationId,
          sequence: Number.MAX_SAFE_INTEGER,
          type: "error",
          status: "failed",
          revision: 1,
          content: String(event.payload?.userMessage || ""),
          resultJson: JSON.stringify(event.payload || {}),
          errorCode: String(event.payload?.errorCode || ""),
          createdAt: event.createdAt || "",
          updatedAt: event.createdAt || "",
        });
      }
      const messageId = String(event.messageId || "").trim();
      if (messageId) {
        for (let index = items.length - 1; index >= 0; index -= 1) {
          if (items[index].type !== "text") continue;
          items[index] = {
            ...items[index],
            messageId,
            isFinal: terminalStatus === "completed" ? 1 : items[index].isFinal,
            status: terminalStatus === "completed" ? "completed" : items[index].status,
          };
          break;
        }
      }
      turn = {
        ...turn,
        requestId: event.requestId || turn.requestId,
        executionId: event.executionId || turn.executionId,
        sequence: Number(event.turnSequence || turn.sequence || 0),
        status: terminalStatus,
        updatedAt: event.createdAt || turn.updatedAt,
        completedAt: event.createdAt || turn.completedAt,
        items,
      };
      this.byTurnId.set(turnId, turn);
      if (this.activeTurnIdValue === turnId) {
        this.activeTurnIdValue = "";
        this.activeExecutionIdValue = "";
      }
      return "applied";
    }

    const blockId = String(event.blockId || "").trim();
    if (!blockId) {
      this.byTurnId.set(turnId, { ...turn, updatedAt: event.createdAt || turn.updatedAt });
      return "applied";
    }

    const items = (turn.items || []).map(cloneItem);
    const index = items.findIndex((item) => String(item.id || "") === blockId);
    const current = index >= 0 ? items[index] : undefined;
    const incomingRevision = Number(event.revision || 0);
    const currentRevision = Number(current?.revision || 0);
    if (current && incomingRevision > 0 && currentRevision > incomingRevision) return "stale";

    let item: AssistantTurnItem = current || this.newItem(turn, event);
    const payload = event.payload || {};
    const delta = String(payload.delta ?? "");
    let content = String(item.content || "");
    let argumentsJson = String(item.argumentsJson || "");
    let resultJson = String(item.resultJson || "");
    if (type.endsWith(".delta") && delta) {
      if (type === "tool.arguments.delta" || String(payload.field || "") === "arguments") argumentsJson += delta;
      else content += delta;
    }
    if (!type.endsWith(".delta") && Object.prototype.hasOwnProperty.call(payload, "content")) content = String(payload.content ?? "");
    if (type === "tool.arguments.completed" || Object.prototype.hasOwnProperty.call(payload, "arguments")) argumentsJson = String(payload.arguments ?? argumentsJson);
    if (Object.prototype.hasOwnProperty.call(payload, "result")) resultJson = typeof payload.result === "string" ? payload.result : JSON.stringify(payload.result);

    item = {
      ...item,
      sequence: Number(event.blockSequence || item.sequence || 0),
      type: itemTypeFromBlock(eventBlockType(event)),
      status: blockStatus(type, event.status, item.status),
      revision: incomingRevision > 0 ? incomingRevision : item.revision,
      callId: event.callId || item.callId,
      toolName: String(payload.toolName || "").trim() || item.toolName,
      content,
      argumentsJson,
      resultJson,
      errorCode: Object.prototype.hasOwnProperty.call(payload, "errorCode") ? String(payload.errorCode || "") : item.errorCode,
      durationMs: typeof payload.durationMs === "number" ? Number(payload.durationMs) : item.durationMs,
      isFinal: type === "text.completed" ? 1 : item.isFinal,
      messageId: event.messageId || item.messageId,
      updatedAt: event.createdAt || item.updatedAt,
    };
    if (index >= 0) items[index] = item;
    else items.push(item);
    items.sort((a, b) => Number(a.sequence || 0) - Number(b.sequence || 0));

    let nextTurnStatus = turn.status;
    if (!isTerminalTurn(nextTurnStatus) && nextTurnStatus !== "waiting_approval" && nextTurnStatus !== "cancelling") {
      const hasRunningTool = items.some((candidate) => candidate.type === "tool_call" && !["completed", "failed", "interrupted"].includes(normalizedTurnStatus(candidate.status)));
      if (hasRunningTool) nextTurnStatus = "waiting_tool";
      else if (nextTurnStatus === "waiting_tool") nextTurnStatus = "running";
    }
    turn = {
      ...turn,
      requestId: event.requestId || turn.requestId,
      executionId: event.executionId || turn.executionId,
      sequence: Number(event.turnSequence || turn.sequence || 0),
      status: nextTurnStatus,
      updatedAt: event.createdAt || turn.updatedAt,
      items,
    };
    this.byTurnId.set(turnId, turn);
    if (!isTerminalTurn(turn.status)) {
      this.activeTurnIdValue = turn.id;
      this.activeExecutionIdValue = String(turn.executionId || "");
    }
    return "applied";
  }

  private newTurn(event: AgentUIEvent): AssistantTurnData {
    return {
      id: String(event.turnId || ""),
      conversationId: event.conversationId,
      requestId: event.requestId || "",
      executionId: event.executionId || "",
      parentTurnId: event.parentTurnId || "",
      parentBlockId: event.parentBlockId || "",
      agentId: event.agentId || "",
      sequence: Number(event.turnSequence || 0),
      status: initialTurnStatus(event),
      createdAt: event.createdAt || "",
      updatedAt: event.createdAt || "",
      items: [],
    };
  }

  private newItem(turn: AssistantTurnData, event: AgentUIEvent): AssistantTurnItem {
    return {
      id: String(event.blockId || ""),
      turnId: turn.id,
      conversationId: turn.conversationId,
      sequence: Number(event.blockSequence || 0),
      type: itemTypeFromBlock(eventBlockType(event)),
      status: String(event.status || "running"),
      revision: Number(event.revision || 0),
      callId: event.callId || undefined,
      toolName: String(event.payload?.toolName || "") || undefined,
      content: "",
      argumentsJson: "",
      resultJson: "",
      createdAt: event.createdAt || "",
      updatedAt: event.createdAt || "",
    };
  }

  private turnFromRuntime(raw: RuntimeTurnSnapshot, existing?: AssistantTurnData): AssistantTurnData {
    const turn: AssistantTurnData = {
      ...(existing ? cloneTurn(existing) : {
        id: raw.turnId,
        conversationId: "",
        sequence: Number(raw.turnSequence || 0),
        status: "running",
        items: [],
      }),
      id: raw.turnId,
      requestId: raw.requestId || existing?.requestId || "",
      executionId: raw.executionId || existing?.executionId || "",
      parentTurnId: raw.parentTurnId || existing?.parentTurnId || "",
      parentBlockId: raw.parentBlockId || existing?.parentBlockId || "",
      agentId: raw.agentId || existing?.agentId || "",
      sequence: Number(raw.turnSequence || existing?.sequence || 0),
      status: normalizedTurnStatus(raw.status || existing?.status || "running"),
      createdAt: raw.startedAt || existing?.createdAt || "",
      updatedAt: raw.lastModifiedAt || raw.startedAt || existing?.updatedAt || "",
      items: existing?.items.map(cloneItem) || [],
    };
    for (const block of raw.blocks || []) {
      const payload = block.payload || {};
      const currentIndex = turn.items.findIndex((item) => item.id === block.blockId);
      const current = currentIndex >= 0 ? turn.items[currentIndex] : undefined;
      const item: AssistantTurnItem = {
        ...(current || {} as AssistantTurnItem),
        id: block.blockId,
        turnId: turn.id,
        conversationId: current?.conversationId || turn.conversationId,
        sequence: Number(block.blockSequence || current?.sequence || 0),
        type: itemTypeFromBlock(block.type),
        status: block.status || current?.status || "running",
        revision: Number(block.revision || current?.revision || 0),
        callId: block.callId || current?.callId,
        toolName: String(payload.toolName || "") || current?.toolName,
        content: block.content || String(payload.content || current?.content || ""),
        argumentsJson: payload.arguments === undefined ? current?.argumentsJson || "" : String(payload.arguments),
        resultJson: payload.result === undefined ? current?.resultJson || "" : (typeof payload.result === "string" ? payload.result : JSON.stringify(payload.result)),
        errorCode: payload.errorCode === undefined ? current?.errorCode : String(payload.errorCode || ""),
        durationMs: typeof payload.durationMs === "number" ? Number(payload.durationMs) : current?.durationMs,
        createdAt: current?.createdAt || raw.startedAt || "",
        updatedAt: raw.lastModifiedAt || raw.startedAt || current?.updatedAt || "",
      };
      if (currentIndex >= 0) turn.items[currentIndex] = item;
      else turn.items.push(item);
    }
    turn.items.sort((a, b) => Number(a.sequence || 0) - Number(b.sequence || 0));
    return turn;
  }

  private remember(eventId: string | undefined): void {
    const id = String(eventId || "").trim();
    if (!id || this.seenEventIds.has(id)) return;
    this.seenEventIds.add(id);
    this.seenEventOrder.push(id);
    if (this.seenEventOrder.length > 8192) {
      for (const removed of this.seenEventOrder.splice(0, 2048)) this.seenEventIds.delete(removed);
    }
  }
}
