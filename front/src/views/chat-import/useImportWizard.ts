// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { parseSpeakerNames } from "./utils";
import { useOwnedMemoryManagement } from "@/composables/useOwnedMemoryManagement";
import {
  parseText,
  confirmImport,
  generateSummary,
  getBatchSummary,
  createConversationFromImport,
  fetchCharactersApi,
  fetchBatchesApi,
  fetchBatchDetailApi,
  deleteBatchApi,
} from "./api";

export function useImportWizard() {
  const router = useRouter();
  const ownedMemory = useOwnedMemoryManagement();

  const characters = ref<any[]>([]);
  const characterId = ref("");

  const rawText = ref("");
  const batchTitle = ref("");
  const parseFormat = ref("auto");
  const parsing = ref(false);
  const showSpeakerOptions = ref<string[]>([]);
  const userSpeakerInput = ref("");
  const assistantSpeakerInput = ref("");
  const defaultRole = ref("user");

  const parseResult = ref<any>(null);
  const editableItems = ref<any[]>([]);
  const confirming = ref(false);
  const genSummary = ref(false);
  const extractMemories = ref(false);

  const importedBatchId = ref("");
  const importedConvId = ref("");
  const importedOwnerConversationId = ref("");
  const importedExecutionScope = ref<any>(null);
  const genSummaryLoading = ref(false);
  const extractLoading = ref(false);
  const memCandidates = ref<any[]>([]);

  async function fetchCharacters() {
    try {
      const result = await fetchCharactersApi();
      characters.value = Array.isArray(result)
        ? result
        : result?.characters || result?.data || [];
    } catch (error) {
  console.error("[original-error]", error);
}
  }

  async function handleParse() {
    if (!rawText.value.trim()) return;
    parsing.value = true;
    try {
      const body: any = {
        rawText: rawText.value,
        format: parseFormat.value,
        title: batchTitle.value || undefined,
        defaultRole: defaultRole.value,
      };
      const userNames = parseSpeakerNames(userSpeakerInput.value);
      const assistantNames = parseSpeakerNames(assistantSpeakerInput.value);
      if (userNames.length > 0) body.userSpeakerNames = userNames;
      if (assistantNames.length > 0)
        body.assistantSpeakerNames = assistantNames;

      const result = await parseText(body);
      parseResult.value = result;
      editableItems.value = (result?.items || []).map(
        (item: any, idx: number) => ({
          ...item,
          lineNo: idx + 1,
          _sensitive:
            result?.sensitiveMatches?.some(
              (m: any) => m.lineNo === item.lineNo,
            ) || false,
        }),
      );
      importedBatchId.value = "";
      importedConvId.value = "";
      importedOwnerConversationId.value = "";
      importedExecutionScope.value = null;
    } catch (error) {
  console.error("[original-error]", error);
} finally {
      parsing.value = false;
    }
  }

  function onFileChange(file: any) {
    const reader = new FileReader();
    reader.onload = (e) => {
      rawText.value = (e.target?.result as string) || "";
      batchTitle.value = file.name.replace(/\.[^.]+$/, "");
      handleParse();
    };
    reader.readAsText(file.raw);
  }

  async function handleConfirm() {
    if (editableItems.value.length === 0) return;
    await ElMessageBox.confirm(
      `确认导入 ${editableItems.value.length} 条消息？将创建一个新的会话。`,
      "确认导入",
      { type: "warning", confirmButtonText: "确认" },
    );
    confirming.value = true;
    try {
      const result = await confirmImport({
        items: editableItems.value.map(
          ({ lineNo, _sensitive, ...item }) => item,
        ),
        title: batchTitle.value || "已导入的聊天",
        characterId: characterId.value,
        defaultRole: defaultRole.value,
        detectedFormat: parseResult.value?.detectedFormat,
      });
      importedBatchId.value = result?.batchId || "";
      if (!importedBatchId.value) throw new Error("后端未返回导入批次");
      await promoteImportedBatch();
      ElMessage.success(
        `已导入并加入设备聊天：${result?.messageCount || editableItems.value.length} 条消息`,
      );
      await fetchBatches();

      if (genSummary.value) await handleGenSummary();
      if (extractMemories.value) await handleExtractMemories();
    } catch (err: any) {
      if (err !== "cancel") ElMessage.error(err?.message || err || "导入失败");
    } finally {
      confirming.value = false;
    }
  }

  async function promoteImportedBatch() {
    if (!importedBatchId.value) throw new Error("导入批次不存在");
    if (importedConvId.value) return;
    const result = await createConversationFromImport(importedBatchId.value);
    if (!result?.conversationId || !result?.executionScope) throw new Error("设备聊天未返回正式会话");
    importedConvId.value = result.conversationId;
    importedOwnerConversationId.value = result.ownerConversationId;
    importedExecutionScope.value = result.executionScope;
  }

  async function handleGenSummary() {
    if (!importedBatchId.value) return;
    genSummaryLoading.value = true;
    try {
      await generateSummary(importedBatchId.value);
      await getBatchSummary(importedBatchId.value);
      ElMessage.success("摘要生成成功");
    } catch (err: any) {
      ElMessage.error(err?.message || "摘要生成失败");
    } finally {
      genSummaryLoading.value = false;
    }
  }

  async function handleExtractMemories() {
    if (!importedExecutionScope.value || !importedOwnerConversationId.value) return;
    extractLoading.value = true;
    try {
      const rows = await ownedMemory.write(
        importedExecutionScope.value,
        crypto.randomUUID(),
        {
          action: "generate",
          conversationId: importedOwnerConversationId.value,
          conversationOrigin: {
            ownerId: importedExecutionScope.value.resourceOwnerId,
            id: importedOwnerConversationId.value,
          },
        },
        true,
      );
      memCandidates.value = rows.map((row: any) => ({
        id: row.id,
        revision: row.revision,
        key: row.body?.key || "记忆",
        value: row.body?.content?.value || row.body?.content?.text || "",
        memoryType: row.body?.content?.memoryType || "",
        importance: row.body?.content?.importance || 0,
      }));
      ElMessage.success(`已提取 ${memCandidates.value.length} 条记忆候选`);
    } catch (err: any) {
      ElMessage.error(err?.message || "提取记忆候选失败");
    } finally {
      extractLoading.value = false;
    }
  }

  async function handleContinueChat() {
    try {
      await promoteImportedBatch();
      ElMessage.success("正在跳转聊天...");
      router.push({ path: "/chat", query: { conversationId: importedConvId.value, characterId: importedExecutionScope.value.roleId } });
    } catch (err: any) {
      ElMessage.error(err?.message || "创建会话失败");
    }
  }

  const batches = ref<any[]>([]);
  const batchTotal = ref(0);
  const batchPage = ref(1);
  const detailVisible = ref(false);
  const detailItems = ref<any[]>([]);

  async function fetchBatches() {
    try {
      const r = await fetchBatchesApi({ page: batchPage.value, pageSize: 20 });
      batches.value = r?.items || [];
      batchTotal.value = r?.total || 0;
    } catch (error) {
  console.error("[original-error]", error);
}
  }

  async function viewBatch(id: string) {
    try {
      const r = await fetchBatchDetailApi(id);
      detailItems.value = r?.items || r || [];
      detailVisible.value = true;
    } catch (error) {
  console.error("[original-error]", error);
}
  }

  async function delBatch(id: string) {
    await ElMessageBox.confirm("确定删除此批次？", "确认", { type: "warning" });
    try {
      await deleteBatchApi(id);
      ElMessage.success("已删除");
      await fetchBatches();
    } catch (error) {
  console.error("[original-error]", error);
}
  }

  async function promoteBatch(batch: any) {
    try {
      const result = await createConversationFromImport(batch.id, batch.roleId || "");
      await fetchBatches();
      ElMessage.success("已加入设备聊天");
      router.push({
        path: "/chat",
        query: {
          conversationId: result.conversationId,
          characterId: result.executionScope.roleId,
        },
      });
    } catch (err: any) {
      ElMessage.error(err?.message || "加入设备聊天失败");
    }
  }

  return {
    rawText,
    batchTitle,
    parseFormat,
    parsing,
    showSpeakerOptions,
    userSpeakerInput,
    assistantSpeakerInput,
    defaultRole,
    parseResult,
    editableItems,
    confirming,
    genSummary,
    extractMemories,
    importedBatchId,
    importedConvId,
    genSummaryLoading,
    extractLoading,
    memCandidates,
    characters,
    characterId,
    fetchCharacters,
    handleParse,
    onFileChange,
    handleConfirm,
    handleGenSummary,
    handleExtractMemories,
    handleContinueChat,
    batches,
    batchTotal,
    batchPage,
    detailVisible,
    detailItems,
    fetchBatches,
    viewBatch,
    delBatch,
    promoteBatch,
    router,
  };
}
