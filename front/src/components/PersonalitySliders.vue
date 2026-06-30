<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="personality-sliders">
    <div class="ps-header">
      <span class="ps-title">性格微调</span>
      <el-button text size="small" type="primary" @click="resetToDefaults">重置默认</el-button>
    </div>

    <div v-for="group in groups" :key="group.key" class="ps-group">
      <div class="ps-group-title">{{ group.label }}</div>
      <div class="ps-grid">
        <div v-for="dim in group.dims" :key="dim.key" class="ps-item">
          <div class="ps-label-row">
            <span class="ps-label">{{ dim.label }}</span>
            <span class="ps-desc">{{ dim.desc }}</span>
            <span class="ps-value">{{ model[dim.key] }}</span>
          </div>
          <div class="ps-slider-row">
            <span class="ps-left">{{ dim.left }}</span>
            <el-slider
              :model-value="model[dim.key]"
              :min="0"
              :max="100"
              :step="1"
              :show-tooltip="false"
              class="ps-slider"
              @update:model-value="(v: number) => setSlider(dim.key, v)"
            />
            <span class="ps-right">{{ dim.right }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, computed, watch } from "vue"

export interface PersonalityConfig {
  familiarity: number
  formality: number
  customerServiceAvoidance: number
  directness: number
  verbosity: number
  structureLevel: number
  shortSentence: number
  toneWords: number
  warmth: number
  emotionalExpression: number
  comfortLevel: number
  preachingAvoidance: number
  rationality: number
  humor: number
  teasing: number
  initiative: number
  patience: number
  companionship: number
  boundary: number
  dependencyAvoidance: number
  execution: number
  explanationDepth: number
  judgment: number
  clarification: number
  intimacyExpression: number
  flirtiness: number
  romanticTone: number
  suggestivenessAvoidance: number
  intimacyBoundary: number
}

const props = defineProps<{
  modelValue: PersonalityConfig
}>()

const emit = defineEmits<{
  (e: "update:modelValue", value: PersonalityConfig): void
}>()

const DEFAULT_CONFIG: PersonalityConfig = {
  familiarity: 78, formality: 22, customerServiceAvoidance: 92,
  directness: 75, verbosity: 32, structureLevel: 40, shortSentence: 85, toneWords: 45,
  warmth: 58, emotionalExpression: 45, comfortLevel: 55, preachingAvoidance: 88,
  rationality: 62, humor: 35, teasing: 30, initiative: 50, patience: 60,
  companionship: 55, boundary: 85, dependencyAvoidance: 85,
  execution: 75, explanationDepth: 55, judgment: 75, clarification: 35,
  intimacyExpression: 25, flirtiness: 0, romanticTone: 0,
  suggestivenessAvoidance: 100, intimacyBoundary: 90,
}

const parsedModelValue = typeof props.modelValue === 'string' ? JSON.parse(props.modelValue) : props.modelValue
const model = reactive<PersonalityConfig>({ ...DEFAULT_CONFIG, ...(parsedModelValue || {}) })

// 监听外部 modelValue 变化，同步内部状态（切换角色时）
watch(() => props.modelValue, (newVal) => {
  if (newVal) {
    Object.assign(model, DEFAULT_CONFIG, newVal)
  }
}, { deep: true })

function setSlider(key: keyof PersonalityConfig, value: number) {
  model[key] = Math.max(0, Math.min(100, Math.round(value))) as any
  emit("update:modelValue", { ...model })
}

type DimDef = { key: keyof PersonalityConfig; label: string; desc: string; left: string; right: string }

interface SliderGroup {
  key: string
  label: string
  dims: DimDef[]
}

