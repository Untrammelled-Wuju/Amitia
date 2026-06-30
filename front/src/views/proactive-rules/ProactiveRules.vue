<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="proactive-page">
    <div class="page-header">
      <h2 class="page-title">主动消息规则</h2>
      <el-tag :type="schedulerRunning ? 'success' : 'danger'" size="small">
        {{ schedulerRunning ? '调度器运行中' : '调度器已停止' }}
      </el-tag>
    </div>

    <el-alert type="warning" :closable="false" show-icon style="margin-bottom:16px">
      <template #title>主动消息默认关闭，需手动开启规则后才会发送。安静时段和每日上限会自动约束发送频率。</template>
    </el-alert>

    <ActiveMessageSettings
      :settings="activeMsgSettings"
      :saving-settings="savingSettings"
      @save="saveActiveMsgSettings"
    />

    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header-row">
          <span class="section-title">调度状态</span>
          <el-button type="primary" size="small" @click="fetchStatus">刷新</el-button>
        </div>
      </template>
      <div class="status-grid">
        <div class="status-item">
          <span class="status-label">调度器</span>
          <el-tag :type="schedulerRunning ? 'success' : 'info'" size="small">{{ schedulerRunning ? '运行中' : '已停止' }}</el-tag>
        </div>
        <div class="status-item">
          <span class="status-label">已启用规则</span>
          <span class="status-value">{{ enabledRuleCount }}</span>
        </div>
        <div class="status-item">
          <span class="status-label">规则总数</span>
          <span class="status-value">{{ totalRuleCount }}</span>
        </div>
      </div>
    </el-card>

    <RuleListTable
      :rules="rules"
      :loading="loading"
      :resetting-presets="resettingPresets"
      :type-label="typeLabel"
      :channel-label="channelLabel"
      @create="openCreateDialog"
      @edit="openEditDialog"
      @test="testRule"
      @trigger="triggerRule"
      @delete="deleteRule"
      @reset-presets="resetPresetRules"
    />

    <RuleEditDialog
      v-model="dialogVisible"
      :is-editing="isEditing"
      :form="form"
      :saving="saving"
      :rule-types="RULE_TYPES"
      :conversations="conversations"
      :characters="characters"
      @save="saveRule"
    />

    <TestResultDialog
      v-model="testVisible"
      :test-result="testResult"
    />
  </div>
</template>

<script setup lang="ts">
import { useProactiveRules } from "./composables/useProactiveRules"
import ActiveMessageSettings from "./components/ActiveMessageSettings.vue"
import RuleListTable from "./components/RuleListTable.vue"
import RuleEditDialog from "./components/RuleEditDialog.vue"
import TestResultDialog from "./components/TestResultDialog.vue"

const {
  rules, loading, saving, dialogVisible, isEditing,
  testVisible, testResult, schedulerRunning,
  activeMsgSettings, savingSettings,
  enabledRuleCount, totalRuleCount,
  conversations, characters, resettingPresets, form,
  RULE_TYPES,
  channelLabel, typeLabel,
  openCreateDialog, openEditDialog, saveRule,
  toggleRule, deleteRule, testRule, triggerRule,
  resetPresetRules, saveActiveMsgSettings, fetchStatus,
} = useProactiveRules()
</script>

<style scoped>
.proactive-page { padding: 20px; }

.page-header { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }

.page-title { margin: 0; font-size: var(--ac-font-size-xl); color: var(--ac-color-text); }

.section-card { margin-bottom: 16px; }

.section-title { font-weight: 600; font-size: var(--ac-font-size-base); }

.card-header-row { display: flex; justify-content: space-between; align-items: center; }

.status-grid { display: flex; gap: 32px; }

.status-item { display: flex; align-items: center; gap: 8px; }

.status-label { color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); }

.status-value { font-weight: 600; font-size: var(--ac-font-size-lg); color: var(--ac-color-primary); }
</style>
