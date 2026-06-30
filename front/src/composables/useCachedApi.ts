// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, type Ref } from "vue"
import { useApi } from "./useApi"

/**
 * Cached API composable.
 * Returns cached data immediately from localStorage, then refreshes from API.
 * All successful GET responses are cached; mutations clear related cache.
 */
export function useCachedApi() {
  const { get: rawGet, post: rawPost, put: rawPut, del: rawDel } = useApi()

  function cacheKey(url: string): string {
    return "api_cache:" + url.replace(/[^a-zA-Z0-9]/g, "_")
  }

  function loadCache<T>(url: string): T | null {
    try {
      const raw = localStorage.getItem(cacheKey(url))
      if (raw) return JSON.parse(raw) as T
    } catch {}
    return null
  }

  function saveCache(url: string, data: any): void {
    try {
      localStorage.setItem(cacheKey(url), JSON.stringify(data))
    } catch {}
  }

  function invalidateCache(prefix: string): void {
    const keys = Object.keys(localStorage).filter(k => k.startsWith("api_cache:" + prefix))
    keys.forEach(k => localStorage.removeItem(k))
  }

  /**
   * GET with cache: returns cached data via the ref immediately,
   * then fetches fresh data and updates the ref + cache.
   */
  async function cachedGet<T>(url: string, params?: Record<string, any>): Promise<{ data: Ref<T | null>; refresh: () => Promise<void> }> {
    const fullUrl = url + (params ? "?" + new URLSearchParams(params).toString() : "")
    const data = ref<T | null>(loadCache<T>(fullUrl)) as Ref<T | null>

    const refresh = async () => {
      try {
        const result = await rawGet<T>(url, params)
        if (result !== null && result !== undefined) {
          data.value = result
          saveCache(fullUrl, result)
        }
      } catch {}
    }

    // Fetch in background (don't await - return immediately)
    refresh()

    return { data, refresh }
  }

  /** GET without cache (for sensitive or real-time data) */
  async function liveGet<T>(url: string, params?: Record<string, any>): Promise<T> {
    return rawGet<T>(url, params)
  }

  /** POST: invalidates related cache after success */
  async function cachedPost<T>(url: string, body?: any, cachePrefix?: string): Promise<T> {
    const result = await rawPost<T>(url, body)
    if (cachePrefix) invalidateCache(cachePrefix)
    return result
  }

  /** PUT: invalidates related cache after success */
  async function cachedPut<T>(url: string, body?: any, cachePrefix?: string): Promise<T> {
    const result = await rawPut<T>(url, body)
    if (cachePrefix) invalidateCache(cachePrefix)
    return result
  }

  /** DELETE: invalidates related cache after success */
  async function cachedDel<T>(url: string, cachePrefix?: string): Promise<T> {
    const result = await rawDel<T>(url)
    if (cachePrefix) invalidateCache(cachePrefix)
    return result
  }

  return { cachedGet, liveGet, cachedPost, cachedPut, cachedDel, saveCache, loadCache, invalidateCache }
}
