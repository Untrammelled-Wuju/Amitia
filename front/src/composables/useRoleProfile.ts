// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue"
import { useApi } from "./useApi"

// ============================================================
// Types (camelCase, 前端友好)
// ============================================================

export interface RoleProfile {
  id: string
  roleName: string
  gender: "MALE" | "FEMALE" | "NON_BINARY" | "CUSTOM" | "UNSPECIFIED"
  genderLabel: string | null
  pronoun: string
  selfReference: string
  userAddressingStyle: string | null
  genderExpression: number
  createdAt: string
  updatedAt: string
}

export interface RoleProfileUpdateInput {
  roleName?: string
  gender?: string
  genderLabel?: string | null
  pronoun?: string
  selfReference?: string
  userAddressingStyle?: string | null
  genderExpression?: number
}

export const GENDER_OPTIONS = [
  { label: "不强调性别", value: "UNSPECIFIED" },
  { label: "男生", value: "MALE" },
  { label: "女生", value: "FEMALE" },
  { label: "非二元", value: "NON_BINARY" },
  { label: "自定义", value: "CUSTOM" },
]

export const PRONOUN_OPTIONS = [
  { label: "TA", value: "TA" },
  { label: "他", value: "他" },
  { label: "她", value: "她" },
]

export const ADDRESSING_OPTIONS = [
  { label: "自然称呼", value: "自然称呼" },
  { label: "亲近一点", value: "亲近一点" },
  { label: "礼貌一点", value: "礼貌一点" },
  { label: "自定义", value: "自定义" },
]

export const GENDER_PRONOUN_MAP: Record<string, string> = {
  MALE: "他",
  FEMALE: "她",
  NON_BINARY: "TA",
  UNSPECIFIED: "TA",
  CUSTOM: "TA",
}

// ============================================================
// Composable
// ============================================================

export function useRoleProfile() {
  const { get, put } = useApi()
  const loading = ref(false)

  /** 获取角色性别画像 */
  async function getRoleProfile(characterId?: string): Promise<RoleProfile> {
    loading.value = true
    try {
      const params: any = {}
      if (characterId) params.characterId = characterId
      const data = await get<RoleProfile>("/api/companion/role-profile", params)
      return data
    } finally {
      loading.value = false
    }
  }

  /** 更新角色性别画像 */
  async function updateRoleProfile(input: RoleProfileUpdateInput, characterId?: string): Promise<RoleProfile> {
    loading.value = true
    try {
      const url = characterId ? "/api/companion/role-profile?characterId=" + encodeURIComponent(characterId) : "/api/companion/role-profile"
      const data = await put<RoleProfile>(url, input)
      return data
    } finally {
      loading.value = false
    }
  }

  return {
    loading,
    getRoleProfile,
    updateRoleProfile,
  }
}