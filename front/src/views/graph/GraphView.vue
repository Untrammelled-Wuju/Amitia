<template>
  <div class="graph-page">
    <h2 class="page-title">记忆图谱</h2>
    <div class="graph-controls">
      <el-input v-model="searchId" placeholder="输入节点ID查询" size="small" style="width:200px" clearable />
      <el-slider v-model="depth" :min="1" :max="4" show-input size="small" style="width:160px" />
      <span class="slider-label">跳数</span>
      <el-button size="small" type="primary" @click="fetchGraph">查询</el-button>
      <el-button size="small" @click="showFullscreen = !showFullscreen">全屏</el-button>
    </div>
    <div class="graph-stats" v-if="stats">
      <span>节点: {{ stats.nodeCount }}</span>
      <span>边: {{ stats.edgeCount }}</span>
    </div>
    <div :class="['graph-container', { fullscreen: showFullscreen }]">
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
import { ref, onMounted } from "vue"
import { useApi } from "@/composables/useApi"

const { get } = useApi()
const chartRef = ref<HTMLElement>()
const depth = ref(2)
const searchId = ref("")
const showFullscreen = ref(false)
const detailVisible = ref(false)
const selectedNode = ref<any>(null)
const stats = ref<any>(null)
let chartInstance: any = null

async function fetchGraph() {
  try {
    const s = await get<any>("/api/graph/stats?userId=default")
    stats.value = s
  } catch {}
  if (searchId.value) {
    try {
      const data = await get<any>(`/api/graph/node/${encodeURIComponent(searchId.value)}/neighbors?depth=${depth.value}&userId=default`)
      renderGraph(data)
    } catch {}
  }
}

async function renderGraph(data: any) {
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
  chartInstance.setOption({
    series: [{
      type: "graph", layout: "force", roam: true, draggable: true,
      data: [], links: [],
      force: { repulsion: 200, edgeLength: [100, 300] },
      label: { show: true, fontSize: 11 },
    }]
  }, true)
}

onMounted(() => { fetchGraph() })
</script>

<style scoped>
.graph-page { height: 100%; display: flex; flex-direction: column; }
.page-title { font-size: var(--ac-font-size-lg); font-weight: 600; margin-bottom: 12px; }
.graph-controls { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; flex-wrap: wrap; }
.slider-label { color: #909399; font-size: 13px; white-space: nowrap; }
.graph-stats { display: flex; gap: 20px; margin-bottom: 8px; font-size: 13px; color: #606266; }
.graph-container { flex: 1; min-height: 400px; border: 1px solid #e4e7ed; border-radius: 8px; overflow: hidden; }
.graph-container.fullscreen { position: fixed; inset: 0; z-index: 1000; background: #fff; }
.chart { width: 100%; height: 100%; }
.node-detail p { margin: 8px 0; font-size: 14px; }
</style>
