export interface OwnedConversationExport {
  filename: string;
  mimeType: string;
  content: string;
}

export function createOwnedConversationExport(conversationId: string, rows: any[], format: string): OwnedConversationExport {
  if (!conversationId || !["json", "markdown"].includes(format)) throw new Error("会话或导出格式无效");
  const messages = rows.filter((row) => ["user", "assistant"].includes(row.role)).map((row) => ({
    id: String(row.id || ""),
    ownerId: String(row.ownerId || ""),
    characterId: String(row.characterId || ""),
    role: row.role,
    content: String(row.content || ""),
    createdAt: String(row.createdAt || ""),
    ...(row.imageUrl ? { imageUrl: String(row.imageUrl) } : {}),
    ...(row.audioUrl ? { audioUrl: String(row.audioUrl) } : {}),
  }));
  if (messages.some((row) => !row.id || !row.ownerId)) throw new Error("导出消息缺少数据来源，请重新加载");
  const content = format === "json"
    ? JSON.stringify({ formatVersion: 1, conversationId, messages }, null, 2)
    : messages.map((row) => `## ${row.role === "user" ? "用户" : "AI"} · ${row.createdAt}\n\n${row.content}${row.imageUrl ? `\n\n![图片](${row.imageUrl})` : ""}${row.audioUrl ? `\n\n[语音](${row.audioUrl})` : ""}`).join("\n\n");
  if (messages.length > 4096 || new TextEncoder().encode(content).byteLength > 32 * 1024 * 1024) throw new Error("导出内容超过单次上限，请分段导出");
  return { filename: `amitia-conversation.${format === "json" ? "json" : "md"}`, mimeType: format === "json" ? "application/json;charset=utf-8" : "text/markdown;charset=utf-8", content };
}
