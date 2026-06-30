<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="section-card">
    <div class="section-title">上班规则  <el-button size="small" @click="saveWorkProfile" :loading="workSaving" type="primary" style="margin-left:12px">保存</el-button></div>
    <div class="work-grid">
      <div class="work-item">
        <label class="gender-label">启用上班状态</label>
        <el-switch v-model="workForm.enabled" />
        <span class="gender-hint">开启后工作日自动切换为上班模式</span>
      </div>
      <div class="work-item">
        <label class="gender-label">工作日</label>
        <el-select v-model="workForm.workDaysArr" multiple placeholder="选择工作日" size="default" style="width:100%">
          <el-option v-for="opt in WEEKDAY_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
      </div>
      <div class="work-item">
        <label class="gender-label">上班时间</label>
        <el-time-picker v-model="workForm.workStartTime" format="HH:mm" value-format="HH:mm" size="default" style="width:140px" />
      </div>
      <div class="work-item">
        <label class="gender-label">下班时间</label>
        <el-time-picker v-model="workForm.workEndTime" format="HH:mm" value-format="HH:mm" size="default" style="width:140px" />
      </div>
      <div class="work-item">
        <label class="gender-label">午休开始</label>
        <el-time-picker v-model="workForm.lunchBreakStartTime" format="HH:mm" value-format="HH:mm" size="default" style="width:140px" />
      </div>
      <div class="work-item">
        <label class="gender-label">午休结束</label>
        <el-time-picker v-model="workForm.lunchBreakEndTime" format="HH:mm" value-format="HH:mm" size="default" style="width:140px" />
      </div>
      <div class="work-item">
        <label class="gender-label">通勤时间(分)</label>
        <div style="display:flex;gap:8px;align-items:center">
          <el-input-number v-model="workForm.commuteMinMinutes" :min="5" :max="90" size="default" style="width:100px" />
          <span style="font-size:11px;color:var(--ac-color-text-placeholder)">到</span>
          <el-input-number v-model="workForm.commuteMaxMinutes" :min="5" :max="120" size="default" style="width:100px" />
        </div>
      </div>
      <div class="work-item">
        <label class="gender-label">准备时间(分)</label>
        <div style="display:flex;gap:8px;align-items:center">
          <el-input-number v-model="workForm.prepareMinMinutes" :min="10" :max="60" size="default" style="width:100px" />
          <span style="font-size:11px;color:var(--ac-color-text-placeholder)">到</span>
          <el-input-number v-model="workForm.prepareMaxMinutes" :min="10" :max="90" size="default" style="width:100px" />
        </div>
      </div>
      <div class="work-item">
        <label class="gender-label">回复模式</label>
        <el-select v-model="workForm.replyMode" size="default" style="width:140px">
          <el-option v-for="opt in WORK_REPLY_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <span class="gender-hint">工作期间的回复方式</span>
      </div>
      <div class="work-item">
        <label class="gender-label">允许加班</label>
        <el-switch v-model="workForm.allowOvertime" />
        <span class="gender-hint">开启后工作日晚间可能触发加班状态</span>
      </div>
      <div class="work-item" v-if="workForm.allowOvertime">
        <label class="gender-label">加班概率(%)</label>
        <el-input-number v-model="workForm.overtimeProbability" :min="0" :max="100" size="default" style="width:120px" />
      </div>
      <div class="work-item" v-if="workForm.allowOvertime">
        <label class="gender-label">加班时长(分)</label>
        <div style="display:flex;gap:8px;align-items:center">
          <el-input-number v-model="workForm.overtimeMinMinutes" :min="10" :max="120" size="default" style="width:100px" />
          <span style="font-size:11px;color:var(--ac-color-text-placeholder)">到</span>
          <el-input-number v-model="workForm.overtimeMaxMinutes" :min="10" :max="300" size="default" style="width:100px" />
        </div>
      </div>
      <div class="work-item" v-if="workForm.allowOvertime">
        <label class="gender-label">加班回复模式</label>
        <el-select v-model="workForm.overtimeReplyMode" size="default" style="width:140px">
          <el-option v-for="opt in WORK_REPLY_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
      </div>
      <div class="work-item">
        <label class="gender-label">延迟回复</label>
        <el-switch v-model="workForm.delayedReplyEnabled" />
        <span class="gender-hint">开启后工作期间可能延迟回消息</span>
      </div>
      <div class="work-item">
        <label class="gender-label">通勤分享</label>
        <el-switch v-model="workForm.commuteHomeShareEnabled" />
        <span class="gender-hint">下班路上是否主动分享今日状态</span>
      </div>
      <div class="work-item" v-if="workForm.commuteHomeShareEnabled">
        <label class="gender-label">分享概率(%)</label>
        <el-input-number v-model="workForm.commuteHomeShareProbability" :min="0" :max="100" size="default" style="width:120px" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"
import { ElMessage } from "element-plus"
import { useWorkProfile, WEEKDAY_OPTIONS, WORK_REPLY_OPTIONS } from "../../../composables/useWorkProfile"

const props = defineProps<{
  characterId: string
}>()

const { updateWorkProfile } = useWorkProfile()

const workForm = defineModel<any>("workForm", { required: true })

const workSaving = ref(false)

async function saveWorkProfile() {
  workSaving.value = true
  try {
    await updateWorkProfile({
      enabled: workForm.value.enabled,
      workDays: workForm.value.workDaysArr.join(","),
      workStartTime: workForm.value.workStartTime,
      workEndTime: workForm.value.workEndTime,
      lunchBreakStartTime: workForm.value.lunchBreakStartTime,
      lunchBreakEndTime: workForm.value.lunchBreakEndTime,
      commuteMinMinutes: workForm.value.commuteMinMinutes,
      commuteMaxMinutes: workForm.value.commuteMaxMinutes,
      prepareMinMinutes: workForm.value.prepareMinMinutes,
      prepareMaxMinutes: workForm.value.prepareMaxMinutes,
      replyMode: workForm.value.replyMode,
      allowOvertime: workForm.value.allowOvertime,
      overtimeProbability: workForm.value.overtimeProbability,
      overtimeMinMinutes: workForm.value.overtimeMinMinutes,
      overtimeMaxMinutes: workForm.value.overtimeMaxMinutes,
      overtimeReplyMode: workForm.value.overtimeReplyMode,
      delayedReplyEnabled: workForm.value.delayedReplyEnabled,
      commuteHomeShareEnabled: workForm.value.commuteHomeShareEnabled,
      commuteHomeShareProbability: workForm.value.commuteHomeShareProbability,
    } as any, props.characterId || undefined)
    ElMessage.success("上班规则已保存")
  } catch {
    ElMessage.error("保存失败")
  } finally {
    workSaving.value = false
  }
}
</script>
