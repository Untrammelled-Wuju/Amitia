<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="status-grid">
    <div class="status-card" :class="deployClass">
      <div class="sc-icon"><el-icon :size="22"><Monitor /></el-icon></div>
      <div class="sc-body">
        <div class="sc-label">本地桌面</div>
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
.status-grid { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 18px; margin-bottom: 18px; }
.status-card {
  display: flex;
  align-items: center;
  gap: 18px;
  min-height: 138px;
  padding: 24px 20px;
  border-radius: 14px;
  background: var(--console-card);
  border: 1px solid var(--console-border);
  box-shadow: var(--console-shadow);
}
.sc-icon {
  flex-shrink: 0;
  width: 52px;
  height: 52px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 13px;
  background: rgba(74, 222, 128, 0.2); color: #86efac;
}
.status-off .sc-icon { background: rgba(248, 113, 113, 0.2); color: #fca5a5; }
.status-warn .sc-icon { background: rgba(248, 113, 113, 0.2); color: #fca5a5; }

.sc-body { flex: 1; min-width: 0; }
.sc-label { font-size: 16px; font-weight: 650; color: var(--console-text); margin-bottom: 8px; }
.sc-value {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 3px 10px;
  border-radius: 7px;
  background: var(--console-value-ok-bg);
  color: #86efac;
  font-size: 13px;
  font-weight: 700;
}
.status-warn .sc-value { background: var(--console-value-off-bg); color: #fca5a5; }
.status-off .sc-value { background: var(--console-value-off-bg); color: #fca5a5; }
.sc-sub { font-size: 14px; color: var(--console-text-muted); margin-top: 10px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

@media (max-width: 1280px) {
  .status-grid { grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); }
}
</style>
