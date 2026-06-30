// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { apiClient } from "@/composables/useApi"

async function apiGet(url: string, params?: any) {
  const res = await apiClient.get(url, { params })
  return res.data?.data ?? res.data
}

async function apiPost(url: string, data?: any) {
  const res = await apiClient.post(url, data || {})
  return res.data?.data ?? res.data
}

async function apiDel(url: string) {
  const res = await apiClient.delete(url)
  return res.data?.data ?? res.data
}

export function parseText(body: any) {
  return apiPost("/api/imports/parse-text", body)
}

export function confirmImport(body: any) {
  return apiPost("/api/imports/confirm", body)
}

export function generateSummary(batchId: string) {
  return apiPost(`/api/imports/batches/${batchId}/generate-summary`)
}

export function getBatchSummary(batchId: string) {
  return apiGet(`/api/imports/batches/${batchId}/summary`)
}

export function extractMemoryCandidates(batchId: string) {
  return apiPost(`/api/imports/batches/${batchId}/extract-memory-candidates`)
}

export function confirmMemories(batchId: string, selectedIds: string[]) {
  return apiPost(`/api/imports/batches/${batchId}/confirm-memories`, { selectedIds })
}

export function createConversationFromImport(importBatchId: string) {
  return apiPost("/api/web-chat/conversations/from-import", { importBatchId })
}

export function fetchBatchesApi(params?: { page?: number; pageSize?: number }) {
  return apiGet("/api/imports/batches", params)
}

export function fetchBatchDetailApi(id: string) {
  return apiGet(`/api/imports/batches/${id}`)
}

export function deleteBatchApi(id: string) {
  return apiDel(`/api/imports/batches/${id}`)
}
