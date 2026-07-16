import { ref } from "vue"
import { useApi } from "../../../composables/useApi"

export function useMemorySearch() {
  const { post } = useApi()
  const searchDialogVisible = ref(false)
  const searchQuery = ref("")
  const searchResults = ref<any[]>([])
  const searched = ref(false)
  const globalQuery = ref("")
  const globalSearching = ref(false)
  const globalSearched = ref(false)
  const showGlobalResults = ref(false)
  const globalResults = ref({ memories: [] as any[], profiles: [] as any[], episodics: [] as any[], worldBooks: [] as any[] })
  const globalResultCount = ref(0)

  function searchMemory() { searchDialogVisible.value = true; searched.value = false; searchResults.value = []; searchQuery.value = "" }
  async function doSearch() {
    if (!searchQuery.value.trim()) return
    try {
      const result = await post<any>("/api/memories/hybrid-search", { keyword: searchQuery.value.trim(), limit: 10 })
      searchResults.value = (result?.items || []).map((item: any) => ({ id: item.memory?.id || item.id, key: item.memory?.key || item.key, value: item.memory?.value || item.value, memoryType: item.memory?.memoryType || item.memoryType, score: item.score ?? 0, matchType: item.matchType || "hybrid", memoryLayer: item.memoryLayer || "" }))
      searched.value = true
    } catch {
      try { const result = await post<any>("/api/memories/search", { keyword: searchQuery.value.trim(), limit: 10 }); searchResults.value = (result?.items || []).map((item: any) => ({ ...item, score: 0 })); searched.value = true } catch { searchResults.value = []; searched.value = true }
    }
  }
  function clearGlobalSearch() { globalQuery.value = ""; globalSearched.value = false; showGlobalResults.value = false; globalResults.value = { memories: [], profiles: [], episodics: [], worldBooks: [] }; globalResultCount.value = 0 }
  async function doGlobalSearch() {
    if (!globalQuery.value.trim()) return
    globalSearching.value = true
    try {
      const { useMemoryHub } = await import("../../../composables/useMemoryHub")
      const results = await useMemoryHub().globalSearch(globalQuery.value.trim())
      globalResults.value = results
      globalResultCount.value = results.memories.length + results.profiles.length + results.episodics.length + results.worldBooks.length
    } catch { globalResults.value = { memories: [], profiles: [], episodics: [], worldBooks: [] }; globalResultCount.value = 0 }
    globalSearched.value = true
    showGlobalResults.value = true
    globalSearching.value = false
  }
  return { searchDialogVisible, searchQuery, searchResults, searched, globalQuery, globalSearching, globalSearched, showGlobalResults, globalResults, globalResultCount, searchMemory, doSearch, clearGlobalSearch, doGlobalSearch }
}
