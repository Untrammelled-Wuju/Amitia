<template>
  <div class="extension-page">
    <ExtensionPageHeader title="执行记录" description="按角色查看技能运行状态、触发来源、耗时与审计关联信息。">
      <template #actions><el-button :icon="Refresh" :loading="loading" @click="load">刷新</el-button></template>
    </ExtensionPageHeader>

    <el-card shadow="never" class="filter-card">
      <el-form :inline="true" label-position="top" @submit.prevent>
        <el-form-item label="技能">
          <el-select v-model="filters.skillId" clearable filterable placeholder="全部技能" @change="search">
            <el-option v-for="skill in skills" :key="skill.id" :label="skill.name" :value="skill.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="filters.status" clearable placeholder="全部状态" @change="search">
            <el-option v-for="item in statuses" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="触发方式">
          <el-select v-model="filters.trigger" clearable placeholder="全部触发方式" @change="search">
            <el-option label="模型调用" value="llm" />
            <el-option label="手动执行" value="manual" />
            <el-option label="定时协议" value="schedule" />
            <el-option label="系统事件协议" value="system_event" />
          </el-select>
        </el-form-item>
        <el-form-item label="渠道">
          <el-input v-model.trim="filters.channel" clearable placeholder="如 web、qq、wechat" @keyup.enter="search" @clear="search" />
        </el-form-item>
        <el-form-item label="开始时间">
          <el-date-picker v-model="filters.range" type="datetimerange" start-placeholder="起始时间" end-placeholder="结束时间" value-format="YYYY-MM-DDTHH:mm:ssZ" @change="search" />
        </el-form-item>
      </el-form>
    </el-card>

    <el-alert v-if="loadError" :title="loadError" type="error" show-icon :closable="false">
      <template #default><el-button link type="primary" @click="load">重新加载</el-button></template>
    </el-alert>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="runs.items" row-key="runId" empty-text="暂无执行记录" stripe @row-click="openRun">
        <el-table-column prop="startedAt" label="开始时间" min-width="170"><template #default="{ row }">{{ formatTime(row.startedAt) }}</template></el-table-column>
        <el-table-column prop="skillId" label="技能" min-width="230"><template #default="{ row }"><button class="run-link" type="button" @click.stop="openRun(row)">{{ skillLabel(row.skillId) }}</button></template></el-table-column>
        <el-table-column prop="trigger" label="触发" width="110"><template #default="{ row }">{{ triggerLabel(row.trigger) }}</template></el-table-column>
        <el-table-column prop="channel" label="渠道" width="110"><template #default="{ row }">{{ row.channel || "—" }}</template></el-table-column>
        <el-table-column prop="status" label="状态" width="120"><template #default="{ row }"><el-tag :type="statusType(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
        <el-table-column prop="durationMs" label="耗时" width="110"><template #default="{ row }">{{ row.durationMs }} ms</template></el-table-column>
        <el-table-column prop="traceId" label="traceId" min-width="180" show-overflow-tooltip />
      </el-table>
      <div class="pagination-bar">
        <span>共 {{ runs.total }} 条</span>
        <el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="runs.total" :page-sizes="[20, 50, 100]" layout="sizes, prev, pager, next" @current-change="load" @size-change="search" />
      </div>
    </el-card>

    <el-drawer v-model="drawerVisible" title="运行详情" size="min(680px, 92vw)">
      <template v-if="selectedRun">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="runId" :span="2"><code>{{ selectedRun.runId }}</code></el-descriptions-item>
          <el-descriptions-item label="技能" :span="2"><code>{{ selectedRun.skillId }}</code></el-descriptions-item>
          <el-descriptions-item label="状态"><el-tag :type="statusType(selectedRun.status)">{{ statusLabel(selectedRun.status) }}</el-tag></el-descriptions-item>
          <el-descriptions-item label="耗时">{{ selectedRun.durationMs }} ms</el-descriptions-item>
          <el-descriptions-item label="触发">{{ triggerLabel(selectedRun.trigger) }}</el-descriptions-item>
          <el-descriptions-item label="渠道">{{ selectedRun.channel || "—" }}</el-descriptions-item>
          <el-descriptions-item label="角色"><code>{{ selectedRun.characterId }}</code></el-descriptions-item>
          <el-descriptions-item label="会话"><code>{{ selectedRun.conversationId || "—" }}</code></el-descriptions-item>
          <el-descriptions-item label="traceId" :span="2"><code>{{ selectedRun.traceId || "—" }}</code></el-descriptions-item>
          <el-descriptions-item v-if="selectedRun.errorCode" label="错误码">{{ selectedRun.errorCode }}</el-descriptions-item>
          <el-descriptions-item v-if="selectedRun.errorDetail" label="错误详情" :span="selectedRun.errorCode ? 1 : 2">{{ selectedRun.errorDetail }}</el-descriptions-item>
        </el-descriptions>
        <el-card shadow="never" class="summary-card"><template #header>输入摘要</template><pre>{{ selectedRun.inputSummary || "—" }}</pre></el-card>
        <el-card shadow="never" class="summary-card"><template #header>输出摘要</template><pre>{{ selectedRun.outputSummary || "—" }}</pre></el-card>
        <el-card v-if="selectedRun.sideEffects?.length" shadow="never" class="summary-card"><template #header>副作用记录</template><pre>{{ pretty(selectedRun.sideEffects) }}</pre></el-card>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue"
