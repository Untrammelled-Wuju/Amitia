<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <section class="stats-panel">
    <div class="panel-title">今日数据</div>
    <div class="stats-grid">
      <div class="stat-item blue">
        <div class="stat-icon"><el-icon><ChatDotRound /></el-icon></div>
        <div>
          <div class="stat-label">消息</div>
          <div class="stat-value">{{ todayMessages }}</div>
        </div>
      </div>
      <div class="stat-item purple">
        <div class="stat-icon"><el-icon><User /></el-icon></div>
        <div>
          <div class="stat-label">会话</div>
          <div class="stat-value">{{ totalConvs }}</div>
        </div>
      </div>
      <div class="stat-item violet">
        <div class="stat-icon"><el-icon><Grape /></el-icon></div>
        <div>
          <div class="stat-label">记忆</div>
          <div class="stat-value">{{ totalMemories }}</div>
        </div>
      </div>
      <div class="stat-item orange">
        <div class="stat-icon"><el-icon><UserFilled /></el-icon></div>
        <div>
          <div class="stat-label">角色</div>
          <div class="stat-value">{{ totalChars }}</div>
        </div>
      </div>
      <div class="stat-item amber">
        <div class="stat-icon"><el-icon><Box /></el-icon></div>
        <div>
          <div class="stat-label">模型调用</div>
          <div class="stat-value">{{ todayCalls }}</div>
        </div>
      </div>
      <div class="stat-item green">
        <div class="stat-icon"><el-icon><Coin /></el-icon></div>
        <div>
          <div class="stat-label">消耗Token</div>
          <div class="stat-value">{{ formatTokens(todayTokens) }}</div>
        </div>
      </div>
      <div class="stat-item green feedback">
        <div class="stat-icon"><el-icon><Pointer /></el-icon></div>
        <div>
          <div class="stat-label">Feedback</div>
          <div class="stat-value">{{ feedbackTotal }} <span>total</span></div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { Box, ChatDotRound, Coin, Grape, Pointer, User, UserFilled } from "@element-plus/icons-vue"

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
.stats-panel {
  min-height: 336px;
  padding: 22px;
  border: 1px solid var(--console-border);
  border-radius: 14px;
  background: var(--console-card);
  box-shadow: var(--console-shadow);
}

.panel-title { margin-bottom: 20px; font-size: 18px; font-weight: 800; color: var(--console-text); }

.stats-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0;
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 66px;
  padding: 6px 20px 14px 0;
  border-bottom: 1px solid var(--console-border-soft);
}

.stat-item:nth-child(odd) {
  border-right: 1px solid var(--console-border-soft);
}

.stat-item:nth-child(even) {
  padding-left: 20px;
}

.stat-item.feedback {
  border-bottom: 0;
}

.stat-icon {
  width: 42px;
  height: 42px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 11px;
  font-size: 22px;
}

.blue .stat-icon { color: var(--ac-color-primary); background: var(--console-blue-soft); }
.purple .stat-icon { color: var(--ac-color-primary); background: var(--console-purple-soft); }
.violet .stat-icon { color: var(--ac-color-primary-light); background: var(--console-violet-soft); }
.orange .stat-icon { color: var(--ac-color-warning); background: var(--console-orange-soft); }
.amber .stat-icon { color: var(--ac-color-warning); background: var(--console-amber-soft); }
.green .stat-icon { color: var(--ac-color-success); background: var(--console-green-soft); }

.stat-label { margin-bottom: 4px; color: var(--console-text-muted); font-size: 14px; }
.stat-value { color: var(--console-text); font-size: 22px; font-weight: 750; line-height: 1.1; }
.stat-value span { font-size: 14px; font-weight: 600; }

@media (max-width: 640px) {
  .stats-grid { grid-template-columns: 1fr; }
  .stat-item,
  .stat-item:nth-child(even) {
    padding-left: 0;
    border-right: 0;
  }
}
</style>
