<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="ai-char-settings">
    <div class="page-header">
      <div>
        <h2>角色性格设置</h2>
        <p class="page-desc">调整 AI 角色的回复风格、陪伴方式和安全边界</p>
      </div>
    </div>

    <div class="top-bar">
      <div class="info-row">
        <div class="info-item">
          <label>角色名称</label>
          <el-input v-model="form.name" placeholder="角色名称" style="width:200px" size="default" />
        </div>
        <div class="info-item">
          <label>角色描述</label>
          <el-input v-model="form.description" placeholder="简要描述" style="width:280px" size="default" />
        </div>
        <div class="info-item">
          <label>默认角色</label>
          <el-switch v-model="form.isDefault" @change="setAsDefault" />
        </div>
        <div class="info-item" v-if="charId">
          <label>角色 ID</label>
          <span class="char-id">{{ charId }}</span>
        </div>
      </div>
      <div class="action-row">
        <el-button @click="editCharPrompt" :loading="promptLoading">
          <el-icon><View /></el-icon> 修改角色提示词
        </el-button>
        <el-button @click="resetConfig" :loading="resetting" type="warning" plain>
          重置默认
        </el-button>
        <el-button type="primary" @click="saveConfig" :loading="saving">
          保存
        </el-button>
      </div>
    </div>

    <div class="sections">
      <RoleGenderSection v-model:genderForm="genderForm" />

      <PersonalitySlidersSection
        v-model="form.personalityConfig"
        v-model:activeCollapse="activeCollapse"
      />

      <LifestyleTendencySection :characterId="charId" />

      <SleepSettingSection v-model:sleepForm="sleepForm" />

      <LifeScenarioSection
        v-model:lifeIdentity="lifeIdentity"
        v-model:lifeIdentityCustom="lifeIdentityCustom"
        @change="onLifeIdentityChange"
      />

      <FixedEventsSection
        v-if="showCourseSection"
        :characterId="charId"
      />

      <SpecialEventsSection :characterId="charId" />

      <WorkProfileSection
        v-if="showWorkSection"
        v-model:workForm="workForm"
        :characterId="charId"
      />
    </div>

    <PromptEditorDialog
      v-model="showPromptEditor"
      v-model:editingPrompt="editingPrompt"
      :charId="charId"
      :charName="form.name"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, inject, type Ref } from "vue"
import { ElMessage } from "element-plus"
import { View } from "@element-plus/icons-vue"
import { useApi } from "../../composables/useApi"
import { useCachedApi } from "../../composables/useCachedApi"
import { useRoleProfile } from "../../composables/useRoleProfile"
import { useSleepSetting } from "../../composables/useSleepSetting"
import { useWorkProfile } from "../../composables/useWorkProfile"
import PersonalitySlidersSection from "./components/PersonalitySlidersSection.vue"
import RoleGenderSection from "./components/RoleGenderSection.vue"
import LifestyleTendencySection from "./components/LifestyleTendencySection.vue"
import SleepSettingSection from "./components/SleepSettingSection.vue"
import LifeScenarioSection from "./components/LifeScenarioSection.vue"
import FixedEventsSection from "./components/FixedEventsSection.vue"
import SpecialEventsSection from "./components/SpecialEventsSection.vue"
import WorkProfileSection from "./components/WorkProfileSection.vue"
import PromptEditorDialog from "./components/PromptEditorDialog.vue"

const { get, post } = useApi()
const { invalidateCache } = useCachedApi()

const injectedCharacterId = inject<Ref<string | null>>("currentCharacterId", ref(null))
const refreshHealth = inject<() => void>("refreshHealth", () => {})

const { updateRoleProfile } = useRoleProfile()
const { updateSleepSetting } = useSleepSetting()
const { getSleepSetting } = useSleepSetting()
const { getWorkProfile, updateWorkProfile } = useWorkProfile()
const { getRoleProfile } = useRoleProfile()

const charId = ref("")
const saving = ref(false)
const resetting = ref(false)
const promptLoading = ref(false)
const showPromptEditor = ref(false)
const activeCollapse = ref<string[]>([])
const editingPrompt = ref("")

