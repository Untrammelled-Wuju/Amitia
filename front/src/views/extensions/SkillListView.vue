<template>
  <div class="extension-page">
    <header class="page-header">
      <div>
        <h1>技能</h1>
        <p>管理 Amitia 内置能力与现有工具适配状态。禁用后会立即从模型工具列表中移除。</p>
      </div>
      <el-button :icon="Refresh" :loading="loading" @click="load">刷新</el-button>
    </header>

    <el-card shadow="never" class="filter-card">
      <el-form :inline="true" label-position="top" @submit.prevent>
        <el-form-item label="状态">
          <el-select v-model="filters.enabled" clearable placeholder="全部状态" @change="load">
            <el-option label="已启用" :value="true" />
            <el-option label="已禁用" :value="false" />
          </el-select>
        </el-form-item>
        <el-form-item label="触发方式">
          <el-select v-model="filters.trigger" clearable placeholder="全部触发方式" @change="load">
            <el-option label="模型调用" value="llm" />
            <el-option label="手动执行" value="manual" />
            <el-option label="定时协议" value="schedule" />
            <el-option label="系统事件协议" value="system_event" />
          </el-select>
        </el-form-item>
        <el-form-item label="来源">
          <el-select v-model="filters.source" clearable placeholder="全部来源" @change="load">
            <el-option label="旧工具适配" value="legacy_tool" />
            <el-option label="内置技能" value="builtin" />
            <el-option label="工坊工作流" value="workflow" />
          </el-select>
        </el-form-item>
      </el-form>
    </el-card>

    <el-alert v-if="loadError" :title="loadError" type="error" show-icon :closable="false" class="load-error">
      <template #default><el-button link type="primary" @click="load">重新加载</el-button></template>
    </el-alert>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="skills" row-key="id" empty-text="暂无技能" stripe>
        <el-table-column label="技能" min-width="250">
          <template #default="{ row }">
            <button class="skill-link" type="button" @click="openDetail(row.id)">
              <span class="skill-name">{{ row.name }}</span>
              <code>{{ row.id }}</code>
            </button>
          </template>
        </el-table-column>
        <el-table-column prop="version" label="版本" width="90" />
        <el-table-column label="来源" width="110">
          <template #default="{ row }">{{ sourceLabel(row.source) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? "已启用" : "已禁用" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="兼容性" width="120">
          <template #default="{ row }">
            <el-tooltip :content="row.compatibilityReason || '与当前引擎兼容'">
              <el-tag :type="row.compatible ? 'success' : 'danger'">{{ row.compatible ? "兼容" : "不兼容" }}</el-tag>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="触发方式" min-width="180">
          <template #default="{ row }">
            <div class="tag-list">
              <el-tag v-for="trigger in row.triggers" :key="trigger" size="small" type="info">{{ triggerLabel(trigger) }}</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="能力" width="80" align="center">
          <template #default="{ row }">{{ row.capabilities.length }}</template>
        </el-table-column>
        <el-table-column label="最近运行" width="130">
          <template #default="{ row }">
            <el-tag v-if="row.latestRun" :type="statusType(row.latestRun.status)" size="small">{{ statusLabel(row.latestRun.status) }}</el-tag>
            <span v-else class="muted">暂无</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="190" fixed="right">
          <template #default="{ row }">
            <div class="row-actions">
              <el-button @click="openDetail(row.id)">详情</el-button>
              <el-button
                :type="row.enabled ? 'danger' : 'primary'"
                plain
                :loading="changingId === row.id"
                :disabled="!row.compatible && !row.enabled"
                @click="toggle(row)"
              >
                {{ row.enabled ? "禁用" : "启用" }}
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue"
import { useRouter } from "vue-router"
import { ElMessage, ElMessageBox } from "element-plus"
import { Refresh } from "@element-plus/icons-vue"
import { fetchSkills, resolveCharacterId, setSkillEnabled } from "./api"
import type { RunStatus, SkillTrigger, SkillView } from "./types"

const router = useRouter()
const loading = ref(false)
const loadError = ref("")
const changingId = ref("")
const characterId = ref("")
const skills = ref<SkillView[]>([])
const filters = reactive<{ enabled?: boolean; trigger: SkillTrigger | ""; source: string }>({ enabled: undefined, trigger: "", source: "" })

async function load() {
  loading.value = true
  loadError.value = ""
  try {
    if (!characterId.value) characterId.value = await resolveCharacterId()
    if (!characterId.value) throw new Error("请先创建或选择角色")
    skills.value = await fetchSkills(characterId.value, filters)
  } catch (error: any) {
    loadError.value = error?.response?.data?.detail || error?.detail || error?.message || "技能列表加载失败"
  } finally {
    loading.value = false
  }
}

function openDetail(id: string) {
  router.push(`/extensions/skills/${encodeURIComponent(id)}`)
}

async function toggle(skill: SkillView) {
  if (skill.enabled) {
    await ElMessageBox.confirm(`禁用后，模型将不能再调用“${skill.name}”。历史记录和授权不会删除。`, "确认禁用技能", { type: "warning", confirmButtonText: "禁用", cancelButtonText: "取消" })
  }
  changingId.value = skill.id
  try {
    await setSkillEnabled(skill.id, !skill.enabled)
    ElMessage.success(skill.enabled ? "技能已禁用" : "技能已启用")
    await load()
  } finally {
    changingId.value = ""
  }
}

function sourceLabel(source: string) {
  if (source === "legacy_tool") return "旧工具适配"
  if (source === "builtin") return "内置技能"
  if (source === "workflow") return "工坊工作流"
  return source
}

function triggerLabel(trigger: SkillTrigger) {
  return { llm: "模型", manual: "手动", schedule: "定时", system_event: "系统事件" }[trigger]
}

function statusLabel(status: RunStatus) {
  return { pending: "等待中", running: "运行中", succeeded: "成功", failed: "失败", cancelled: "已取消", timed_out: "超时", partially_succeeded: "部分成功" }[status]
}

function statusType(status: RunStatus) {
  if (status === "succeeded") return "success"
  if (status === "running" || status === "pending" || status === "partially_succeeded") return "warning"
  if (status === "failed" || status === "timed_out") return "danger"
  return "info"
}

onMounted(load)
</script>

<style scoped>
.extension-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.page-header h1 {
  color: var(--ac-color-text);
  font-size: 24px;
  line-height: 32px;
}

.page-header p {
  margin-top: 6px;
  color: var(--ac-color-text-secondary);
  line-height: 1.6;
}

.filter-card :deep(.el-card__body) {
  padding-bottom: 2px;
}

.filter-card :deep(.el-form-item) {
  min-width: 180px;
  margin-right: 16px;
}

.table-card :deep(.el-card__body) {
  padding: 0;
  overflow-x: auto;
}

.skill-link {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  min-height: 44px;
  padding: 4px 0;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  text-align: left;
}

.skill-link:focus-visible {
  outline: 2px solid var(--ac-color-primary);
  outline-offset: 2px;
}

.skill-name {
  color: var(--ac-color-text);
  font-weight: 600;
}

.skill-link code,
.muted {
  color: var(--ac-color-text-muted);
  font-size: var(--ac-font-size-xs);
}

.tag-list,
.row-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.load-error {
  flex-shrink: 0;
}

@media (max-width: 720px) {
  .page-header {
    align-items: stretch;
    flex-direction: column;
  }

  .filter-card :deep(.el-form),
  .filter-card :deep(.el-form-item) {
    display: block;
    width: 100%;
  }
}
</style>
