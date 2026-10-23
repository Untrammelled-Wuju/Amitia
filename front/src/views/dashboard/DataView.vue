<template>
  <div>
    <el-alert v-if="usageNotice" :title="usageNotice" type="info" :closable="false" />
    <div class="data-cards">
      <div class="data-card">
        <div class="dc-icon blue">
          <el-icon :size="22"><ChatDotRound /></el-icon>
        </div>
        <div class="dc-body">
          <div class="dc-label">会话</div>
          <div class="dc-value">{{ totalConvs }}</div>
        </div>
      </div>
      <div class="data-card">
        <div class="dc-icon orange">
          <el-icon :size="22"><UserFilled /></el-icon>
        </div>
        <div class="dc-body">
          <div class="dc-label">角色</div>
          <div class="dc-value">{{ totalChars }}</div>
        </div>
      </div>
    </div>

    <div class="section-header-row">
      <span class="section-header">趋势图表</span>
      <el-button
        class="refresh-btn"
        :loading="refreshing"
        @click="manualRefresh"
        size="small"
        text
      >
        <el-icon :size="16"><Refresh /></el-icon>
      </el-button>
    </div>
    <div v-if="!dataLoaded" class="charts-loading">
      <el-icon class="is-loading" :size="28"><Loading /></el-icon>
    </div>
    <div v-else class="charts-row">
      <div v-for="m in chartMetrics" :key="m.key" class="chart-card">
        <div class="chart-card-title">{{ m.label }}</div>
        <v-chart
          :ref="
            (el: any) => {
              if (el) chartRefs[m.key] = el;
            }
          "
          class="chart-mini"
          :option="chartOptions[m.key]"
        />
      </div>
    </div>

    <div class="section-header" style="margin-top: 22px">数据概览</div>
    <div class="stat-cards">
      <div v-for="s in statMetrics" :key="s.key" class="stat-card">
        <div class="stat-color" :style="{ background: s.color }"></div>
        <div class="stat-body">
          <div class="stat-label">{{ s.label }}</div>
          <div class="stat-main">
            <span class="stat-total">{{ fmtNum(statTotals[s.key]) }}</span>
            <span class="stat-unit">{{ s.unit }}</span>
          </div>
          <div class="stat-sub">
            日均 {{ fmtNum(statAvgs[s.key]) }} {{ s.unit }}
          </div>
        </div>
      </div>
    </div>

    <template v-if="feedbackRecent.length">
      <div class="section-header" style="margin-top: 22px">最近消息反馈</div>
      <div class="feedback-types">
        <el-tag v-for="(count, type) in feedbackByType" :key="type" size="small" type="info">
          {{ feedbackTypeLabel(type) }} {{ count }}
        </el-tag>
      </div>
      <el-table :data="feedbackRecent" size="small" class="feedback-table">
        <el-table-column label="时间" min-width="150">
          <template #default="{ row }">{{ formatFeedbackTime(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="类型" width="100">
          <template #default="{ row }">{{ feedbackTypeLabel(row.feedbackType) }}</template>
        </el-table-column>
        <el-table-column prop="messageId" label="消息" min-width="220" show-overflow-tooltip />
        <el-table-column prop="reason" label="原因" min-width="220" show-overflow-tooltip />
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button link type="danger" @click="removeFeedback(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  ChatDotRound,
  UserFilled,
  Refresh,
  Loading,
} from "@element-plus/icons-vue";
import { useDashboardData } from "./composables/useDashboardData";
import { useTheme } from "@/composables/useTheme";
import VChart from "vue-echarts";
import { use } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { BarChart } from "echarts/charts";
import { TooltipComponent, GridComponent } from "echarts/components";

import { usageMetricSummary } from "@/runtime/usage-view";
import { ownedManagementRequest } from "@/runtime/owned-chat-management";

use([CanvasRenderer, BarChart, TooltipComponent, GridComponent]);

