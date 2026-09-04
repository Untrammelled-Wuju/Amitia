import { ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "../../../composables/useApi";
import {
  rowAllowContextUse,
  rowAllowProactiveMention,
  rowRequiresConfirmation,
  rowScopeType,
  rowSensitivity,
} from "../memoryFormatters";

export function useMemoryList(injectedCharacterId?: any) {
  const { get, post, put, del } = useApi();
  const memories = ref<any[]>([]);
  const keyword = ref("");
  const typeFilter = ref("");
  const sourceFilter = ref("");
  const scopeTypeFilter = ref("");
  const retentionLevelFilter = ref(0);
  const decayStateFilter = ref("");
  const characterFilter = ref(injectedCharacterId?.value || "");
  const characters = ref<any[]>([]);
  const sortBy = ref("importance_desc");
  const page = ref(1);
  const pageSize = ref(20);
  const total = ref(0);
  const selectedIds = ref<string[]>([]);
  const tableRef = ref<any>(null);
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

  async function fetchList() {
    const params: any = { page: page.value, pageSize: pageSize.value };
    if (characterFilter.value) params.characterId = characterFilter.value;
    if (keyword.value) params.keyword = keyword.value;
    if (typeFilter.value) params.memoryType = typeFilter.value;
    if (sourceFilter.value) params.source = sourceFilter.value;
    if (scopeTypeFilter.value) params.scopeType = scopeTypeFilter.value;
    if (retentionLevelFilter.value) params.retentionLevel = retentionLevelFilter.value;
    if (decayStateFilter.value) params.decayState = decayStateFilter.value;
    if (sortBy.value) params.sortBy = sortBy.value;
    try {
      const r = await get<any>("/api/memories", params);
      memories.value = r?.items || [];
      total.value = r?.total || 0;
    } catch {}
  }

  function handleSelectionChange(rows: any[]) {
    selectedIds.value = rows.map((r) => r.id);
  }

  async function delMem(id: string) {
    await ElMessageBox.confirm("确定删除？", "提示", { type: "warning" });
    await del("/api/memories/" + id);
    ElMessage.success("已删除");
    fetchList();
  }

  async function restoreMemory(row: any) {
    try {
      await post(`/api/memories/${row.id}/restore`);
      ElMessage.success("记忆已恢复");
      await fetchList();
    } catch (error: any) {
      ElMessage.error(error?.message || "恢复失败");
    }
  }

  async function togglePinned(row: any) {
    try {
      await put(`/api/memories/${row.id}`, { pinned: !row.pinned });
      ElMessage.success(row.pinned ? "已取消固定" : "已固定为核心记忆");
      await fetchList();
    } catch (error: any) {
      ElMessage.error(error?.message || "操作失败");
    }
  }

  async function toggleScope(row: any) {
    const newScopeType =
      (row.scopeType ||
        row.scope_type ||
        (row.scope === "user" ? "user_global" : "user_character")) ===
      "user_global"
        ? "user_character"
        : "user_global";
    const newScope = newScopeType === "user_global" ? "user" : "character";
    try {
      await put("/api/memories/" + row.id, {
        scope: newScope,
        scopeType: newScopeType,
      });
      row.scope = newScope;
      row.scopeType = newScopeType;
      ElMessage.success(
        newScopeType === "user_global"
          ? "已升级为全局记忆"
          : "已降级为角色记忆",
      );
    } catch {}
  }

  async function batchVerify() {
    if (selectedIds.value.length === 0) return;
    try {
      await post("/api/memories/batch-verify", {
        ids: selectedIds.value,
        status: "user_verified",
      });
      ElMessage.success("批量确认成功");
      selectedIds.value = [];
      fetchList();
    } catch {
      ElMessage.error("操作失败");
    }
  }

  async function batchSetImportant() {
    if (selectedIds.value.length === 0) return;
    try {
      await post("/api/memories/batch-importance", {
        ids: selectedIds.value,
        importance: 10,
      });
      ElMessage.success("已标为重要");
      selectedIds.value = [];
      fetchList();
    } catch {
      ElMessage.error("操作失败");
    }
  }

  async function batchDelete() {
    if (selectedIds.value.length === 0) return;
    await ElMessageBox.confirm(
      "确定删除选中的 " +
        selectedIds.value.length +
        " 条记忆？此操作不可撤销。",
      "提示",
      { type: "warning" },
    );
    try {
      await Promise.all(
        selectedIds.value.map((id) => del("/api/memories/" + id)),
      );
      ElMessage.success("批量删除成功");
      selectedIds.value = [];
      tableRef.value?.clearSelection?.();
      fetchList();
    } catch {
      ElMessage.error("批量删除失败");
    }
  }

  async function handleClearAll() {
    await ElMessageBox.confirm(
      "确定清空当前角色全部 " + total.value + " 条记忆？此操作不可撤销。",
      "警告",
      {
        type: "warning",
        confirmButtonText: "确定清空",
        confirmButtonClass: "el-button--danger",
      },
    );
    const characterId = characterFilter.value || injectedCharacterId?.value;
    if (!characterId) {
      ElMessage.warning("请先选择角色再清空");
      return;
    }
    await del("/api/memories?characterId=" + characterId);
    ElMessage.success("已清空");
    fetchList();
  }

  async function handleExport() {
    try {
      const params: any = { pageSize: 10000 };
      if (characterFilter.value) params.characterId = characterFilter.value;
      if (typeFilter.value) params.memoryType = typeFilter.value;
      if (sourceFilter.value) params.source = sourceFilter.value;
      if (scopeTypeFilter.value) params.scopeType = scopeTypeFilter.value;
      if (retentionLevelFilter.value) params.retentionLevel = retentionLevelFilter.value;
      if (decayStateFilter.value) params.decayState = decayStateFilter.value;
      const all = await get<any>("/api/memories", params);
      const items = all?.items || [];
      const data = items.map((memory: any) => ({
        key: memory.key,
        value: memory.value,
        type: memory.memoryType,
        subtype: memory.memorySubtype || "",
        importance: memory.importance,
        confidence: memory.confidence,
        retentionLevel: memory.retentionLevel,
        memoryStrength: memory.memoryStrength,
        decayState: memory.decayState,
        pinned: !!memory.pinned,
        reinforceCount: memory.reinforceCount || 0,
        retrievedCount: memory.retrievedCount || 0,
        injectedCount: memory.injectedCount || 0,
        lastReinforcedAt: memory.lastReinforcedAt || null,
        source: memory.source,
        scope: memory.scope,
        scopeType: rowScopeType(memory),
        sensitivity: rowSensitivity(memory),
        allowContextUse: rowAllowContextUse(memory),
        allowProactiveMention: rowAllowProactiveMention(memory),
        requiresConfirmation: rowRequiresConfirmation(memory),
      }));
      const blob = new Blob([JSON.stringify(data, null, 2)], {
        type: "application/json",
      });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download =
        "memories-" + new Date().toISOString().slice(0, 10) + ".json";
      anchor.click();
      URL.revokeObjectURL(url);
      ElMessage.success("已导出 " + items.length + " 条记忆");
    } catch {
      ElMessage.error("导出失败");
    }
  }

  async function loadCharacters() {
    try {
      characters.value = (await get<any[]>("/api/characters")) || [];
    } catch {}
  }

  return {
    memories,
    keyword,
    typeFilter,
    sourceFilter,
    scopeTypeFilter,
    retentionLevelFilter,
    decayStateFilter,
    characterFilter,
    characters,
    sortBy,
    page,
    pageSize,
    total,
    selectedIds,
    tableRef,
    globalQuery,
    globalSearching,
    globalSearched,
    showGlobalResults,
    globalResults,
    globalResultCount,
    fetchList,
    handleSelectionChange,
    delMem,
    restoreMemory,
    togglePinned,
    toggleScope,
    batchVerify,
    batchSetImportant,
    batchDelete,
    handleClearAll,
    handleExport,
    loadCharacters,
  };
}
