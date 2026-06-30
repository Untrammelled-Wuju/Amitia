<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="dashboard">
    <h2 class="page-title">概览</h2>

    <AccessRiskAlert
      :access-risk="accessRisk"
      :cloud-risk="cloudRisk"
    />

    <StatusOverviewCards
      :deploy-class="deployClass"
      :deploy-label="deployLabel"
      :model-class="modelClass"
      :model-label="modelLabel"
      :model-name="modelName"
      :wechat-class="wechatClass"
      :wechat-label="wechatLabel"
      :qq-class="qqClass"
      :qq-label="qqLabel"
      :runtime-health="runtimeHealth"
    />

    <RuntimeHealthPanel
      :runtime-health="runtimeHealth"
      :runtime-health-loading="runtimeHealthLoading"
      :health-module-label="healthModuleLabel"
      :health-status-label="healthStatusLabel"
      @run-health-check="runHealthCheck"
    />

    <UsageStatsPanel
      :today-messages="todayMessages"
      :total-convs="totalConvs"
      :total-memories="totalMemories"
      :total-chars="totalChars"
      :today-calls="todayCalls"
      :today-tokens="todayTokens"
      :max-today-stat="maxTodayStat"
      :feedback-total="feedbackTotal"
      :feedback-by-type="feedbackByType"
      :bar-percent="barPercent"
      :format-tokens="formatTokens"
    />

    <RecentErrorsPanel
      :recent-errors="recentErrors"
      :fmt-date-short="fmtDateShort"
      @refresh="fetchRecentErrors"
    />

    <DiagnosticsPanel
      :diag-result="diagResult"
      :diag-loading="diagLoading"
      :has-suggestions="hasSuggestions"
      :suggestion-items="suggestionItems"
      :fmt-date-short="fmtDateShort"
      @run-diagnostics="runDiagnostics"
    />

    <RecentImportsPanel
      :recent-imports="recentImports"
      :fmt-date-short="fmtDateShort"
    />

    <el-card shadow="never" class="quick-actions-panel">
      <template #header>
        <span class="panel-title">快速入口</span>
      </template>
      <div class="quick-actions">
        <router-link to="/chat" class="qa-item">
          <div class="qa-icon chat"><el-icon :size="20"><ChatDotRound /></el-icon></div>
          <span>聊天</span>
        </router-link>
        <router-link to="/wechat" class="qa-item">
          <div class="qa-icon wechat"><el-icon :size="20"><Connection /></el-icon></div>
          <span>微信接入</span>
        </router-link>
        <router-link to="/qq" class="qa-item">
          <div class="qa-icon wechat"><el-icon :size="20"><ChatDotSquare /></el-icon></div>
          <span>QQ接入</span>
        </router-link>
        <router-link to="/model" class="qa-item">
          <div class="qa-icon model"><el-icon :size="20"><Cpu /></el-icon></div>
          <span>模型设置</span>
        </router-link>
        <router-link to="/character" class="qa-item">
          <div class="qa-icon char"><el-icon :size="20"><UserFilled /></el-icon></div>
          <span>角色管理</span>
        </router-link>
        <router-link to="/import" class="qa-item">
          <div class="qa-icon import"><el-icon :size="20"><Upload /></el-icon></div>
          <span>导入数据</span>
        </router-link>
        <router-link to="/logs" class="qa-item">
          <div class="qa-icon logs"><el-icon :size="20"><Document /></el-icon></div>
          <span>系统日志</span>
        </router-link>
        <router-link to="/settings" class="qa-item">
          <div class="qa-icon logs"><el-icon :size="20"><Setting /></el-icon></div>
          <span>设置</span>
        </router-link>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import {
  ChatDotRound, Connection, ChatDotSquare, Cpu, UserFilled,
  Upload, Document, Setting,
} from "@element-plus/icons-vue"
import { useDashboardData } from "./composables/useDashboardData"
import AccessRiskAlert from "./components/AccessRiskAlert.vue"
import StatusOverviewCards from "./components/StatusOverviewCards.vue"
import RuntimeHealthPanel from "./components/RuntimeHealthPanel.vue"
import UsageStatsPanel from "./components/UsageStatsPanel.vue"
import DiagnosticsPanel from "./components/DiagnosticsPanel.vue"
import RecentErrorsPanel from "./components/RecentErrorsPanel.vue"
import RecentImportsPanel from "./components/RecentImportsPanel.vue"

const {
  accessRisk, cloudRisk,
  deployClass, deployLabel, modelClass, modelLabel, modelName,
  wechatClass, wechatLabel, qqClass, qqLabel,
  runtimeHealth, runtimeHealthLoading,
  todayMessages, totalConvs, totalMemories, totalChars, todayCalls, todayTokens,
  maxTodayStat, feedbackTotal, feedbackByType,
  recentErrors, recentImports,
  diagResult, diagLoading, hasSuggestions, suggestionItems,
  healthModuleLabel, healthStatusLabel,
  barPercent, formatTokens, fmtDateShort,
  runHealthCheck, fetchRecentErrors, runDiagnostics,
} = useDashboardData()
</script>

<style scoped>
.dashboard { }
.page-title { font-size: var(--ac-font-size-lg); font-weight: 600; margin-bottom: 16px; color: var(--ac-color-text); }

.panel-title { font-size: var(--ac-font-size-sm); font-weight: 600; color: var(--ac-color-text); }

.quick-actions-panel { margin-bottom: 14px; }
.quick-actions { display: grid; grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); gap: 10px; }
.qa-item { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 16px 10px; border-radius: var(--ac-radius-md); background: var(--ac-color-bg-secondary); text-decoration: none; color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); transition: all var(--ac-transition-fast); cursor: pointer; }
.qa-item:hover { background: var(--ac-color-primary-bg); color: var(--ac-color-primary); }
.qa-icon { width: 40px; height: 40px; border-radius: var(--ac-radius-sm); display: flex; align-items: center; justify-content: center; background: var(--ac-color-surface); border: 1px solid var(--ac-color-border-light); }
.qa-icon.chat { color: var(--ac-color-primary); }
.qa-icon.wechat { color: var(--ac-color-success); }
.qa-icon.model { color: var(--ac-color-warning); }
.qa-icon.char { color: #8b7ec8; }
.qa-icon.import { color: #c8806a; }
.qa-icon.logs { color: var(--ac-color-text-secondary); }
</style>
