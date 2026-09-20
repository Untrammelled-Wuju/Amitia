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
      matched[0].reasoningContent = "";
      matched[0].reasoningDurationMs = 0;
      for (const duplicate of matched.slice(1)) {
        duplicate.assistantTurnSuppressed = true;
      }
    }
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
  };
}