const PRESET_IDENTITIES = ["SCHOOL", "WORK", "UNEMPLOYED", "HOME"]
const lifeIdentity = ref("CUSTOM")
const lifeIdentityCustom = ref("")
const isCustomLifeIdentity = computed(() => !PRESET_IDENTITIES.includes(lifeIdentity.value))
const showCourseSection = computed(() => lifeIdentity.value === "SCHOOL" || isCustomLifeIdentity.value)
const showWorkSection = computed(() => lifeIdentity.value === "WORK" || isCustomLifeIdentity.value)

async function onLifeIdentityChange(val: string) {
  lifeIdentity.value = val
  if (PRESET_IDENTITIES.includes(val)) {
    lifeIdentityCustom.value = ""
  }
  try {
    const payload: any = {
      lifeIdentity: isCustomLifeIdentity.value ? lifeIdentityCustom.value || lifeIdentity.value : lifeIdentity.value,
      name: form.name.trim(),
      description: form.description.trim(),
      personalityConfig: form.personalityConfig,
    }
    if (charId.value) payload.id = charId.value
    await post<any>("/api/ai/character/save", payload)
  } catch { }
}

const DEFAULT_CONFIG = {
  familiarity: 78, formality: 22, customerServiceAvoidance: 92,
  directness: 75, verbosity: 32, structureLevel: 40, shortSentence: 85, toneWords: 45,
  warmth: 58, comfortLevel: 55, preachingAvoidance: 88,
  companionship: 55, boundary: 85, dependencyAvoidance: 85,
  execution: 75, explanationDepth: 55, judgment: 75, clarification: 35,
  rationality: 50, humor: 40, initiative: 50, teasing: 30, patience: 60, dailyLimit: 3,
  intimacyExpression: 25, flirtiness: 0, romanticTone: 0,
  suggestivenessAvoidance: 100, intimacyBoundary: 90,
}

const form = reactive({
  name: "轻熟朋友",
  description: "自然、简短、有反应，有一点熟悉感，但不过度装熟。",
  isDefault: false,
  personalityConfig: { ...DEFAULT_CONFIG } as Record<string, number>,
  chatStyleConfig: null as any,
  sceneRules: null as any,
})

const genderForm = reactive({
  roleName: "小暖",
  gender: "UNSPECIFIED" as string,
  genderLabel: null as string | null,
  pronoun: "TA",
  selfReference: "我",
  userAddressingStyle: "自然称呼" as string | null,
  genderExpression: 30,
})

const sleepForm = reactive({
  sleepReplyEnabled: false,
  sleepReplyMode: "NO_REPLY",
})

const workForm = reactive({
  enabled: false,
  workDaysArr: ["MON", "TUE", "WED", "THU", "FRI"] as string[],
  workStartTime: "09:00",
  workEndTime: "18:00",
  lunchBreakStartTime: "12:00",
  lunchBreakEndTime: "13:30",
  commuteMinMinutes: 15,
  commuteMaxMinutes: 45,
  prepareMinMinutes: 20,
  prepareMaxMinutes: 60,
  replyMode: "SHORT_REPLY",
  allowOvertime: false,
  overtimeProbability: 10,
  overtimeMinMinutes: 30,
  overtimeMaxMinutes: 180,
  overtimeReplyMode: "SHORT_REPLY",
  delayedReplyEnabled: false,
  commuteHomeShareEnabled: true,
  commuteHomeShareProbability: 60,
})

