// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
export const CHANNELS = [
  { label: "Web", value: "web" },
  { label: "微信", value: "wechat" },
  { label: "QQ", value: "qq" },
  { label: "导入", value: "import" },
  { label: "测试", value: "test" },
]

export function channelLabel(ch: string): string {
  return CHANNELS.find(x => x.value === ch)?.label || ch
}

export function fmtShort(d: string): string {
  if (!d) return ""
  try { return new Date(d).toLocaleDateString("zh-CN") } catch { return d }
}

export function fmtTime(d: string): string {
  if (!d) return ""
  try { return new Date(d).toLocaleString("zh-CN") } catch { return d }
}

export function moodEmoji(label: string): string {
  const map: Record<string, string> = {
    tired: '😨', happy: '😈', stressed: '😶', sad: '😻', angry: '😻', confused: '😳'
  }
  return map[label] || ''
}
