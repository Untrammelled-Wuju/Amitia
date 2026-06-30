<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <span class="section-title">切换模式</span>
    </template>

    <div class="mode-switch-row">
      <div
        class="mode-option"
        :class="{ active: modeDeployMode === 'desktop-local' }"
        @click="selectMode('desktop-local')"
      >
        <div class="mo-icon"><el-icon :size="24"><Monitor /></el-icon></div>
        <div class="mo-label">桌面本地模式</div>
        <div class="mo-desc">Core 在本机运行，可免登录</div>
        <div class="mo-tag-row">
          <el-tag size="small" effect="plain" type="success">本机 127.0.0.1</el-tag>
        </div>
      </div>

      <div class="mode-arrow">
        <el-icon v-if="modeDeployMode === 'desktop-local'" :size="20"><Right /></el-icon>
        <div v-else class="mode-arrow-label">当前</div>
      </div>

      <div
        class="mode-option"
        :class="{ active: modeDeployMode === 'cloud-web' }"
        @click="selectMode('cloud-web')"
      >
        <div class="mo-icon"><el-icon :size="24"><Cloudy /></el-icon></div>
        <div class="mo-label">私有云模式</div>
        <div class="mo-desc">Core 部署在云服务器，需登录</div>
        <div class="mo-tag-row">
          <el-tag size="small" effect="plain" type="warning">HTTPS 访问</el-tag>
        </div>
      </div>
    </div>

    <div v-if="selectedMode && selectedMode !== modeDeployMode" class="impact-box">
      <div class="impact-box-header">
        <el-icon><Warning /></el-icon>
        <span>模式切换的影响</span>
      </div>
      <ul class="impact-list">
        <template v-if="selectedMode === 'cloud-web'">
          <li>Core 将在你的云服务器上运行</li>
          <li>Web UI 通过 HTTPS 访问</li>
          <li><strong>登录变为必需</strong>（系统自动开启）</li>
          <li>微信桥 Bridge 在同一云服务器或内网运行</li>
          <li>你的个人电脑<strong>不需要常开</strong></li>
          <li>需要配置 publicBaseUrl 指向你的域名</li>
        </template>
        <template v-else>
          <li>Core 将在本机运行（127.0.0.1）</li>
          <li>登录可选择关闭（免登录模式）</li>
          <li>微信桥 Bridge 在本机启动</li>
          <li>你的电脑需要<strong>保持开机</strong></li>
          <li>仅限本机访问，不暴露到网络</li>
        </template>
      </ul>
      <div class="impact-actions">
        <el-button type="primary" :loading="switching" @click="$emit('confirmSwitch', selectedMode)">
          确认切换到{{ selectedMode === 'cloud-web' ? '私有云模式' : '桌面本地模式' }}
        </el-button>
        <el-button @click="clearSelection">取消</el-button>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue"
import { Monitor, Cloudy, Right, Warning } from "@element-plus/icons-vue"
import type { DeployMode } from "@/types"

defineProps<{
  modeDeployMode: string
  switching: boolean
}>()

const emit = defineEmits<{
  confirmSwitch: [mode: DeployMode]
}>()

const selectedMode = ref<DeployMode | null>(null)

function selectMode(m: DeployMode) {
  if (selectedMode.value === m) {
    selectedMode.value = null
    return
  }
  selectedMode.value = m
}

function clearSelection() {
  selectedMode.value = null
}
</script>

<style scoped>
.section-card { margin-bottom: 16px; }
.section-title { font-size: 14px; font-weight: 600; color: var(--ac-color-text); display: flex; align-items: center; gap: 6px; }
.mode-switch-row { display: flex; align-items: center; gap: 20px; margin-bottom: 12px; }
.mode-option {
  flex: 1; padding: 16px; border: 2px solid var(--ac-color-border);
  border-radius: 10px; text-align: center; cursor: pointer;
  transition: all 0.2s; background: var(--ac-color-surface);
}
.mode-option:hover { border-color: var(--ac-color-primary); }
.mode-option.active {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-bg, #f0f8f2);
}
.mo-icon { margin-bottom: 8px; color: var(--ac-color-text-secondary); }
.mode-option.active .mo-icon { color: var(--ac-color-primary); }
.mo-label { font-size: 14px; font-weight: 600; color: var(--ac-color-text); }
.mo-desc { font-size: 12px; color: var(--ac-color-text-secondary); margin-top: 4px; }
.mo-tag-row { margin-top: 8px; }
.mode-arrow { display: flex; align-items: center; color: var(--ac-color-text-muted); flex-shrink: 0; }
.mode-arrow-label { font-size: 12px; color: var(--ac-color-text-muted); background: var(--ac-color-bg-secondary); padding: 2px 8px; border-radius: 4px; }
.impact-box { padding: 16px; border: 1px solid #fde68a; border-radius: 8px; background: #fffbeb; margin-top: 12px; }
.impact-box-header { display: flex; align-items: center; gap: 6px; font-size: 14px; font-weight: 600; color: #92400e; margin-bottom: 10px; }
.impact-list { margin: 0; padding-left: 20px; font-size: 13px; color: #78350f; line-height: 1.8; }
.impact-list li { margin-bottom: 2px; }
.impact-actions { margin-top: 12px; display: flex; gap: 8px; }
</style>
