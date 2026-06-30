// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "../../../composables/useApi"

export function useCharacterImportExport() {
  const { get, post } = useApi()

  const exportingPack = ref(false)
  const showImportDialog = ref(false)
  const importPackName = ref("")
  const importPreview = ref<any | null>(null)
  const importPreviewing = ref(false)
  const importConfirmText = ref("")
  const importing = ref(false)
  const packHistory = ref<any[]>([])

  async function exportPack(characterId: string, characterName: string) {
    if (!characterId) return
    exportingPack.value = true
    try {
      await post<any>(`/api/characters/${characterId}/export-pack`)
      ElMessage.success(`已导出角色包: ${characterName}`)
    } catch (err: any) {
      ElMessage.error("导出失败: " + (err.response?.data?.message || err.message))
    } finally {
      exportingPack.value = false
    }
  }

  async function previewImport() {
    if (!importPackName.value.trim()) return
    importPreviewing.value = true
    importPreview.value = null
    try {
      const d = await get<any>("/api/characters/packs/preview", { name: importPackName.value })
      importPreview.value = d
    } catch (err: any) {
      ElMessage.error("预览失败: " + (err.response?.data?.message || err.message))
    } finally {
      importPreviewing.value = false
    }
  }

  async function confirmImport(): Promise<any> {
    if (importConfirmText.value !== "确认导入") return null
    importing.value = true
    try {
      const d = await post<any>("/api/characters/packs/import", {
        name: importPackName.value,
        confirmText: "确认导入",
      })
      ElMessage.success("导入成功")
      importPreview.value = null
      importConfirmText.value = ""
      showImportDialog.value = false
      await loadPackHistory()
      return d
    } catch (err: any) {
      ElMessage.error("导入失败: " + (err.response?.data?.message || err.message))
      return null
    } finally {
      importing.value = false
    }
  }

  async function loadPackHistory() {
    try {
      packHistory.value = await get<any[]>("/api/characters/packs/history") || []
    } catch {
      packHistory.value = []
    }
  }

  function cancelImportPreview() {
    importPreview.value = null
    importConfirmText.value = ""
  }

  return {
    exportingPack,
    showImportDialog,
    importPackName,
    importPreview,
    importPreviewing,
    importConfirmText,
    importing,
    packHistory,
    exportPack,
    previewImport,
    confirmImport,
    loadPackHistory,
    cancelImportPreview,
  }
}
