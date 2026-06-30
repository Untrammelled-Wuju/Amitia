<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <span class="step-badge">1</span> 粘贴聊天记录
    </template>
    <div class="input-area">
      <el-input
        :model-value="rawText"
        type="textarea"
        :rows="8"
        placeholder="粘贴聊天记录。示例：

用户：我今天有点累
AI：那就休息一下。

[2026-05-18 12:00] 我：晚饭吃什么？
[2026-05-18 12:01] 你：随你喜欢

2026/05/18 12:00 张三
你好！
2026/05/18 12:01 李四
你好呀！"
        @update:model-value="$emit('update:rawText', $event)"
      />
      <div class="input-options">
        <el-input :model-value="batchTitle" placeholder="标题（可选）" size="small" style="width:200px" @update:model-value="$emit('update:batchTitle', $event)" />
        <div class="format-picker">
          <span class="fp-label">格式：</span>
          <el-radio-group :model-value="parseFormat" size="small" @update:model-value="$emit('update:parseFormat', $event)">
            <el-radio-button value="auto">自动</el-radio-button>
            <el-radio-button value="standard">标准</el-radio-button>
            <el-radio-button value="timestamp">时间戳</el-radio-button>
            <el-radio-button value="multiline">多行</el-radio-button>
            <el-radio-button value="wechat">微信</el-radio-button>
          </el-radio-group>
        </div>
      </div>
      <el-collapse :model-value="showSpeakerOptions" style="border:none" @update:model-value="$emit('update:showSpeakerOptions', $event)">
        <el-collapse-item title="自定义发言者名称映射" name="options">
          <div class="speaker-options">
            <div class="so-group">
              <span class="so-label">用户发言者（逗号分隔）：</span>
              <el-input :model-value="userSpeakerInput" placeholder="例如：张三、我、自己" size="small" @update:model-value="$emit('update:userSpeakerInput', $event)" />
            </div>
            <div class="so-group">
              <span class="so-label">AI 发言者（逗号分隔）：</span>
              <el-input :model-value="assistantSpeakerInput" placeholder="例如：AI、李四、Bot" size="small" @update:model-value="$emit('update:assistantSpeakerInput', $event)" />
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
      <div class="input-actions">
        <el-button type="primary" :icon="Reading" :loading="parsing" :disabled="!rawText.trim()" @click="$emit('parse')">
          解析
        </el-button>
        <el-upload :auto-upload="false" :show-file-list="false" :on-change="(file: any) => $emit('fileChange', file)">
          <el-button :icon="Upload">上传 .txt / .md</el-button>
        </el-upload>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { Reading, Upload } from "@element-plus/icons-vue"

defineProps<{
  rawText: string
  batchTitle: string
  parseFormat: string
  parsing: boolean
  showSpeakerOptions: string[]
  userSpeakerInput: string
  assistantSpeakerInput: string
}>()

defineEmits<{
  'update:rawText': [value: string]
  'update:batchTitle': [value: string]
  'update:parseFormat': [value: string]
  'update:showSpeakerOptions': [value: string[]]
  'update:userSpeakerInput': [value: string]
  'update:assistantSpeakerInput': [value: string]
  parse: []
  fileChange: [file: any]
}>()
</script>

<style scoped>
.section-card { margin-bottom: 12px; }
.step-badge {
  display: inline-flex; align-items: center; justify-content: center;
  width: 22px; height: 22px; border-radius: 50%;
  background: var(--ac-color-primary); color: #fff;
  font-size: 11px; font-weight: 700; margin-right: 6px; flex-shrink: 0;
}
.input-area { display: flex; flex-direction: column; gap: 10px; }
.input-options { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.format-picker { display: flex; align-items: center; gap: 6px; }
.fp-label { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-secondary); white-space: nowrap; }
.input-actions { display: flex; gap: 8px; }
.speaker-options { display: flex; flex-direction: column; gap: 10px; padding: 8px 0; }
.so-group { display: flex; flex-direction: column; gap: 4px; }
.so-label { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-secondary); }
@media (max-width: 768px) {
  .input-options { flex-direction: column; align-items: stretch; }
}
</style>
