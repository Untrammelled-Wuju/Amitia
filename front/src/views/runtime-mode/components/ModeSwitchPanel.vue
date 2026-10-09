<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <span class="section-title">修改当前 Core 的服务端部署形态</span>
    </template>

    <p class="mode-scope-note">这里修改的是当前 Core 自身的服务端运行配置，不会切换这台设备连接的服务器，也不会自动将 Core 迁移到云主机。</p>
    <div class="mode-switch-row">
      <div
        class="mode-option"
        :class="{ active: modeDeployMode === 'desktop-local' }"
        @click="selectMode('desktop-local')"
      >
        <div class="mo-icon">
          <el-icon :size="24"><Monitor /></el-icon>
        </div>
        <div class="mo-label">本机 Core 部署</div>
        <div class="mo-desc">将当前 Core 配置为本机运行，监听本地地址</div>
        <div class="mo-tag-row">
          <el-tag size="small" effect="plain" type="success"
            >本机 127.0.0.1</el-tag
          >
        </div>
      </div>

      <div class="mode-arrow">
        <el-icon v-if="modeDeployMode === 'desktop-local'" :size="20"
          ><Right
        /></el-icon>
        <div v-else class="mode-arrow-label">当前</div>
      </div>

      <div
        class="mode-option"
        :class="{ active: modeDeployMode === 'cloud-web' }"
        @click="selectMode('cloud-web')"
      >
        <div class="mo-icon">
          <el-icon :size="24"><Cloudy /></el-icon>
        </div>
        <div class="mo-label">Core 私有云部署</div>
        <div class="mo-desc">将当前 Core 配置为适合服务器对外提供服务的形态</div>
        <div class="mo-tag-row">
          <el-tag size="small" effect="plain" type="warning">HTTPS 访问</el-tag>
        </div>
      </div>
    </div>

    <div
      v-if="selectedMode && selectedMode !== modeDeployMode"
      class="impact-box"
    >
      <div class="impact-box-header">
        <el-icon><Warning /></el-icon>
        <span>修改 Core 部署配置的影响</span>
      </div>
      <ul class="impact-list">
        <template v-if="selectedMode === 'cloud-web'">
          <li>当前 Core 将切换为私有云服务配置，但<strong>不会自动迁移至云服务器</strong></li>
          <li>需要在目标服务器自行部署 Core 并配置 HTTPS 反向代理</li>
          <li>需要启用登录验证和配置 publicBaseUrl</li>
          <li>配置变更后可能需要重启 Core 才会生效</li>
        </template>
        <template v-else>
          <li>当前 Core 将切换为本地服务配置，但<strong>不会自动迁移运行位置</strong></li>
          <li>仅在本机地址监听，不会作为对外的云端服务提供访问</li>
          <li>配置变更后可能需要重启 Core 才会生效</li>
        </template>
      </ul>
      <div class="impact-actions">
        <el-button
          type="primary"
          :loading="switching"
          @click="$emit('confirmSwitch', selectedMode)"
        >
          确认修改 Core 为{{
            selectedMode === "cloud-web" ? "私有云部署配置" : "本机部署配置"
          }}
        </el-button>
        <el-button @click="clearSelection">取消</el-button>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { Monitor, Cloudy, Right, Warning } from "@element-plus/icons-vue";
import type { DeployMode } from "@/types";

defineProps<{
  modeDeployMode: string;
  switching: boolean;
}>();

const emit = defineEmits<{
  confirmSwitch: [mode: DeployMode];
}>();

const selectedMode = ref<DeployMode | null>(null);

function selectMode(m: DeployMode) {
  if (selectedMode.value === m) {
    selectedMode.value = null;
    return;
  }
  selectedMode.value = m;
}

function clearSelection() {
  selectedMode.value = null;
}
</script>

<style scoped>
.section-card {
  margin-bottom: 16px;
}
.mode-scope-note { margin: 0 0 18px; color: var(--text-secondary); font-size: 13px; line-height: 1.6; }
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-text);
  display: flex;
  align-items: center;
  gap: 6px;
}
.mode-switch-row {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 12px;
}
.mode-option {
  flex: 1;
  padding: 16px;
  border: 2px solid var(--ac-color-border);
  border-radius: 10px;
  text-align: center;
  cursor: pointer;
  transition: all 0.2s;
  background: var(--ac-color-surface);
}
.mode-option:hover {
  border-color: var(--ac-color-primary);
}
.mode-option.active {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-bg);
}
.mo-icon {
  margin-bottom: 8px;
  color: var(--ac-color-text-secondary);
}
.mode-option.active .mo-icon {
  color: var(--ac-color-primary);
}
.mo-label {
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-text);
}
.mo-desc {
  font-size: 12px;
  color: var(--ac-color-text-secondary);
  margin-top: 4px;
}
.mo-tag-row {
  margin-top: 8px;
}
.mode-arrow {
  display: flex;
  align-items: center;
  color: var(--ac-color-text-muted);
  flex-shrink: 0;
}
.mode-arrow-label {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  background: var(--ac-color-bg-secondary);
  padding: 2px 8px;
  border-radius: 4px;
}
.impact-box {
  padding: 16px;
  border: 1px solid var(--ac-color-warning);
  border-radius: 8px;
  background: var(--ac-color-warning-bg);
  margin-top: 12px;
}
.impact-box-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-warning);
  margin-bottom: 10px;
}
.impact-list {
  margin: 0;
  padding-left: 20px;
  font-size: 13px;
  color: var(--ac-color-text-secondary);
  line-height: 1.8;
}
.impact-list li {
  margin-bottom: 2px;
}
.impact-actions {
  margin-top: 12px;
  display: flex;
  gap: 8px;
}
</style>
