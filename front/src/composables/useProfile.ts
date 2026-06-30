// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue"
import { apiClient } from "../ui-index"

export interface UserProfile {
  id: string
  userId: string
  category: string
  attributeName: string
  attributeValue: string
  confidence: number
  sourceConvId: string
  verifiedAt: string
  createdAt: string
  updatedAt: string
}

export interface ProfileListResponse {
  items: UserProfile[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

const categoryLabels: Record<string, string> = {
  personal_info: "个人信息",
  preference: "偏好",
  habit: "习惯",
  fear: "恐惧",
  relationship: "关系",
  health: "健康",
  plan: "计划",
}

export function useProfile() {
  const profiles = ref<UserProfile[]>([])
  const loading = ref(false)
  const total = ref(0)

  async function fetchProfiles(params?: { userId?: string; category?: string; page?: number; pageSize?: number }) {
    loading.value = true
    try {
      const res = await apiClient.get<ProfileListResponse>("/api/profiles", { params })
      profiles.value = res.data.items || []
      total.value = res.data.total || 0
    } catch (e) {
      console.error("获取画像失败", e)
    } finally {
      loading.value = false
    }
  }

  async function createProfile(data: { userId?: string; category: string; attributeName: string; attributeValue: string; confidence?: number }) {
    await apiClient.post("/api/profiles", data)
    await fetchProfiles()
  }

  async function updateProfile(id: string, data: { attributeValue?: string; confidence?: number; verified?: boolean }) {
    await apiClient.put(`/api/profiles/${id}`, data)
    await fetchProfiles()
  }

  async function deleteProfile(id: string) {
    await apiClient.delete(`/api/profiles/${id}`)
    await fetchProfiles()
  }

  function categoryLabel(cat: string): string {
    return categoryLabels[cat] || cat
  }

  function confidenceColor(confidence: number): string {
    if (confidence >= 80) return "success"
    if (confidence >= 50) return "warning"
    return "danger"
  }

  return {
    profiles,
    loading,
    total,
    fetchProfiles,
    createProfile,
    updateProfile,
    deleteProfile,
    categoryLabel,
    confidenceColor,
  }
}
