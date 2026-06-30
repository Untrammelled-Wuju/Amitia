<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="section-card">
    <div class="section-title">睡觉回复</div>
    <div class="sleep-setting-grid">
      <div class="sleep-item">
        <label class="gender-label">睡觉后是否回复</label>
        <el-switch v-model="sleepForm.sleepReplyEnabled" />
        <span class="gender-hint">开启后角色睡觉时仍可回复，但回复会简短困倦</span>
      </div>
      <div class="sleep-item" v-if="sleepForm.sleepReplyEnabled">
        <label class="gender-label">睡觉回复模式</label>
        <el-select v-model="sleepForm.sleepReplyMode" placeholder="选择模式" size="default" style="width:100%">
          <el-option v-for="opt in SLEEP_REPLY_MODE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <span class="gender-hint">决定角色睡觉后的回复方式</span>
      </div>
      <div class="sleep-item" v-if="!sleepForm.sleepReplyEnabled">
        <label class="gender-label">关闭时行为</label>
        <el-select v-model="sleepForm.sleepReplyMode" placeholder="选择模式" size="default" style="width:100%">
          <el-option v-for="opt in SLEEP_OFF_MODE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <span class="gender-hint">不回复时可显示系统提示，或完全静默</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { SLEEP_REPLY_MODE_OPTIONS } from "../../../composables/useSleepSetting"

const SLEEP_OFF_MODE_OPTIONS = [
  { label: "不回复", value: "NO_REPLY" },
  { label: "显示系统提示", value: "SYSTEM_NOTICE" },
]

const sleepForm = defineModel<any>("sleepForm", { required: true })
</script>
