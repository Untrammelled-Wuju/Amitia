// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
export function formatTime(iso: string): string {
  if (!iso) return "-"
  try {
    return new Date(iso).toLocaleString("zh-CN")
  } catch {
    return iso
  }
}
