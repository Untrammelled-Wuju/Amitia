<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="status-grid">
    <div class="status-card" :class="deployClass">
      <div class="sc-icon"><el-icon :size="22"><Monitor /></el-icon></div>
      <div class="sc-body">
        <div class="sc-label">部署模式</div>
        <div class="sc-value">{{ deployLabel }}</div>
      </div>
    </div>

    <div class="status-card" :class="modelClass">
      <div class="sc-icon"><el-icon :size="22"><Cpu /></el-icon></div>
      <div class="sc-body">
        <div class="sc-label">模型状态</div>
        <div class="sc-value">{{ modelLabel }}</div>
        <div class="sc-sub" v-if="modelName">{{ modelName }}</div>
      </div>
    </div>

    <div class="status-card" :class="wechatClass">
      <div class="sc-icon"><el-icon :size="22"><Connection /></el-icon></div>
      <div class="sc-body">
        <div class="sc-label">微信连接</div>
        <div class="sc-value">{{ wechatLabel }}</div>
      </div>
    </div>

    <div class="status-card" :class="qqClass">
      <div class="sc-icon"><el-icon :size="22"><ChatDotSquare /></el-icon></div>
      <div class="sc-body">
        <div class="sc-label">QQ连接</div>
        <div class="sc-value">{{ qqLabel }}</div>
      </div>
    </div>

    <div class="status-card" :class="runtimeHealthClass">
      <div class="sc-icon"><el-icon :size="22"><CircleCheck /></el-icon></div>
      <div class="sc-body">
        <div class="sc-label">系统健康</div>
        <div class="sc-value">{{ runtimeHealthLabel }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { Monitor, Cpu, Connection, ChatDotSquare, CircleCheck } from "@element-plus/icons-vue"

const props = defineProps<{
  deployClass: string
  deployLabel: string
  modelClass: string
  modelLabel: string
  modelName: string
  wechatClass: string
  wechatLabel: string
  qqClass: string
  qqLabel: string
  runtimeHealth: any
}>()

const runtimeHealthClass = computed(() =>
  props.runtimeHealth?.overall === "ok" ? "status-ok" :
  props.runtimeHealth?.overall === "warning" ? "status-warn" : "status-off"
)

const runtimeHealthLabel = computed(() =>
  props.runtimeHealth?.overall === "ok" ? "正常" :
  props.runtimeHealth?.overall === "warning" ? "注意" :
  props.runtimeHealth?.overall === "error" ? "异常" : "未知"
)
</script>

<style scoped>
.status-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 10px; margin-bottom: 14px; }
.status-card { display: flex; align-items: center; gap: 12px; padding: 14px 16px; border-radius: var(--ac-radius-md); background: var(--ac-color-surface); border: 1px solid var(--ac-color-border-light); transition: border-color var(--ac-transition-fast); }
.status-card.status-ok { border-left: 3px solid var(--ac-color-success); }
.status-card.status-warn { border-left: 3px solid var(--ac-color-warning); }
.status-card.status-off { border-left: 3px solid var(--ac-color-text-muted); }
.sc-icon { flex-shrink: 0; color: var(--ac-color-text-secondary); }
.sc-body { flex: 1; min-width: 0; }
.sc-label { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); margin-bottom: 2px; }
.sc-value { font-size: var(--ac-font-size-base); font-weight: 600; color: var(--ac-color-text); }
.sc-sub { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); margin-top: 2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