const { totalConvs, totalChars, dailyData, usageNotice, usageBusinessReady, feedbackTotal, feedbackByType, feedbackRecent, refreshAll } = useDashboardData();
const { resolvedMode } = useTheme();

const chartRefs: Record<string, any> = {};
const refreshing = ref(false);
const dataLoaded = ref(false);

const chartMetrics = [
  { key: "messages", label: "消息", color: "var(--ac-color-primary)" },
  { key: "modelCalls", label: "模型调用", color: "var(--ac-color-success)" },
  { key: "tokens", label: "Token 消耗", color: "var(--ac-color-danger)" },
];

const statMetrics = [
  {
    key: "conversations",
    label: "会话",
    color: "var(--ac-color-primary)",
    unit: "个",
  },
  {
    key: "memories",
    label: "记忆",
    color: "var(--ac-color-warning)",
    unit: "条",
  },
  {
    key: "feedback",
    label: "Feedback",
    color: "var(--ac-color-success)",
    unit: "条",
  },
];

function fmtNum(n: number | null | undefined): string {
  if (n === undefined || n === null) return "不可用";
  if (n >= 10000) return (n / 10000).toFixed(1) + "w";
  if (n >= 1000) return (n / 1000).toFixed(1) + "k";
  return String(n);
}

function feedbackTypeLabel(value: string): string {
  const labels: Record<string, string> = {
    good: "有帮助",
    too_long: "过长",
    too_cold: "过于冷淡",
    too_exaggerated: "过度夸张",
    not_understand: "未理解",
    unsafe: "不安全",
    other: "需改进",
  };
  return labels[value] || value || "未知";
}

function formatFeedbackTime(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN");
}

async function removeFeedback(row: any) {
  await ElMessageBox.confirm("确定删除这条消息反馈？", "删除反馈", { type: "warning" });
  const result = await ownedManagementRequest<{ deleted?: boolean }>("feedback/delete", { feedbackId: row.id }, "post", row.roleId);
  if (result.deleted !== true) throw new Error("反馈删除尚未确认");
  ElMessage.success("反馈已删除");
  await refreshAll();
}

function readCSSVar(name: string): string {
  if (typeof document === "undefined") return "";
  const val = getComputedStyle(document.documentElement)
    .getPropertyValue(name)
    .trim();
  return val;
}

function resolveColor(color: string): string {
  if (!color.startsWith("var(") || !color.endsWith(")")) return color;
  return readCSSVar(color.slice(4, -1).trim()) || readCSSVar("--tp-primary");
}

function safeArray(v: any): any[] {
  return Array.isArray(v) ? v : [];
}

const statTotals = computed(() => {
  const totals = Object.fromEntries(statMetrics.map(s => [s.key,
    usageMetricSummary(safeArray(dailyData.value), s.key, usageBusinessReady.value).total,
  ]));
  totals.feedback = feedbackTotal.value;
  return totals;
});

const statAvgs = computed(() => Object.fromEntries(statMetrics.map(s => [s.key,
  usageMetricSummary(safeArray(dailyData.value), s.key, usageBusinessReady.value).average,
])));

const chartOptions = computed(() => {
  resolvedMode.value;
  const result: Record<string, any> = {};
  for (const m of chartMetrics) {
    const arr = safeArray(dailyData.value);
    const xData = arr.length
      ? arr.map((d: any) => (d.date || "").slice(5))
      : [];
    const data = arr.map((d: any) => typeof d[m.key] === "number" ? d[m.key] : null);
    result[m.key] = {
      tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
      grid: { left: 0, right: 8, top: 8, bottom: 20 },
      xAxis: {
        type: "category",
        data: xData,
        axisLabel: {
          color: readCSSVar("--console-text-muted"),
          fontSize: 10,
          rotate: 45,
        },
        axisLine: { lineStyle: { color: readCSSVar("--console-border") } },
      },
      yAxis: {
        type: "value",
        splitLine: {
          lineStyle: { color: readCSSVar("--console-border-soft") },
        },
        axisLabel: { color: readCSSVar("--console-text-muted"), fontSize: 10 },
      },
      series: [
        {
          type: "bar",
          data,
          itemStyle: {
            color: resolveColor(m.color),
            borderRadius: [3, 3, 0, 0],
          },
          barMaxWidth: 14,
        },
      ],
    };
  }
  return result;
});

