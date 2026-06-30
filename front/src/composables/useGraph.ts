// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { useApi } from "./useApi"

export function useGraph() {
  const { get } = useApi()

  async function getStats() {
    return get<any>("/api/graph/stats")
  }

  async function getNeighbors(id: string, depth = 2) {
    return get<any>(`/api/graph/node/${encodeURIComponent(id)}/neighbors?depth=${depth}`)
  }

  async function findPath(from: string, to: string, maxDepth = 4) {
    return get<any>(`/api/graph/path?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&maxDepth=${maxDepth}`)
  }

  return { getStats, getNeighbors, findPath }
}
