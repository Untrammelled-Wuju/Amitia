<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="section-card">
    <div class="section-title">角色性别</div>
    <div class="gender-grid">
      <div class="gender-item">
        <label class="gender-label">角色性别</label>
        <el-select
          v-model="gender"
          placeholder="选择性别"
          size="default"
          style="width: 100%"
        >
          <el-option
            v-for="opt in GENDER_OPTIONS"
            :key="opt.value"
            :label="opt.label"
            :value="opt.value"
          />
        </el-select>
        <span class="gender-hint">角色的基础性别画像，不影响聊天功能</span>
      </div>
      <div class="gender-item">
        <label class="gender-label">角色代词</label>
        <el-select
          v-model="pronoun"
          placeholder="选择代词"
          size="default"
          style="width: 100%"
          :disabled="gender !== 'CUSTOM'"
        >
          <el-option
            v-for="opt in PRONOUN_OPTIONS"
            :key="opt.value"
            :label="opt.label"
            :value="opt.value"
          />
        </el-select>
        <span class="gender-hint">在对话中引用角色时使用的代词</span>
      </div>
      <div class="gender-item" v-if="gender === 'CUSTOM'">
        <label class="gender-label">自定义性别标签</label>
        <el-input
          v-model="genderLabel"
          placeholder='例如"伙伴""守护者"'
          size="default"
        />
        <span class="gender-hint">CUSTOM 模式下可自由定义性别标签</span>
      </div>
      <div class="gender-item" v-if="gender === 'CUSTOM'">
        <label class="gender-label">自定义代词</label>
        <el-input
          v-model="pronoun"
          placeholder='例如"TA""它"'
          size="default"
        />
        <span class="gender-hint">CUSTOM 模式下可自由定义代词</span>
      </div>
      <div class="gender-item">
        <label class="gender-label">用户称呼风格</label>
        <el-select
          v-model="userAddressingStyle"
          placeholder="选择风格"
          size="default"
          style="width: 100%"
          clearable
        >
          <el-option
            v-for="opt in ADDRESSING_OPTIONS"
            :key="opt.value"
            :label="opt.label"
            :value="opt.value"
          />
        </el-select>
        <span class="gender-hint">角色称呼用户的风格</span>
      </div>
      <div class="gender-item">
        <label class="gender-label">
          性别表达强度
          <span class="gender-value">{{ genderExpression }}</span>
        </label>
        <div class="sr-body">
          <span class="sr-left">中性</span>
          <div class="sr-slider-wrap">
            <el-slider
              v-model="genderExpression"
              :min="0"
              :max="100"
              :step="1"
              class="sr-slider"
            />
          </div>
          <span class="sr-right">明显</span>
        </div>
        <span class="gender-hint"
          >控制角色在语言风格、生活习惯中体现性别特征的强弱</span
        >
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from "vue";
import {
  GENDER_OPTIONS,
  PRONOUN_OPTIONS,
  ADDRESSING_OPTIONS,
  GENDER_PRONOUN_MAP,
} from "../../../composables/useRoleProfile";

let genderForm = defineModel<any>("genderForm", { required: true });
const gender = computed({
  get: () => genderForm.value.gender,
  set: (value) => {
    genderForm.value.gender = value;
  },
});
const pronoun = computed({
  get: () => genderForm.value.pronoun,
  set: (value) => {
    genderForm.value.pronoun = value;
  },
});
const genderLabel = computed({
  get: () => genderForm.value.genderLabel,
  set: (value) => {
    genderForm.value.genderLabel = value;
  },
});
const userAddressingStyle = computed({
  get: () => genderForm.value.userAddressingStyle,
  set: (value) => {
    genderForm.value.userAddressingStyle = value;
  },
});
const genderExpression = computed({
  get: () => genderForm.value.genderExpression,
  set: (value) => {
    genderForm.value.genderExpression = value;
  },
});

watch(
  () => genderForm.value.gender,
  (newGender) => {
    if (newGender !== "CUSTOM" && GENDER_PRONOUN_MAP[newGender]) {
      genderForm.value.pronoun = GENDER_PRONOUN_MAP[newGender];
    }
  },
);
</script>
