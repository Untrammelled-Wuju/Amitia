// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { get, post, put } from "@/composables/request"

export interface LongRunningStatus {
  running: boolean
  tasks: Array<{
    id: string
    title: string
    character_id: string
    updated_at: string
  }>
}

export interface LongRunningConfig {
  maxTasks: number
  timeoutMinutes: number
}

export function fetchStatusApi() {
  return get<LongRunningStatus>("/api/runtime/long-running/status")
}

export function fetchConfigApi() {
  return get<LongRunningConfig>("/api/runtime/long-running/config")
}

export function saveConfigApi(data: LongRunningConfig) {
  return put("/api/runtime/long-running/config", data)
}

export function cleanupTempApi() {
  return post<{ deleted: number; freedBytes: number }>("/api/runtime/cleanup-temp")
}

export function rotateLogsApi() {
  return post<{ rotated: string[]; skipped: string[] }>("/api/runtime/rotate-logs")
}

export function checkDbIntegrityApi() {
  return post<{ ok: boolean; errors: string[] }>("/api/runtime/check-db-integrity")
}
