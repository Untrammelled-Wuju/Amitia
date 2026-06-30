<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div>
    <el-card class="section-card">
      <template #header>
        <span class="card-title">第一步：选择清理条件</span>
      </template>
      <el-form :model="form" label-width="120px" label-position="top" class="cleanup-form">
        <el-row :gutter="20">
          <el-col :xs="24" :sm="12">
            <el-form-item label="清理此日期之前的数据">
              <el-date-picker
                v-model="form.beforeDate"
                type="date"
                placeholder="选择日期"
                value-format="YYYY-MM-DD"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item label="或清理超过 N 天的数据">
              <el-input-number
                v-model="form.olderThanDays"
                :min="1"
                :max="3650"
                placeholder="如 90"
                style="width: 100%"
              />
              <span class="form-hint">留空则不按天数过滤</span>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :xs="24" :sm="12">
            <el-form-item label="按渠道筛选">
              <el-select
                v-model="form.channels"
                multiple
                placeholder="不选择则清理所有渠道"
                style="width: 100%"
              >
                <el-option label="Web" value="web" />
                <el-option label="微信" value="wechat" />
                <el-option label="桌面端" value="desktop" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item label="按来源筛选">
              <el-select
                v-model="form.sources"
                multiple
                placeholder="不选择则清理所有来源"
                style="width: 100%"
              >
                <el-option label="手动" value="manual" />
                <el-option label="导入" value="import" />
                <el-option label="微信" value="wechat" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :xs="24" :sm="12">
            <el-form-item>
              <el-checkbox v-model="form.includeMemories">同时清理关联的记忆数据</el-checkbox>
              <div class="form-hint">默认不清理记忆，除非你明确需要</div>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item>
          <el-button
            type="primary"
            :loading="previewLoading"
            @click="previewCleanup"
          >
            预览清理结果
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card v-if="previewResult" class="section-card preview-card">
      <template #header>
        <span class="card-title">第二步：确认清理范围</span>
      </template>
      <div class="preview-stats">
        <div class="preview-stat">
          <div class="preview-stat-label">待清理会话</div>
          <div class="preview-stat-value">{{ previewResult.conversationCount }}</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">待清理消息</div>
          <div class="preview-stat-value">{{ previewResult.messageCount }}</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">估算释放空间</div>
          <div class="preview-stat-value">{{ previewResult.estimatedSize }}</div>
        </div>
        <div v-if="previewResult.memoryCount > 0" class="preview-stat warn">
          <div class="preview-stat-label">关联记忆</div>
          <div class="preview-stat-value">{{ previewResult.memoryCount }}</div>
        </div>
      </div>
      <el-alert
        type="warning"
        title="清理前将自动备份数据库至 data/backups/ 目录"
        :closable="false"
        show-icon
        style="margin-bottom: 12px"
      />
      <div class="confirm-section">
        <div class="confirm-row">
          <span class="confirm-label">输入「确认清理」以执行：</span>
          <el-input
            v-model="confirmText"
            placeholder="确认清理"
            style="width: 140px"
          />
        </div>
        <el-button
          type="danger"
          :disabled="confirmText !== '确认清理'"
          :loading="confirmLoading"
          @click="executeCleanup"
          class="confirm-btn"
        >
          确认清理
        </el-button>
      </div>
    </el-card>

    <el-card v-if="cleanupResult" class="section-card result-card">
      <template #header>
        <span class="card-title">清理完成</span>
      </template>
      <div class="cleanup-report">
        <div class="report-item highlight">
          <span class="report-label">释放空间：</span>
          <span class="report-value">{{ cleanupResult.freedFormatted }}</span>
        </div>
        <div class="report-item">
          <span class="report-label">清理会话数：</span>
          <span class="report-value">{{ cleanupResult.conversationCount }}</span>
        </div>
        <div class="report-item">
          <span class="report-label">清理消息数：</span>
          <span class="report-value">{{ cleanupResult.messageCount }}</span>
        </div>
        <div v-if="cleanupResult.backupPath" class="report-item">
          <span class="report-label">备份路径：</span>
          <span class="report-value mono">{{ cleanupResult.backupPath }}</span>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { useChatCleanup } from "../composables/useChatCleanup"

const {
  form,
  previewLoading,
  previewResult,
  confirmText,
  confirmLoading,
  cleanupResult,
  previewCleanup,
  executeCleanup,
} = useChatCleanup()
</script>

<style scoped>
.section-card {
  margin-bottom: 16px;
  border: 1px solid var(--el-border-color-light);
}
.card-title {
  font-size: 15px;
  font-weight: 600;
}
.cleanup-form {
  margin-top: 0;
}
.form-hint {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
  margin-left: 8px;
}
.preview-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}
.preview-stat {
  background: var(--el-fill-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  padding: 14px;
  text-align: center;
}
.preview-stat.warn {
  border-color: var(--el-color-warning-light-5);
  background: var(--el-color-warning-light-9);
}
.preview-stat-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}
.preview-stat-value {
  font-size: 20px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}
.preview-stat.warn .preview-stat-value {
  color: var(--el-color-warning);
}
.confirm-section {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}
.confirm-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.confirm-label {
  font-size: 14px;
  color: var(--el-text-color-regular);
}
.cleanup-report {
  font-size: 14px;
}
.report-item {
  padding: 6px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}
.report-item.highlight {
  font-weight: 600;
}
.report-label {
  color: var(--el-text-color-secondary);
}
.report-value {
  color: var(--el-text-color-primary);
  font-weight: 500;
}
.report-value.mono {
  font-family: monospace;
  font-size: 12px;
}
.result-card {
  border-color: var(--el-color-success-light-5);
}

@media (max-width: 600px) {
  .preview-stats {
    grid-template-columns: 1fr 1fr;
  }
  .confirm-section {
    flex-direction: column;
    align-items: flex-start;
  }
  .confirm-btn {
    width: 100%;
  }
}
</style>
