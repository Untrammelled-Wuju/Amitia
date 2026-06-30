// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { apiClient } from "@/composables/useApi"

export async function postScan(scope: string[]) {
  const res = await apiClient.post("/api/privacy/scan", { scope })
  return res.data?.data || res.data
}

export async function getScanResults(params: any) {
  const res = await apiClient.get("/api/privacy/scan-results", { params })
  return res.data?.data || res.data
}

export async function postMask(ids: number[], confirmToken: string) {
  const res = await apiClient.post("/api/privacy/mask", { ids, confirmToken })
  return res.data?.data || res.data
}
