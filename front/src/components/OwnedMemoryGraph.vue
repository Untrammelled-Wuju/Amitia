<template>
  <section class="memory-graph" aria-label="记忆关系图谱">
    <div class="graph-toolbar">
      <el-input v-if="!hideSearch" v-model="keyword" clearable placeholder="搜索图谱节点" aria-label="搜索图谱节点" class="graph-search" />
      <span class="graph-count">{{ graph.nodes.length }} 个节点 · {{ graph.links.length }} 条关系</span>
      <el-button :disabled="!graph.nodes.length" @click="resetView">重置视图</el-button>
    </div>
    <p class="graph-hint">拖拽节点调整布局，滚轮缩放，拖拽空白区域移动图谱</p>
    <VChart v-if="graph.nodes.length" :key="viewRevision" :option="option" :init-options="{ renderer: 'svg' }" autoresize class="graph-canvas" role="img" :aria-label="`记忆图谱，${graph.nodes.length} 个节点，${graph.links.length} 条关系`" />
    <el-empty v-else-if="!loading && !error" :description="keyword ? '没有匹配的图谱节点' : '当前角色在此数据来源中暂无图谱关系'" />
    <div v-else class="graph-placeholder" />
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { use } from "echarts/core";
import { GraphChart } from "echarts/charts";
import { TooltipComponent } from "echarts/components";
import { SVGRenderer } from "echarts/renderers";
import VChart from "vue-echarts";
import { buildOwnedMemoryGraph } from "@/runtime/owned-memory-graph";
import { useTheme } from "@/composables/useTheme";

use([GraphChart, TooltipComponent, SVGRenderer]);
const props = defineProps<{ resources: any[]; loading: boolean; error: string; hideSearch?: boolean }>();
const keyword = ref("");
const viewRevision = ref(0);
const { resolvedMode } = useTheme();
const graph = computed(() => {
  const result = buildOwnedMemoryGraph(props.resources);
  const search = keyword.value.trim().toLocaleLowerCase();
  if (!search) return result;
  const matches = new Set(result.nodes.filter((node) => node.name.toLocaleLowerCase().includes(search)).map((node) => node.id));
  const links = result.links.filter((link) => matches.has(link.source) || matches.has(link.target));
  const visible = new Set([...matches, ...links.flatMap((link) => [link.source, link.target])]);
  return { nodes: result.nodes.filter((node) => visible.has(node.id)), links };
});
const option = computed(() => {
  const dark = resolvedMode.value === "dark";
  const styles = getComputedStyle(document.documentElement);
  const color = (name: string, fallback: string) => styles.getPropertyValue(name).trim() || fallback;
  return {
    animation: false,
    tooltip: { renderMode: "richText", formatter: (params: any) => params.dataType === "edge" ? params.data.name || "关系" : params.data.name },
    series: [{
      type: "graph", layout: "force", roam: true, draggable: true,
      data: graph.value.nodes, links: graph.value.links,
      force: { repulsion: 280, edgeLength: [100, 180], gravity: 0.08 },
      scaleLimit: { min: 0.2, max: 5 },
      label: { show: true, position: "bottom", color: color("--ac-color-text-primary", dark ? "#e5e7eb" : "#1f2937"), fontSize: 12, width: 160, overflow: "truncate" },
      itemStyle: { color: color("--ac-color-primary", "#409eff") },
      lineStyle: { color: color("--ac-color-text-secondary", dark ? "#9ca3af" : "#64748b"), opacity: 0.6, curveness: 0.15 },
      edgeSymbol: ["none", "arrow"], edgeSymbolSize: 8,
      edgeLabel: { show: true, formatter: (params: any) => params.data.name, color: color("--ac-color-text-secondary", dark ? "#9ca3af" : "#64748b"), fontSize: 11 },
      emphasis: { focus: "adjacency" },
    }],
  };
});
function resetView() { viewRevision.value++; }
</script>

<style scoped>
.memory-graph { min-width: 0; margin: 16px 0; }
.graph-toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; }
.graph-search { width: 240px; max-width: 100%; }
.graph-count, .graph-hint { color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); }
.graph-count { margin-right: auto; }
.graph-hint { margin: 12px 0; line-height: 1.5; }
.graph-canvas, .graph-placeholder { width: 100%; height: clamp(360px, 60vh, 680px); min-width: 0; }
.graph-canvas { border: 1px solid var(--ac-color-border); border-radius: var(--ac-radius-md); background: var(--ac-color-surface); }
@media (max-width: 640px) { .graph-search { width: 100%; } }
</style>
