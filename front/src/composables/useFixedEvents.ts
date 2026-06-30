// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue"
import { useApi } from "./useApi"

// ============================================================
// Types
// ============================================================

export interface FixedEvent {
  id: number
  title: string
  eventType: "CLASS" | "STUDY" | "MEETING" | "CUSTOM_BUSY"
  startTime: string
  endTime: string
  repeatType: string
  repeatDays: string | null
  prepareMinMinutes: number
  prepareMaxMinutes: number
  activeMessageAllowed: boolean
  replyMode: "NO_REPLY" | "SHORT_REPLY" | "NORMAL_REPLY" | "DELAY_REPLY"
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface FixedEventInput {
  title: string
  eventType?: string
  startTime: string
  endTime: string
  repeatType?: string
  repeatDays?: string | null
  prepareMinMinutes?: number
  prepareMaxMinutes?: number
  activeMessageAllowed?: boolean
  replyMode?: string
  enabled?: boolean
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

export const EVENT_TYPE_OPTIONS = [
  { label: "上课", value: "CLASS" },
  { label: "自习", value: "STUDY" },
  { label: "全职上班", value: "FULL_TIME_WORK" },
  { label: "兼职", value: "PART_TIME_WORK" },
  { label: "会议", value: "MEETING" },
  { label: "自定义忙碌", value: "CUSTOM_BUSY" },
]

export const LIFE_IDENTITY_OPTIONS = [
  { label: "上学", value: "SCHOOL" },
  { label: "工作", value: "WORK" },
  { label: "待业", value: "UNEMPLOYED" },
  { label: "居家", value: "HOME" },
  { label: "自定义", value: "CUSTOM" },
]

export const ADJUSTMENT_TYPE_OPTIONS = [
  { label: "停课", value: "CANCEL" },
  { label: "调课", value: "RESCHEDULE" },
  { label: "补课", value: "MAKEUP" },
]

export const REPLY_MODE_OPTIONS = [
  { label: "不回复", value: "NO_REPLY" },
  { label: "简短回复", value: "SHORT_REPLY" },
  { label: "正常回复", value: "NORMAL_REPLY" },
]

// ============================================================
// Composable
// ============================================================

export function useFixedEvents() {
  const { get, post, put, del } = useApi()
  const loading = ref(false)

  async function getFixedEvents(characterId?: string): Promise<FixedEvent[]> {
    loading.value = true
    try {
      const params: any = {}
      if (characterId) params.characterId = characterId
      const data = await get<FixedEvent[]>("/api/companion/fixed-events", params)
      return data
    } finally {
      loading.value = false
    }
  }

  async function createFixedEvent(input: FixedEventInput, characterId?: string): Promise<FixedEvent> {
    loading.value = true
    try {
      const url = characterId ? `/api/companion/fixed-events?characterId=${encodeURIComponent(characterId)}` : "/api/companion/fixed-events"
    const data = await post<FixedEvent>(url, input)
      return data
    } finally {
      loading.value = false
    }
  }

  async function updateFixedEvent(id: number, input: Partial<FixedEventInput>, characterId?: string): Promise<FixedEvent> {
    loading.value = true
    try {
      const url = characterId ? `/api/companion/fixed-events/${id}?characterId=` + encodeURIComponent(characterId) : `/api/companion/fixed-events/${id}`
      const data = await put<FixedEvent>(url, input)
      return data
    } finally {
      loading.value = false
    }
  }

  async function deleteFixedEvent(id: number, characterId?: string): Promise<void> {
    loading.value = true
    try {
      const delUrl = characterId ? `/api/companion/fixed-events/${id}?characterId=${encodeURIComponent(characterId)}` : `/api/companion/fixed-events/${id}`
    await del<void>(delUrl)
    } finally {
      loading.value = false
    }
  }

  async function setEventEnabled(id: number, enabled: boolean): Promise<FixedEvent> {
    loading.value = true
    try {
      const data = await post<FixedEvent>(`/api/companion/fixed-events/${id}/enabled`, { enabled })
      return data
    } finally {
      loading.value = false
    }
  }

  return {
    loading,
    getFixedEvents,
    createFixedEvent,
    updateFixedEvent,
    deleteFixedEvent,
    setEventEnabled,
  }
}
