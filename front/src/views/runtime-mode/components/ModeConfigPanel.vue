<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="mode-hero" :class="modeClass">
    <div class="mode-hero-icon">
      <el-icon v-if="mode.deployMode === 'desktop-local'" :size="32"><Monitor /></el-icon>
      <el-icon v-else :size="32"><Cloudy /></el-icon>
    </div>
    <div class="mode-hero-body">
      <div class="mode-hero-label">{{ modeLabel }}</div>
      <div class="mode-hero-desc">{{ modeDescription }}</div>
      <div class="mode-hero-addr">
        <el-tag size="small" effect="plain" type="info">{{ mode.host }}:{{ mode.port }}</el-tag>
        <el-tag
          v-if="mode.web.enabled"
          size="small"
          :type="mode.web.requireAuth ? 'warning' : 'success'"
          effect="plain"
          style="margin-left:6px"
        >
          {{ mode.web.requireAuth ? '需要登录' : '可选登录' }}
        </el-tag>
      </div>
    </div>
  </div>

  <el-card shadow="never" class="section-card">
    <template #header>
      <div class="section-header-row">
        <span class="section-title">当前配置</span>
        <el-button size="small" :loading="validating" @click="$emit('validate')">
          <el-icon><Checked /></el-icon>
          校验配置
        </el-button>
      </div>
    </template>

    <el-descriptions :column="2" border size="small">
      <el-descriptions-item label="部署模式">
        <el-tag :type="mode.deployMode === 'desktop-local' ? 'success' : 'warning'" size="small">
          {{ mode.deployMode === 'desktop-local' ? '桌面本地' : '私有云' }}
        </el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="Core 地址">
        {{ mode.host }}:{{ mode.port }}
      </el-descriptions-item>
      <el-descriptions-item label="Web UI">{{ mode.web.enabled ? '已启用' : '已禁用' }}</el-descriptions-item>
      <el-descriptions-item label="登录验证">
        <el-tag :type="mode.web.requireAuth ? 'warning' : 'success'" size="small">
          {{ mode.web.requireAuth ? '必需' : '可选' }}
        </el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="Bridge 模式">
        <el-tag :type="mode.bridge.mode === 'cloud' ? 'warning' : 'success'" size="small">
          {{ mode.bridge.mode === 'cloud' ? '云端' : '本地' }}
        </el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="Bridge 地址">
        {{ mode.bridge.enabled ? mode.bridge.host + ':' + mode.bridge.port : '已禁用' }}
      </el-descriptions-item>
      <el-descriptions-item v-if="mode.deployMode === 'cloud-web'" label="公开地址">
        {{ mode.web.publicBaseUrl || '未配置' }}
      </el-descriptions-item>
      <el-descriptions-item label="数据目录">{{ mode.storage.dataDir }}</el-descriptions-item>
    </el-descriptions>

    <div v-if="validationResult" class="validation-result" :class="validationResult.valid ? 'vr-ok-block' : 'vr-error-block'">
      <div class="vr-header">
        <el-icon v-if="validationResult.valid"><CircleCheck /></el-icon>
        <el-icon v-else><CircleClose /></el-icon>
        <span>{{ validationResult.valid ? '配置校验通过' : '配置存在问题' }}</span>
      </div>
      <div v-if="validationResult.errors.length > 0" class="vr-checks">
        <div
          v-for="(error, index) in validationResult.errors"
          :key="index"
          class="vr-check-item vr-error"
        >
          <span class="vr-check-icon">
            <el-icon><CircleClose /></el-icon>
          </span>
          <div class="vr-check-body">
            <div class="vr-check-name">错误 {{ index + 1 }}</div>
            <div class="vr-check-msg">{{ error }}</div>
          </div>
        </div>
      </div>
      <div v-if="!validationResult.valid && validationResult.errors.length === 0" class="vr-checks">
        <div class="vr-check-item vr-error">
          <span class="vr-check-icon">
            <el-icon><CircleClose /></el-icon>
          </span>
          <div class="vr-check-body">
            <div class="vr-check-name">校验失败</div>
            <div class="vr-check-msg">未返回具体错误信息</div>
          </div>
        </div>
      </div>
    </div>
  </el-card>

  <el-card v-if="mode.deployMode === 'cloud-web'" shadow="never" class="section-card">
    <template #header>
      <span class="section-title">
        <el-icon><List /></el-icon>
        云端部署检查项
      </span>
    </template>
    <el-checkbox-group v-model="cloudChecklistModel" class="checklist-group">
      <el-checkbox label="1" disabled>Core 绑定 0.0.0.0 允许外部访问</el-checkbox>
      <el-checkbox label="2" disabled>配置 HTTPS 反向代理（nginx/Caddy）</el-checkbox>
      <el-checkbox label="3" :value="!!mode.web.publicBaseUrl">
        设置 publicBaseUrl 指向 HTTPS 域名
      </el-checkbox>
      <el-checkbox label="4" :value="mode.bridge.mode === 'cloud'">
        Bridge 在同一云服务器运行（cloud 模式）
      </el-checkbox>
      <el-checkbox label="5" disabled>防火墙开放配置端口</el-checkbox>
      <el-checkbox label="6" :value="mode.web.requireAuth">
        登录验证已启用
      </el-checkbox>
      <el-checkbox label="7" disabled>定期备份数据库</el-checkbox>
    </el-checkbox-group>
  </el-card>

  <el-card v-if="mode.deployMode === 'desktop-local'" shadow="never" class="section-card">
    <template #header>
      <span class="section-title">
        <el-icon><InfoFilled /></el-icon>
        本机运行详情
      </span>
    </template>
    <el-descriptions :column="1" border size="small">
      <el-descriptions-item label="Core 端口">
        <code>{{ mode.host }}:{{ mode.port }}</code>
        <span class="form-tip">仅本机可访问（127.0.0.1）</span>
      </el-descriptions-item>
      <el-descriptions-item label="Bridge 端口">
        <code>{{ mode.bridge.host }}:{{ mode.bridge.port }}</code>
        <span class="form-tip">微信桥在同一台机器上运行</span>
      </el-descriptions-item>
      <el-descriptions-item label="数据位置">
        <code>{{ mode.storage.dataDir }}</code>
        <span class="form-tip">所有数据存储在本地</span>
      </el-descriptions-item>
      <el-descriptions-item label="外部访问">
        <el-tag size="small" type="info">未暴露</el-tag>
        <span class="form-tip">不监听外部网络接口</span>
      </el-descriptions-item>
      <el-descriptions-item label="开机要求">
        <el-tag size="small" type="warning">需要常开</el-tag>
        <span class="form-tip">电脑需要保持开机才能运行</span>
      </el-descriptions-item>
    </el-descriptions>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from "vue"
