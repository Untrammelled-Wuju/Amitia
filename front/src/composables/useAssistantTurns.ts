import { type Ref, ref } from "vue";
import { useApi } from "./useApi";
import type { AssistantTurnData } from "@/conversation/rendering/types";

export function useAssistantTurns(
  conversationId: Ref<string>,
  messages: Ref<any[]>,
) {
  const { get } = useApi();
  const turns = ref<AssistantTurnData[]>([]);

  function clearTurnProjection() {
    for (const message of messages.value) {
      if (!message || typeof message !== "object") continue;
      delete message.assistantTurn;
      delete message.assistantTurnSuppressed;
    }
  }

  function applyTurns() {
    clearTurnProjection();
    const byRequest = new Map<string, AssistantTurnData>();
    const byGroup = new Map<string, AssistantTurnData>();
    for (const turn of turns.value) {
      if (turn.requestId) byRequest.set(String(turn.requestId), turn);
      if (turn.responseGroupId) byGroup.set(String(turn.responseGroupId), turn);
    }
    for (const turn of turns.value) {
      const matched = messages.value.filter((message) => {
        if (!message || message.role !== "assistant") return false;
        const requestId = String(message.requestId || "");
        const groupId = String(message.responseGroupId || message.response_group_id || "");
        return (
          (!!turn.requestId && requestId === turn.requestId) ||
          (!!turn.responseGroupId && groupId === turn.responseGroupId) ||
          (!!turn.requestId && groupId === turn.requestId) ||
          (!!turn.id && String(message.turnId || "") === turn.id)
        );
      });
      if (!matched.length) continue;
      matched[0].assistantTurn = turn;
      for (const duplicate of matched.slice(1)) {
        duplicate.assistantTurnSuppressed = true;
      }
    }
    const claimedTurnIds = new Set(
      messages.value
        .filter((message) => message?.assistantTurn)
        .map((message) => String(message.assistantTurn.id || "")),
    );
    for (const turn of turns.value) {
      if (!turn?.id || claimedTurnIds.has(String(turn.id))) continue;
      const requestId = String(turn.requestId || "");
      const provisionalId = `turn:${turn.id}`;
      const index = messages.value.findIndex(
        (message) =>
          String(message?.id || "") === provisionalId ||
          (!!requestId &&
            message?.role === "assistant" &&
            message?.generationPending === true &&
            String(message?.requestId || "") === requestId),
      );
      const provisional = {
        id: provisionalId,
        role: "assistant",
        content: "",
        requestId,
        responseGroupId: String(turn.responseGroupId || requestId),
        turnId: String(turn.id),
        createdAt: String(turn.createdAt || new Date().toISOString()),
        status: "streaming",
        generationPending: true,
        assistantTurn: turn,
      };
      if (index >= 0) messages.value[index] = { ...messages.value[index], ...provisional };
      else messages.value.push(provisional);
    }
  }

  function applyRealtimeStreamEvent(raw: any) {
    const rawConversationId = String(raw?.conversationId || "");
    if (rawConversationId !== String(conversationId.value || "")) return;
    const metadata =
      raw?.data && typeof raw.data === "object" ? raw.data : {};
    const turnId = String(metadata.turnId || "");
    if (!turnId) return;
    let turn = turns.value.find((item) => String(item.id) === turnId);
    if (!turn) {
      turn = {
        id: turnId,
        conversationId: rawConversationId,
        sequence: 0,
        status: "running",
        requestId: String(metadata.requestId || ""),
        responseGroupId: "",
        createdAt: String(raw?.createdAt || new Date().toISOString()),
        items: [],
      } as AssistantTurnData;
      turns.value.push(turn);
    }
    if (metadata.status) turn.status = String(metadata.status);
    if (metadata.eventType === "turn.updated") {
      turn.status = String(metadata.status || turn.status || "completed");
    }
    const incomingItem = metadata.item;
    if (incomingItem && typeof incomingItem === "object") {
      const item = incomingItem as any;
      const itemId = String(item.id || "");
      const callId = String(item.callId || "");
      const itemType = String(item.type || item.itemType || "");
      const index = turn.items.findIndex(
        (candidate: any) =>
          (!!itemId && String(candidate.id || "") === itemId) ||
          (!!callId &&
            !!itemType &&
            String(candidate.callId || "") === callId &&
            String(candidate.type || "") === itemType),
      );
      if (index >= 0) turn.items[index] = { ...turn.items[index], ...item };
      else turn.items.push(item);
      turn.items.sort(
        (left: any, right: any) =>
          Number(left.sequence || 0) - Number(right.sequence || 0),
      );
    }
    applyTurns();
  }

  async function loadTurns() {
    const id = String(conversationId.value || "").trim();
    if (!id) {
      turns.value = [];
      applyTurns();
      return;
    }
    try {
      const response = await get<any>(
        `/api/web-chat/conversations/${encodeURIComponent(id)}/turns?limit=500`,
      );
      const items = Array.isArray(response?.items)
        ? response.items
        : Array.isArray(response)
          ? response
          : [];
      turns.value = items.map((turn: any) => ({
        ...turn,
        items: [...(turn.items || [])].sort(
          (left: any, right: any) => Number(left.sequence || 0) - Number(right.sequence || 0),
        ),
      }));
      applyTurns();
    } catch {
      turns.value = [];
      applyTurns();
    }
  }

  return {
    turns,
    applyTurns,
    loadTurns,
    applyRealtimeStreamEvent,
  };
}
