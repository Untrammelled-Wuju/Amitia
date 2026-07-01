import { ref, reactive } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "../../composables/useApi"

export function useMemoryList(injectedCharacterId?: any) {
  const { get, post, put, del } = useApi()
  const memories = ref<any[]>([])
  const keyword = ref("")
  const typeFilter = ref("")
  const sourceFilter = ref("")
  const scopeTypeFilter = ref("")
  const characterFilter = ref(injectedCharacterId?.value || "")
  const characters = ref<any[]>([])
  const sortBy = ref("importance_desc")
  const page = ref(1)
  const pageSize = ref(20)
  const total = ref(0)
  const selectedIds = ref<string[]>([])
  const tableRef = ref<any>(null)
  const globalQuery = ref("")
  const globalSearching = ref(false)
  const globalSearched = ref(false)
  const showGlobalResults = ref(false)
  const globalResults = ref({ memories: [] as any[], profiles: [] as any[], episodics: [] as any[], worldBooks: [] as any[] })
  const globalResultCount = ref(0)

  async function fetchList() {
    const params: any = { page: page.value, pageSize: pageSize.value }
    if (characterFilter.value) params.characterId = characterFilter.value
    if (keyword.value) params.keyword = keyword.value
    if (typeFilter.value) params.memoryType = typeFilter.value
    if (sourceFilter.value) params.source = sourceFilter.value
    if (scopeTypeFilter.value) params.scopeType = scopeTypeFilter.value
    if (sortBy.value) params.sortBy = sortBy.value
    try {
      const r = await get<any>("/api/memories", params)
      memories.value = r?.items || []
      total.value = r?.total || 0
    } catch {}
  }

  function handleSelectionChange(rows: any[]) { selectedIds.value = rows.map(r => r.id) }

  async function delMem(id: string) {
    await ElMessageBox.confirm("确定删除？", "提示", { type: "warning" })
    await del("/api/memories/" + id)
    ElMessage.success("已删除")
    fetchList()
  }

  async function toggleScope(row: any) {
    const newScopeType = (row.scopeType || row.scope_type || (row.scope === "user" ? "user_global" : "user_character")) === "user_global" ? "user_character" : "user_global"
    const newScope = newScopeType === "user_global" ? "user" : "character"
    try {
      await put("/api/memories/" + row.id, { scope: newScope, scopeType: newScopeType })
      row.scope = newScope
      row.scopeType = newScopeType
      ElMessage.success(newScopeType === "user_global" ? "已升级为全局记忆" : "已降级为角色记忆")
    } catch {}
  }

  async function batchVerify() {
    if (selectedIds.value.length === 0) return
    try { await post("/api/memories/batch-verify", { ids: selectedIds.value, status: "user_verified" }); ElMessage.success("批量确认成功"); selectedIds.value = []; fetchList() } catch { ElMessage.error("操作失败") }
  }

  async function batchSetImportant() {
    if (selectedIds.value.length === 0) return
    try { await post("/api/memories/batch-importance", { ids: selectedIds.value, importance: 10 }); ElMessage.success("已标为重要"); selectedIds.value = []; fetchList() } catch { ElMessage.error("操作失败") }
  }

  async function batchDelete() {
    if (selectedIds.value.length === 0) return
    await ElMessageBox.confirm("确定删除选中的 " + selectedIds.value.length + " 条记忆？此操作不可撤销。", "提示", { type: "warning" })
    try {
      await Promise.all(selectedIds.value.map(id => del("/api/memories/" + id)))
      ElMessage.success("批量删除成功")
      selectedIds.value = []
      tableRef.value?.clearSelection?.()
      fetchList()
    } catch { ElMessage.error("批量删除失败") }
  }

  return {
    memories, keyword, typeFilter, sourceFilter, scopeTypeFilter, characterFilter, characters, sortBy,
    page, pageSize, total, selectedIds, tableRef,
    globalQuery, globalSearching, globalSearched, showGlobalResults, globalResults, globalResultCount,
    fetchList, handleSelectionChange, delMem, toggleScope, batchVerify, batchSetImportant, batchDelete
  }
}