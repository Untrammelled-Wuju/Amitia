<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="step-panel">
    <h2>创建 AI 陪伴角色</h2>
    <p class="step-desc">给你的 AI 陪伴角色一个名字和性格，让它更有人情味。</p>
    <el-form label-position="top" size="default">
      <el-form-item label="角色名称">
        <el-input v-model="charNameModel" placeholder="例如：小暖" />
      </el-form-item>
      <el-form-item label="角色身份">
        <el-input v-model="charIdentityModel" placeholder="AI 虚拟陪伴角色" />
      </el-form-item>
      <el-form-item label="性格描述">
        <el-input v-model="charPersonalityModel" type="textarea" :rows="2" placeholder="温和、体贴、有耐心" />
      </el-form-item>
    </el-form>
  </div>
</template>
<script setup lang="ts">
import { computed } from "vue"
const props = defineProps<{ charName: string; charIdentity: string; charPersonality: string }>()
const emit = defineEmits<{
  (e: "update:charName", v: string): void
  (e: "update:charIdentity", v: string): void
  (e: "update:charPersonality", v: string): void
}>()
const charNameModel = computed({ get: () => props.charName, set: (v) => emit("update:charName", v) })
const charIdentityModel = computed({ get: () => props.charIdentity, set: (v) => emit("update:charIdentity", v) })
const charPersonalityModel = computed({ get: () => props.charPersonality, set: (v) => emit("update:charPersonality", v) })
</script>
