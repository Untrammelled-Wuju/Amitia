<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="info-grid">
    <el-card shadow="never" class="info-panel">
      <template #header>
        <span class="panel-title">今日数据</span>
      </template>
      <div class="today-chart">
        <div class="tc-row">
          <span class="tc-label">消息</span>
          <div class="tc-bar-wrap">
            <div class="tc-bar tc-bar-msg" :style="{ width: barPercent(todayMessages) }"></div>
          </div>
          <span class="tc-num">{{ todayMessages }}</span>
        </div>
        <div class="tc-row">
          <span class="tc-label">会话</span>
          <div class="tc-bar-wrap">
            <div class="tc-bar tc-bar-conv" :style="{ width: barPercent(totalConvs) }"></div>
          </div>
          <span class="tc-num">{{ totalConvs }}</span>
        </div>
        <div class="tc-row">
          <span class="tc-label">记忆</span>
          <div class="tc-bar-wrap">
            <div class="tc-bar tc-bar-mem" :style="{ width: barPercent(totalMemories) }"></div>
          </div>
          <span class="tc-num">{{ totalMemories }}</span>
        </div>
        <div class="tc-row">
          <span class="tc-label">角色</span>
          <div class="tc-bar-wrap">
            <div class="tc-bar tc-bar-char" :style="{ width: barPercent(totalChars) }"></div>
          </div>
          <span class="tc-num">{{ totalChars }}</span>
        </div>
        <div class="tc-row">
          <span class="tc-label">模型调用</span>
          <div class="tc-bar-wrap">
            <div class="tc-bar tc-bar-usage" :style="{ width: barPercent(todayCalls) }"></div>
          </div>
          <span class="tc-num">{{ todayCalls }}</span>
        </div>
        <div class="tc-row">
          <span class="tc-label">消耗Token</span>
          <div class="tc-bar-wrap">
            <div class="tc-bar tc-bar-token" :style="{ width: barPercent(Math.min(todayTokens, maxTodayStat)) }"></div>
          </div>
          <span class="tc-num">{{ formatTokens(todayTokens) }}</span>
        </div>
      </div>
    </el-card>

    <el-card shadow="never" class="info-panel">
      <template #header>
        <span class="panel-title">Feedback</span>
      </template>
      <div v-if="feedbackTotal > 0" class="feedback-summary">
        <div class="fb-total">{{ feedbackTotal }} total</div>
        <div class="fb-bars">
          <div v-for="(cnt, type) in feedbackByType" :key="type" class="fb-bar-row">
            <span class="fb-type">{{ type }}</span>
            <span class="fb-cnt">{{ cnt }}</span>
          </div>
        </div>
      </div>
      <div v-else class="empty-hint">No feedback yet</div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  todayMessages: number
  totalConvs: number
  totalMemories: number
  totalChars: number
  todayCalls: number
  todayTokens: number
  maxTodayStat: number
  feedbackTotal: number
  feedbackByType: Record<string, number>
  barPercent: (val: number) => string
  formatTokens: (n: number) => string
}>()
</script>

<style scoped>
.info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 14px; }
@media (max-width: 640px) { .info-grid { grid-template-columns: 1fr; } }
.info-panel { min-height: 100px; }
.panel-title { font-size: var(--ac-font-size-sm); font-weight: 600; color: var(--ac-color-text); }

.today-chart { display: flex; flex-direction: column; gap: 10px; }
.tc-row { display: flex; align-items: center; gap: 10px; }
.tc-label { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-secondary); width: 32px; flex-shrink: 0; text-align: right; }
.tc-bar-wrap { flex: 1; height: 20px; background: var(--ac-color-bg-secondary); border-radius: 4px; overflow: hidden; }
.tc-bar { height: 100%; border-radius: 4px; transition: width 0.6s ease; min-width: 2px; }
.tc-bar-msg { background: var(--ac-color-primary); }
.tc-bar-conv { background: #5a9e6f; }
.tc-bar-mem { background: #b8952e; }
.tc-bar-char { background: #8b7ec8; }
.tc-bar-usage { background: #c8806a; }
.tc-bar-token { background: #6a8fc8; }
.tc-num { font-size: var(--ac-font-size-sm); font-weight: 700; color: var(--ac-color-text); width: 40px; flex-shrink: 0; text-align: right; }

.feedback-summary { display: flex; flex-direction: column; gap: 10px; }
.fb-total { font-size: var(--ac-font-size-base); font-weight: 500; color: var(--ac-color-text); }
.fb-bars { display: flex; flex-direction: column; gap: 6px; }
.fb-bar-row { display: flex; justify-content: space-between; align-items: center; font-size: var(--ac-font-size-sm); }
.fb-type { color: var(--ac-color-text-secondary); }
.fb-cnt { font-weight: 600; color: var(--ac-color-text); }

.empty-hint { font-size: var(--ac-font-size-sm); color: var(--ac-color-text-muted); padding: 8px 0; }
</style>
