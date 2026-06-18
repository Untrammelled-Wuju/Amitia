<template>
  <div class="graph-page">
    <h2 class="page-title">记忆图谱</h2>
    <div class="graph-controls">
      <el-input v-model="searchId" placeholder="节点ID" size="small" style="width:140px" clearable />
      <el-select v-model="typeFilter" placeholder="节点类型" size="small" style="width:130px" clearable @change="applyFilter">
        <el-option label="全部类型" value="" />
        <el-option v-for="t in typeOptions" :key="t.value" :label="t.label" :value="t.value" />
      </el-select>
      <el-input v-model="labelKeyword" placeholder="搜索标签" size="small" style="width:140px" clearable @input="applyFilter" />
      <el-slider v-model="depth" :min="1" :max="4" show-input size="small" style="width:140px" />
      <span class="slider-label">跳数</span>
      <el-button size="small" type="primary" @click="fetchGraph">查询</el-button>
      <el-button size="small" @click="toggleFullscreen">{{ showFullscreen ? '退出全屏' : '全屏' }}</el-button>
    </div>
    <div class="graph-stats" v-if="stats">
      <span>节点: {{ filteredCount }} / {{ allNodes.length }}</span>
      <span>边: {{ filteredLinks.length }}</span>
      <span v-for="t in stats.byType" :key="t.entity_type" class="stat-type" :style="{color: typeColor(t.entity_type)}">{{ typeLabel(t.entity_type) }}: {{ t.count }}</span>
    </div>
    <div :class="['graph-container', { fullscreen: showFullscreen }]">
      <div class="fullscreen-bar" v-if="showFullscreen">
        <span class="fullscreen-title">记忆图谱 - 全屏</span>
        <el-button size="small" circle @click="toggleFullscreen" class="fullscreen-close">✕</el-button>
      </div>
      <div ref="chartRef" class="chart"></div>
    </div>
    <el-drawer v-model="detailVisible" title="节点详情" size="360px">
      <div v-if="selectedNode" class="node-detail">
        <p><strong>标签:</strong> {{ selectedNode.label }}</p>
        <p><strong>类型:</strong> {{ selectedNode.entity_type }}</p>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from "vue"
import { useApi } from "@/composables/useApi"

const { get } = useApi()
const chartRef = ref<HTMLElement>()
const depth = ref(2)
const searchId = ref("")
const typeFilter = ref("")
const labelKeyword = ref("")
const showFullscreen = ref(false)
const detailVisible = ref(false)
const selectedNode = ref<any>(null)
const stats = ref<any>(null)
const allNodes = ref<any[]>([])
const allLinks = ref<any[]>([])
let chartInstance: any = null

const typeLabels: Record<string,string> = { memory: "记忆", profile: "画像", episodic: "情景", worldbook: "世界书" }
const typeColors: Record<string,string> = { memory: "#409eff", profile: "#e6a23c", episodic: "#f56c6c", worldbook: "#67c23a" }
function typeLabel(t: string) { return typeLabels[t] || t }
function typeColor(t: string) { return typeColors[t] || "#909399" }
const typeOptions = computed(() => (stats.value?.byType || []).map((t: any) => ({ label: typeLabel(t.entity_type), value: t.entity_type })))

const filteredNodes = computed(() => {
  return allNodes.value.filter((n: any) => {
    if (typeFilter.value && n.entity_type !== typeFilter.value) return false
    if (labelKeyword.value && !(n.label || "").includes(labelKeyword.value)) return false
    return true
  })
})
const filteredNodeIds = computed(() => new Set(filteredNodes.value.map((n: any) => n.id)))
const filteredLinks = computed(() => allLinks.value.filter((l: any) => filteredNodeIds.value.has(l.source) && filteredNodeIds.value.has(l.target)))
const filteredCount = computed(() => filteredNodes.value.length)

function toggleFullscreen() {
  showFullscreen.value = !showFullscreen.value
  nextTick(() => {
    if (chartInstance) chartInstance.resize()
  })
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && showFullscreen.value) {
    toggleFullscreen()
  }
}

watch(showFullscreen, () => {
  nextTick(() => {
    if (chartInstance) chartInstance.resize()
  })
})

async function fetchGraph() {
  try {
    const s = await get<any>("/api/graph/stats?userId=default")
    stats.value = s
  } catch {}
  if (searchId.value) {
    try {
      const url = `/api/graph/node/${encodeURIComponent(searchId.value)}/neighbors?depth=${depth.value}&userId=default`
      const data = await get<any>(url)
      allNodes.value = data?.neighbors || data?.result || []
      allLinks.value = data?.edges || data?.links || []
      renderGraph()
    } catch {}
  }
}

function applyFilter() { renderGraph() }

async function renderGraph() {
  if (!chartRef.value) return
  if (!chartInstance) {
    chartInstance = (await import("echarts")).init(chartRef.value)
    chartInstance.on("click", (params: any) => {
      if (params.dataType === "node") {
        selectedNode.value = params.data
        detailVisible.value = true
      }
    })
  }
  var nodes = filteredNodes.value.map((n: any) => ({
    id: n.id, name: n.label || n.id,
    symbolSize: 30,
    itemStyle: { color: typeColor(n.entity_type) },
  }))
  var links = filteredLinks.value.map((l: any) => ({
    source: l.source || l.in, target: l.target || l.out,
  }))
  chartInstance.setOption({
    series: [{
      type: "graph", layout: "force", roam: true, draggable: true,
      data: nodes, links: links,
      force: { repulsion: 300, edgeLength: [120, 300] },
      label: { show: true, fontSize: 11 },
    }]
  }, true)
}

onMounted(() => {
  fetchGraph()
  window.addEventListener("keydown", onKeydown)
})

onUnmounted(() => {
  window.removeEventListener("keydown", onKeydown)
  if (chartInstance) {
    chartInstance.dispose()
    chartInstance = null
  }
})
</script>

<style scoped>
.graph-page { height: 100%; display: flex; flex-direction: column; }
.page-title { font-size: var(--ac-font-size-lg); font-weight: 600; margin-bottom: 12px; }
.graph-controls { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; flex-wrap: wrap; }
.slider-label { color: #909399; font-size: 13px; white-space: nowrap; }
.graph-stats { display: flex; gap: 16px; margin-bottom: 8px; font-size: 13px; color: #606266; flex-wrap: wrap; }
.stat-type { font-weight: 500; }
.graph-container { flex: 1; min-height: 400px; border: 1px solid #e4e7ed; border-radius: 8px; overflow: hidden; position: relative; }
.graph-container.fullscreen { position: fixed; inset: 0; z-index: 1000; background: #fff; border: none; border-radius: 0; }
.fullscreen-bar { display: flex; align-items: center; justify-content: space-between; padding: 8px 16px; background: #f5f7fa; border-bottom: 1px solid #e4e7ed; }
.fullscreen-title { font-size: 14px; font-weight: 600; color: #303133; }
.fullscreen-close { font-size: 14px; }
.chart { width: 100%; height: 100%; }
.graph-container.fullscreen .chart { height: calc(100% - 41px); }
.node-detail p { margin: 8px 0; font-size: 14px; }
</style>
