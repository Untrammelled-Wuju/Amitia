// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, reactive } from "vue";
import { ElMessage } from "element-plus";
import { ownedManagementRequest } from "@/runtime/owned-chat-management";

export function useDatabaseHealth() {
  const stats = reactive({
    totalConversations: 0,
    totalMessages: 0,
    dbSize: "计算中...",
  });

  const vacuumLoading = ref(false);
  const vacuumResult = ref<any>(null);

  async function loadStats() {
    try {
      const d = await ownedManagementRequest("stats");
      stats.totalConversations = d.totalConversations;
      stats.totalMessages = d.totalMessages;
      stats.dbSize = d.dbSizeFormatted || "--";
    } catch {
      stats.totalConversations = 0;
      stats.totalMessages = 0;
      stats.dbSize = "--";
    }
  }

  async function runVacuum() {
    vacuumLoading.value = true;
    try {
      const d = await ownedManagementRequest("cleanup/vacuum", { requestId: crypto.randomUUID() }, "post");
      vacuumResult.value = d;
      ElMessage.success(`优化完成，释放 ${d.freedFormatted}`);
    } catch (err: any) {
      ElMessage.error(
        "优化失败: " + (err.response?.data?.message || err.message),
      );
    } finally {
      vacuumLoading.value = false;
    }
  }

  return {
    stats,
    vacuumLoading,
    vacuumResult,
    loadStats,
    runVacuum,
  };
}