const groups: SliderGroup[] = [
  {
    key: "intimacy", label: "关系亲近感",
    dims: [
      { key: "familiarity", label: "熟悉感", desc: "像认识很久的程度", left: "礼貌", right: "熟悉" },
      { key: "formality", label: "正式度", desc: "用语正式程度", left: "随意", right: "正式" },
      { key: "customerServiceAvoidance", label: "反客服感", desc: "避免客服语气的强度", left: "允许", right: "严禁" },
    ],
  },
  {
    key: "expression", label: "表达风格",
    dims: [
      { key: "directness", label: "直接性", desc: "表达的直截了当程度", left: "委婉", right: "直接" },
      { key: "verbosity", label: "啰嗦度", desc: "回复的详细啰嗦程度", left: "简洁", right: "详细" },
      { key: "structureLevel", label: "结构化", desc: "条理分明的程度", left: "随性", right: "条理" },
      { key: "shortSentence", label: "短句偏好", desc: "使用短句逐行发送", left: "段落", right: "短句" },
      { key: "toneWords", label: "语气词", desc: "语气词使用频率", left: "不用", right: "多用" },
    ],
  },
  {
    key: "emotion", label: "情绪温度",
    dims: [
      { key: "warmth", label: "温暖度", desc: "回复的情感温度", left: "冷静", right: "温暖" },
      { key: "emotionalExpression", label: "情感表达", desc: "表达情感的强度", left: "克制", right: "充沛" },
      { key: "comfortLevel", label: "舒适感", desc: "带来的安心舒适程度", left: "中立", right: "舒适" },
      { key: "preachingAvoidance", label: "反说教", desc: "避免说教的强度", left: "允许", right: "严禁" },
    ],
  },
  {
    key: "thinking", label: "思维风格",
    dims: [
      { key: "rationality", label: "理性度", desc: "理性分析 vs 感性共情", left: "感性", right: "理性" },
      { key: "humor", label: "幽默感", desc: "回复的幽默程度", left: "严肃", right: "幽默" },
      { key: "teasing", label: "调侃度", desc: "适度调侃用户的程度", left: "避免", right: "可调侃" },
      { key: "initiative", label: "主动性", desc: "主动发起话题的程度", left: "被动", right: "主动" },
      { key: "patience", label: "耐心度", desc: "回复的耐心程度", left: "直接", right: "耐心" },
    ],
  },
  {
    key: "boundary", label: "关系边界",
    dims: [
      { key: "companionship", label: "陪伴感", desc: "陪伴的存在感", left: "独立", right: "陪伴" },
      { key: "boundary", label: "边界感", desc: "关系边界的清晰度", left: "放松", right: "严谨" },
      { key: "dependencyAvoidance", label: "反依赖", desc: "避免用户依赖的强度", left: "允许", right: "严禁" },
    ],
  },
  {
    key: "execution", label: "执行力",
    dims: [
      { key: "execution", label: "执行导向", desc: "给出具体方案的程度", left: "倾听", right: "方案" },
      { key: "explanationDepth", label: "解释深度", desc: "深入解释原由的程度", left: "简述", right: "深入" },
      { key: "judgment", label: "判断力", desc: "对问题做出判断的意愿", left: "谨慎", right: "果断" },
      { key: "clarification", label: "追问澄清", desc: "主动追问澄清的倾向", left: "不追问", right: "会追问" },
    ],
  },
  {
    key: "intimacySafe", label: "亲密安全",
    dims: [
      { key: "intimacyExpression", label: "亲密表达", desc: "表达亲密的程度", left: "克制", right: "可表达" },
      { key: "flirtiness", label: "调情感", desc: "轻度调情的程度", left: "零容忍", right: "可接受" },
      { key: "romanticTone", label: "浪漫色调", desc: "浪漫色彩的浓度", left: "零容忍", right: "可接受" },
      { key: "suggestivenessAvoidance", label: "反暧昧", desc: "避免暧昧暗示的强度", left: "允许", right: "严禁" },
      { key: "intimacyBoundary", label: "亲密边界", desc: "亲密行为的边界严格度", left: "放松", right: "严格" },
    ],
  },
]

function resetToDefaults() {
  Object.assign(model, DEFAULT_CONFIG)
  emit("update:modelValue", { ...model })
}
</script>

<style scoped>
.personality-sliders {
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  padding: 14px 16px;
  background: var(--ac-color-surface);
  max-height: 600px;
  overflow-y: auto;
}

.ps-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--ac-color-border-light);
}

.ps-title {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text);
}

.ps-group {
  margin-bottom: 12px;
}

.ps-group-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--ac-color-primary);
  text-transform: uppercase;
  margin-bottom: 6px;
  padding-left: 2px;
  letter-spacing: 0.5px;
}

.ps-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 6px;
}

.ps-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.ps-label-row {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.ps-label {
  font-size: var(--ac-font-size-xs);
  font-weight: 500;
  color: var(--ac-color-text);
  min-width: 52px;
}

.ps-desc {
  font-size: 10px;
  color: var(--ac-color-text-muted);
  flex: 1;
}

.ps-value {
  font-size: 10px;
  font-weight: 700;
  color: var(--ac-color-primary);
  min-width: 24px;
  text-align: right;
}

.ps-slider-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.ps-left, .ps-right {
  font-size: 10px;
  color: var(--ac-color-text-placeholder);
  white-space: nowrap;
  min-width: 24px;
}

.ps-left { text-align: right; }
.ps-right { text-align: left; }

.ps-slider {
  flex: 1;
}
</style>