onMounted(async () => {
  const cid = injectedCharacterId?.value
  if (cid) {
    try {
      const data = await get<any>("/api/characters/" + cid)
      if (data) {
        charId.value = data.id || cid
        form.name = data.name || form.name
        form.description = data.description || form.description
        form.isDefault = !!data.isDefault
        if (data.personalityConfig) {
          form.personalityConfig = { ...DEFAULT_CONFIG, ...data.personalityConfig }
        }
        if (data.lifeIdentity) {
          if (PRESET_IDENTITIES.includes(data.lifeIdentity)) {
            lifeIdentity.value = data.lifeIdentity
          } else {
            lifeIdentity.value = "CUSTOM"
            lifeIdentityCustom.value = data.lifeIdentity
          }
        }
      }
    } catch { }
  }
  if (!charId.value) {
    try {
      const data = await get<any>("/api/ai/character/default")
      if (data) {
        charId.value = data.id || ""
        form.name = data.name || form.name
        form.description = data.description || form.description
        form.isDefault = !!data.isDefault
        if (data.personalityConfig) {
          form.personalityConfig = { ...DEFAULT_CONFIG, ...data.personalityConfig }
        }
        if (data.lifeIdentity) {
          if (PRESET_IDENTITIES.includes(data.lifeIdentity)) {
            lifeIdentity.value = data.lifeIdentity
          } else {
            lifeIdentity.value = "CUSTOM"
            lifeIdentityCustom.value = data.lifeIdentity
          }
        }
      }
    } catch { }
    try {
      const chars = await get<any[]>("/api/characters?includeDisabled=true")
      const active = chars.find((c: any) => c.isActive) || chars.find((c: any) => c.isDefault) || chars[0]
      if (active) {
        charId.value = active.id
        form.name = active.name || form.name
        form.description = active.description || form.description
        if (active.lifeIdentity) {
          if (PRESET_IDENTITIES.includes(active.lifeIdentity)) {
            lifeIdentity.value = active.lifeIdentity
          } else {
            lifeIdentity.value = "CUSTOM"
            lifeIdentityCustom.value = active.lifeIdentity
          }
        }
      }
    } catch { }
  }

  try {
    const rp = await getRoleProfile(injectedCharacterId?.value ?? undefined)
    if (rp) {
      genderForm.roleName = rp.roleName || "小暖"
      genderForm.gender = rp.gender || "UNSPECIFIED"
      genderForm.genderLabel = rp.genderLabel
      genderForm.pronoun = rp.pronoun || "TA"
      genderForm.selfReference = rp.selfReference || "我"
      genderForm.userAddressingStyle = rp.userAddressingStyle
      genderForm.genderExpression = rp.genderExpression ?? 30
    }
  } catch { }

  try {
    const ss = await getSleepSetting(injectedCharacterId?.value ?? undefined)
    if (ss) {
      sleepForm.sleepReplyEnabled = ss.sleepReplyEnabled
      sleepForm.sleepReplyMode = ss.sleepReplyMode
    }
  } catch { }

  try {
    const wp = await getWorkProfile(injectedCharacterId?.value ?? undefined)
    if (wp) {
      workForm.enabled = wp.enabled
      workForm.workDaysArr = wp.workDays ? wp.workDays.split(",") : ["MON","TUE","WED","THU","FRI"]
      workForm.workStartTime = wp.workStartTime
      workForm.workEndTime = wp.workEndTime
      workForm.lunchBreakStartTime = wp.lunchBreakStartTime
      workForm.lunchBreakEndTime = wp.lunchBreakEndTime
      workForm.commuteMinMinutes = wp.commuteMinMinutes
      workForm.commuteMaxMinutes = wp.commuteMaxMinutes
      workForm.prepareMinMinutes = wp.prepareMinMinutes
      workForm.prepareMaxMinutes = wp.prepareMaxMinutes
      workForm.replyMode = wp.replyMode
      workForm.allowOvertime = wp.allowOvertime
      workForm.overtimeProbability = wp.overtimeProbability
      workForm.overtimeMinMinutes = wp.overtimeMinMinutes
      workForm.overtimeMaxMinutes = wp.overtimeMaxMinutes
      workForm.overtimeReplyMode = wp.overtimeReplyMode
      workForm.delayedReplyEnabled = wp.delayedReplyEnabled
      workForm.commuteHomeShareEnabled = wp.commuteHomeShareEnabled
      workForm.commuteHomeShareProbability = wp.commuteHomeShareProbability
    }
  } catch { }
})

