<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <section class="history-panel" aria-label="执行历史">
    <header class="panel-toolbar">
      <div class="header-actions">
        <slot name="toolbar" />
        <el-select
          v-model="filterState"
          size="small"
          aria-label="筛选执行结果"
          @change="
            () => {
              page = 1;
              fetchItems();
            }
          "
          ><el-option label="全部结果" value="" /><el-option
            v-for="(label, state) in stateLabels"
            :key="state"
            :label="label"
            :value="state" /></el-select
        >
      </div>
    </header>
    <el-table :data="items" v-loading="loading" class="history-table" row-key="id">
      <el-table-column type="expand"
        ><template #default="{ row }"
          ><div class="history-expanded">
            <el-descriptions :column="1"
              ><el-descriptions-item label="触发类型">{{ row.triggerType }}</el-descriptions-item
              ><el-descriptions-item label="优先级">{{ row.priority }}</el-descriptions-item
              ><el-descriptions-item label="渠道">{{ row.channel || "web" }}</el-descriptions-item
              ><el-descriptions-item label="重试次数">{{ row.attemptCount }}</el-descriptions-item
              ><el-descriptions-item label="执行说明">{{ row.reason || "—" }}</el-descriptions-item
              ><el-descriptions-item v-if="row.lastError" label="错误信息"
                ><span class="error-text">{{ row.lastError }}</span></el-descriptions-item
              ><el-descriptions-item label="投递明细"
                ><div
                  v-for="(target, index) in row.result?.targets || []"
                  :key="index"
                  class="delivery-target"
                >
                  <span>{{ target.kind }} · {{ target.id }}</span
                  ><el-tag size="small" type="info">{{ target.state }}</el-tag
                  ><span v-if="target.error" class="error-text">{{ target.error }}</span>
                </div>
                <span v-if="!row.result?.targets?.length">暂无投递回执</span></el-descriptions-item
              ></el-descriptions
            >
          </div></template
        ></el-table-column
      >
      <el-table-column label="任务" prop="title" min-width="220" show-overflow-tooltip />
      <el-table-column label="执行结果" width="120"
        ><template #default="{ row }"
          ><el-tag :type="stateType(row.state)" round>{{ stateLabel(row.state) }}</el-tag></template
        ></el-table-column
      >
      <el-table-column label="说明" min-width="200" show-overflow-tooltip
        ><template #default="{ row }"
          ><span :class="{ 'error-text': row.lastError }">{{
            row.lastError || row.reason || "—"
          }}</span></template
        ></el-table-column
      >
      <el-table-column label="记录时间" width="190"
        ><template #default="{ row }">{{ fmtDate(row.createdAt) }}</template></el-table-column
      >
      <template #empty
        ><el-empty description="暂无执行记录" :image-size="64"
          ><span class="empty-hint">任务执行后，结果会显示在这里</span></el-empty
        ></template
      >
    </el-table>
    <footer class="history-footer">
      <span>共 {{ total }} 条记录</span>
      <div v-if="total > pageSize" class="pagination">
        <el-button
          :disabled="page <= 1 || loading"
          @click="
            page--;
            fetchItems();
          "
          >上一页</el-button
        ><span aria-live="polite">{{ page }} / {{ Math.ceil(total / pageSize) }}</span
        ><el-button
          :disabled="page * pageSize >= total || loading"
          @click="
            page++;
            fetchItems();
          "
          >下一页</el-button
        >
      </div>
    </footer>
  </section>
</template>
<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { ElMessage } from "element-plus";
import { request } from "../../../composables/request";
import type { TriggerHistory } from "../../../types";

const props = withDefaults(defineProps<{ kind?: "user_reminder" | "proactive_task" }>(), {
  kind: "user_reminder",
});

const items = ref<TriggerHistory[]>([]);
const loading = ref(false);
const filterState = ref("");
const page = ref(1);
const total = ref(0);
const pageSize = 20;

async function fetchItems() {
  loading.value = true;
  try {
    const params: any = { page: page.value, pageSize, kind: props.kind };
    if (filterState.value) params.state = filterState.value;
    const res: any = await request.get("/api/reminders/trigger-history", { params });
    if (
      !Array.isArray(res?.items) ||
      !Number.isInteger(res.total) ||
      res.items.some((item: any) => !item || typeof item.state !== "string" || !item.id)
    )
      throw new Error("执行历史响应无效");
    items.value = res.items;
    total.value = res.total;
  } catch (error: any) {
    items.value = [];
    total.value = 0;
    ElMessage.error(error?.message || "执行历史加载失败");
  } finally {
    loading.value = false;
  }
}

function stateType(s: string) {
  const map: Record<string, string> = {
    waiting: "warning",
    running: "primary",
    completed: "success",
    partial: "warning",
    blocked: "danger",
    skipped: "info",
    failed: "danger",
    cancelled: "info",
  };
  return map[s] || "info";
}

const stateLabels: Record<string, string> = {
  waiting: "待处理",
  running: "执行中",
  completed: "已完成",
  partial: "部分完成",
  blocked: "已阻断",
  skipped: "已跳过",
  failed: "失败",
  cancelled: "已取消",
};
function stateLabel(s: string) {
  return stateLabels[s] || s;
}

function fmtDate(d: string) {
  if (!d) return "";
  try {
    return new Date(d).toLocaleString("zh-CN");
  } catch {
    return d;
  }
}

defineExpose({ refresh: fetchItems });
let refreshTimer: ReturnType<typeof setInterval> | undefined;
onMounted(() => {
  fetchItems();
  refreshTimer = setInterval(() => {
    if (!loading.value) void fetchItems();
  }, 5000);
});
onUnmounted(() => clearInterval(refreshTimer));
</script>

<style scoped>
.panel-toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  min-height: 32px;
  margin-bottom: 10px;
}
.header-actions,
.pagination {
  display: flex;
  align-items: center;
  gap: 12px;
}
.header-actions .el-select {
  width: 144px;
}
.history-table {
  --el-table-header-bg-color: var(--tp-panel-soft);
  --el-table-border-color: var(--tp-border-light);
  font-size: 14px;
}
.history-table :deep(th.el-table__cell) {
  height: 40px;
  font-weight: 500;
  color: var(--tp-text-secondary);
}
.history-table :deep(td.el-table__cell) {
  padding: 10px 0;
}
.history-expanded {
  padding: 12px 24px;
  background: var(--tp-panel-soft);
  overflow-wrap: anywhere;
}
.history-expanded :deep(.el-descriptions__body) {
  background: transparent;
}
.delivery-target {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}
.error-text {
  color: var(--tp-danger);
}
.history-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 12px;
  color: var(--tp-text-secondary);
  font-size: 13px;
}
.empty-hint {
  color: var(--tp-text-secondary);
  font-size: 13px;
}
@media (max-width: 760px) {
  .header-actions {
    width: 100%;
    flex-wrap: wrap;
  }
  .history-footer {
    flex-wrap: wrap;
  }
}
</style>
