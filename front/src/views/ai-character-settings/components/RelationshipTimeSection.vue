<template>
  <section v-if="availability !== 'unavailable'" class="section-card relationship-time-section" aria-labelledby="relationship-time-title">
    <div class="section-heading">
      <div>
        <div id="relationship-time-title" class="section-title">关系时间</div>
        <div class="section-subtitle">让角色理解相识时长、互动节奏与久别重逢，同时保持关系状态连续。</div>
      </div>
      <el-tag v-if="availability === 'available'" type="success" effect="plain">服务可用</el-tag>
      <el-tag v-else-if="availability === 'error'" type="danger" effect="plain">加载失败</el-tag>
      <el-tag v-else type="info" effect="plain">正在确认</el-tag>
    </div>

    <el-skeleton v-if="loading" :rows="4" animated />

    <el-alert
      v-else-if="availability === 'error'"
      :title="loadError || '关系时间设置加载失败'"
      description="请确认后端服务正常后重试。"
      type="error"
      show-icon
      :closable="false"
    >
      <template #default>
        <el-button class="retry-button" @click="load">重新加载</el-button>
      </template>
    </el-alert>

    <template v-else>
      <el-form label-position="top" :model="settings" class="settings-grid">
        <el-form-item label="关系时间感知">
          <el-switch v-model="settings.enabled" active-text="开启" inactive-text="关闭" />
          <div class="field-help">关闭后不向角色注入相识时长、互动节奏或连续性信息。</div>
        </el-form-item>
        <el-form-item label="久别重逢">
          <el-switch v-model="settings.reunionEnabled" :disabled="!settings.enabled" active-text="开启" inactive-text="关闭" />
          <div class="field-help">只在符合节奏偏差与重逢规则时表达，不会因自然离线降低关系值。</div>
        </el-form-item>
        <el-form-item label="表达强度">
          <el-radio-group v-model="settings.sensitivity" :disabled="!settings.enabled">
            <el-radio-button value="conservative">克制</el-radio-button>
            <el-radio-button value="balanced">平衡</el-radio-button>
            <el-radio-button value="expressive">鲜明</el-radio-button>
          </el-radio-group>
          <div class="field-help">控制时间感受的表达程度，不改变关系状态本身。</div>
        </el-form-item>
        <el-form-item label="最多重逢表达句数">
          <el-input-number v-model="settings.maxMentionSentences" :min="0" :max="2" :disabled="!settings.enabled || !settings.reunionEnabled" />
          <div class="field-help">设为 0 时保留重逢识别，但不生成重逢表达。</div>
        </el-form-item>
        <el-form-item label="允许提及相识时长">
          <el-switch v-model="settings.allowRelationshipAge" :disabled="!settings.enabled" />
        </el-form-item>
        <el-form-item label="允许提及重逢">
          <el-switch v-model="settings.allowReunionMention" :disabled="!settings.enabled || !settings.reunionEnabled" />
        </el-form-item>
      </el-form>

      <div class="status-panel" aria-live="polite">
        <div class="status-heading">
          <div>
            <div class="status-title">当前关系状态</div>
            <div class="field-help">状态来自已成功提交的互动，不使用仅被观察到但失败的请求。</div>
          </div>
          <el-button :loading="stateLoading" @click="loadState">刷新状态</el-button>
        </div>
        <el-alert v-if="stateError" :title="stateError" type="error" show-icon :closable="false" />
        <el-descriptions v-else-if="state" :column="2" border size="small">
          <el-descriptions-item label="首次互动">{{ formatTime(state.firstInteractionAt) }}</el-descriptions-item>
          <el-descriptions-item label="关系持续">{{ state.relationshipAgeDays }} 天</el-descriptions-item>
          <el-descriptions-item label="最近成功互动">{{ formatTime(state.lastSuccessfulExchangeAt) }}</el-descriptions-item>
          <el-descriptions-item label="期望互动间隔">{{ formatDuration(state.expectedGapSeconds) }}</el-descriptions-item>
          <el-descriptions-item label="连续性">{{ formatScore(state.continuityScore) }}</el-descriptions-item>
          <el-descriptions-item label="重新适应剩余回合">{{ state.reacclimationTurnsLeft }}</el-descriptions-item>
        </el-descriptions>
        <el-empty v-else description="尚未产生可展示的关系时间状态" :image-size="48" />
      </div>

      <div class="section-actions">
        <el-button type="primary" :loading="saving" :disabled="!characterId" @click="save">保存关系时间设置</el-button>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { getRelationshipTimeSettings, getRelationshipTimeState, getTemporalDiagnostics, updateRelationshipTimeSettings, type RelationshipTimeContext, type RelationshipTimeSettings } from "@/api/temporal"

