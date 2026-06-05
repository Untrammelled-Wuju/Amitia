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
      <el-pagination v-model:current-page="page" :page-size="pageSize" :total="total" layout="prev, pager, next" small @current-change="fetchTimeline" />
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

async function fetchTimeline() {
  loading.value = true
  try {
    const params: any = { page: page.value, pageSize }
    if (injectedCharacterId?.value) params.characterId = injectedCharacterId.value
    if (sourceFilter.value) params.source = sourceFilter.value
    if (typeFilter.value) params.memoryType = typeFilter.value
    const r = await get<any>("/api/memories/timeline", params)
    items.value = r?.items || []
    total.value = r?.total || 0
  } catch {}
  loading.value = false
}

function dotClass(evt: string): string {
  if (evt.includes("deleted")) return "dot-deleted"
  if (evt.includes("edited")) return "dot-edited"
  if (evt.includes("accepted")) return "dot-accepted"
  if (evt.includes("rejected")) return "dot-rejected"
  if (evt.includes("pending")) return "dot-pending"
  return "dot-created"
}

function tagType(evt: string): string {
  if (evt.includes("deleted")) return "danger"
  if (evt.includes("edited")) return "warning"
  if (evt.includes("accepted")) return "success"
  if (evt.includes("rejected")) return "info"
  if (evt.includes("pending")) return ""
  return "primary"
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
</style>
