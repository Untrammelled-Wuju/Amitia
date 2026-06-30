// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue"
import { useApi } from "./useApi"

export interface WorkProfile {
  id: number
  enabled: boolean
  workDays: string
  workStartTime: string
  workEndTime: string
  lunchBreakStartTime: string
  lunchBreakEndTime: string
  commuteMinMinutes: number
  commuteMaxMinutes: number
  prepareMinMinutes: number
  prepareMaxMinutes: number
  activeMessageAllowed: boolean
  replyMode: string
  allowOvertime: boolean
  overtimeProbability: number
  overtimeMinMinutes: number
  overtimeMaxMinutes: number
  overtimeEndMinTime: string
  overtimeEndMaxTime: string
  overtimeActiveMessageAllowed: boolean
  overtimeReplyMode: string
  commuteHomeShareEnabled: boolean
  commuteHomeShareProbability: number
  delayedReplyEnabled: boolean
}

export const WEEKDAY_OPTIONS = [
  { label: "周一", value: "MON" },
  { label: "周二", value: "TUE" },
  { label: "周三", value: "WED" },
  { label: "周四", value: "THU" },
  { label: "周五", value: "FRI" },
  { label: "周六", value: "SAT" },
  { label: "周日", value: "SUN" },
]

export const WORK_REPLY_OPTIONS = [
  { label: "不回复", value: "NO_REPLY" },
  { label: "简短回复", value: "SHORT_REPLY" },
  { label: "正常回复", value: "NORMAL_REPLY" },
]

export function useWorkProfile() {
  const { get, put } = useApi()
  const loading = ref(false)

  async function getWorkProfile(characterId?: string): Promise<WorkProfile> {
    loading.value = true
    try {
      const params: any = {}
      if (characterId) params.characterId = characterId
      const data = await get<WorkProfile>("/api/companion/work-profile", params)
      return data
    } finally {
      loading.value = false
    }
  }

  async function updateWorkProfile(input: Partial<WorkProfile>, characterId?: string): Promise<WorkProfile> {
    loading.value = true
    try {
      const url = characterId ? "/api/companion/work-profile?characterId=" + encodeURIComponent(characterId) : "/api/companion/work-profile"
      const data = await put<WorkProfile>(url, input)
      return data
    } finally {
      loading.value = false
    }
  }

  return { loading, getWorkProfile, updateWorkProfile }
}