function numericValue(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return null;
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
    numericValue(a?.sortTimestamp) ??
    (a?.createdAt ? new Date(a.createdAt).getTime() : 0);
  const bTime =
    numericValue(b?.sortTimestamp) ??
    (b?.createdAt ? new Date(b.createdAt).getTime() : 0);
  if (Number.isFinite(aTime) && Number.isFinite(bTime) && aTime !== bTime)
    return aTime - bTime;
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
      ? new Date(messages[anchorIndex].createdAt).getTime()
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
    ["contentType", "content_type"],
    ["emoteId", "emote_id"],
    ["altText", "alt_text"],
    ["isAnimated", "is_animated"],
    ["width", "media_width"],
    ["height", "media_height"],
    ["imageUrl", "image_url"],
    ["originalAssetReference", "original_asset_reference"],
    ["fallbackAssetReference", "fallback_asset_reference"],
    ["responseGroupId", "response_group_id"],
    ["deliverySequence", "delivery_sequence"],
    ["emoteDecisionStatus", "emote_decision_status"],
    ["createdAt", "created_at"],
  ] as const;
  for (const [target, fallback] of mappings) {
    const value = payload?.[target] ?? payload?.[fallback];
    if (value !== undefined && value !== null) message[target] = value;
  }
  return message;
}

export function mergeChatMessage(messages: any[], incoming: any): boolean {
  const id = String(incoming?.id || "");
  if (!id) return false;
  const index = messages.findIndex(
    (message) => String(message?.id || "") === id,
  );
  if (index < 0) return false;
  messages[index] = { ...messages[index], ...incoming };
  return true;
}
