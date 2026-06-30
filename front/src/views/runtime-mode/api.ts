// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { apiClient } from "@/composables/useApi"
import type { RuntimeModeResponse, RuntimeModeValidationResult, DeployMode } from "@/types"

export async function fetchModeApi(): Promise<RuntimeModeResponse | null> {
  try {
    const res = await apiClient.get("/api/runtime/mode")
    return (res.data as any) || null
  } catch {
    return null
  }
}

export async function switchModeApi(deployMode: DeployMode): Promise<void> {
  await apiClient.put("/api/runtime/mode", { deployMode })
}

export async function validateModeApi(): Promise<RuntimeModeValidationResult> {
  const res = await apiClient.post("/api/runtime/mode/validate")
  return (res.data as any)
}
