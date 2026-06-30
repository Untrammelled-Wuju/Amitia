<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from "vue"
import { useApi } from "@/composables/useApi"
import { useTheme } from "@/composables/useTheme"

const { get } = useApi()
function extractId(obj: any): string {
  if (!obj) return ""
  if (typeof obj === "string") return obj.replace("entity_node:", "")
  if (obj.ID) return obj.ID
  return String(obj)
}
const chartRef = ref<HTMLElement>()
const depth = ref(2)
const searchId = ref("")
const typeFilter = ref("")
const labelKeyword = ref("")
const showFullscreen = ref(false)
const stats = ref<any>(null)
const allNodes = ref<any[]>([])
const allLinks = ref<any[]>([])
let chartInstance: any = null
const { resolvedMode } = useTheme()

const typeLabels: Record<string,string> = { memory: "记忆", profile: "画像", episodic: "情景", worldbook: "世界书", worldbook_trigger: "触发片段", user: "用户", character: "角色", entity: "实体" }
const typeColors: Record<string,string> = { memory: "#409eff", profile: "#e6a23c", episodic: "#f56c6c", worldbook: "#67c23a", worldbook_trigger: "#909399", user: "#8e6ad8", character: "#14b8a6", entity: "#64748b" }
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
const filteredNodeIds = computed(() => new Set(filteredNodes.value.map((n: any) => extractId(n.id))))
const filteredLinks = computed(() => allLinks.value.filter((l: any) => {
  const source = extractId(l.in || l.source)
  const target = extractId(l.out || l.target)
  return filteredNodeIds.value.has(source) && filteredNodeIds.value.has(target)
}))
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
      var rawNeighbors = data?.neighbors || data?.result || []
      var neighborIds = new Set()
      if (Array.isArray(rawNeighbors)) {
        for (var n of rawNeighbors) {
          if (typeof n === "string") neighborIds.add(n)
          else if (n && n.id) neighborIds.add(n.id)
        }
      }
      allNodes.value = []
      allLinks.value = []
      renderGraph()
      try {
        var nodesData = await get<any>("/api/graph/nodes?userId=default")
        var allNodesData = nodesData?.data || nodesData || []
        if (Array.isArray(allNodesData) && allNodesData.length > 0) {
          allNodes.value = allNodesData.filter(function(n) {
            var nid = extractId(n.id)
            return neighborIds.has(nid) || nid === searchId.value
          })
          if (allNodes.value.length === 0 && allNodesData.length > 0) {
            allNodes.value = allNodesData.filter(function(n) {
              var nid = extractId(n.id)
              return nid.indexOf(searchId.value) !== -1
            })
          }
          renderGraph()
        }
      } catch {}
      try {
        var edgesData = await get<any>("/api/graph/edges?userId=default")
        var allEdgesData = edgesData?.data || edgesData || []
        if (Array.isArray(allEdgesData) && allEdgesData.length > 0) {
          var nodeIdSet = new Set(allNodes.value.map(function(n) { return extractId(n.id) }))
          allLinks.value = allEdgesData.filter(function(e) {
            return nodeIdSet.has(extractId(e.in)) && nodeIdSet.has(extractId(e.out))
          })
          renderGraph()
        }
      } catch {}
    } catch {
      allNodes.value = []
      allLinks.value = []
      renderGraph()
    }
  } else {
    try {
      var nodesData = await get<any>("/api/graph/nodes?userId=default")
      var allNodesArr = nodesData?.data || nodesData || []
      if (Array.isArray(allNodesArr) && allNodesArr.length > 0) {
        allNodes.value = allNodesArr
        try {
          var edgesData = await get<any>("/api/graph/edges?userId=default")
          var allEdgesArr = edgesData?.data || edgesData || []
          if (Array.isArray(allEdgesArr) && allEdgesArr.length > 0) {
            allLinks.value = allEdgesArr
          }
        } catch {}
        renderGraph()
      } else {
        allNodes.value = []
        allLinks.value = []
        renderGraph()
      }
    } catch {
      allNodes.value = []
      allLinks.value = []
      renderGraph()
    }
  }
}

function applyFilter() { renderGraph() }

async function renderGraph() {
  if (!chartRef.value) return
  if (!chartInstance) {
    chartInstance = (await import("echarts")).init(chartRef.value)
  }
  var nodes = filteredNodes.value.map((n: any) => {
    var nid = extractId(n.id)
    return {
      id: nid, name: n.label || nid,
      label: n.label || nid,
      entity_type: n.entity_type,
      symbolSize: 30,
      itemStyle: { color: typeColor(n.entity_type) },
    }
  })
  var links = filteredLinks.value.map((l: any) => {
    return { source: extractId(l.in || l.source), target: extractId(l.out || l.target) }
  })
  var styles = getComputedStyle(document.documentElement)
  var textColor = styles.getPropertyValue("--ac-color-text").trim() || "#333"
  var bgColor = styles.getPropertyValue("--ac-color-bg").trim() || "#fff"
  var lineColor = styles.getPropertyValue("--ac-color-border").trim() || "#e0e0e0"
  chartInstance.setOption({
    backgroundColor: bgColor,
    tooltip: {
      trigger: "item",
      formatter: (params: any) => {
        var d = params.data
        return "<strong>" + (d.label || d.name) + "</strong><br/>ID: " + d.id + "<br/>类型: " + typeLabel(d.entity_type)
      },
    },
    series: [{
      type: "graph", layout: "force", roam: true, draggable: true,
      data: nodes, links: links,
      force: { repulsion: 300, edgeLength: [120, 300] },
      label: { show: true, fontSize: 11, color: textColor },
      lineStyle: { color: lineColor, curveness: 0.2 },
    }]
  }, true)
}

watch(resolvedMode, () => {
  nextTick(() => renderGraph())
})

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
.graph-container.fullscreen { position: fixed; inset: 0; z-index: 1000; background: var(--ac-color-bg); border: none; border-radius: 0; }
.fullscreen-bar { display: flex; align-items: center; justify-content: space-between; padding: 8px 16px; background: var(--ac-color-bg-secondary); border-bottom: 1px solid #e4e7ed; }
.fullscreen-title { font-size: 14px; font-weight: 600; color: var(--ac-color-text-primary); }
.fullscreen-close { font-size: 14px; color: var(--ac-color-text-primary); }
.chart { width: 100%; height: 100%; }
.graph-container.fullscreen .chart { height: calc(100% - 41px); }
</style>
