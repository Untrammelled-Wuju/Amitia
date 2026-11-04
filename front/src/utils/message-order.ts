function numericValue(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return null;
}

export function parseMessageTime(value: unknown): number {
  const raw = numericValue(value);
  if (raw !== null) return raw;
  const text = String(value ?? "").trim();
  if (!text) return 0;
  const normalized = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}(:\d{2})?(\.\d+)?$/.test(text)
    ? text.replace(" ", "T")
    : text;
  const parsed = Date.parse(normalized);
  return Number.isFinite(parsed) ? parsed : 0;
}

export function compareChatMessages(a: any, b: any): number {
  if (a?.groupRoundId && a.groupRoundId === b?.groupRoundId && a?.deliveryId && b?.deliveryId) {
    const aIndex = numericValue(a.deliveryIndex);
    const bIndex = numericValue(b.deliveryIndex);
    if (aIndex !== null && bIndex !== null && aIndex !== bIndex) return aIndex - bIndex;
  }
  const aId = String(a?.id || "");
  const bId = String(b?.id || "");
  const aAnchorId = String(a?.anchorMessageId || "");
  const bAnchorId = String(b?.anchorMessageId || "");
  if (aAnchorId && aAnchorId === bId) return 1;
  if (bAnchorId && bAnchorId === aId) return -1;

  const aSequence = numericValue(a?.sequence);
  const bSequence = numericValue(b?.sequence);
  if (aSequence !== null && bSequence !== null && aSequence !== bSequence)
    return aSequence - bSequence;

  const aTime =
    numericValue(a?.sortTimestamp) ?? parseMessageTime(a?.createdAt);
  const bTime =
    numericValue(b?.sortTimestamp) ?? parseMessageTime(b?.createdAt);
  if (aTime !== 0 || bTime !== 0) {
    if (aTime !== bTime) return aTime - bTime;
  }
  if (aSequence !== null && bSequence === null) return -1;
  if (aSequence === null && bSequence !== null) return 1;
  return 0;
}

export function normalizeRealtimeMessage(payload: any): any {
  const message = { ...payload, id: payload?.messageId };
  if (!message.clientMessageId && message.role === "user" && message.requestId) {
    message.clientMessageId = message.requestId;
  }
  return message;
}

export function getClientMessageId(message: any): string {
  const explicit = String(message?.clientMessageId || "").trim();
  if (explicit) return explicit;
  if (message?.role !== "user") return "";
  return String(message?.requestId || "").trim();
}

export function getMessageRequestId(message: any): string {
  return String(message?.requestId || "").trim();
}

export function getMessageSenderId(message: any): string {
  if (message?.role !== "assistant") return "";
  return String(message?.characterId || message?.senderId || "").trim();
}

export function getMessageCharacterId(message: any): string {
  return String(message?.characterId || message?.senderId || "").trim();
}

export function shouldShowRoleSwitch(previous: any, current: any): boolean {
  const previousCharacterId = getMessageCharacterId(previous);
  const currentCharacterId = getMessageCharacterId(current);
  return (
    !!previousCharacterId &&
    !!currentCharacterId &&
    previousCharacterId !== currentCharacterId
  );
}

export function shouldShowAssistantIdentity(
  current: any,
  previous: any,
): boolean {
  if (current?.role !== "assistant") return true;
  if (previous?.role !== "assistant") return true;
  const currentSenderId = getMessageSenderId(current);
  const previousSenderId = getMessageSenderId(previous);
  if (currentSenderId && previousSenderId) {
    return currentSenderId !== previousSenderId;
  }
  return true;
}

export function getMessageUIKey(message: any, index = 0): string {
  const uiKey = String(message?.uiKey || "").trim();
  if (uiKey) return uiKey;
  const clientMessageId = getClientMessageId(message);
  if (clientMessageId) return `client:${clientMessageId}`;
  const id = String(message?.id || "").trim();
  if (id) return `server:${id}`;
  return `fallback:${String(message?.role || "unknown")}:${String(message?.createdAt || "")}:${index}`;
}

function messageCollectionKey(message: any, index: number): string {
  const id = String(message?.id || "").trim();
  if (id) return `id:${id}`;
  const clientMessageId = String(message?.clientMessageId || "").trim();
  if (clientMessageId) return `client:${clientMessageId}`;
  const requestId = String(message?.requestId || "").trim();
  if (requestId) return `request:${requestId}:${String(message?.role || "")}:${index}`;
  return `fallback:${String(message?.role || "")}:${String(
    message?.createdAt || "",
  )}:${index}`;
}

function definedFields(message: any): Record<string, any> {
  return Object.fromEntries(
    Object.entries(message || {}).filter(([, value]) => value !== undefined),
  );
}

export function mergeMessageCollections(
  current: any[],
  incoming: any[],
): any[] {
  const byKey = new Map<string, any>();
  const append = (message: any, index: number) => {
    if (!message || typeof message !== "object") return;
    const key = messageCollectionKey(message, index);
    const existing = byKey.get(key);
    byKey.set(
      key,
      existing ? { ...existing, ...definedFields(message) } : { ...message },
    );
  };
  current.forEach(append);
  incoming.forEach(append);
  return [...byKey.values()].sort(compareChatMessages);
}
