// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue"
import { useApi } from "./useApi"

export interface SpecialEvent {
  id: number
  title: string
  eventType: string
  startDate: string | null
  endDate: string | null
  startTime: string | null
  endTime: string | null
  repeatType: string
  repeatDays: string | null
  enabled: boolean
  priority: number
  activeMessageAllowed: boolean
  replyMode: string
  affectSchedule: boolean
  affectSleep: boolean
  affectMeal: boolean
  affectEnergy: boolean
  payload: any | null
}

export const SPECIAL_EVENT_TYPE_OPTIONS = [
  { label: "考试周", value: "EXAM_WEEK" },
  { label: "具体考试", value: "EXAM" },
  { label: "周末兼职", value: "PART_TIME_WORK" },
  { label: "晚上健身", value: "EVENING_WORKOUT" },
  { label: "图书馆学习", value: "LIBRARY_STUDY" },
  { label: "生病休息", value: "SICK_REST" },
  { label: "自定义", value: "CUSTOM" },
]

export const SPECIAL_REPLY_OPTIONS = [
  { label: "不回复", value: "NO_REPLY" },
  { label: "简短回复", value: "SHORT_REPLY" },
  { label: "正常回复", value: "NORMAL_REPLY" },
]

export const WEEKDAY_OPTIONS = [
  { label: "周一", value: "MON" }, { label: "周二", value: "TUE" },
  { label: "周三", value: "WED" }, { label: "周四", value: "THU" },
  { label: "周五", value: "FRI" }, { label: "周六", value: "SAT" },
  { label: "周日", value: "SUN" },
]

export function useSpecialEvents() {
  const { get, post, put, del } = useApi()
  const loading = ref(false)

  async function getSpecialEvents(characterId?: string): Promise<SpecialEvent[]> {
    loading.value = true
    try {
      const params: any = {}
      if (characterId) params.characterId = characterId
      return await get<SpecialEvent[]>("/api/companion/special-events", params)
    } finally { loading.value = false }
  }

  async function createSpecialEvent(input: any, characterId?: string): Promise<SpecialEvent> {
    loading.value = true
    try {
      const url = characterId ? "/api/companion/special-events?characterId=" + encodeURIComponent(characterId) : "/api/companion/special-events"
      return await post<SpecialEvent>(url, input)
    } finally { loading.value = false }
  }

  async function updateSpecialEvent(id: number, input: any, characterId?: string): Promise<SpecialEvent> {
    loading.value = true
    try {
      const url = characterId ? `/api/companion/special-events/${id}?characterId=` + encodeURIComponent(characterId) : `/api/companion/special-events/${id}`
      return await put<SpecialEvent>(url, input)
    } finally { loading.value = false }
  }

  async function deleteSpecialEvent(id: number, characterId?: string): Promise<void> {
    loading.value = true
    try {
      const url = characterId ? `/api/companion/special-events/${id}?characterId=` + encodeURIComponent(characterId) : `/api/companion/special-events/${id}`
      await del<void>(url)
    } finally { loading.value = false }
  }

  return { loading, getSpecialEvents, createSpecialEvent, updateSpecialEvent, deleteSpecialEvent }
}