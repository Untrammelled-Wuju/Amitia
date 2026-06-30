// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, reactive, computed, inject } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { useApi } from "../../../composables/useApi"
import { type TemplateItem, DEFAULT_BOUNDARY, DEFAULT_PERSONALITY_CONFIG, type PersonalityConfig } from "./types"

export { type TemplateItem } from "./types"

export function useCharacterConfig() {
  const { get, post, put, del } = useApi()
  const refreshHealth = inject<() => void>("refreshHealth", () => {})
  const defaultPersonalityConfig = DEFAULT_PERSONALITY_CONFIG

  const templates = ref<any[]>([])
  const showTemplateDialog = ref(false)
  const templateLoading = ref(false)

  const characters = ref<any[]>([])
  const selected = ref<any>(null)
  const selectedId = ref("")
  const activeTab = ref("edit")
  const saving = ref(false)
  const showFullPrompt = ref(false)
  const showFullBounds = ref(false)

  const form = reactive({
    name: "", avatar: "", identity: "", personality: "",
    speakingStyle: "", relationshipStyle: "",
    systemPrompt: "", boundaryRules: DEFAULT_BOUNDARY,
    isActive: true, description: "", basePrompt: "", isDefault: false, status: "enabled",
    personalityConfig: { ...DEFAULT_PERSONALITY_CONFIG } as PersonalityConfig,
    chatStyleConfig: null as any,
    sceneRules: null as any,
  })

  function normalizePersonalityConfig(value: any): PersonalityConfig {
    const raw = typeof value === "string" ? JSON.parse(value) : (value || {})
    return { ...defaultPersonalityConfig, ...(raw as Partial<PersonalityConfig>) }
  }

  const hasOtherActive = computed(() =>
    characters.value.some(c => c.isActive && c.id !== selectedId.value)
  )

  async function fetchTemplates() {
    templateLoading.value = true
    try { templates.value = await get<any[]>("/api/character-templates") || [] }
    catch { templates.value = [] }
    finally { templateLoading.value = false }
  }

  async function fetchChars() {
    try { characters.value = await get<any[]>("/api/characters") || [] } catch {}
  }

  function selectChar(c: any) {
    selected.value = c
    selectedId.value = c.id
    activeTab.value = "edit"
    form.name = c.name || ""
    form.avatar = c.avatar || ""
    form.identity = c.identity || ""
    form.personality = c.personality || ""
    form.speakingStyle = c.speakingStyle || ""
    form.relationshipStyle = c.relationshipStyle || ""
    form.systemPrompt = c.systemPrompt || ""
    form.boundaryRules = c.boundaryRules ?? DEFAULT_BOUNDARY
    form.description = c.description || ""
    form.basePrompt = c.basePrompt || ""
    form.isDefault = !!c.isDefault
    form.status = c.status || "enabled"
    form.personalityConfig = normalizePersonalityConfig(c.personalityConfig)
    form.chatStyleConfig = c.chatStyleConfig || null
    form.sceneRules = c.sceneRules || null
    form.isActive = !!c.isActive
  }

  function createNew() {
    selected.value = { id: "", name: "", isActive: false }
    selectedId.value = ""
    activeTab.value = "edit"
    form.name = ""; form.avatar = ""; form.identity = ""; form.personality = ""
    form.speakingStyle = ""; form.relationshipStyle = ""; form.systemPrompt = ""; form.boundaryRules = ""
    form.isActive = true
  }

  async function createFromTemplate(tpl: TemplateItem) {
    try {
      const result = await post<any>(`/api/character-templates/${tpl.id}/create-character`, { name: tpl.name })
      if (result) {
        showTemplateDialog.value = false
        await fetchChars()
        selectChar(result)
      }
    } catch (err: any) {
      console.error("Failed to create from template:", err)
    }
  }

  function copyChar(c: any) {
    createNew()
    form.name = (c.name || "") + " (副本)"
    form.avatar = c.avatar || ""; form.identity = c.identity || ""; form.personality = c.personality || ""
    form.speakingStyle = c.speakingStyle || ""; form.relationshipStyle = c.relationshipStyle || ""
    form.systemPrompt = c.systemPrompt || ""; form.boundaryRules = c.boundaryRules ?? DEFAULT_BOUNDARY
    form.description = c.description || ""; form.basePrompt = c.basePrompt || ""
    form.isDefault = false; form.status = "enabled"
    form.personalityConfig = normalizePersonalityConfig(c.personalityConfig)
    form.chatStyleConfig = c.chatStyleConfig || null; form.sceneRules = c.sceneRules || null
    form.isActive = false
    ElMessage.success("已复制角色，请修改后保存")
  }

  async function saveChar() {
    if (!form.name.trim()) { ElMessage.warning("请输入角色名称"); return }
    saving.value = true
    try {
      const payload = { ...form }
      if (selected.value?.id) {
        await put(`/api/characters/${selected.value.id}`, payload)
        ElMessage.success("保存成功")
      } else {
        const created = await post<any>("/api/characters", payload)
        ElMessage.success("创建成功")
        if (created?.id) { selected.value = { ...payload, id: created.id }; selectedId.value = created.id }
      }
      await fetchChars()
      if (selectedId.value) {
        const refreshed = characters.value.find((c: any) => c.id === selectedId.value)
        if (refreshed) selectChar(refreshed)
      }
      refreshHealth()
    } catch {} finally { saving.value = false }
  }

  function resetPrompt() {
    ElMessageBox.confirm("恢复默认提示词？当前内容将丢失。", "提示", { type: "warning" })
      .then(() => { form.systemPrompt = ""; ElMessage.success("已恢复") })
      .catch(() => {})
  }

  function resetBounds() {
    ElMessageBox.confirm("恢复默认边界规则？", "提示", { type: "warning" })
      .then(() => { form.boundaryRules = ""; ElMessage.success("已恢复") })
      .catch(() => {})
  }

  function selectCharById(id: string) {
    const found = characters.value.find(c => c.id === id)
    if (found) selectChar(found)
  }

  async function delChar(c: any) {
    if (c.isActive) {
      const others = characters.value.filter(x => x.id !== c.id)
      if (others.length === 0) { ElMessage.warning("不能删除唯一的角色"); return }
    }
    await ElMessageBox.confirm(`确定删除角色「${c.name}」？此操作不可撤销。`, "确认删除", { type: "warning", confirmButtonText: "删除", confirmButtonClass: "el-button--danger" })
    try {
      await del(`/api/characters/${c.id}`)
      ElMessage.success("已删除")
      if (selectedId.value === c.id) { selected.value = null; selectedId.value = "" }
      await fetchChars()
      if (selectedId.value) {
        const refreshed = characters.value.find((c: any) => c.id === selectedId.value)
        if (refreshed) selectChar(refreshed)
      }
      refreshHealth()
    } catch {}
  }

  return {
    templates, showTemplateDialog, templateLoading,
    characters, selected, selectedId, activeTab, saving,
    showFullPrompt, showFullBounds,
    form, hasOtherActive,
    fetchTemplates, fetchChars,
    selectChar, createNew, createFromTemplate,
    copyChar, saveChar, resetPrompt, resetBounds, delChar,
    selectCharById,
  }
}
