export interface OwnedConversationOrigin { ownerId: string; id: string }

export function conversationReference(origin: OwnedConversationOrigin): string {
  if (!origin.ownerId || !origin.id || origin.ownerId.length > 512 || origin.id.length > 512 || origin.ownerId.includes("\0") || origin.id.includes("\0")) throw new Error("会话来源无效");
  return `meshconv1:${encodeURIComponent(origin.ownerId)}:${encodeURIComponent(origin.id)}`;
}

export function parseConversationReference(reference: string): OwnedConversationOrigin | undefined {
  if (!reference.startsWith("meshconv1:")) return undefined;
  const parts = reference.split(":");
  if (parts.length !== 3) throw new Error("会话来源无效");
  const origin = { ownerId: decodeURIComponent(parts[1]), id: decodeURIComponent(parts[2]) };
  if (conversationReference(origin) !== reference) throw new Error("会话来源无效");
  return origin;
}

export function ownedConversationRow(row: any, ownerId: string): any {
  const origin = row.conversationOrigin || { ownerId, id: row.id };
  return { ...row, id: conversationReference(origin), resourceId: row.id, ownerId, conversationOrigin: origin, pinnedAt: typeof row.pinned === "boolean" ? row.pinned ? row.updatedAt || "pinned" : "" : row.pinnedAt, archivedAt: typeof row.archived === "boolean" ? row.archived ? row.updatedAt || "archived" : "" : row.archivedAt };
}
