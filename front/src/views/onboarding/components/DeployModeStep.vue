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

    <div v-if="modelValue === 'cloud'" class="cloud-url-section">
      <label class="cloud-url-label">Core 服务器地址</label>
      <el-input
        :model-value="serverURL"
        @update:model-value="emit('update:serverURL', $event)"
        placeholder="例如 http://192.168.1.100:18899"
        clearable
      />
      <p class="cloud-url-hint">输入你部署在服务器上的 Core 完整地址，需包含协议和端口</p>
    </div>
  </div>
</template>
<script setup lang="ts">
defineProps<{ modelValue: string; serverURL?: string }>()
const emit = defineEmits<{ (e: "update:modelValue", v: string): void; (e: "update:serverURL", v: string): void }>()
</script>
<style scoped>
.cloud-url-section {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--ac-color-border-light);
}

.cloud-url-label {
  display: block;
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-text);
  margin-bottom: 8px;
}

.cloud-url-hint {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  margin-top: 6px;
}
</style>