async function saveConfig() {
  saving.value = true
  try {
    const payload: any = {
      name: form.name.trim(),
      description: form.description.trim(),
      personalityConfig: form.personalityConfig,
      isDefault: form.isDefault,
      lifeIdentity: isCustomLifeIdentity.value ? lifeIdentityCustom.value || lifeIdentity.value : lifeIdentity.value,
    }
    if (charId.value) payload.id = charId.value
    const result = await post<any>("/api/ai/character/save", payload)
    if (result?.id) charId.value = result.id

    try {
      await updateRoleProfile({
        roleName: form.name.trim(),
        gender: genderForm.gender,
        genderLabel: genderForm.gender === "CUSTOM" ? genderForm.genderLabel : null,
        pronoun: genderForm.pronoun,
        selfReference: genderForm.selfReference,
        userAddressingStyle: genderForm.userAddressingStyle,
        genderExpression: genderForm.genderExpression,
      }, injectedCharacterId?.value ?? undefined)
    } catch (e: any) { console.warn("Role profile save failed:", e) }

    try {
      await updateSleepSetting({
        sleepReplyEnabled: sleepForm.sleepReplyEnabled,
        sleepReplyMode: sleepForm.sleepReplyMode,
      }, injectedCharacterId?.value ?? undefined)
    } catch (e: any) { console.warn("Sleep setting save failed:", e) }

    try {
      await updateWorkProfile({
        enabled: workForm.enabled,
        workDays: workForm.workDaysArr.join(","),
        workStartTime: workForm.workStartTime,
        workEndTime: workForm.workEndTime,
        lunchBreakStartTime: workForm.lunchBreakStartTime,
        lunchBreakEndTime: workForm.lunchBreakEndTime,
        commuteMinMinutes: workForm.commuteMinMinutes,
        commuteMaxMinutes: workForm.commuteMaxMinutes,
        prepareMinMinutes: workForm.prepareMinMinutes,
        prepareMaxMinutes: workForm.prepareMaxMinutes,
        replyMode: workForm.replyMode,
        allowOvertime: workForm.allowOvertime,
        overtimeProbability: workForm.overtimeProbability,
        overtimeMinMinutes: workForm.overtimeMinMinutes,
        overtimeMaxMinutes: workForm.overtimeMaxMinutes,
        overtimeReplyMode: workForm.overtimeReplyMode,
        delayedReplyEnabled: workForm.delayedReplyEnabled,
        commuteHomeShareEnabled: workForm.commuteHomeShareEnabled,
        commuteHomeShareProbability: workForm.commuteHomeShareProbability,
      } as any, injectedCharacterId?.value ?? undefined)
    } catch (e: any) { console.warn("Work profile save failed:", e) }

    ElMessage.success("保存成功")
  } catch { } finally {
    saving.value = false
  }
}

async function setAsDefault(val: boolean) {
  if (!charId.value) return
  try {
    if (val) {
      await post<any>("/api/ai/character/" + charId.value + "/set-default")
      ElMessage.success("已设为默认角色")
    } else {
      await post<any>("/api/ai/character/reset-default")
      ElMessage.success("已取消默认角色")
    }
    if (val) {
      localStorage.setItem("uai-default-char", JSON.stringify({
        id: charId.value,
        name: form.name,
        identity: form.personalityConfig?.identity || form.description || "",
        updatedAt: Date.now(),
      }))
    } else {
      localStorage.removeItem("uai-default-char")
    }
    invalidateCache("_api_characters")
    localStorage.removeItem("webchat-char-id")
    window.dispatchEvent(new CustomEvent("default-char-changed"))
    refreshHealth()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg || "操作失败")
    form.isDefault = !val
  }
}

async function resetConfig() {
  resetting.value = true
  try {
    if (charId.value) {
      const result = await post<any>("/api/ai/character/reset-default", { id: charId.value })
      if (result) {
        form.name = result.name || form.name
        form.description = result.description || form.description
        if (result.personalityConfig) {
          form.personalityConfig = { ...DEFAULT_CONFIG, ...result.personalityConfig }
        }
      }
    } else {
      form.personalityConfig = { ...DEFAULT_CONFIG }
      form.name = "轻熟朋友"
      form.description = "自然、简短、有反应，有一点熟悉感，但不过度装熟。"
    }
    ElMessage.success("已重置为默认配置")
  } catch {
    ElMessage.warning("重置失败，请先保存角色后再试")
  } finally {
    resetting.value = false
  }
}

