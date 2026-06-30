<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="step-panel">
    <h2>选择部署方式</h2>
    <p class="step-desc">决定阿米提亚的运行环境和访问方式。</p>
    <div class="deploy-cards">
      <div class="deploy-card" :class="{ selected: modelValue === 'desktop' }" @click="emit('update:modelValue', 'desktop')">
        <div class="dc-radio"><span class="dc-dot" :class="{ on: modelValue === 'desktop' }"></span></div>
        <div class="dc-body">
          <div class="dc-title">桌面本地模式</div>
          <p class="dc-desc">Core 和 Web 都运行在本机，仅本机访问。数据完全在本地，微信桥接直接用</p>
        </div>
      </div>
      <div class="deploy-card" :class="{ selected: modelValue === 'cloud' }" @click="emit('update:modelValue', 'cloud')">
        <div class="dc-radio"><span class="dc-dot" :class="{ on: modelValue === 'cloud' }"></span></div>
        <div class="dc-body">
          <div class="dc-title">私有云模式</div>
          <p class="dc-desc">Core 部署在服务器上，手机/多设备可访问。微信桥接仍需一台本地电脑运行</p>
        </div>
      </div>
    </div>
  </div>
</template>
<script setup lang="ts">
defineProps<{ modelValue: string }>()
const emit = defineEmits<{ (e: "update:modelValue", v: string): void }>()
</script>
