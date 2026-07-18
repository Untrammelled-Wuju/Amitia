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

    <el-card class="debug-card temporal-runtime-card" shadow="never" v-loading="temporalLoading">
      <template #header><div class="temporal-header"><span>Temporal Runtime</span><div class="temporal-badges"><el-tag size="small">{{ temporalDiagnostics?.snapshotVersion || '未加载' }}</el-tag><el-tag size="small" :type="relationshipFeatureEnabled ? 'success' : 'info'">Relationship Time {{ relationshipFeatureEnabled ? '已启用' : '未启用' }}</el-tag></div></div></template>
      <el-alert v-if="temporalError" :title="temporalError" type="error" show-icon :closable="false" class="temporal-alert"><el-button @click="loadTemporal">重试时间诊断</el-button></el-alert>
      <el-tabs v-else-if="temporalDiagnostics" v-model="temporalActiveTab">
        <el-tab-pane label="Core Time" name="core">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="用户当地时间">{{ formatTemporalCivil(temporalDiagnostics.snapshot.userTime) }}</el-descriptions-item>
            <el-descriptions-item label="角色当地时间">{{ formatTemporalCivil(temporalDiagnostics.snapshot.characterTime) }}</el-descriptions-item>
            <el-descriptions-item label="时区是否不同">{{ temporalDiagnostics.snapshot.signals.timezoneDiffers ? '是' : '否' }}</el-descriptions-item>
            <el-descriptions-item label="用户安静时段">{{ temporalDiagnostics.snapshot.signals.quietHours ? '是' : '否' }}</el-descriptions-item>
            <el-descriptions-item label="Clock 来源">{{ temporalDiagnostics.core.clockSource }}</el-descriptions-item>
            <el-descriptions-item label="TZDB">{{ temporalDiagnostics.core.tzdb }}</el-descriptions-item>
          </el-descriptions>
        </el-tab-pane>
        <el-tab-pane label="Anchors" name="anchors">
          <el-table :data="temporalDiagnostics.snapshot.salientAnchors || []" size="small" stripe>
            <el-table-column prop="type" label="类型" width="150" /><el-table-column prop="title" label="名称" min-width="180" /><el-table-column prop="distanceDays" label="距离天数" width="100" /><el-table-column prop="salience" label="显著性" width="90" />
          </el-table>
          <el-empty v-if="!temporalDiagnostics.snapshot.salientAnchors?.length" description="当前没有显著时间锚点" :image-size="42" />
        </el-tab-pane>
        <el-tab-pane label="Relationship Time" name="relationship">
          <el-alert v-if="!relationshipFeatureEnabled" title="Relationship Time 后端功能开关未启用" description="当前不会生成关系时间状态或影响角色表达。" type="info" show-icon :closable="false" />
          <template v-else-if="relationshipContext">
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="首次互动时间">{{ formatDateTime(relationshipContext.firstInteractionAt) }}</el-descriptions-item>
              <el-descriptions-item label="关系持续天数">{{ relationshipContext.relationshipAgeDays }} 天</el-descriptions-item>
              <el-descriptions-item label="全局最后成功互动">{{ formatDateTime(relationshipContext.globalLastCommittedAt) }}</el-descriptions-item>
              <el-descriptions-item label="当前角色最后成功互动">{{ formatDateTime(relationshipContext.relationshipLastCommittedAt) }}</el-descriptions-item>
              <el-descriptions-item label="本次关系 Gap">{{ formatDuration(relationshipContext.relationshipGapSeconds) }}</el-descriptions-item>
              <el-descriptions-item label="本次全局 Gap">{{ formatDuration(relationshipContext.globalGapSeconds) }}</el-descriptions-item>
              <el-descriptions-item label="期望互动间隔">{{ formatDuration(relationshipContext.expectedGapSeconds) }}</el-descriptions-item>
              <el-descriptions-item label="Normalized Gap">{{ formatDecimal(relationshipContext.normalizedGap) }}</el-descriptions-item>
              <el-descriptions-item label="连续性分数">{{ formatPercent(relationshipContext.continuityScore) }}</el-descriptions-item>
              <el-descriptions-item label="重新适应剩余回合">{{ relationshipContext.reacclimationTurnsLeft }}</el-descriptions-item>
              <el-descriptions-item label="存储紧张度">{{ formatDecimal(relationshipContext.storedTension) }}</el-descriptions-item>
              <el-descriptions-item label="有效紧张度">{{ formatDecimal(relationshipContext.effectiveTension) }}</el-descriptions-item>
            </el-descriptions>
            <el-alert v-if="relationshipDiagnosticsList.length" title="诊断信息" type="warning" show-icon :closable="false" class="diagnostics-alert">
              <ul class="diagnostics-list"><li v-for="item in relationshipDiagnosticsList" :key="item">{{ item }}</li></ul>
            </el-alert>
          </template>
          <el-empty v-else description="功能已启用，但当前没有可展示的关系时间状态" :image-size="42" />
        </el-tab-pane>
        <el-tab-pane label="Reunion" name="reunion">
          <el-alert v-if="!relationshipFeatureEnabled" title="久别重逢不可用" description="Relationship Time 后端功能开关未启用。" type="info" show-icon :closable="false" />
          <template v-else>
            <el-descriptions v-if="relationshipContext?.reunion" :column="2" border size="small" class="reunion-current">
              <el-descriptions-item label="当前重逢类型">{{ reunionKindLabel(relationshipContext.reunion.kind) }}</el-descriptions-item>
              <el-descriptions-item label="当前重逢等级">{{ reunionLevelLabel(relationshipContext.reunion.level) }}</el-descriptions-item>
              <el-descriptions-item label="Episode 状态">{{ reunionStateLabel(relationshipContext.reunion.state) }}</el-descriptions-item>
              <el-descriptions-item label="是否允许表达">{{ relationshipContext.reunion.shouldExpress ? '是' : '否' }}</el-descriptions-item>
              <el-descriptions-item label="表达权所属 Interaction">{{ relationshipContext.reunion.claimedByInteractionId || '—' }}</el-descriptions-item>
              <el-descriptions-item label="表达权到期时间">{{ formatDateTime(relationshipContext.reunion.claimExpiresAt) }}</el-descriptions-item>
            </el-descriptions>
            <el-empty v-else description="当前没有活跃重逢事件" :image-size="42" />
            <div class="table-heading">最近重逢事件</div>
            <el-table v-if="reunionEpisodes.length" :data="reunionEpisodes" size="small" stripe>
              <el-table-column label="检测时间" min-width="160"><template #default="{ row }">{{ formatDateTime(row.detectedAtUtc) }}</template></el-table-column>
              <el-table-column label="类型" min-width="160"><template #default="{ row }">{{ reunionKindLabel(row.reunionKind) }}</template></el-table-column>
              <el-table-column label="等级" width="100"><template #default="{ row }">{{ reunionLevelLabel(row.reunionLevel) }}</template></el-table-column>
              <el-table-column label="间隔" width="130"><template #default="{ row }">{{ formatDuration(row.relationshipGapSeconds) }}</template></el-table-column>
              <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag size="small" :type="reunionStateTag(row.status)">{{ reunionStateLabel(row.status) }}</el-tag></template></el-table-column>
              <el-table-column prop="handledInteractionId" label="处理 Interaction" min-width="170" show-overflow-tooltip />
              <el-table-column prop="suppressionReason" label="抑制原因" min-width="140" show-overflow-tooltip />
            </el-table>
            <el-empty v-else description="暂无最近重逢事件" :image-size="42" />
          </template>
        </el-tab-pane>
        <el-tab-pane label="Prompt Contribution" name="prompt"><pre class="prompt-block">{{ temporalDiagnostics.promptSections?.[0]?.content || '本轮未注入时间上下文' }}</pre></el-tab-pane>
        <el-tab-pane label="Commit Effects" name="effects"><pre v-if="temporalDiagnostics.commitEffects?.length" class="json-block">{{ prettyJSON(temporalDiagnostics.commitEffects) }}</pre><el-empty v-else description="当前没有时间提交效果" :image-size="42" /></el-tab-pane>
      </el-tabs>
      <el-empty v-else description="Temporal Runtime 诊断尚未加载" :image-size="48" />
    </el-card>

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
import { computed, ref, reactive, onMounted, inject, type Ref } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { useApi } from "../../composables/useApi"
import { getRelationshipTimeDiagnostics, getTemporalDiagnostics, listReunionEpisodes, type CivilTimeSnapshot, type RelationshipTimeContext, type RelationshipTimeDiagnostics, type ReunionEpisode, type SnapshotField, type TemporalDiagnostics } from "@/api/temporal"

