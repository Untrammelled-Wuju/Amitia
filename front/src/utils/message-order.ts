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

function field(message: any, camel: string, snake: string) {
  return message?.[camel] ?? message?.[snake];
}

function isTransientModelError(message: any): boolean {
  return ["vision_error", "text_error", "voice_error", "vector_error"].includes(
    message?.msgType,
  );
}

export function compareChatMessages(a: any, b: any): number {
  const aGroup = String(field(a, "responseGroupId", "response_group_id") || "");
  const bGroup = String(field(b, "responseGroupId", "response_group_id") || "");
  const aDelivery = numericValue(
    field(a, "deliverySequence", "delivery_sequence"),
  );
  const bDelivery = numericValue(
    field(b, "deliverySequence", "delivery_sequence"),
  );
  if (
    aGroup &&
    aGroup === bGroup &&
    aDelivery !== null &&
    bDelivery !== null &&
    aDelivery !== bDelivery
  ) {
    return aDelivery - bDelivery;
  }

  const aId = String(a?.id || "");
  const bId = String(b?.id || "");
  const aAnchorId = String(a?.anchorMessageId || "");
  const bAnchorId = String(b?.anchorMessageId || "");
  if (aAnchorId && aAnchorId === bId) return 1;
  if (bAnchorId && bAnchorId === aId) return -1;

  const aRequestId = String(a?.requestId || "");
  const bRequestId = String(b?.requestId || "");
  if (aRequestId && aRequestId === bRequestId) {
    const aError = isTransientModelError(a);
    const bError = isTransientModelError(b);
    if (a?.role === "user" && bError) return -1;
    if (b?.role === "user" && aError) return 1;
    if (aError && bError) {
      const aOrder = numericValue(a?.transientOrder);
      const bOrder = numericValue(b?.transientOrder);
      if (aOrder !== null && bOrder !== null && aOrder !== bOrder)
        return aOrder - bOrder;
    }
  }

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

export function insertTransientModelError(messages: any[], incoming: any): void {
  const requestId = String(incoming?.requestId || "");
  const anchorMessageId = String(incoming?.anchorMessageId || "");
  let anchorIndex = -1;
  if (anchorMessageId) {
    anchorIndex = messages.findIndex(
      (message) => String(message?.id || "") === anchorMessageId,
    );
  }
  if (anchorIndex < 0 && requestId) {
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const message = messages[index];
      if (
        message?.role === "user" &&
        String(message?.requestId || "") === requestId
      ) {
        anchorIndex = index;
        break;
      }
    }
  }

  const relatedErrors = messages.filter((message) => {
    if (!isTransientModelError(message)) return false;
    if (requestId && String(message?.requestId || "") === requestId) return true;
    return (
      !!anchorMessageId &&
      String(message?.anchorMessageId || "") === anchorMessageId
    );
  });
  const transientOrder =
    relatedErrors.reduce(
      (maximum, message) =>
        Math.max(maximum, numericValue(message?.transientOrder) || 0),
      0,
    ) + 1;
  const anchorTime =
    anchorIndex >= 0 && messages[anchorIndex]?.createdAt
      ? parseMessageTime(messages[anchorIndex].createdAt)
      : 0;
  const previousTime = relatedErrors.reduce(
    (maximum, message) =>
      Math.max(maximum, numericValue(message?.sortTimestamp) || 0),
    0,
  );
  const message = {
    ...incoming,
    transientOrder,
    sortTimestamp: Math.max(
      Date.now(),
      Number.isFinite(anchorTime) ? anchorTime + transientOrder : 0,
      previousTime + 1,
    ),
  };
  let insertIndex = anchorIndex >= 0 ? anchorIndex + 1 : messages.length;
  while (insertIndex < messages.length) {
    const candidate = messages[insertIndex];
    const sameRequest =
      requestId && String(candidate?.requestId || "") === requestId;
    const sameAnchor =
      anchorMessageId &&
      String(candidate?.anchorMessageId || "") === anchorMessageId;
    if (!isTransientModelError(candidate) || (!sameRequest && !sameAnchor)) break;
    insertIndex += 1;
  }
  messages.splice(insertIndex, 0, message);
}

