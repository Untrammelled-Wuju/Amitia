<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="page">
    <div class="page-header">
      <h2 class="page-title">记忆时间线</h2>
      <div class="header-filters">
        <el-select v-model="sourceFilter" placeholder="来源" clearable size="small" style="width:90px" @change="fetchTimeline">
          <el-option label="手动" value="manual" />
          <el-option label="聊天" value="chat" />
          <el-option label="导入" value="import" />
        </el-select>
        <el-select v-model="typeFilter" placeholder="类型" clearable size="small" style="width:90px" @change="fetchTimeline">
          <el-option label="偏好" value="preference" />
          <el-option label="事件" value="event" />
          <el-option label="习惯" value="habit" />
          <el-option label="昵称" value="nickname" />
          <el-option label="关系" value="relationship" />
          <el-option label="其他" value="custom" />
        </el-select>
        <el-select v-model="timelineType" placeholder="来源类型" clearable size="small" style="width:110px" @change="fetchTimeline">
          <el-option label="结构化记忆" value="memory" />
          <el-option label="情景记忆" value="episodic" />
        </el-select>
        <el-button size="small" @click="showLayers = !showLayers">层级筛选</el-button>
        <el-button size="small" @click="diffMode = !diffMode" :type="diffMode ? 'warning' : ''">Diff</el-button>
        <el-button size="small" @click="exportCSV" :disabled="items.length === 0">CSV导出</el-button>
      </div>
      <div v-if="showLayers" class="layer-filters">
        <el-checkbox v-model="layerFilters.working" label="工作记忆" size="small" @change="applyFilters" />
        <el-checkbox v-model="layerFilters.profile" label="用户画像" size="small" @change="applyFilters" />
        <el-checkbox v-model="layerFilters.episodic" label="情景记忆" size="small" @change="applyFilters" />
        <el-checkbox v-model="layerFilters.fact" label="结构化事实" size="small" @change="applyFilters" />
        <el-checkbox v-model="layerFilters.worldbook" label="世界书" size="small" @change="applyFilters" />
        <el-checkbox v-model="layerFilters.graph" label="图谱" size="small" @change="applyFilters" />
      </div>
      <div v-if="diffMode" class="diff-bar">
        <span>对比时间点1: </span><el-date-picker v-model="diffTime1" type="datetime" size="small" placeholder="选择时间点1" />
        <span>时间点2: </span><el-date-picker v-model="diffTime2" type="datetime" size="small" placeholder="选择时间点2" />
        <el-button size="small" @click="doDiff" :disabled="!diffTime1 || !diffTime2">对比</el-button>
        <span v-if="diffResult" class="diff-summary">新增{{ diffResult.added }} 修改{{ diffResult.modified }} 删除{{ diffResult.deleted }}</span>
      </div>
    </div>

    <div class="timeline" v-if="items.length > 0">
      <div v-for="item in items" :key="item.id + '-' + item.event_type" class="tl-item">
        <div class="tl-dot" :class="dotClass(item.event_type)"></div>
        <div class="tl-card">
          <div class="tl-header">
            <el-tag size="small" :type="tagType(item.event_type)">{{ eventLabel(item.event_type) }}</el-tag>
            <span class="tl-time">{{ formatDate(item.created_at) }}</span>
            <span v-if="item.character_name" class="tl-char">{{ item.character_name }}</span>
          </div>
          <div class="tl-body">
            <template v-if="item.event_type === 'memory_deleted'">
              <span class="tl-deleted">删除了一条记忆</span>
            </template>
            <template v-else-if="item.event_type === 'memory_edited'">
              <span class="tl-edited">编辑了记忆</span>
            </template>
            <template v-else>
              <div class="tl-key" v-if="item.key">{{ item.key }}</div>
              <div class="tl-value">{{ item.value || '' }}</div>
              <div class="tl-meta">
                <span v-if="item.source">{{ sourceLabel(item.source) }}</span>
                <span v-if="item.memory_type">{{ typeLabel(item.memory_type) }}</span>
                <span v-if="item.importance">重要性 {{ item.importance }}</span>
              </div>
            </template>
          </div>
        </div>
      </div>
    </div>

    <el-empty v-else-if="!loading" description="暂无时间线记录" :image-size="80" />

    <div class="pagination" v-if="total > pageSize">
      <el-pagination v-model:current-page="page" :page-size="pageSize" :total="total" layout="prev, pager, next" size="small" @current-change="fetchTimeline" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, inject, type Ref } from "vue"
import { useApi } from "../../composables/useApi"

const injectedCharacterId = inject<Ref<string | null>>('currentCharacterId', ref(null))

