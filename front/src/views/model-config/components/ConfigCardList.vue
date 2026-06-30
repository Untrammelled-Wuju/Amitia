<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div>
    <div class="config-cards" v-if="configs.length > 0">
      <div
        v-for="cfg in configs"
        :key="cfg.id"
        class="config-card"
        :class="{ 'is-active': cfg.isActive }"
      >
        <div class="card-top">
          <div class="card-header">
            <span class="card-name">{{ cfg.name }}</span>
            <el-tag v-if="cfg.isActive" type="success" size="small" effect="dark">当前</el-tag>
          </div>
          <div class="card-type">
            <el-tag size="small" :type="cfg.apiType === 'ollama' ? 'info' : 'primary'">
              {{ providerName(cfg.apiType) }}
            </el-tag>
            <span class="card-model">{{ cfg.modelName }}</span>
          </div>
        </div>

        <div class="card-details">
          <div class="detail-row">
            <span class="dl">Base URL</span>
            <span class="dv">{{ cfg.baseUrl || "未设置" }}</span>
          </div>
          <div class="detail-row">
            <span class="dl">API Key</span>
            <span class="dv">{{ cfg.hasApiKey ? "已设置" : "未设置" }}</span>
          </div>
          <div class="detail-row">
            <span class="dl">温度</span>
            <span class="dv">{{ cfg.temperature ?? 0.7 }}</span>
          </div>
          <div class="detail-row">
            <span class="dl">最大 Token</span>
            <span class="dv">{{ cfg.maxTokens ?? 4096 }}</span>
          </div>
        </div>

        <div class="card-test" v-if="cfg.lastTestStatus">
          <div class="test-indicator" :class="cfg.lastTestStatus">
            <span class="test-dot"></span>
            <span>上次测试: {{ cfg.lastTestStatus === 'success' ? '通过' : '失败' }}</span>
          </div>
          <div class="test-msg" v-if="cfg.lastTestMessage">{{ cfg.lastTestMessage }}</div>
          <div class="test-time" v-if="cfg.lastTestAt">{{ fmtDate(cfg.lastTestAt) }}</div>
        </div>

        <div class="card-actions">
          <el-button
            size="small"
            :loading="testingId === cfg.id"
            @click="emit('test', cfg.id)"
          >
            {{ testingId === cfg.id ? '测试中...' : '测试连接' }}
          </el-button>
          <el-button size="small" @click="emit('edit', cfg)">编辑</el-button>
          <el-button
            v-if="!cfg.isActive"
            size="small"
            type="primary"
            @click="emit('setActive', cfg.id)"
          >
            设为默认
          </el-button>
          <el-button
            size="small"
            type="danger"
            :disabled="cfg.isActive && configs.length <= 1"
            @click="emit('delete', cfg.id)"
          >
            删除
          </el-button>
        </div>
      </div>
    </div>

    <el-empty v-else description="还没有模型配置" :image-size="80">
      <el-button type="primary" @click="emit('add')">新增配置</el-button>
    </el-empty>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{
  configs: any[]
  testingId: number | null
  providers: any[]
}>()

const emit = defineEmits<{
  (e: "test", id: number): void
  (e: "edit", cfg: any): void
  (e: "setActive", id: number): void
  (e: "delete", id: number): void
  (e: "add"): void
}>()

function providerName(apiType: string): string {
  const p = props.providers.find((pr: any) => pr.id === apiType)
  return p?.name || apiType
}

function fmtDate(dateStr: string): string {
  if (!dateStr) return ""
  try { return new Date(dateStr).toLocaleString("zh-CN") } catch { return dateStr }
}
</script>

<style scoped>
.config-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;
}

.config-card {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  padding: 16px;
  transition: border-color .2s;
}

.config-card.is-active {
  border-color: var(--ac-color-success);
}

.card-top {
  margin-bottom: 10px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.card-name {
  font-size: var(--ac-font-size-base);
  font-weight: 600;
  color: var(--ac-color-text);
}

.card-type {
  display: flex;
  align-items: center;
  gap: 8px;
}

.card-model {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-secondary);
  font-family: monospace;
}

.card-details {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px 12px;
  margin-bottom: 10px;
}

.detail-row {
  display: flex;
  gap: 6px;
  font-size: var(--ac-font-size-sm);
  overflow: hidden;
}

.dl {
  color: var(--ac-color-text-muted);
  flex-shrink: 0;
  min-width: 60px;
}

.dv {
  color: var(--ac-color-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: flex;
  align-items: center;
}

.card-test {
  background: var(--ac-color-bg-secondary);
  border-radius: var(--ac-radius-sm);
  padding: 10px 12px;
  margin-bottom: 10px;
  font-size: var(--ac-font-size-sm);
}

.test-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
}

.test-indicator.success { color: var(--ac-color-success); }
.test-indicator.failed { color: var(--ac-color-danger); }

.test-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.test-indicator.success .test-dot { background: var(--ac-color-success); }
.test-indicator.failed .test-dot { background: var(--ac-color-danger); }

.test-msg {
  margin-top: 4px;
  color: var(--ac-color-text-secondary);
  font-size: var(--ac-font-size-sm);
}

.test-time {
  margin-top: 2px;
  color: var(--ac-color-text-muted);
  font-size: var(--ac-font-size-xs);
}

.card-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

@media (max-width: 640px) {
  .card-details {
    grid-template-columns: 1fr;
  }
}
</style>