const injectedCharacterId = inject<Ref<string | null>>('currentCharacterId', ref(null))
const { get, post } = useApi()
const loading = ref(false)
const regenerating = ref(false)
const triggeringActive = ref(false)
const triggeringDelayed = ref(false)
const triggeringDaily = ref(false)
const temporalDiagnostics = ref<TemporalDiagnostics | null>(null)
const relationshipDiagnostics = ref<RelationshipTimeDiagnostics | null>(null)
const reunionEpisodes = ref<ReunionEpisode[]>([])
const temporalLoading = ref(false)
const temporalError = ref("")
const temporalActiveTab = ref("core")

const relationshipFeatureEnabled = computed(() => temporalDiagnostics.value?.core.featureFlags.relationshipTimeEnabled === true)
const relationshipContext = computed(() => {
  const candidates = [
    relationshipDiagnostics.value?.state,
    relationshipDiagnostics.value?.relationshipTime,
    temporalDiagnostics.value?.snapshot.relationshipTime,
    temporalDiagnostics.value?.relationshipTime,
  ]
  for (const candidate of candidates) {
    const value = unwrapRelationshipContext(candidate)
    if (value) return value
  }
  return null
})
const relationshipDiagnosticsList = computed(() => Array.from(new Set([
  ...(relationshipDiagnostics.value?.diagnostics || []),
  ...(relationshipContext.value?.diagnostics || []),
  ...(temporalDiagnostics.value?.diagnostics || []),
])))

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
    ElMessage.error("生活状态调试数据加载失败")
  } finally {
    loading.value = false
  }
  await loadTemporal()
}