const { get } = useApi()
const items = ref<any[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = 30
const total = ref(0)
const sourceFilter = ref("")
const typeFilter = ref("")
const timelineType = ref("")
const showLayers = ref(false)
const layerFilters = ref({ working: true, profile: true, episodic: true, fact: true, worldbook: true, graph: true })
const diffMode = ref(false)
const diffTime1 = ref("")
const diffTime2 = ref("")
const diffResult = ref<any>(null)
const allItems = ref<any[]>([])

async function fetchTimeline() {
  loading.value = true
  try {
    const params: any = { page: page.value, pageSize }
    params.userId = "default"
    if (sourceFilter.value) params.source = sourceFilter.value
    if (typeFilter.value) params.memoryType = typeFilter.value
    if (timelineType.value) params.type = timelineType.value
    const r = await get<any>("/api/memories/timeline", params)
    items.value = r?.items || []
    allItems.value = r?.items || []
    total.value = r?.total || 0
  } catch {}
  loading.value = false
}

function dotClass(evt: string): string {
  if (!evt) return ""
  if (evt.includes("deleted")) return "dot-deleted"
  if (evt.includes("edited")) return "dot-edited"
  if (evt.includes("accepted")) return "dot-accepted"
  if (evt.includes("rejected")) return "dot-rejected"
  if (evt.includes("pending")) return "dot-pending"
  if (evt.includes("created")) return "dot-created"
  return ""
}

function tagType(evt: string): string {
  if (!evt) return ""
  if (evt.includes("deleted")) return "danger"
  if (evt.includes("edited")) return "warning"
  if (evt.includes("accepted")) return "success"
  if (evt.includes("rejected")) return "info"
  if (evt.includes("pending")) return ""
  if (evt.includes("created")) return "primary"
  return ""
}

function eventLabel(evt: string): string {
  const labels: Record<string, string> = {
    memory_created: "新增", memory_edited: "编辑", memory_deleted: "删除",
    candidate_accepted: "已采纳", candidate_rejected: "已拒绝", candidate_pending: "待确认",
    memory_operation: "操作"
  }
  return labels[evt] || evt
}

function sourceLabel(s: string): string {
  const labels: Record<string, string> = { manual: "手动", chat: "聊天", import: "导入" }
  return labels[s] || s
}

function typeLabel(t: string): string {
  const labels: Record<string, string> = {
    preference: "偏好", event: "事件", habit: "习惯", nickname: "昵称", relationship: "关系", custom: "其他"
  }
  return labels[t] || t
}

function formatDate(d: string): string {
  if (!d) return ""
  return new Date(d).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
}

function guessLayer(item: any): string {
  if (item.source === 'profile' || item.source === 'user_profile') return 'profile'
  if (item.source === 'episodic') return 'episodic'
  if (item.source === 'worldbook') return 'worldbook'
  if (item.source === 'graph') return 'graph'
  if (item.memory_type === 'working') return 'working'
  return 'fact'
}

function layerColor(layer: string): string {
  const colors: Record<string,string> = { working: '#409eff', profile: '#e6a23c', episodic: '#f56c6c', fact: '#67c23a', worldbook: '#909399', graph: '#b37feb' }
  return colors[layer] || '#409eff'
}

function applyFilters() { fetchTimeline() }

function exportCSV() {
  const headers = ['层级','类型','时间','内容']
  const rows = items.value.map((i: any) => [guessLayer(i), eventLabel(i.event_type), formatDate(i.created_at), i.value || i.key || ''])
  const csv = [headers.join(',')].concat(rows.map(r => r.map(c => '"' + String(c).replace(/"/g,'""') + '"').join(','))).join('\n')
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a'); a.href = url; a.download = 'memory-timeline.csv'; a.click()
  URL.revokeObjectURL(url)
}

function doDiff() {
  if (!diffTime1.value || !diffTime2.value) return
  const t1 = new Date(diffTime1.value).getTime()
  const t2 = new Date(diffTime2.value).getTime()
  const before = allItems.value.filter((i: any) => new Date(i.created_at).getTime() <= t1)
  const after = allItems.value.filter((i: any) => new Date(i.created_at).getTime() <= t2)
  const beforeIds = new Set(before.map((i: any) => i.id))
  const afterIds = new Set(after.map((i: any) => i.id))
  diffResult.value = {
    added: after.filter((i: any) => !beforeIds.has(i.id)).length,
    modified: 0,
    deleted: before.filter((i: any) => !afterIds.has(i.id)).length,
  }
}

onMounted(() => fetchTimeline())
</script>

<style scoped>
.page { max-width: 900px; margin: 0 auto; }
.page-header { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; flex-wrap: wrap; }
.page-title { margin: 0; }
.header-filters { display: flex; gap: 6px; }

.timeline { position: relative; padding-left: 24px; }
.timeline::before { content: ""; position: absolute; left: 7px; top: 0; bottom: 0; width: 2px; background: var(--ac-color-border); }

.tl-item { position: relative; margin-bottom: 16px; }
.tl-dot { position: absolute; left: -20px; top: 14px; width: 12px; height: 12px; border-radius: 50%; border: 2px solid var(--ac-color-bg); z-index: 1; }
.dot-created { background: var(--ac-color-primary); }
.dot-edited { background: var(--ac-color-warning); }
.dot-deleted { background: var(--ac-color-danger); }
.dot-accepted { background: var(--ac-color-success); }
.dot-rejected { background: var(--ac-color-text-muted); }
.dot-pending { background: var(--ac-color-text-placeholder); }

.tl-card { background: var(--ac-color-surface); border: 1px solid var(--ac-color-border-light); border-radius: var(--ac-radius-md); padding: 12px 14px; }
.tl-header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; flex-wrap: wrap; }
.tl-time { font-size: 12px; color: var(--ac-color-text-muted); }
.tl-char { font-size: 12px; color: var(--ac-color-text-secondary); margin-left: auto; }

.tl-body { font-size: var(--ac-font-size-sm); }
.tl-key { font-weight: 600; margin-bottom: 2px; }
.tl-value { color: var(--ac-color-text-secondary); line-height: 1.5; white-space: pre-wrap; word-break: break-word; }
.tl-deleted { color: var(--ac-color-text-muted); font-style: italic; }
.tl-edited { color: var(--ac-color-warning); }
.tl-meta { display: flex; gap: 10px; margin-top: 6px; font-size: 11px; color: var(--ac-color-text-muted); }

.pagination { display: flex; justify-content: center; margin-top: 20px; }
.layer-filters { display: flex; flex-wrap: wrap; gap: 10px; padding: 8px 0; margin-bottom: 8px; }
.diff-bar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; padding: 8px 0; background: var(--ac-color-bg-secondary); border-radius: var(--ac-radius-sm); margin-bottom: 8px; }
.diff-summary { font-size: 13px; font-weight: 600; color: var(--ac-color-warning); }
</style>
