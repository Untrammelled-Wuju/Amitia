// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, reactive } from "vue";
import { ElMessage } from "element-plus";
import { ownedManagementRequest } from "@/runtime/owned-chat-management";

export function useChatCleanup() {
  const form = reactive({
    beforeDate: "" as string,
    olderThanDays: null as number | null,
    channels: [] as string[],
    sources: [] as string[],
    includeMemories: false,
  });

  const previewLoading = ref(false);
  const previewResult = ref<any>(null);
  const confirmText = ref("");
  const confirmLoading = ref(false);
  const cleanupResult = ref<any>(null);

  async function previewCleanup() {
    previewLoading.value = true;
    previewResult.value = null;
    cleanupResult.value = null;
    confirmText.value = "";
    try {
      const payload: any = {};
      if (form.beforeDate) payload.beforeDate = form.beforeDate;
      if (form.olderThanDays) payload.olderThanDays = form.olderThanDays;
      if (form.channels.length > 0) payload.channels = form.channels;
      if (form.sources.length > 0) payload.sources = form.sources;
      payload.includeMemories = form.includeMemories;

      const d = await ownedManagementRequest("cleanup/preview", payload, "post");
      previewResult.value = d;
      ElMessage.success(
        `预览完成：${d.conversationCount} 个会话，${d.messageCount} 条消息`,
      );
    } catch (err: any) {
      ElMessage.error(
        "预览失败: " + (err.response?.data?.message || err.message),
      );
    } finally {
      previewLoading.value = false;
    }
  }

  async function executeCleanup() {
    if (confirmText.value !== "确认清理" || !previewResult.value?.previewId)
      return;
    confirmLoading.value = true;
    try {
      const d = await ownedManagementRequest("cleanup/confirm", {
        previewId: previewResult.value.previewId,
        confirmText: "确认清理",
        requestId: crypto.randomUUID(),
        characterId: previewResult.value.executionScope.roleId,
        expectedExecutionScope: previewResult.value.executionScope,
      }, "post");
      cleanupResult.value = d;
      ElMessage.success("清理完成");
    } catch (err: any) {
      ElMessage.error(
        "清理失败: " + (err.response?.data?.message || err.message),
      );
    } finally {
      confirmLoading.value = false;
    }
  }

  return {
    form,
    previewLoading,
    previewResult,
    confirmText,
    confirmLoading,
    cleanupResult,
    previewCleanup,
    executeCleanup,
  };
}
