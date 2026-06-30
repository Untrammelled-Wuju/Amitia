<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="step-panel">
    <h2>QQ 接入（可选）</h2>
    <p class="step-desc">通过 QQ 机器人，在 QQ 中与 AI 角色对话。</p>
    <div class="toggle-card">
      <div class="tc-desc">需要后端运行 QQ 桥接服务。可稍后在设置中配置。</div>
      <el-switch v-model="qqModel" />
    </div>
    <template v-if="modelValue">
      <template v-if="qqConnected">
        <el-alert type="success" :closable="false" show-icon style="margin-top:16px;margin-bottom:12px">
          <template #title>QQ Bot 已成功连接</template>
        </el-alert>
        <el-card shadow="never" class="section-card">
          <template #header>
            <div class="card-header-row">
              <span class="card-header-title">连接状态</span>
              <el-button size="small" @click="emit('resetQQ')">重新连接</el-button>
            </div>
          </template>
          <div class="status-main">
            <div class="status-row">
              <div class="status-indicator ok"></div>
              <span class="status-label">已连接</span>
            </div>
            <div class="status-detail-grid">
              <div class="sd-item">
                <span class="sd-label">Bot ID</span>
                <span class="sd-value">{{ qqAccountId }}</span>
              </div>
              <div class="sd-item">
                <span class="sd-label">协议</span>
                <span class="sd-value">QQBot (WebSocket)</span>
              </div>
              <div class="sd-item">
                <span class="sd-label">消息数</span>
                <span class="sd-value">{{ qqMessageCount }}</span>
              </div>
            </div>
          </div>
        </el-card>
        <el-alert type="warning" :closable="false" show-icon style="margin-top:12px">
          <template #title>主动推送须知</template>
          添加QQ好友后，必须主动给机器人发一条消息，系统才能记录你的用户ID用于主动推送。用户ID每7天自动刷新，届时需重新发送一条消息。
        </el-alert>
      </template>

      <template v-if="!qqConnected">
        <el-form label-position="top" size="default" style="margin-top:16px">
          <el-form-item label="AppID">
            <el-input v-model="qqAppIdModel" placeholder="QQ 机器人 AppID" />
          </el-form-item>
          <el-form-item label="Token">
            <el-input v-model="qqTokenModel" placeholder="QQ 机器人 Token" type="password" show-password />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="emit('connectQQ')" :loading="qqConnecting" :disabled="!qqConnected && (!qqAppId || !qqToken)">
              {{ qqConnecting ? '连接中...' : '连接' }}
            </el-button>
            <span v-if="qqError" style="color:#f56c6c;margin-left:8px">{{ qqError }}</span>
          </el-form-item>
        </el-form>
        <div class="step-illustration" style="margin-top:12px">
          <span>使用步骤：</span>
          <span>1. 前往 <a href="https://q.qq.com/" target="_blank" style="color:#409eff">QQ开放平台</a> 创建机器人</span>
          <span>2. 获取 AppID 和 Token</span>
          <span>3. 填入上方表单，点击"连接"</span>
          <span>4. 连接成功后，在QQ中 @机器人 即可对话</span>
        </div>
        <el-alert type="warning" :closable="false" show-icon style="margin-top:12px">
          <template #title>主动推送须知</template>
          连接成功后，添加QQ好友必须主动给机器人发一条消息，系统才能记录你的用户ID用于主动推送。用户ID每7天自动刷新，届时需重新发送一条消息。
        </el-alert>
      </template>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed } from "vue"
const props = defineProps<{
  modelValue: boolean
  qqConnected: boolean; qqConnecting: boolean
  qqAccountId: string; qqMessageCount: number; qqError: string
  qqAppId: string; qqToken: string
}>()
const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void
  (e: "update:qqAppId", v: string): void
  (e: "update:qqToken", v: string): void
  (e: "connectQQ"): void
  (e: "resetQQ"): void
}>()
const qqModel = computed({ get: () => props.modelValue, set: (v) => emit("update:modelValue", v) })
const qqAppIdModel = computed({ get: () => props.qqAppId, set: (v) => emit("update:qqAppId", v) })
const qqTokenModel = computed({ get: () => props.qqToken, set: (v) => emit("update:qqToken", v) })
</script>
