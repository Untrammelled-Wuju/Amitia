const CONVERSATION_REFERENCE_PREFIX = "meshconv1:";

export function conversationReference(origin) {
  const ownerId = typeof origin?.ownerId === "string" ? origin.ownerId : "";
  const id = typeof origin?.id === "string" ? origin.id : "";
  if (!ownerId || !id) throw new Error("会话来源无效");
  return `${CONVERSATION_REFERENCE_PREFIX}${encodeURIComponent(ownerId)}:${encodeURIComponent(id)}`;
}

function decodeSegment(reference, segment) {
  try {
    return decodeURIComponent(segment);
  } catch (error) {
    throw new Error(`会话来源无效：${reference}`, { cause: error });
  }
}

export function parseConversationReference(value) {
  if (typeof value !== "string" || !value.startsWith(CONVERSATION_REFERENCE_PREFIX)) return undefined;
  const parts = value.split(":");
  if (parts.length !== 3) throw new Error(`会话来源无效：${value}`);
  const origin = { ownerId: decodeSegment(value, parts[1]), id: decodeSegment(value, parts[2]) };
  if (conversationReference(origin) !== value) throw new Error(`会话来源无效：${value}`);
  return origin;
}

export function conversationIdOf(value) {
  const origin = parseConversationReference(value);
  return origin ? origin.id : value;
}

export function normalizeConversationRequest(input) {
  const source = input && typeof input === "object" ? input : {};
  const conversationId = conversationIdOf(source.conversationId);
  const sessionId = conversationIdOf(source.sessionId);
  if (conversationId === source.conversationId && sessionId === source.sessionId) return source;
  const next = { ...source };
  if (conversationId !== source.conversationId) next.conversationId = conversationId;
  if (sessionId !== source.sessionId) next.sessionId = sessionId;
  return next;
}
