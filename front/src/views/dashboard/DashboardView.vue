<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="dashboard">
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

    <div class="dashboard-grid">
      <RuntimeHealthPanel
        class="panel-service"
        :runtime-health="runtimeHealth"
        :runtime-health-loading="runtimeHealthLoading"
        :health-module-label="healthModuleLabel"
        :health-status-label="healthStatusLabel"
        @run-health-check="runHealthCheck"
      />

      <UsageStatsPanel
        class="panel-stats"
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
        class="panel-errors"
        :recent-errors="recentErrors"
        :fmt-date-short="fmtDateShort"
        @refresh="fetchRecentErrors"
      />

      <DiagnosticsPanel
        class="panel-diagnostics"
        :diag-result="diagResult"
        :diag-loading="diagLoading"
        :has-suggestions="hasSuggestions"
        :suggestion-items="suggestionItems"
        :fmt-date-short="fmtDateShort"
        @run-diagnostics="runDiagnostics"
      />

      <RecentImportsPanel
        class="panel-imports"
        :recent-imports="recentImports"
        :fmt-date-short="fmtDateShort"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useDashboardData } from "./composables/useDashboardData"
import StatusOverviewCards from "./components/StatusOverviewCards.vue"
import RuntimeHealthPanel from "./components/RuntimeHealthPanel.vue"
import UsageStatsPanel from "./components/UsageStatsPanel.vue"
import DiagnosticsPanel from "./components/DiagnosticsPanel.vue"
import RecentErrorsPanel from "./components/RecentErrorsPanel.vue"
import RecentImportsPanel from "./components/RecentImportsPanel.vue"

const {
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
.dashboard {
  min-height: 100%;
  color: var(--console-text);
}

.dashboard-grid {
  display: grid;
  grid-template-columns: 1.08fr 1.2fr 1fr;
  grid-template-areas:
    "service stats errors"
    "diagnostics diagnostics imports";
  gap: 18px;
}

.panel-service { grid-area: service; }
.panel-stats { grid-area: stats; }
.panel-errors { grid-area: errors; }
.panel-diagnostics { grid-area: diagnostics; }
.panel-imports { grid-area: imports; }

@media (max-width: 1180px) {
  .dashboard-grid {
    grid-template-columns: 1fr 1fr;
    grid-template-areas:
      "service stats"
      "errors errors"
      "diagnostics imports";
  }
}

@media (max-width: 780px) {
  .dashboard-grid {
    grid-template-columns: 1fr;
    grid-template-areas:
      "service"
      "stats"
      "errors"
      "diagnostics"
      "imports";
  }
}
</style>
