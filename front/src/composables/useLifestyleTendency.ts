// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue"
import { useApi } from "./useApi"

// ============================================================
// Types
// ============================================================

export interface LifestyleTendencyDTO {
  id: number
  punctualityTendency: number
  earlyPrepareTendency: number
  selfDisciplineTendency: number
  sleepinessTendency: number
  randomnessTendency: number
  activityEnergy: number
  socialEnergy: number
  careTendency: number
  dailyShareTendency: number
  manuallyConfigured: boolean
  createdAt: string
  updatedAt: string
}

export type LifestyleTendencyInput = Partial<
  Pick<LifestyleTendencyDTO, "punctualityTendency" | "earlyPrepareTendency" | "selfDisciplineTendency"
    | "sleepinessTendency" | "randomnessTendency" | "activityEnergy"
    | "socialEnergy" | "careTendency" | "dailyShareTendency">
>

export const LIFESTYLE_SLIDERS = [
  {
    key: "punctualityTendency" as const,
    label: "卡点程度",
    hint: "越高越容易接近最后时间行动，但不会故意迟到。",
    left: "从容", right: "踩点",
    group: "作息习惯",
  },
  {
    key: "earlyPrepareTendency" as const,
    label: "提前准备程度",
    hint: "越高越容易提前进入准备状态。",
    left: "随性", right: "提前",
    group: "作息习惯",
  },
  {
    key: "selfDisciplineTendency" as const,
    label: "自律程度",
    hint: "越高作息越稳定，起床睡觉更规律。",
    left: "随性", right: "自律",
    group: "作息习惯",
  },
  {
    key: "sleepinessTendency" as const,
    label: "赖床程度",
    hint: "越高越容易晚起或午睡。",
    left: "清醒", right: "嗜睡",
    group: "作息习惯",
  },
  {
    key: "randomnessTendency" as const,
    label: "作息随机程度",
    hint: "越高每天作息变化越明显。",
    left: "规律", right: "随性",
    group: "作息习惯",
  },
  {
    key: "activityEnergy" as const,
    label: "日常精力",
    hint: "越高白天状态越活跃，精力更充沛。",
    left: "低能量", right: "高能量",
    group: "精力状态",
  },
  {
    key: "socialEnergy" as const,
    label: "社交精力",
    hint: "越高越愿意主动聊天，回复更积极。",
    left: "安静", right: "活跃",
    group: "主动互动",
  },
  {
    key: "careTendency" as const,
    label: "关心倾向",
    hint: "越高越容易主动关心用户、询问状态。",
    left: "独立", right: "贴心",
    group: "主动互动",
  },
  {
    key: "dailyShareTendency" as const,
    label: "日常分享倾向",
    hint: "越高越容易分享自己的日常状态。",
    left: "寡言", right: "分享",
    group: "主动互动",
  },
]

export const LIFESTYLE_GROUPS = [
  { name: "作息习惯", keys: ["punctualityTendency", "earlyPrepareTendency", "selfDisciplineTendency", "sleepinessTendency", "randomnessTendency"] },
  { name: "精力状态", keys: ["activityEnergy"] },
  { name: "主动互动", keys: ["socialEnergy", "careTendency", "dailyShareTendency"] },
]

// ============================================================
// Composable
// ============================================================

export function useLifestyleTendency() {
  const { get, put, post } = useApi()
  const loading = ref(false)

  async function getLifestyleTendency(characterId?: string): Promise<LifestyleTendencyDTO> {
    loading.value = true
    try {
      const params: any = {}
      if (characterId) params.characterId = characterId
      const data = await get<LifestyleTendencyDTO>("/api/companion/lifestyle-tendency", params)
      return data
    } finally {
      loading.value = false
    }
  }

  async function updateLifestyleTendency(input: LifestyleTendencyInput, characterId?: string): Promise<LifestyleTendencyDTO> {
    loading.value = true
    try {
      const url = characterId ? "/api/companion/lifestyle-tendency?characterId=" + encodeURIComponent(characterId) : "/api/companion/lifestyle-tendency"
      const data = await put<LifestyleTendencyDTO>(url, input)
      return data
    } finally {
      loading.value = false
    }
  }

  async function resetLifestyleTendency(characterId?: string): Promise<LifestyleTendencyDTO> {
    loading.value = true
    try {
      const url = characterId ? "/api/companion/lifestyle-tendency/reset?characterId=" + encodeURIComponent(characterId) : "/api/companion/lifestyle-tendency/reset"
      const data = await post<LifestyleTendencyDTO>(url)
      return data
    } finally {
      loading.value = false
    }
  }

  return {
    loading,
    getLifestyleTendency,
    updateLifestyleTendency,
    resetLifestyleTendency,
  }
}