export function normalizeRealtimeMessage(payload: any): any {
  const message = { ...payload };
  const mappings = [
    ["id", "messageId"],
    ["conversationId", "conversation_id"],
    ["msgType", "msg_type"],
    ["extensionType", "extension_type"],
    ["contentType", "content_type"],
    ["altText", "alt_text"],
    ["isAnimated", "is_animated"],
    ["width", "media_width"],
    ["height", "media_height"],
    ["imageUrl", "image_url"],
    ["originalAssetReference", "original_asset_reference"],
    ["fallbackAssetReference", "fallback_asset_reference"],
    ["responseGroupId", "response_group_id"],
    ["deliverySequence", "delivery_sequence"],
    ["createdAt", "created_at"],
    ["requestId", "request_id"],
    ["clientMessageId", "client_message_id"],
  ] as const;
  for (const [target, fallback] of mappings) {
    const value = payload?.[target] ?? payload?.[fallback];
    if (value !== undefined && value !== null) message[target] = value;
  }
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
  return String(message?.requestId || message?.request_id || "").trim();
}

export function getMessageSenderId(message: any): string {
  if (message?.role !== "assistant") return "";
  return String(
    field(message, "characterId", "character_id") ||
      message?.senderId ||
      message?.sender_id ||
      "",
  ).trim();
}

export function getMessageResponseGroupId(message: any): string {
  if (message?.role !== "assistant") return "";
  return String(
    field(message, "responseGroupId", "response_group_id") ||
      getMessageRequestId(message),
  ).trim();
}

export function getMessageCharacterId(message: any): string {
  return String(
    field(message, "characterId", "character_id") ||
      message?.senderId ||
      message?.sender_id ||
      "",
  ).trim();
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
  const currentGroupId = getMessageResponseGroupId(current);
  const previousGroupId = getMessageResponseGroupId(previous);
  if (currentGroupId && previousGroupId) {
    return currentGroupId !== previousGroupId;
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
  return `fallback:${String(message?.role || "unknown")}:${String(message?.createdAt || message?.timestamp || "")}:${index}`;
}

export function hasAssistantReplyAfterLatestUser(messages: any[]): boolean {
  let latestUserIndex = -1;
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index]?.role === "user") {
      latestUserIndex = index;
      break;
    }
  }
  if (latestUserIndex < 0) return false;
  for (let index = latestUserIndex + 1; index < messages.length; index += 1) {
    const message = messages[index];
    if (message?.role !== "assistant") continue;
    if (
      String(message?.content || "").trim() !== "" ||
      ["streaming", "sending"].includes(String(message?.status || ""))
    ) {
      return true;
    }
  }
  return false;
}

export function mergeChatMessage(messages: any[], incoming: any): boolean {
  const id = String(incoming?.id || "");
  const clientMessageId = getClientMessageId(incoming);
  const requestId = getMessageRequestId(incoming);
  const uiKey = String(incoming?.uiKey || "").trim();
  if (!id && !clientMessageId && !requestId && !uiKey) return false;
  const index = messages.findIndex(
    (message) =>
      (!!id && String(message?.id || "") === id) ||
      (!!clientMessageId && getClientMessageId(message) === clientMessageId) ||
      (!!uiKey && String(message?.uiKey || "") === uiKey) ||
      (!!requestId &&
        incoming?.role === "assistant" &&
        message?.role === "assistant" &&
        message?.generationPending === true &&
        getMessageRequestId(message) === requestId),
  );
  if (index < 0) return false;
  const current = messages[index];
  const incomingStatus =
    incoming?.status ||
    (incoming?.role === "assistant" ? "sent" : undefined);
  const definedIncoming = Object.fromEntries(
    Object.entries(incoming).filter(([, value]) => value !== undefined),
  );
  messages[index] = {
    ...current,
    ...definedIncoming,
    status: incomingStatus || current?.status,
    clientMessageId:
      getClientMessageId(incoming) || getClientMessageId(current) || undefined,
    uiKey: current?.uiKey || incoming?.uiKey,
    animateIn: current?.animateIn ?? incoming?.animateIn,
    generationPending: false,
  };
  return true;
}

