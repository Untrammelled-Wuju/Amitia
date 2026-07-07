<template>
  <div>
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

    <div class="section-header">趋势图表</div>
    <div class="charts-row">
      <div v-for="m in chartMetrics" :key="m.key" class="chart-card">
        <div class="chart-card-title">{{ m.label }}</div>
        <v-chart :ref="(el: any) => { if (el) chartRefs[m.key] = el }" class="chart-mini" :option="chartOption(m.key)" />
      </div>
    </div>

    <div class="section-header" style="margin-top: 22px;">数据概览</div>
    <div class="stat-cards">
      <div v-for="s in statMetrics" :key="s.key" class="stat-card">
        <div class="stat-color" :style="{ background: s.color }"></div>
        <div class="stat-body">
          <div class="stat-label">{{ s.label }}</div>
          <div class="stat-main">
            <span class="stat-total">{{ fmtNum(statTotal(s.key)) }}</span>
            <span class="stat-unit">{{ s.unit }}</span>
          </div>
          <div class="stat-sub">日均 {{ fmtNum(statAvg(s.key)) }} {{ s.unit }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue"
import { ChatDotRound, UserFilled } from "@element-plus/icons-vue"
import { useApi } from "../../composables/useApi"
import VChart from "vue-echarts"
import { use } from "echarts/core"
import { CanvasRenderer } from "echarts/renderers"
import { BarChart } from "echarts/charts"
import { TooltipComponent, GridComponent } from "echarts/components"

use([CanvasRenderer, BarChart, TooltipComponent, GridComponent])

const { get } = useApi()
const totalConvs = ref(0)
const totalChars = ref(0)
const dailyData = ref<any[]>([])
const chartRefs: Record<string, any> = {}

const chartMetrics = [
  { key: "messages", label: "消息", color: "#4f7cff" },
  { key: "modelCalls", label: "模型调用", color: "#16a34a" },
  { key: "tokens", label: "Token 消耗", color: "#e11d48" },
]

const statMetrics = [
  { key: "conversations", label: "会话", color: "#7c3aed", unit: "个" },
  { key: "memories", label: "记忆", color: "#f97316", unit: "条" },
  { key: "feedback", label: "Feedback", color: "#0891b2", unit: "条" },
]

function fmtNum(n: number): string {
  if (n >= 10000) return (n / 10000).toFixed(1) + "w"
  if (n >= 1000) return (n / 1000).toFixed(1) + "k"
  return String(n)
}

function readCSSVar(name: string, fallback: string): string {
  if (typeof document === "undefined") return fallback
  const val = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return val || fallback
}

function safeArray(v: any): any[] {
  return Array.isArray(v) ? v : []
}

function statTotal(key: string): number {
  const arr = safeArray(dailyData.value)
  return arr.reduce((sum, d) => sum + (d[key] || 0), 0)
}

function statAvg(key: string): number {
  const arr = safeArray(dailyData.value)
  if (!arr.length) return 0
  return Math.round(arr.reduce((sum, d) => sum + (d[key] || 0), 0) / arr.length)
}

function chartOption(key: string) {
  const arr = safeArray(dailyData.value)
  const m = [...chartMetrics, ...statMetrics].find((m) => m.key === key)!
  const xData = arr.length ? arr.map((d: any) => (d.date || "").slice(5)) : []
  const data = arr.map((d: any) => d[key] || 0)
  return {
    tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
    grid: { left: 0, right: 8, top: 8, bottom: 20 },
    xAxis: { type: "category", data: xData, axisLabel: { color: readCSSVar("--console-text-muted", "#667085"), fontSize: 10, rotate: 45 }, axisLine: { lineStyle: { color: readCSSVar("--console-border", "#E6EBF4") } } },
    yAxis: { type: "value", splitLine: { lineStyle: { color: readCSSVar("--console-border-soft", "#EEF2F7") } }, axisLabel: { color: readCSSVar("--console-text-muted", "#667085"), fontSize: 10 } },
    series: [{
      type: "bar",
      data,
      itemStyle: { color: m.color, borderRadius: [3, 3, 0, 0] },
      barMaxWidth: 14,
    }],
  }
}

function handleResize() {
  Object.values(chartRefs).forEach((ref: any) => ref?.chart?.resize())
}

async function fetchData() {
  try {
    const chars = await get<any[]>("/api/characters")
    if (Array.isArray(chars) && chars.length > 0) totalChars.value = chars.length
  } catch {}

  try {
    const data = await get<any>("/api/chats/stats")
    if (data && typeof data.totalConversations === "number") totalConvs.value = data.totalConversations
  } catch {}

  try {
    const periodic = await get<any>("/api/usage/periodic")
    if (periodic) {
      const d = periodic.daily
      if (Array.isArray(d) && d.length > 0) dailyData.value = d
    }
  } catch {}
}

onMounted(() => {
  fetchData()
  window.addEventListener("resize", handleResize)
})

onUnmounted(() => {
  window.removeEventListener("resize", handleResize)
})
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
  background: var(--console-card);
  border: 1px solid var(--console-border);
  box-shadow: var(--console-shadow);
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

.dc-icon.blue { color: #2563eb; background: var(--console-blue-soft); }
.dc-icon.orange { color: #f97316; background: var(--console-orange-soft); }

.dc-body { flex: 1; min-width: 0; }
.dc-label { font-size: 16px; font-weight: 650; color: var(--console-text); margin-bottom: 8px; }
.dc-value {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 3px 10px;
  border-radius: 7px;
  background: var(--console-value-ok-bg);
  color: #12a150;
  font-size: 13px;
  font-weight: 400;
}

.section-header { font-size: 15px; font-weight: 700; color: var(--console-text); margin-bottom: 12px; }

.charts-row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.chart-card {
  background: var(--console-card);
  border: 1px solid var(--console-border);
  border-radius: 10px;
  padding: 14px 12px 10px;
  box-shadow: var(--console-shadow);
}

.chart-card-title { font-size: 14px; font-weight: 700; color: var(--console-text); margin-bottom: 2px; padding: 0 2px; }

.chart-mini { width: 100%; height: 220px; }

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
  background: var(--console-card);
  border: 1px solid var(--console-border);
  box-shadow: var(--console-shadow);
}

.stat-color {
  flex-shrink: 0;
  width: 4px;
  border-radius: 2px;
}

.stat-body { flex: 1; min-width: 0; }

.stat-label { font-size: 14px; font-weight: 650; color: var(--console-text-secondary); margin-bottom: 6px; }

.stat-main { display: flex; align-items: baseline; gap: 4px; margin-bottom: 4px; }

.stat-total { font-size: 26px; font-weight: 800; color: var(--console-text); line-height: 1; }

.stat-unit { font-size: 13px; color: var(--console-text-muted); }

.stat-sub { font-size: 12px; color: var(--console-text-muted); }

@media (max-width: 900px) {
  .charts-row, .stat-cards { grid-template-columns: 1fr; }
}
</style>
