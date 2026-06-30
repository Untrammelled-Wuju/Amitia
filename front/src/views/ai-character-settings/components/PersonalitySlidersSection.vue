<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="slider-sections">
    <div class="section-card">
      <div class="section-title">基础性格</div>
      <div class="slider-grid">
        <SliderRow :model-value="modelValue.familiarity" @update:model-value="(v: number) => emitUpdate('familiarity', v)" label="熟悉感" left="客气" right="熟悉" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.formality" @update:model-value="(v: number) => emitUpdate('formality', v)" label="正式度" left="口语" right="正式" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.warmth" @update:model-value="(v: number) => emitUpdate('warmth', v)" label="亲和度" left="冷静" right="温暖" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.directness" @update:model-value="(v: number) => emitUpdate('directness', v)" label="直接程度" left="委婉" right="直接" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.rationality" @update:model-value="(v: number) => emitUpdate('rationality', v)" label="理性程度" left="感性" right="理性" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.humor" @update:model-value="(v: number) => emitUpdate('humor', v)" label="幽默感" left="严肃" right="幽默" :min="0" :max="100" />
      </div>
    </div>

    <div class="section-card">
      <div class="section-title">聊天习惯</div>
      <div class="slider-grid">
        <SliderRow :model-value="modelValue.verbosity" @update:model-value="(v: number) => emitUpdate('verbosity', v)" label="回复长度" left="简短" right="详细" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.shortSentence" @update:model-value="(v: number) => emitUpdate('shortSentence', v)" label="短句程度" left="段落" right="短句" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.toneWords" @update:model-value="(v: number) => emitUpdate('toneWords', v)" label="语气词使用" left="不用" right="多用" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.initiative" @update:model-value="(v: number) => emitUpdate('initiative', v)" label="主动性" left="被动回应" right="主动找话题" :min="0" :max="100" />
        <div class="slider-hint">
          数值越高，AI 越可能主动延续话题或提醒你。
          系统会自动限制频率（每日 <el-input-number :model-value="modelValue.dailyLimit" @update:model-value="(v: number | undefined) => emitUpdate('dailyLimit', v ?? 3)" :min="1" :max="200" size="small" style="width:80px" /> 条）
        </div>
        <SliderRow :model-value="modelValue.teasing" @update:model-value="(v: number) => emitUpdate('teasing', v)" label="吐槽程度" left="从不禁" right="可吐槽" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.customerServiceAvoidance" @update:model-value="(v: number) => emitUpdate('customerServiceAvoidance', v)" label="客服腔抑制" left="官方" right="自然" :min="0" :max="100" />
      </div>
    </div>

    <div class="section-card">
      <div class="section-title">陪伴设置</div>
      <div class="slider-grid">
        <SliderRow :model-value="modelValue.companionship" @update:model-value="(v: number) => emitUpdate('companionship', v)" label="陪伴感" left="独立" right="陪伴" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.comfortLevel" @update:model-value="(v: number) => emitUpdate('comfortLevel', v)" label="安抚强度" left="中立" right="安抚" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.patience" @update:model-value="(v: number) => emitUpdate('patience', v)" label="耐心" left="直接" right="耐心" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.preachingAvoidance" @update:model-value="(v: number) => emitUpdate('preachingAvoidance', v)" label="说教抑制" left="可说教" right="严禁说教" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.boundary" @update:model-value="(v: number) => emitUpdate('boundary', v)" label="边界感" left="放松" right="严谨" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.dependencyAvoidance" @update:model-value="(v: number) => emitUpdate('dependencyAvoidance', v)" label="依赖引导抑制" left="允许" right="严禁" :min="0" :max="100" />
      </div>
    </div>

    <div class="section-card">
      <div class="section-title">任务能力</div>
      <div class="slider-grid">
        <SliderRow :model-value="modelValue.execution" @update:model-value="(v: number) => emitUpdate('execution', v)" label="执行力" left="倾听" right="给方案" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.explanationDepth" @update:model-value="(v: number) => emitUpdate('explanationDepth', v)" label="解释深度" left="简述" right="深入" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.structureLevel" @update:model-value="(v: number) => emitUpdate('structureLevel', v)" label="结构化程度" left="随性" right="条理" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.judgment" @update:model-value="(v: number) => emitUpdate('judgment', v)" label="判断力" left="谨慎" right="果断" :min="0" :max="100" />
        <SliderRow :model-value="modelValue.clarification" @update:model-value="(v: number) => emitUpdate('clarification', v)" label="追问倾向" left="不追问" right="会追问" :min="0" :max="100" />
      </div>
    </div>

    <el-collapse :model-value="activeCollapse" @update:model-value="(v: string[]) => emit('update:activeCollapse', v)" class="section-collapse">
      <el-collapse-item title="亲密边界（高级设置）" name="intimacy">
        <el-alert type="info" :closable="false" show-icon style="margin-bottom:12px">
          <template #title>
            该设置只控制亲近表达方式，不允许生成色情、露骨或越界内容。
            系统会自动进行安全裁剪。
          </template>
        </el-alert>
        <div class="slider-grid">
          <SliderRow :model-value="modelValue.intimacyExpression" @update:model-value="(v: number) => emitUpdate('intimacyExpression', v)" label="亲密表达强度" left="克制" right="可表达" :min="0" :max="100" />
          <SliderRow :model-value="modelValue.flirtiness" @update:model-value="(v: number) => emitUpdate('flirtiness', v)" label="暧昧倾向" left="零容忍" right="轻微" :min="0" :max="100" />
          <SliderRow :model-value="modelValue.romanticTone" @update:model-value="(v: number) => emitUpdate('romanticTone', v)" label="恋爱感" left="无" right="轻微" :min="0" :max="100" />
          <SliderRow :model-value="modelValue.suggestivenessAvoidance" @update:model-value="(v: number) => emitUpdate('suggestivenessAvoidance', v)" label="性暗示规避" left="允许" right="严禁" :min="0" :max="100" />
          <SliderRow :model-value="modelValue.intimacyBoundary" @update:model-value="(v: number) => emitUpdate('intimacyBoundary', v)" label="亲密边界" left="宽松" right="严格" :min="0" :max="100" />
        </div>
      </el-collapse-item>
    </el-collapse>
  </div>
</template>

<script setup lang="ts">
import SliderRow from "../../../components/SliderRow.vue"

const props = defineProps<{
  modelValue: Record<string, number>
  activeCollapse: string[]
}>()

const emit = defineEmits<{
  (e: "update:modelValue", v: Record<string, number>): void
  (e: "update:activeCollapse", v: string[]): void
}>()

function emitUpdate(key: string, value: number) {
  emit("update:modelValue", { ...props.modelValue, [key]: Math.round(value) })
}
</script>