async function loadTemporal() {
  temporalLoading.value = true
  temporalError.value = ""
  const characterId = injectedCharacterId?.value ?? ""
  const requests: Promise<unknown>[] = [getTemporalDiagnostics(characterId), getRelationshipTimeDiagnostics(characterId)]
  if (characterId) requests.push(listReunionEpisodes(characterId))
  const results = await Promise.allSettled(requests)
  if (results[0].status === "fulfilled") temporalDiagnostics.value = results[0].value as TemporalDiagnostics
  else {
    temporalDiagnostics.value = null
    temporalError.value = "Temporal Runtime 诊断加载失败，请检查后端服务后重试"
  }
  relationshipDiagnostics.value = results[1].status === "fulfilled" ? results[1].value as RelationshipTimeDiagnostics : null
  if (characterId && results[2]?.status === "fulfilled") {
    const result = results[2].value as ReunionEpisode[] | { items: ReunionEpisode[] }
    reunionEpisodes.value = Array.isArray(result) ? result : result.items || []
  } else {
    reunionEpisodes.value = relationshipDiagnostics.value?.episodes || []
  }
  temporalLoading.value = false
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

function formatTemporalCivil(value: CivilTimeSnapshot) {
  return `${value.localTime.replace("T", " ").slice(0, 16)} · ${value.timezone} · ${value.daypart || "无时段"}`
}

function unwrapRelationshipContext(value: RelationshipTimeContext | SnapshotField<RelationshipTimeContext> | null | undefined) {
  if (!value || typeof value !== "object") return null
  if ("version" in value && "characterId" in value) return value as RelationshipTimeContext
  const field = value as SnapshotField<RelationshipTimeContext>
  return field.value || field.data || null
}

function formatDateTime(value?: string) {
  if (!value) return "—"
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(date)
}

function formatDuration(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return "—"
  const days = Math.floor(seconds / 86400)
  const hours = Math.round((seconds % 86400) / 3600)
  return days > 0 ? `${days} 天${hours > 0 ? ` ${hours} 小时` : ""}` : `${Math.max(1, hours)} 小时`
}

function formatDecimal(value: number) { return Number.isFinite(value) ? value.toFixed(2) : "—" }
function formatPercent(value: number) { return Number.isFinite(value) ? `${Math.round(Math.max(0, Math.min(1, value)) * 100)}%` : "—" }
function reunionKindLabel(value: string) { return ({ global_return: "全局回归", relationship_reconnect: "关系重连", reply_to_recent_proactive: "回应近期主动消息" } as Record<string,string>)[value] || value || "—" }
function reunionLevelLabel(value: string) { return ({ none: "无", noticeable: "明显", long: "较久", extended: "长期", dormant: "休眠" } as Record<string,string>)[value] || value || "—" }
function reunionStateLabel(value: string) { return ({ pending: "待处理", claimed: "已认领", handled: "已处理", suppressed: "已抑制", expired: "已过期" } as Record<string,string>)[value] || value || "—" }
function reunionStateTag(value: string) { if (value === "handled") return "success"; if (value === "claimed") return "warning"; if (value === "suppressed" || value === "expired") return "info"; return "primary" }

function prettyJSON(value: unknown) { return JSON.stringify(value, null, 2) }

onMounted(() => loadAll())
</script>

<style scoped>
.debug-panel { padding: 20px; max-width: 1100px; }
.page-header { display:flex; align-items:center; gap:12px; margin-bottom:16px; flex-wrap:wrap; }
.page-header h2 { font-size:18px; font-weight:600; margin:0; }
.debug-card { margin-bottom: 14px; }
.temporal-runtime-card { border-color: var(--ac-color-border-light); }
.temporal-header,.temporal-badges { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.temporal-alert,.diagnostics-alert,.reunion-current { margin-bottom: 14px; }
.diagnostics-list { margin: 8px 0 0; padding-left: 20px; line-height: 1.6; }
.table-heading { margin: 16px 0 10px; font-weight: 600; color: var(--ac-color-text); }
.temporal-alert .el-button { min-height: 44px; margin-top: 10px; }
.prompt-block, .json-block { margin: 0; padding: 12px; border-radius: var(--ac-radius-sm); background: var(--ac-color-surface-hover); color: var(--ac-color-text); white-space: pre-wrap; overflow-wrap: anywhere; font-size: 12px; line-height: 1.6; }
@media(max-width:700px){.debug-panel{padding:12px;max-width:100%}.temporal-header{align-items:flex-start;flex-direction:column}.temporal-badges{justify-content:flex-start}:deep(.temporal-runtime-card .el-descriptions__body .el-descriptions__table){display:block}:deep(.temporal-runtime-card .el-descriptions__cell){display:block;width:100%}}
</style>