export function upsertStreamingAssistantMessage(
  messages: any[],
  incoming: any,
  preferredId?: string | null,
): number {
  const id = String(incoming?.id || preferredId || "").trim();
  const requestId = getMessageRequestId(incoming);
  if (!id && !requestId) return -1;
  let index = -1;
  if (preferredId) {
    index = messages.findIndex(
      (message) => String(message?.id || "") === String(preferredId),
    );
  }
  if (index < 0) {
    index = messages.findIndex(
      (message) =>
        (!!id && String(message?.id || "") === id) ||
        (!!requestId &&
          incoming?.role === "assistant" &&
          message?.role === "assistant" &&
          message?.generationPending === true &&
          getMessageRequestId(message) === requestId),
    );
  }
  if (index >= 0) {
    const current = messages[index];
    const definedIncoming = Object.fromEntries(
      Object.entries(incoming).filter(([, value]) => value !== undefined),
    );
    messages[index] = {
      ...current,
      ...definedIncoming,
      id: id || current?.id,
      content: incoming?.content || current?.content || "",
      uiKey: current?.uiKey || incoming?.uiKey,
      animateIn: current?.animateIn ?? incoming?.animateIn,
      generationPending: false,
    };
    return index;
  }
  if (!id) return -1;
  messages.push({ ...incoming, id });
  return messages.length - 1;
}

export function mergeServerMessages(messages: any[], serverItems: any[]): any[] {
  const normalizedServer = serverItems.map(normalizeRealtimeMessage);
  const serverById = new Map<string, any>();
  const serverClientIds = new Set<string>();
  const currentById = new Map<string, any>();
  const currentByClientMessageId = new Map<string, any>();
  const currentAssistantPlaceholdersByRequestId = new Map<string, any>();
  const serverAssistantRequestIds = new Set<string>();
  const serverConversationId = String(
    normalizedServer.find((message) => message?.conversationId)?.conversationId || "",
  );

  for (const message of normalizedServer) {
    const id = String(message?.id || "");
    const clientMessageId = getClientMessageId(message);
    const requestId = getMessageRequestId(message);
    if (id) serverById.set(id, message);
    if (clientMessageId) serverClientIds.add(clientMessageId);
    if (message?.role === "assistant" && requestId) {
      serverAssistantRequestIds.add(requestId);
    }
  }
  for (const current of messages) {
    const id = String(current?.id || "");
    const clientMessageId = getClientMessageId(current);
    const requestId = getMessageRequestId(current);
    if (id) currentById.set(id, current);
    if (clientMessageId) currentByClientMessageId.set(clientMessageId, current);
    if (
      current?.role === "assistant" &&
      current?.generationPending === true &&
      requestId
    ) {
      currentAssistantPlaceholdersByRequestId.set(requestId, current);
    }
  }

  const merged = normalizedServer.map((message) => {
    const requestId = getMessageRequestId(message);
    let existing =
      currentById.get(String(message?.id || "")) ||
      currentByClientMessageId.get(getClientMessageId(message));
    if (!existing && message?.role === "assistant" && requestId) {
      existing = currentAssistantPlaceholdersByRequestId.get(requestId);
      if (existing) {
        currentAssistantPlaceholdersByRequestId.delete(requestId);
      }
    }
    const next = {
      ...existing,
      ...message,
      clientMessageId:
        getClientMessageId(message) ||
        getClientMessageId(existing) ||
        undefined,
      uiKey: existing?.uiKey || getMessageUIKey(message),
      animateIn: existing?.animateIn ?? false,
      generationPending: false,
    };
    if (next.imageUrl && next.content === "[图片]") {
      return { ...next, content: "" };
    }
    return next;
  });

  for (const local of messages) {
    const id = String(local?.id || "");
    const clientMessageId = getClientMessageId(local);
    const requestId = getMessageRequestId(local);
    if (serverById.has(id)) continue;
    if (clientMessageId && serverClientIds.has(clientMessageId)) continue;
    if (
      local?.role === "assistant" &&
      local?.generationPending === true &&
      requestId &&
      serverAssistantRequestIds.has(requestId)
    ) {
      continue;
    }
    const localConversationId = String(local?.conversationId || "");
    if (
      serverConversationId &&
      localConversationId &&
      localConversationId !== serverConversationId
    ) {
      continue;
    }
    const status = String(local?.status || "").toLowerCase();
    const transientStatus = [
      "streaming",
      "sending",
      "queued",
      "interrupted",
      "failed",
      "error",
      "timeout",
    ].includes(status);
    const createdAt = parseMessageTime(local?.createdAt || local?.timestamp);
    const recentlyCreated =
      createdAt > 0 && Date.now() - createdAt <= 120000;
    const isLocal =
      id.startsWith("user-") ||
      id.startsWith("failed-") ||
      transientStatus ||
      recentlyCreated;
    if (!isLocal) continue;
    merged.push(local);
  }

  return merged.sort(compareChatMessages);
}
