<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="psyche-card">
    <template #header>
      <span class="panel-title">心理状态概览</span>
    </template>
    <div v-if="loading" class="psyche-loading">
      <div class="pl-row" v-for="i in 4" :key="i"><div class="pl-bar" :style="{ animationDelay: i * 0.15 + 's' }"></div></div>
    </div>
    <div v-else-if="psyche" class="psyche-grid">
      <div class="ps-section">
        <div class="ps-section-header">
          <el-icon :size="16"><MagicStick /></el-icon>
          <span>情感</span>
        </div>
        <div class="ps-bars">
          <div class="ps-bar-row">
            <span class="ps-bar-label">积极</span>
            <div class="ps-bar-track"><div class="ps-bar-fill ps-fill-positive" :style="{ width: (psyche.emotion.positive * 100).toFixed(0) + '%' }"></div></div>
            <span class="ps-bar-val">{{ (psyche.emotion.positive * 100).toFixed(0) }}</span>
          </div>
          <div class="ps-bar-row">
            <span class="ps-bar-label">消极</span>
            <div class="ps-bar-track"><div class="ps-bar-fill ps-fill-negative" :style="{ width: (psyche.emotion.negative * 100).toFixed(0) + '%' }"></div></div>
            <span class="ps-bar-val">{{ (psyche.emotion.negative * 100).toFixed(0) }}</span>
          </div>
          <div class="ps-bar-row">
            <span class="ps-bar-label">唤醒</span>
            <div class="ps-bar-track"><div class="ps-bar-fill ps-fill-arousal" :style="{ width: (psyche.emotion.arousal * 100).toFixed(0) + '%' }"></div></div>
            <span class="ps-bar-val">{{ (psyche.emotion.arousal * 100).toFixed(0) }}</span>
          </div>
        </div>
        <div class="ps-affect-label">
          <el-tag size="small" :type="affectTagType">{{ psyche.affectLabel || '-' }}</el-tag>
        </div>
      </div>

      <div class="ps-section">
        <div class="ps-section-header">
          <el-icon :size="16"><List /></el-icon>
          <span>信念</span>
        </div>
        <div class="ps-bars">
          <div v-if="psyche.beliefs.length === 0" class="ps-empty">暂无信念记录</div>
          <div v-for="b in psyche.beliefs.slice(0, 4)" :key="b.key" class="ps-belief-row">
            <div class="ps-belief-key" :title="b.key">{{ b.key }}</div>
            <div class="ps-belief-conf">
              <div class="ps-bar-track"><div class="ps-bar-fill ps-fill-belief" :style="{ width: (b.confidence * 100).toFixed(0) + '%' }"></div></div>
              <span class="ps-bar-val">{{ (b.confidence * 100).toFixed(0) }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="ps-section">
        <div class="ps-section-header">
          <el-icon :size="16"><Wallet /></el-icon>
          <span>需求</span>
        </div>
        <div class="ps-bars">
          <div v-if="needKeys.length === 0" class="ps-empty">暂无需求数据</div>
          <div v-for="k in needKeys.slice(0, 5)" :key="k" class="ps-bar-row">
            <span class="ps-bar-label">{{ needLabel(k) }}</span>
            <div class="ps-bar-track"><div class="ps-bar-fill ps-fill-need" :style="{ width: Math.min(psyche.needs[k] * 10, 100).toFixed(0) + '%' }"></div></div>
            <span class="ps-bar-val">{{ psyche.needs[k].toFixed(1) }}</span>
          </div>
        </div>
      </div>

      <div class="ps-section">
        <div class="ps-section-header">
          <el-icon :size="16"><Connection /></el-icon>
          <span>关系</span>
        </div>
        <div class="ps-bars">
          <div class="ps-bar-row">
            <span class="ps-bar-label">信任</span>
            <div class="ps-bar-track"><div class="ps-bar-fill ps-fill-trust" :style="{ width: (psyche.relationship.trust * 100).toFixed(0) + '%' }"></div></div>
            <span class="ps-bar-val">{{ (psyche.relationship.trust * 100).toFixed(0) }}</span>
          </div>
          <div class="ps-bar-row">
            <span class="ps-bar-label">熟悉</span>
            <div class="ps-bar-track"><div class="ps-bar-fill ps-fill-familiar" :style="{ width: (psyche.relationship.familiarity * 100).toFixed(0) + '%' }"></div></div>
            <span class="ps-bar-val">{{ (psyche.relationship.familiarity * 100).toFixed(0) }}</span>
          </div>
          <div class="ps-bar-row">
            <span class="ps-bar-label">紧张</span>
            <div class="ps-bar-track"><div class="ps-bar-fill ps-fill-tension" :style="{ width: (psyche.relationship.tension * 100).toFixed(0) + '%' }"></div></div>
            <span class="ps-bar-val">{{ (psyche.relationship.tension * 100).toFixed(0) }}</span>
          </div>
          <div class="ps-bar-row">
            <span class="ps-bar-label">安全</span>
            <div class="ps-bar-track"><div class="ps-bar-fill ps-fill-security" :style="{ width: (psyche.relationship.security * 100).toFixed(0) + '%' }"></div></div>
            <span class="ps-bar-val">{{ (psyche.relationship.security * 100).toFixed(0) }}</span>
          </div>
        </div>
      </div>
    </div>
    <div v-else class="psyche-empty">
      <el-icon :size="20" color="var(--ac-color-text-muted)"><InfoFilled /></el-icon>
      <span>心理模拟引擎尚未初始化，启动对话后自动生成</span>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { MagicStick, List, Wallet, Connection, InfoFilled } from "@element-plus/icons-vue"
import type { PsycheStateSnapshot } from "../../../types"

const props = defineProps<{
  psyche: PsycheStateSnapshot | null
  loading: boolean
}>()

const affectTagType = computed(() => {
  if (!props.psyche) return "info"
  const p = props.psyche.emotion.positive
  const n = props.psyche.emotion.negative
  if (p > 0.6 && n < 0.3) return "success"
  if (n > 0.6) return "danger"
  if (props.psyche.emotion.arousal > 0.7) return "warning"
  return "info"
})

const needKeys = computed(() => {
  if (!props.psyche) return []
  return Object.keys(props.psyche.needs)
})

function needLabel(k: string): string {
  const labels: Record<string, string> = {
    companionship: "陪伴", autonomy: "自主", competence: "胜任",
    intimacy: "亲密", belonging: "归属", recognition: "认可",
    security: "安全", stimulation: "刺激", meaning: "意义",
  }
  return labels[k] || k
}
</script>

<style scoped>
.psyche-card { margin-bottom: 14px; }
.panel-title { font-size: var(--ac-font-size-sm); font-weight: 600; color: var(--ac-color-text); }
.psyche-loading { display: flex; flex-direction: column; gap: 12px; padding: 12px 0; }
.pl-row { height: 14px; }
.pl-bar { width: 60%; height: 100%; border-radius: 4px; background: linear-gradient(90deg, var(--ac-color-bg-secondary) 0%, var(--ac-color-bg-hover) 50%, var(--ac-color-bg-secondary) 100%); background-size: 200% 100%; animation: shimmer 1.5s infinite; }
@keyframes shimmer { 0% { background-position: 200% 0; } 100% { background-position: -200% 0; } }
.psyche-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
@media (max-width: 640px) { .psyche-grid { grid-template-columns: 1fr; } }
.ps-section { display: flex; flex-direction: column; gap: 8px; }
.ps-section-header { display: flex; align-items: center; gap: 6px; font-size: var(--ac-font-size-xs); font-weight: 600; color: var(--ac-color-text-secondary); }
.ps-bars { display: flex; flex-direction: column; gap: 5px; }
.ps-bar-row { display: flex; align-items: center; gap: 6px; }
.ps-bar-label { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); width: 36px; flex-shrink: 0; }
.ps-bar-track { flex: 1; height: 14px; background: var(--ac-color-bg-secondary); border-radius: 3px; overflow: hidden; }
.ps-bar-fill { height: 100%; border-radius: 3px; transition: width 0.8s ease; min-width: 2px; }
.ps-bar-val { font-size: var(--ac-font-size-xs); font-weight: 600; color: var(--ac-color-text); width: 28px; flex-shrink: 0; text-align: right; }
.ps-fill-positive { background: var(--ac-color-success); }
.ps-fill-negative { background: var(--ac-color-danger); }
.ps-fill-arousal { background: var(--ac-color-warning); }
.ps-fill-belief { background: #8b7ec8; }
.ps-fill-need { background: #c8806a; }
.ps-fill-trust { background: #5a9e6f; }
.ps-fill-familiar { background: #6a8fc8; }
.ps-fill-tension { background: #d98a5a; }
.ps-fill-security { background: #5a9ead; }
.ps-affect-label { margin-top: 2px; }
.ps-empty { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); padding: 4px 0; }
.psyche-empty { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 24px 0; color: var(--ac-color-text-muted); font-size: var(--ac-font-size-sm); }
.ps-belief-row { display: flex; align-items: center; gap: 6px; }
.ps-belief-key { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); width: 36px; flex-shrink: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ps-belief-conf { flex: 1; display: flex; align-items: center; gap: 6px; }
</style>

