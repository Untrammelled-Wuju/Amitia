// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue"
import { useApi } from "./useApi"

// ============================================================
// Types
// ============================================================

export interface SleepSetting {
  id: number
  sleepReplyEnabled: boolean
  sleepReplyMode: "NO_REPLY" | "SYSTEM_NOTICE" | "SHORT_SLEEPY_REPLY"
}

export const SLEEP_REPLY_MODE_OPTIONS = [
  { label: "不回复", value: "NO_REPLY" },
  { label: "显示系统提示", value: "SYSTEM_NOTICE" },
  { label: "简短困倦回复", value: "SHORT_SLEEPY_REPLY" },
]

export const SLEEP_REPLY_MODE_LABELS: Record<string, string> = {
  NO_REPLY: "不回复",
  SYSTEM_NOTICE: "显示系统提示",
  SHORT_SLEEPY_REPLY: "简短困倦回复",
}

// ============================================================
// Composable
// ============================================================

export function useSleepSetting() {
  const { get, put } = useApi()
  const loading = ref(false)

  /** 获取睡觉回复设置 */
  async function getSleepSetting(characterId?: string): Promise<SleepSetting> {
    loading.value = true
    try {
      const params: any = {}
      if (characterId) params.characterId = characterId
      const data = await get<SleepSetting>("/api/companion/sleep-setting", params)
      return data
    } finally {
      loading.value = false
    }
  }

  /** 更新睡觉回复设置 */
  async function updateSleepSetting(input: {
    sleepReplyEnabled?: boolean
    sleepReplyMode?: string
  }, characterId?: string): Promise<SleepSetting> {
    loading.value = true
    try {
      const url = characterId ? "/api/companion/sleep-setting?characterId=" + encodeURIComponent(characterId) : "/api/companion/sleep-setting"
      const data = await put<SleepSetting>(url, input)
      return data
    } finally {
      loading.value = false
    }
  }

  return {
    loading,
    getSleepSetting,
    updateSleepSetting,
  }
}