import { Refresh } from "@element-plus/icons-vue"
import ExtensionPageHeader from "./components/ExtensionPageHeader.vue"
import { fetchRun, fetchRuns, fetchSkills, resolveCharacterId } from "./api"
import type { RunPage, RunStatus, RunView, SkillTrigger, SkillView } from "./types"

const loading = ref(false)
const loadError = ref("")
const characterId = ref("")
const skills = ref<SkillView[]>([])
const runs = ref<RunPage>({ items: [], total: 0, page: 1, pageSize: 20 })
const page = ref(1)
const pageSize = ref(20)
const drawerVisible = ref(false)
const selectedRun = ref<RunView>()
const filters = reactive<{ skillId: string; status: RunStatus | ""; trigger: SkillTrigger | ""; channel: string; range: string[] }>({ skillId: "", status: "", trigger: "", channel: "", range: [] })
const statuses: Array<{ value: RunStatus; label: string }> = [
  { value: "pending", label: "等待中" }, { value: "running", label: "运行中" }, { value: "succeeded", label: "成功" },
  { value: "failed", label: "失败" }, { value: "cancelled", label: "已取消" }, { value: "timed_out", label: "超时" },
  { value: "partially_succeeded", label: "部分成功" },
]

async function load() {
  loading.value = true
  loadError.value = ""
  try {
    if (!characterId.value) characterId.value = await resolveCharacterId()
    if (!characterId.value) throw new Error("请先创建或选择角色")
    if (!skills.value.length) skills.value = await fetchSkills(characterId.value, {})
    runs.value = await fetchRuns(characterId.value, { skillId: filters.skillId || undefined, status: filters.status || undefined, trigger: filters.trigger || undefined, channel: filters.channel || undefined, from: filters.range?.[0], to: filters.range?.[1], page: page.value, pageSize: pageSize.value })
  } catch (error: any) {
    loadError.value = error?.detail || error?.response?.data?.detail || error?.message || "执行记录加载失败"
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

async function openRun(row: RunView) {
  drawerVisible.value = true
  selectedRun.value = row
  try {
    selectedRun.value = await fetchRun(characterId.value, row.runId)
  } catch {}
}

function skillLabel(id: string) {
  const skill = skills.value.find((item) => item.id === id)
  return skill ? `${skill.name} · ${id}` : id
}

function triggerLabel(trigger: SkillTrigger) {
  return { llm: "模型调用", manual: "手动执行", schedule: "定时协议", system_event: "系统事件协议" }[trigger]
}

function statusLabel(status: RunStatus) {
  return statuses.find((item) => item.value === status)?.label || status
}

function statusType(status: RunStatus) {
  if (status === "succeeded") return "success"
  if (status === "running" || status === "pending" || status === "partially_succeeded") return "warning"
  if (status === "failed" || status === "timed_out") return "danger"
  return "info"
}

function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN", { hour12: false })
}

function pretty(value: unknown) {
  return JSON.stringify(value, null, 2)
}

onMounted(load)
</script>

<style scoped>
.extension-page { display: flex; flex-direction: column; gap: 16px; min-width: 0; }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.page-header h1 { color: var(--ac-color-text); font-size: 24px; line-height: 32px; }
.page-header p { margin-top: 6px; color: var(--ac-color-text-secondary); line-height: 1.6; }
.filter-card :deep(.el-card__body) { padding-bottom: 2px; }
.filter-card :deep(.el-form-item) { min-width: 180px; margin-right: 16px; }
.table-card :deep(.el-card__body) { padding: 0; overflow-x: auto; }
.run-link { border: 0; background: transparent; color: var(--ac-color-primary); cursor: pointer; text-align: left; }
.run-link:focus-visible { outline: 2px solid var(--ac-color-primary); outline-offset: 2px; }
.pagination-bar { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 16px; color: var(--ac-color-text-secondary); }
code { overflow-wrap: anywhere; }
.summary-card { margin-top: 16px; }
pre { max-height: 300px; overflow: auto; margin: 0; padding: 14px; border-radius: var(--ac-radius-sm); background: var(--ac-color-bg-secondary); color: var(--ac-color-text); font: 12px/1.6 "SFMono-Regular", Consolas, monospace; white-space: pre-wrap; overflow-wrap: anywhere; }
@media (max-width: 720px) {
  .page-header, .pagination-bar { align-items: stretch; flex-direction: column; }
  .filter-card :deep(.el-form), .filter-card :deep(.el-form-item) { display: block; width: 100%; }
}
</style>
