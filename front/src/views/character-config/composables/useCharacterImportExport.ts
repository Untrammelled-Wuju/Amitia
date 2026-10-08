// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { apiClient, useApi } from "../../../composables/useApi";
import { roleAuthorityConfig } from "../../../runtime/role-authority";

export function useCharacterImportExport() {
  const { get, postUpload } = useApi();

  const exportingPack = ref(false);
  const showImportDialog = ref(false);
  const importPackName = ref("");
  const importPreview = ref<any | null>(null);
  const importPreviewing = ref(false);
  const importConfirmText = ref("");
  const importing = ref(false);
  const packHistory = ref<any[]>([]);
  const selectedFile = ref<File | null>(null);
  const previewAuthority = ref("");
  let previewFile: File | null = null;

  async function exportPack(characterId: string, characterName: string) {
    if (!characterId) return;
    exportingPack.value = true;
    try {
      const response = await apiClient.get(
        `/api/characters/${characterId}/export-card`,
        {
          params: { format: "v3_charx", download: "true" },
          responseType: "blob",
        },
      );
      const blob = response.data instanceof Blob
        ? response.data
        : new Blob([response.data], { type: "application/octet-stream" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `${characterName || "character"}.charx`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      ElMessage.success(`已导出角色卡: ${characterName}`);
    } catch (err: any) {
      ElMessage.error(
        "导出失败: " + (err.response?.data?.message || err.message),
      );
    } finally {
      exportingPack.value = false;
    }
  }

  async function previewImport() {
    if (!selectedFile.value) {
      ElMessage.warning("请先选择角色卡片文件");
      return;
    }
    importPreviewing.value = true;
    importPreview.value = null;
    previewAuthority.value = "";
    previewFile = null;
    try {
      const file = selectedFile.value;
      const d = await postUpload<any>(
        "/api/characters/import-card/preview",
        file,
      );
      if (selectedFile.value !== file) return;
      previewAuthority.value = d?.roleAuthority || "";
      previewFile = file;
      importPreview.value = d?.preview
        ? {
            ...d.preview,
            format: d.format || d.preview.format,
            sourceHash: d.sourceHash,
          }
        : d;
    } catch (err: any) {
      ElMessage.error(
        "预览失败: " + (err.response?.data?.message || err.message),
      );
    } finally {
      importPreviewing.value = false;
    }
  }

  async function confirmImport(): Promise<any> {
    if (importing.value) return null;
    if (importConfirmText.value !== "确认导入") return null;
    if (!selectedFile.value) {
      ElMessage.warning("请先选择角色卡片文件");
      return null;
    }
    importing.value = true;
    try {
      if (selectedFile.value !== previewFile || !importPreview.value) throw new Error("请先重新预览角色卡");
      const d = await postUpload<any>(
        "/api/characters/import-card/confirm",
        selectedFile.value,
        "card",
        roleAuthorityConfig(previewAuthority.value),
      );
      ElMessage.success("导入成功");
      importPreview.value = null;
      importConfirmText.value = "";
      selectedFile.value = null;
      showImportDialog.value = false;
      await loadPackHistory();
      return d;
    } catch (err: any) {
      ElMessage.error(
        "导入失败: " + (err.response?.data?.message || err.message),
      );
      return null;
    } finally {
      importing.value = false;
    }
  }

  async function loadPackHistory() {
    try {
      packHistory.value =
        (await get<any[]>("/api/characters/packs/history")) || [];
    } catch {
      packHistory.value = [];
    }
  }

  function cancelImportPreview() {
    previewAuthority.value = "";
    previewFile = null;
    importPreview.value = null;
    importConfirmText.value = "";
    selectedFile.value = null;
  }

  function setSelectedFile(file: File | null) {
    if (selectedFile.value !== file) {
      importPreview.value = null;
      previewAuthority.value = "";
      previewFile = null;
    }
    selectedFile.value = file;
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
    selectedFile,
    exportPack,
    previewImport,
    confirmImport,
    loadPackHistory,
    cancelImportPreview,
    setSelectedFile,
  };
}
