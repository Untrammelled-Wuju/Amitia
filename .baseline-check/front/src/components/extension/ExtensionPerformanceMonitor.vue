<script setup lang="ts">
import { computed } from "vue";
import {
  useExtensionPerformance,
  type PerformanceMetric,
} from "@/composables/useExtensionPerformance";

const { metrics, clearMetrics, checkBudget } = useExtensionPerformance();

const hasMetrics = computed(() => metrics.value.length > 0);

function rowClassName({ row }: { row: PerformanceMetric }): string {
  const violations = checkBudget(row);
  return violations.length > 0 ? "extension-perf-monitor__row--warning" : "";
}

function formatTime(ms: number): string {
  return `${ms}ms`;
}

function violationText(metric: PerformanceMetric): string {
  const violations = checkBudget(metric);
  return violations.map((v) => v.message).join("；");
}
</script>

<template>
  <div class="extension-perf-monitor">
    <div class="extension-perf-monitor__header">
      <span class="extension-perf-monitor__title">扩展性能监控</span>
      <el-button
        size="small"
        type="danger"
        plain
        :disabled="!hasMetrics"
        @click="clearMetrics"
      >
        清理
      </el-button>
    </div>
    <el-table
      v-if="hasMetrics"
      :data="metrics"
      size="small"
      border
      :row-class-name="rowClassName"
    >
      <el-table-column prop="contributionId" label="贡献ID" min-width="160" show-overflow-tooltip />
      <el-table-column prop="slotId" label="槽位" width="140" show-overflow-tooltip />
      <el-table-column label="加载时间" width="100">
        <template #default="{ row }">
          {{ formatTime(row.loadTimeMs) }}
        </template>
      </el-table-column>
      <el-table-column label="渲染时间" width="100">
        <template #default="{ row }">
          {{ row.renderTimeMs !== undefined ? formatTime(row.renderTimeMs) : "-" }}
        </template>
      </el-table-column>
      <el-table-column prop="actionCount" label="操作次数" width="90" />
      <el-table-column prop="errorCount" label="错误次数" width="90" />
      <el-table-column label="预算警告" min-width="200">
        <template #default="{ row }">
          <span class="extension-perf-monitor__warning-text">
            {{ violationText(row) || "正常" }}
          </span>
        </template>
      </el-table-column>
    </el-table>
    <div v-else class="extension-perf-monitor__empty">
      暂无性能数据
    </div>
  </div>
</template>

<style scoped>
.extension-perf-monitor {
  width: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border-radius: 8px;
  background: var(--amitia-color-surface, transparent);
  color: var(--amitia-color-text, inherit);
}
.extension-perf-monitor__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.extension-perf-monitor__title {
  font-size: 14px;
  font-weight: 600;
}
.extension-perf-monitor__empty {
  padding: 24px;
  font-size: 12px;
  text-align: center;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.6));
  border: 1px dashed var(--amitia-color-border, rgba(127, 127, 127, 0.25));
  border-radius: 6px;
}
.extension-perf-monitor__warning-text {
  font-size: 12px;
  word-break: break-word;
}
:deep(.extension-perf-monitor__row--warning) {
  background: rgba(220, 160, 40, 0.08) !important;
}
:deep(.extension-perf-monitor__row--warning td) {
  color: rgb(160, 100, 20);
}
</style>