import {
  Monitor, Cloudy, Checked,
  CircleCheck, CircleClose, InfoFilled, List,
} from "@element-plus/icons-vue"
import type { RuntimeModeResponse, RuntimeModeValidationResult } from "@/types"

const props = defineProps<{
  mode: RuntimeModeResponse
  validating: boolean
  validationResult: RuntimeModeValidationResult | null
  cloudChecklist: string[]
}>()

const emit = defineEmits<{
  validate: []
  'update:cloudChecklist': [value: string[]]
}>()

const cloudChecklistModel = computed({
  get: () => props.cloudChecklist,
  set: (val) => emit('update:cloudChecklist', val),
})

const modeClass = computed(() => ({
  "mode-desktop": props.mode.deployMode === "desktop-local",
  "mode-cloud": props.mode.deployMode === "cloud-web",
}))

const modeLabel = computed(() =>
  props.mode.deployMode === "desktop-local" ? "桌面本地模式" : "私有云模式"
)

const modeDescription = computed(() =>
  props.mode.deployMode === "desktop-local"
    ? "Core 运行在你的电脑上，通过 127.0.0.1 访问，支持免登录使用"
    : "Core 部署在你的云服务器上，通过 HTTPS 远程访问，需要登录。你的电脑不需要常开"
)
</script>

<style scoped>
.section-card { margin-bottom: 16px; }
.section-title { font-size: 14px; font-weight: 600; color: var(--ac-color-text); display: flex; align-items: center; gap: 6px; }
.section-header-row { display: flex; align-items: center; justify-content: space-between; }

.mode-hero { display: flex; align-items: center; gap: 16px; padding: 20px 24px; border-radius: 12px; margin-bottom: 16px; }
.mode-desktop { background: #f0fdf4; border: 1px solid #bbf7d0; color: #166534; }
.mode-cloud { background: #eff6ff; border: 1px solid #bfdbfe; color: #1e40af; }
.mode-hero-icon { flex-shrink: 0; }
.mode-hero-body { flex: 1; min-width: 0; }
.mode-hero-label { font-size: 16px; font-weight: 700; color: var(--ac-color-text); }
.mode-hero-desc { font-size: 13px; color: var(--ac-color-text-secondary); margin-top: 4px; line-height: 1.5; }
.mode-hero-addr { margin-top: 8px; display: flex; align-items: center; gap: 6px; }

.validation-result { margin-top: 16px; padding: 12px 16px; border-radius: 8px; }
.vr-ok-block { background: #f0fdf4; border: 1px solid #bbf7d0; }
.vr-error-block { background: #fef2f2; border: 1px solid #fecaca; }
.vr-header { display: flex; align-items: center; gap: 6px; font-size: 14px; font-weight: 600; margin-bottom: 10px; }
.vr-ok-block .vr-header { color: #166534; }
.vr-error-block .vr-header { color: #991b1b; }
.vr-checks { display: flex; flex-direction: column; gap: 8px; }
.vr-check-item { display: flex; align-items: flex-start; gap: 10px; padding: 8px 10px; border-radius: 6px; }
.vr-ok { }
.vr-warn { background: #fefce8; }
.vr-error { background: #fef2f2; }
.vr-check-icon { font-size: 16px; flex-shrink: 0; margin-top: 2px; }
.vr-ok .vr-check-icon { color: #16a34a; }
.vr-warn .vr-check-icon { color: #ca8a04; }
.vr-error .vr-check-icon { color: #dc2626; }
.vr-check-body { flex: 1; min-width: 0; }
.vr-check-name { font-size: 12px; font-weight: 600; color: var(--ac-color-text); }
.vr-check-msg { font-size: 12px; color: var(--ac-color-text-secondary); margin-top: 2px; }
.vr-check-suggestion { font-size: 11px; color: #92400e; margin-top: 4px; padding: 4px 8px; border-radius: 4px; background: #fef3c7; line-height: 1.4; }

.checklist-group { display: flex; flex-direction: column; gap: 10px; }

.form-tip { display: block; font-size: 11px; color: var(--ac-color-text-muted); margin-top: 2px; }
code { font-family: 'Consolas', 'Courier New', monospace; font-size: 12px; background: var(--ac-color-bg-secondary); padding: 1px 6px; border-radius: 3px; }
</style>
