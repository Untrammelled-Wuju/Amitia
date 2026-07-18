function numericValue(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) return value
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

function field(message: any, camel: string, snake: string) {
  return message?.[camel] ?? message?.[snake]
}

export function compareChatMessages(a: any, b: any): number {
  const aGroup = String(field(a, "responseGroupId", "response_group_id") || "")
  const bGroup = String(field(b, "responseGroupId", "response_group_id") || "")
  const aDelivery = numericValue(field(a, "deliverySequence", "delivery_sequence"))
  const bDelivery = numericValue(field(b, "deliverySequence", "delivery_sequence"))
  if (aGroup && aGroup === bGroup && aDelivery !== null && bDelivery !== null && aDelivery !== bDelivery) {
    return aDelivery - bDelivery
  }

  const aSequence = numericValue(a?.sequence)
  const bSequence = numericValue(b?.sequence)
  if (aSequence !== null && bSequence !== null && aSequence !== bSequence) return aSequence - bSequence

  const aTime = a?.createdAt ? new Date(a.createdAt).getTime() : 0
  const bTime = b?.createdAt ? new Date(b.createdAt).getTime() : 0
  if (Number.isFinite(aTime) && Number.isFinite(bTime) && aTime !== bTime) return aTime - bTime
  return 0
}

export function normalizeRealtimeMessage(payload: any): any {
  const message = { ...payload }
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
  ] as const
  for (const [target, fallback] of mappings) {
    const value = payload?.[target] ?? payload?.[fallback]
    if (value !== undefined && value !== null) message[target] = value
  }
  return message
}

export function mergeChatMessage(messages: any[], incoming: any): boolean {
  const id = String(incoming?.id || "")
  if (!id) return false
  const index = messages.findIndex(message => String(message?.id || "") === id)
  if (index < 0) return false
  messages[index] = { ...messages[index], ...incoming }
  return true
}