async function editCharPrompt() {
  promptLoading.value = true
  try {
    if (charId.value) {
      const char = await get<any>("/api/ai/character/" + charId.value)
      editingPrompt.value = char?.basePrompt || ""
    } else {
      editingPrompt.value = ""
      ElMessage.info("请先保存角色后再编辑提示词")
    }
    showPromptEditor.value = true
  } catch {
    ElMessage.error("加载提示词失败")
  } finally {
    promptLoading.value = false
  }
}
</script>

<style scoped>
.ai-char-settings {
  padding: 20px 24px;
  max-width: 900px;
}

.page-header h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 4px;
}

.page-desc {
  font-size: 13px;
  color: var(--ac-color-text-muted);
  margin: 0;
}

.top-bar {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  padding: 14px 16px;
  margin: 16px 0;
}

.info-row {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
}

.info-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.info-item label {
  font-size: 13px;
  color: var(--ac-color-text-secondary);
  white-space: nowrap;
}

.char-id {
  font-size: 12px;
  font-family: monospace;
  color: var(--ac-color-text-muted);
}

.action-row {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--ac-color-border-light);
}

.sections {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

@media (max-width: 700px) {
  .info-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
}
</style>

<style>
.section-card {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  padding: 14px 16px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 12px;
  color: var(--ac-color-text);
}

.section-collapse {
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-surface);
}

.section-collapse .el-collapse-item__header {
  font-size: 14px;
  font-weight: 600;
  padding: 14px 16px;
}

.section-collapse .el-collapse-item__wrap {
  padding: 0 16px 14px;
}

.slider-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px 20px;
}

@media (max-width: 700px) {
  .slider-grid {
    grid-template-columns: 1fr;
  }
}

.slider-hint {
  grid-column: 1 / -1;
  font-size: 10px;
  color: var(--ac-color-text-muted);
  padding: 2px 0 6px;
  border-bottom: 1px solid var(--ac-color-border-light);
  margin-bottom: 2px;
}

.gender-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px 20px;
}

.gender-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.gender-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ac-color-text);
  display: flex;
  align-items: center;
  gap: 8px;
}

.gender-value {
  font-size: 11px;
  font-weight: 700;
  color: var(--ac-color-primary);
}

.gender-hint {
  font-size: 11px;
  color: var(--ac-color-text-placeholder);
  line-height: 1.3;
}

@media (max-width: 700px) {
  .gender-grid {
    grid-template-columns: 1fr;
  }
}

.sr-body {
  display: flex;
  align-items: center;
  gap: 6px;
}

.sr-left, .sr-right {
  font-size: 10px;
  color: var(--ac-color-text-placeholder);
  min-width: 24px;
}

.sr-left { text-align: right; }

.sr-slider-wrap {
  flex: 1;
}

.sr-slider {
  --el-slider-height: 4px;
}

.course-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid var(--ac-color-border-light);
}
.course-item:last-child { border-bottom: none; }
.course-info { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
.course-title { font-size: 13px; font-weight: 500; }
.course-time { font-size: 12px; color: var(--ac-color-text-secondary); }
.course-days { font-size: 11px; color: var(--ac-color-text-muted); }
.course-reply { font-size: 11px; color: var(--ac-color-primary); }
.course-actions { display: flex; gap: 6px; align-items: center; }

.course-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.form-item { display: flex; flex-direction: column; gap: 4px; }
.form-item label { font-size: 12px; color: var(--ac-color-text-secondary); }
.form-row { display: flex; gap: 12px; }

.work-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px 20px;
}
.work-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
@media (max-width: 700px) {
  .work-grid { grid-template-columns: 1fr; }
}

.sleep-setting-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px 20px;
}

.sleep-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

@media (max-width: 700px) {
  .sleep-setting-grid {
    grid-template-columns: 1fr;
  }
}

.preview-content {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.preview-textarea textarea {
  font-family: monospace;
  font-size: 12px;
  line-height: 1.5;
}
</style>
