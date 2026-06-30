<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="debug-panel">
    <div class="page-header">
      <h2>AI 生活状态调试面板</h2>
      <el-button type="primary" @click="loadAll" :loading="loading">刷新</el-button>
      <el-button type="warning" @click="regenerateAll" :loading="regenerating">重新生成全部</el-button>
      <el-button type="success" @click="triggerDailyRegen" :loading="triggeringDaily">触发每日重生</el-button>
    </div>

    <!-- 当前状态 -->
    <el-card class="debug-card" shadow="hover">
      <template #header><span>当前状态</span></template>
      <el-descriptions v-if="data.currentState" :column="3" border size="small">
        <el-descriptions-item label="当前状态">
          <el-tag :type="stateTagType(data.currentState.currentState)">{{ stateLabel(data.currentState.currentState) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="开始时间">{{ data.currentState.stateStartedAt }}</el-descriptions-item>
        <el-descriptions-item label="结束时间">{{ data.currentState.stateEndsAt || '—' }}</el-descriptions-item>
      </el-descriptions>
      <el-empty v-else description="无状态数据" :image-size="40" />
    </el-card>

    <!-- 今日作息 -->
    <el-card class="debug-card" shadow="hover">
      <template #header>
        <span>今日作息</span>
        <el-button size="small" style="float:right" @click="regenerateAll" :loading="regenerating">重新生成作息</el-button>
      </template>
      <el-descriptions v-if="data.todaySchedule" :column="3" border size="small">
        <el-descriptions-item label="起床">{{ data.todaySchedule.wakeTime?.slice(11,16) || data.todaySchedule.wakeTime }}</el-descriptions-item>
        <el-descriptions-item label="午饭">{{ data.todaySchedule.lunchTime?.slice(11,16) || data.todaySchedule.lunchTime }}</el-descriptions-item>
        <el-descriptions-item label="晚饭">{{ data.todaySchedule.dinnerTime?.slice(11,16) || data.todaySchedule.dinnerTime }}</el-descriptions-item>
        <el-descriptions-item label="午睡">{{ data.todaySchedule.hasNap ? (data.todaySchedule.napStartTime?.slice(11,16)+'~'+data.todaySchedule.napEndTime?.slice(11,16)) : '无' }}</el-descriptions-item>
        <el-descriptions-item label="睡觉">{{ data.todaySchedule.sleepTime?.slice(11,16) || data.todaySchedule.sleepTime }}</el-descriptions-item>
        <el-descriptions-item label="休息日">{{ data.todaySchedule.isRestDay ? '是' : '否' }}</el-descriptions-item>
      </el-descriptions>
      <el-empty v-else description="无作息数据" :image-size="40" />
    </el-card>

    <!-- 状态时间轴 -->
    <el-card class="debug-card" shadow="hover">
      <template #header><span>今日状态时间轴</span></template>
      <el-table v-if="data.timeline?.length" :data="data.timeline" size="small" stripe max-height="400">
        <el-table-column prop="startTime" label="开始" width="160">
          <template #default="{row}">{{ (row.startTime||'').slice(11,16) }}</template>
        </el-table-column>
        <el-table-column prop="endTime" label="结束" width="160">
          <template #default="{row}">{{ (row.endTime||'').slice(11,16) }}</template>
        </el-table-column>
        <el-table-column prop="state" label="状态" width="120">
          <template #default="{row}">
            <el-tag :type="stateTagType(row.state)" size="small">{{ stateLabel(row.state) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="sourceType" label="来源" width="100" />
        <el-table-column prop="priority" label="优先级" width="80" />
        <el-table-column prop="reason" label="原因" min-width="160" show-overflow-tooltip />
      </el-table>
      <el-empty v-else description="无时间轴数据" :image-size="40" />
    </el-card>

    <!-- 主动消息任务 -->
    <el-card class="debug-card" shadow="hover">
      <template #header>
        <span>今日主动消息任务</span>
        <el-button size="small" style="float:right;margin-left:8px" @click="triggerActiveMsg" :loading="triggeringActive">手动触发执行器</el-button>
      </template>
      <el-table v-if="data.activeMessageTasks?.length" :data="data.activeMessageTasks" size="small" stripe max-height="300">
        <el-table-column prop="taskType" label="类型" width="120" />
        <el-table-column prop="dueTime" label="计划时间" width="160">
          <template #default="{row}">{{ (row.dueTime||'').slice(11,16) }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{row}">
            <el-tag :type="taskStatusTag(row.status)" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="cancelReason" label="取消原因" min-width="140" show-overflow-tooltip />
        <el-table-column prop="retryCount" label="重试" width="60" />
        <el-table-column label="操作" width="140">
          <template #default="{row}">
            <el-button size="small" text type="primary" @click="runTask(row.id)" v-if="row.status==='PENDING'">执行</el-button>
            <el-button size="small" text type="danger" @click="cancelTask(row.id)" v-if="row.status==='PENDING'">取消</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else description="无主动消息任务" :image-size="40" />
    </el-card>

    <!-- 延迟回复任务 -->
    <el-card class="debug-card" shadow="hover">
      <template #header>
        <span>延迟回复任务</span>
        <el-button size="small" style="float:right" @click="triggerDelayedReply" :loading="triggeringDelayed">手动触发处理器</el-button>
      </template>
      <el-table v-if="data.delayedReplies?.length" :data="data.delayedReplies" size="small" stripe max-height="300">
        <el-table-column prop="triggerState" label="触发状态" width="110" />
        <el-table-column prop="userMessage" label="用户消息" min-width="160" show-overflow-tooltip>
          <template #default="{row}">{{ (row.userMessage||'').slice(0,40) }}{{ (row.userMessage||'').length>40?'...':'' }}</template>
        </el-table-column>
        <el-table-column prop="expectedReplyAfter" label="预计回复" width="160" />
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{row}">
            <el-tag :type="taskStatusTag(row.status)" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80">
          <template #default="{row}">
            <el-button size="small" text type="danger" @click="cancelDelayed(row.id)" v-if="row.status==='PENDING'">取消</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else description="无延迟回复任务" :image-size="40" />
    </el-card>

    <!-- 最近规则日志 -->
    <el-card class="debug-card" shadow="hover">
      <template #header><span>最近规则日志</span></template>
      <el-table v-if="data.recentRuleLogs?.length" :data="data.recentRuleLogs" size="small" stripe max-height="300">
        <el-table-column prop="created_at" label="时间" width="160" />
        <el-table-column prop="rule_name" label="规则" width="140" />
        <el-table-column prop="target_type" label="目标" width="100" />
        <el-table-column prop="action" label="动作" width="140">
          <template #default="{row}">
            <el-tag size="small">{{ row.action }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="原因" min-width="200" show-overflow-tooltip />
      </el-table>
      <el-empty v-else description="无规则日志" :image-size="40" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, inject, type Ref } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { useApi } from "../../composables/useApi"

const injectedCharacterId = inject<Ref<string | null>>('currentCharacterId', ref(null))
const { get, post } = useApi()
const loading = ref(false)
const regenerating = ref(false)
const triggeringActive = ref(false)
const triggeringDelayed = ref(false)
const triggeringDaily = ref(false)

const data = reactive<any>({
  currentState: null,
  todaySchedule: null,
  timeline: [],
  activeMessageTasks: [],
  delayedReplies: [],
  recentRuleLogs: [],
})

async function loadAll() {
  loading.value = true
  try {
    const res = await get<any>("/api/companion/debug/overview", { characterId: injectedCharacterId?.value ?? undefined })
    Object.assign(data, res)
  } catch {
    ElMessage.error("加载失败")
  } finally {
    loading.value = false
  }
}

async function regenerateAll() {
  try {
    await ElMessageBox.confirm("将重新生成今日作息、状态时间轴和主动消息任务，确定？", "确认", { type: "warning" })
  } catch { return }
  regenerating.value = true
  try {
    await post(`/api/companion/debug/regenerate-all?characterId=${injectedCharacterId?.value ?? ''}`)
    ElMessage.success("已重新生成")
    await loadAll()
  } catch {
    ElMessage.error("重新生成失败")
  } finally {
    regenerating.value = false
  }
}

async function triggerActiveMsg() {
  try {
    await ElMessageBox.confirm("将立即处理所有待发送的主动消息任务，确定？", "确认", { type: "warning" })
  } catch { return }
  triggeringActive.value = true
  try {
    await post(`/api/companion/debug/process-active-messages?characterId=${injectedCharacterId?.value ?? ''}`)
    ElMessage.success("主动消息处理器已触发")
    await loadAll()
  } catch {
    ElMessage.error("触发失败")
  } finally {
    triggeringActive.value = false
  }
}

async function triggerDelayedReply() {
  try {
    await ElMessageBox.confirm("将立即处理所有到期的延迟回复，确定？", "确认", { type: "warning" })
  } catch { return }
  triggeringDelayed.value = true
  try {
    await post(`/api/companion/debug/process-delayed-replies?characterId=${injectedCharacterId?.value ?? ''}`)
    ElMessage.success("延迟回复处理器已触发")
    await loadAll()
  } catch {
    ElMessage.error("触发失败")
  } finally {
    triggeringDelayed.value = false
  }
}

async function triggerDailyRegen() {
  try {
    await ElMessageBox.confirm("将触发每日自动重生逻辑（取消旧任务+生成新任务），确定？", "确认", { type: "warning" })
  } catch { return }
  triggeringDaily.value = true
  try {
    await post(`/api/companion/debug/trigger-daily-regeneration?characterId=${injectedCharacterId?.value ?? ''}`)
    ElMessage.success("每日重生已触发")
    await loadAll()
  } catch {
    ElMessage.error("触发失败")
  } finally {
    triggeringDaily.value = false
  }
}

async function runTask(id: number) {
  try {
    await post(`/api/companion/active-message/tasks/${id}/run?characterId=${injectedCharacterId?.value ?? ''}`)
    ElMessage.success("任务已执行")
    await loadAll()
  } catch {
    ElMessage.error("执行失败")
  }
}

async function cancelTask(id: number) {
  try {
    await post(`/api/companion/active-message/tasks/${id}/cancel?characterId=${injectedCharacterId?.value ?? ''}`)
    ElMessage.success("已取消")
    await loadAll()
  } catch {
    ElMessage.error("取消失败")
  }
}

async function cancelDelayed(id: number) {
  try {
    await post(`/api/companion/delayed-replies/${id}/cancel?characterId=${injectedCharacterId?.value ?? ''}`)
    ElMessage.success("已取消")
    await loadAll()
  } catch {
    ElMessage.error("取消失败")
  }
}

function stateLabel(s: string) {
  const m: Record<string,string> = {
    SLEEPING:"睡觉",WAKING_UP:"刚醒",IDLE:"空闲",EATING_BREAKFAST:"早饭",EATING_LUNCH:"午饭",NAPPING:"午睡",
    EATING_DINNER:"晚饭",BEFORE_SLEEP:"睡前",PREPARING_CLASS:"准备上课",IN_CLASS:"上课中",AFTER_CLASS:"下课",
    STUDYING:"学习",BUSY:"忙碌",PREPARING_WORK:"准备上班",COMMUTING_TO_WORK:"上班路上",WORKING:"工作中",
    LUNCH_BREAK:"午休",COMMUTING_HOME:"下班路上",AFTER_WORK:"下班后",EXAM_WEEK:"考试周",EXAM_PREPARING:"备考",
    IN_EXAM:"考试中",AFTER_EXAM:"考完",PART_TIME_PREPARE:"准备兼职",PART_TIME_WORKING:"兼职中",
    PART_TIME_AFTER:"兼职结束",WORKOUT_PREPARE:"准备健身",WORKING_OUT:"健身中",AFTER_WORKOUT:"健身结束",
    LIBRARY_STUDYING:"图书馆",LIBRARY_BREAK:"图书馆休息",SICK_RESTING:"生病",LOW_ENERGY:"低精力",
    OVERTIME:"加班",OVERTIME_BREAK:"加班休息",AFTER_OVERTIME:"加班结束",LOW_ENERGY_AFTER_WORK:"下班后低精力",
  }
  return m[s] || s
}

function stateTagType(s: string) {
  if (!s) return "info"
  if (s==="SLEEPING"||s==="NAPPING") return "info"
  if (s.includes("CLASS")||s.includes("EXAM")||s.includes("STUDY")||s.includes("LIBRARY")) return "warning"
  if (s.includes("WORK")||s==="OVERTIME"||s.includes("PART_TIME")) return "danger"
  if (s==="IDLE"||s.includes("AFTER_")||s.includes("BREAK")) return "success"
  return "info"
}

function taskStatusTag(s: string) {
  if (s==="SUCCESS") return "success"
  if (s==="FAILED"||s==="CANCELLED") return "danger"
  if (s==="RUNNING") return "warning"
  return "info"
}

onMounted(() => loadAll())
</script>

<style scoped>
.debug-panel { padding: 20px; max-width: 1100px; }
.page-header { display:flex; align-items:center; gap:12px; margin-bottom:16px; flex-wrap:wrap; }
.page-header h2 { font-size:18px; font-weight:600; margin:0; }
.debug-card { margin-bottom: 14px; }
</style>