function handleResize() {
  Object.values(chartRefs).forEach((ref: any) => ref?.chart?.resize());
}

async function manualRefresh() {
  if (refreshing.value) return;
  refreshing.value = true;
  await refreshAll();
  refreshing.value = false;
}

onMounted(async () => {
  window.addEventListener("resize", handleResize);
  await nextTick();
  dataLoaded.value = true;
});

onUnmounted(() => {
  window.removeEventListener("resize", handleResize);
});
</script>

<style scoped>
.data-cards {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
  margin-bottom: 22px;
}

.data-card {
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 90px;
  padding: 16px 18px;
  border-radius: 14px;
  background: var(--ac-color-surface);
  border: 1px solid var(--console-border);
  box-shadow: none;
}

.dc-icon {
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
}

.dc-icon.blue {
  color: var(--tp-info);
  background: var(--console-blue-soft);
}
.dc-icon.orange {
  color: var(--ac-color-warning);
  background: var(--console-orange-soft);
}

.dc-body {
  flex: 1;
  min-width: 0;
}
.dc-label {
  font-size: 16px;
  font-weight: 650;
  color: var(--console-text);
  margin-bottom: 8px;
}
.dc-value {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 3px 10px;
  border-radius: 7px;
  background: var(--console-value-ok-bg);
  color: var(--ac-color-success);
  font-size: 13px;
  font-weight: 400;
}

.section-header {
  font-size: 15px;
  font-weight: 700;
  color: var(--console-text);
  margin-bottom: 12px;
}

.feedback-table {
  margin-top: 10px;
}

.feedback-types {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.section-header-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.section-header-row .section-header {
  margin-bottom: 0;
}

.refresh-btn {
  color: var(--console-text-muted);
  transition: color 0.2s;
}

.refresh-btn:hover {
  color: var(--ac-color-primary);
}

.charts-loading {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100px;
  color: var(--console-text-muted);
}

.charts-row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.chart-card {
  background: var(--ac-color-surface);
  border: 1px solid var(--console-border);
  border-radius: 10px;
  padding: 14px 12px 10px;
  box-shadow: none;
}

.chart-card-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--console-text);
  margin-bottom: 2px;
  padding: 0 2px;
}

.chart-mini {
  width: 100%;
  height: 220px;
}

.stat-cards {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.stat-card {
  display: flex;
  gap: 14px;
  padding: 18px 16px;
  border-radius: 10px;
  background: var(--ac-color-surface);
  border: 1px solid var(--console-border);
  box-shadow: none;
}

.stat-color {
  flex-shrink: 0;
  width: 4px;
  border-radius: 2px;
}

.stat-body {
  flex: 1;
  min-width: 0;
}

.stat-label {
  font-size: 14px;
  font-weight: 650;
  color: var(--console-text-secondary);
  margin-bottom: 6px;
}

.stat-main {
  display: flex;
  align-items: baseline;
  gap: 4px;
  margin-bottom: 4px;
}

.stat-total {
  font-size: 26px;
  font-weight: 800;
  color: var(--console-text);
  line-height: 1;
}

.stat-unit {
  font-size: 13px;
  color: var(--console-text-muted);
}

.stat-sub {
  font-size: 12px;
  color: var(--console-text-muted);
}

@media (max-width: 900px) {
  .charts-row,
  .stat-cards {
    grid-template-columns: 1fr;
  }
}
</style>
