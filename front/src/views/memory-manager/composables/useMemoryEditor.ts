import { reactive, ref } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "../../../composables/useApi"
import { rowAllowContextUse, rowAllowProactiveMention, rowRequiresConfirmation, rowScopeType, rowSensitivity, scopeTypeToScope } from "../memoryFormatters"

export function useMemoryEditor(fetchList: () => Promise<void> | void, currentCharacterId: () => string | null | undefined) {
  const { post, put } = useApi()
  const dialogVisible = ref(false)
  const editing = ref(false)
  const editingId = ref("")
  const saving = ref(false)
  const form = reactive({ key: "", value: "", memoryType: "custom", importance: 5, characterId: "", scope: "character", scopeType: "user_character", source: "manual", sensitivity: "normal", allowContextUse: true, allowProactiveMention: false, requiresConfirmation: false })

  function showCreate() {
    editing.value = false
    editingId.value = ""
    Object.assign(form, { key: "", value: "", memoryType: "custom", importance: 5, characterId: currentCharacterId() || "", scope: "character", scopeType: "user_character", source: "manual", sensitivity: "normal", allowContextUse: true, allowProactiveMention: false, requiresConfirmation: false })
    dialogVisible.value = true
  }
  function showEdit(row: any) {
    editing.value = true
    editingId.value = row.id
    Object.assign(form, { key: row.key, value: row.value, memoryType: row.memoryType, importance: row.importance, characterId: row.characterId || "", scope: row.scope || "character", scopeType: rowScopeType(row), source: row.source || "manual", sensitivity: rowSensitivity(row), allowContextUse: rowAllowContextUse(row), allowProactiveMention: rowAllowProactiveMention(row), requiresConfirmation: rowRequiresConfirmation(row) })
    dialogVisible.value = true
  }
  async function saveMem() {
    saving.value = true
    try {
      const payload = { ...form, source: form.source || "manual", scope: scopeTypeToScope(form.scopeType), allowContextUse: !!form.allowContextUse, allowProactiveMention: !!form.allowProactiveMention, requiresConfirmation: !!form.requiresConfirmation }
      if (editing.value) await put(`/api/memories/${editingId.value}`, payload)
      else await post("/api/memories", payload)
      dialogVisible.value = false
      ElMessage.success(editing.value ? "保存成功" : "新建成功")
      await fetchList()
    } catch (error: any) { ElMessage.error(error?.message || "保存失败") }
    saving.value = false
  }
  return { dialogVisible, editing, editingId, saving, form, showCreate, showEdit, saveMem }
}
