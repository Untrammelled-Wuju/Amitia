import { ref } from "vue";

export function useMemorySearch() {
  const searchDialogVisible = ref(false);
  const globalQuery = ref("");
  const globalSearching = ref(false);
  const globalSearched = ref(false);
  const showGlobalResults = ref(false);
  const globalResults = ref({
    memories: [] as any[],
    profiles: [] as any[],
    episodics: [] as any[],
    worldBooks: [] as any[],
  });
  const globalResultCount = ref(0);

  function searchMemory() {
    searchDialogVisible.value = true;
  }

  function clearGlobalSearch() {
    globalQuery.value = "";
    globalSearched.value = false;
    showGlobalResults.value = false;
    globalResults.value = {
      memories: [],
      profiles: [],
      episodics: [],
      worldBooks: [],
    };
    globalResultCount.value = 0;
  }

  async function doGlobalSearch() {
    if (!globalQuery.value.trim()) return;
    globalSearching.value = true;
    try {
      const { useMemoryHub } = await import("../../../composables/useMemoryHub");
      const results = await useMemoryHub().globalSearch(globalQuery.value.trim());
      globalResults.value = results;
      globalResultCount.value =
        results.memories.length +
        results.profiles.length +
        results.episodics.length +
        results.worldBooks.length;
    } catch {
      globalResults.value = {
        memories: [],
        profiles: [],
        episodics: [],
        worldBooks: [],
      };
      globalResultCount.value = 0;
    }
    globalSearched.value = true;
    showGlobalResults.value = true;
    globalSearching.value = false;
  }

  return {
    searchDialogVisible,
    globalQuery,
    globalSearching,
    globalSearched,
    showGlobalResults,
    globalResults,
    globalResultCount,
    searchMemory,
    clearGlobalSearch,
    doGlobalSearch,
  };
}