const props = defineProps<{ characterId: string }>()
const loading = ref(false)
const saving = ref(false)
const stateLoading = ref(false)
const availability = ref<"checking" | "available" | "unavailable" | "error">("checking")
const loadError = ref("")
const stateError = ref("")
const state = ref<RelationshipTimeContext | null>(null)
const settings = reactive<RelationshipTimeSettings>({
  characterId: "",
  enabled: true,
  reunionEnabled: true,
  sensitivity: "balanced",
  allowMemoryRecall: true,
  allowRelationshipAge: true,
  allowReunionMention: true,
  allowProactiveReference: true,
  maxMentionSentences: 1,
})

async function load() {
  if (!props.characterId) {
    availability.value = "unavailable"
    return
  }
  loading.value = true
  loadError.value = ""
  availability.value = "checking"
  const [settingsResult, diagnosticsResult] = await Promise.allSettled([
    getRelationshipTimeSettings(props.characterId),
    getTemporalDiagnostics(props.characterId),
  ])
  const flagEnabled = diagnosticsResult.status === "fulfilled"
    ? diagnosticsResult.value.core.featureFlags.relationshipTimeEnabled
    : undefined
  if (settingsResult.status === "fulfilled") {
    Object.assign(settings, settingsResult.value)
    settings.characterId = props.characterId
    availability.value = flagEnabled === false ? "unavailable" : "available"
  } else if (flagEnabled === false) {
    availability.value = "unavailable"
  } else {
    availability.value = "error"
    loadError.value = "无法读取关系时间设置"
  }
  loading.value = false
  if (availability.value === "available") await loadState()
}

async function loadState() {
  if (!props.characterId || availability.value !== "available") return
  stateLoading.value = true
  stateError.value = ""
  try {
    state.value = await getRelationshipTimeState(props.characterId)
  } catch {
    state.value = null
    stateError.value = "关系状态加载失败，请重试"
  } finally {
    stateLoading.value = false
  }
}

async function save() {
  if (!props.characterId || availability.value !== "available") return
  saving.value = true
  try {
    settings.characterId = props.characterId
    Object.assign(settings, await updateRelationshipTimeSettings(props.characterId, settings))
    ElMessage.success("关系时间设置已保存")
    await loadState()
  } catch {
    ElMessage.error("关系时间设置保存失败")
  } finally {
    saving.value = false
  }
}

function formatTime(value?: string) {
  if (!value) return "—"
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(date)
}

function formatDuration(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return "—"
  const days = Math.floor(seconds / 86400)
  const hours = Math.round((seconds % 86400) / 3600)
  if (days > 0) return hours > 0 ? `${days} 天 ${hours} 小时` : `${days} 天`
  return `${Math.max(1, hours)} 小时`
}

function formatScore(value: number) {
  if (!Number.isFinite(value)) return "—"
  return `${Math.round(Math.max(0, Math.min(1, value)) * 100)}%`
}

watch(() => props.characterId, load, { immediate: true })
</script>

<style scoped>
.relationship-time-section{display:flex;flex-direction:column;gap:16px}.section-heading,.status-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:16px}.section-title,.status-title{margin-bottom:4px}.section-subtitle,.field-help{font-size:12px;line-height:1.5;color:var(--ac-color-text-muted)}.settings-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:4px 20px}.status-panel{display:flex;flex-direction:column;gap:12px;padding-top:16px;border-top:1px solid var(--ac-color-border-light)}.section-actions{display:flex;justify-content:flex-end}.retry-button{margin-top:12px}.el-button,.el-input-number{min-height:44px}.el-radio-group{display:flex;flex-wrap:wrap}.el-radio-button{min-height:44px}.el-radio-button :deep(.el-radio-button__inner){min-height:44px;display:flex;align-items:center}.el-form-item{min-width:0;margin-bottom:12px}@media(max-width:700px){.section-heading,.status-heading{flex-direction:column}.settings-grid{grid-template-columns:1fr}.section-actions{justify-content:stretch}.section-actions .el-button{width:100%}:deep(.el-descriptions__body .el-descriptions__table){display:block}:deep(.el-descriptions__cell){display:block;width:100%}}
</style>
