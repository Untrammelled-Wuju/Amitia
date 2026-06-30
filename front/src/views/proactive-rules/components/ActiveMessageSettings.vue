<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <div class="card-header-row">
        <span class="section-title">主动消息设置</span>
        <el-button type="primary" size="small" :loading="savingSettings" @click="emit('save')">保存设置</el-button>
      </div>
    </template>
    <el-form :model="settings" label-position="top" size="small">
      <el-row :gutter="16">
        <el-col :span="6">
          <el-form-item label="启用">
            <el-switch v-model="settings.enabled" />
          </el-form-item>
        </el-col>
        <el-col :span="6">
          <el-form-item label="活跃度 (1-100)">
            <el-slider v-model="settings.activeLevel" :min="1" :max="100" show-input :format-tooltip="(v:number)=>v+''" />
          </el-form-item>
        </el-col>
        <el-col :span="6">
          <el-form-item label="最小间隔(分钟)">
            <el-input-number v-model="settings.minInterval" :min="5" :max="480" />
          </el-form-item>
        </el-col>
        <el-col :span="6">
          <el-form-item label="每日最大条数">
            <el-input-number v-model="settings.maxPerDay" :min="1" :max="24" />
          </el-form-item>
        </el-col>
      </el-row>
      <el-row :gutter="16">
        <el-col :span="6">
          <el-form-item label="安静开始">
            <el-time-picker v-model="settings.quietStart" format="HH:mm" value-format="HH:mm" style="width:100%" />
          </el-form-item>
        </el-col>
        <el-col :span="6">
          <el-form-item label="安静结束">
            <el-time-picker v-model="settings.quietEnd" format="HH:mm" value-format="HH:mm" style="width:100%" />
          </el-form-item>
        </el-col>
        <el-col :span="6">
          <el-form-item label="发送渠道">
            <el-select v-model="settings.channel" style="width:100%">
              <el-option label="全部平台" value="all" />
              <el-option label="Web 端" value="web" />
              <el-option label="微信" value="wechat" />
              <el-option label="QQ" value="qq" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="6">
          <el-form-item label="每日AI调用上限">
            <el-input-number v-model="settings.maxDailyCalls" :min="1" :max="50" />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>
  </el-card>
</template>

<script setup lang="ts">
defineProps<{ settings: any; savingSettings: boolean }>()
const emit = defineEmits<{ (e: "save"): void }>()
</script>

<style scoped>
.section-card { margin-bottom: 16px; }
.section-title { font-weight: 600; font-size: var(--ac-font-size-base); }
.card-header-row { display: flex; justify-content: space-between; align-items: center; }
</style>
