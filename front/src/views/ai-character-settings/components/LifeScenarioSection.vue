<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="section-card">
    <div class="section-title">生活场景</div>
    <div class="gender-grid">
      <div class="gender-item">
        <label class="gender-label">选择生活场景</label>
        <el-select v-model="lifeIdentityModel" placeholder="选择生活场景" size="default" style="width:100%" @change="(v: string) => emit('change', v)">
          <el-option v-for="opt in LIFE_IDENTITY_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <span class="gender-hint">选择后将自动显示对应的生活规则配置项</span>
      </div>
      <div class="gender-item" v-if="isCustomLifeIdentity">
        <label class="gender-label">自定义场景描述</label>
        <el-input v-model="lifeIdentityCustomModel" placeholder="例如：自由插画师、考研党、数字游民..." size="default" />
        <span class="gender-hint">手动输入你的生活场景，角色会据此调整行为</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { LIFE_IDENTITY_OPTIONS } from "../../../composables/useFixedEvents"

const PRESET_IDENTITIES = ["SCHOOL", "WORK", "UNEMPLOYED", "HOME"]

const props = defineProps<{
  lifeIdentity: string
  lifeIdentityCustom: string
}>()

const emit = defineEmits<{
  (e: "update:lifeIdentity", v: string): void
  (e: "update:lifeIdentityCustom", v: string): void
  (e: "change", v: string): void
}>()

const isCustomLifeIdentity = computed(() => !PRESET_IDENTITIES.includes(props.lifeIdentity))

const lifeIdentityModel = computed({
  get: () => props.lifeIdentity,
  set: (v) => emit("update:lifeIdentity", v)
})

const lifeIdentityCustomModel = computed({
  get: () => props.lifeIdentityCustom,
  set: (v) => emit("update:lifeIdentityCustom", v)
})
</script>
