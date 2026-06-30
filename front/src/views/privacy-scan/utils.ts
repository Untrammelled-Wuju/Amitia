// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
export function sourceTableLabel(table: string): string {
  const map: Record<string, string> = {
    messages: "聊天消息",
    memories: "记忆",
    import_items: "导入内容",
    import_batches: "导入批次",
    operation_logs: "日志",
  }
  return map[table] || table
}
