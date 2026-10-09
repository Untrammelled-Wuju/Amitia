// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { apiClient } from "@/composables/useApi";
import type {
  RuntimeModeResponse,
  RuntimeModeValidationResult,
  DeployMode,
} from "@/types";

export async function fetchModeApi(): Promise<RuntimeModeResponse | null> {
  try {
    const res = await apiClient.get("/api/runtime/mode");
    const body = res.data as any;
    const payload = body && typeof body === "object" && "data" in body && ("code" in body || "msg" in body)
      ? body.data
      : body;
    return payload && typeof payload === "object" && !Array.isArray(payload)
      ? payload as RuntimeModeResponse
      : null;
  } catch {
    return null;
  }
}

export async function switchModeApi(deployMode: DeployMode): Promise<void> {
  await apiClient.put("/api/runtime/mode", { deployMode });
}

export async function validateModeApi(): Promise<RuntimeModeValidationResult> {
  const res = await apiClient.post("/api/runtime/mode/validate");
  const body = res.data as any;
  return (body && typeof body === "object" && "data" in body && ("code" in body || "msg" in body)
    ? body.data
    : body) as RuntimeModeValidationResult;
}
