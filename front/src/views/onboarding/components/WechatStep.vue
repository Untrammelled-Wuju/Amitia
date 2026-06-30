<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="step-panel">
    <h2>微信接入（可选）</h2>
    <p class="step-desc">通过微信桥接，在微信中与 AI 角色对话。</p>
    <div class="toggle-card">
      <div class="tc-desc">需要桌面端运行微信桥接服务。可稍后在设置中配置。</div>
      <el-switch v-model="wechatModel" />
    </div>
    <template v-if="modelValue">
      <template v-if="wxConnected">
        <el-alert type="success" :closable="false" show-icon style="margin-top:16px;margin-bottom:12px">
          <template #title>微信已成功连接</template>
        </el-alert>
        <el-card shadow="never" class="section-card">
          <template #header>
            <div class="card-header-row">
              <span class="card-header-title">连接状态</span>
              <el-button size="small" @click="emit('startWxLogin')" :loading="wxQrLoading">重新扫码</el-button>
            </div>
          </template>
          <div class="status-main">
            <div class="status-row">
              <div class="status-indicator ok"></div>
              <span class="status-label">已连接</span>
            </div>
            <div class="status-detail-grid">
              <div class="sd-item">
                <span class="sd-label">消息数</span>
                <span class="sd-value">{{ wxMessageCount }}</span>
              </div>
              <div class="sd-item" v-if="wxAccountId">
                <span class="sd-label">账号</span>
                <span class="sd-value">{{ wxAccountId.slice(0, 12) }}...</span>
              </div>
              <div class="sd-item">
                <span class="sd-label">模式</span>
                <span class="sd-value">OpenClaw</span>
              </div>
            </div>
          </div>
        </el-card>
        <el-alert type="warning" :closable="false" show-icon style="margin-top:12px">
          <template #title>主动推送须知</template>
          添加微信好友后，必须主动给机器人发一条消息，系统才能记录你的用户ID用于主动推送。用户ID每7天自动刷新，届时需重新发送一条消息。
        </el-alert>
      </template>

      <template v-if="!wxConnected">
        <el-card shadow="never" class="section-card" style="margin-top:16px">
          <template #header><span class="card-header-title">扫码连接</span></template>
          <div class="login-layout">
            <div class="login-steps">
              <div class="qr-step-row">
                <div class="qr-step-num" :class="{ active: wxQrStep >= 0 }">1</div>
                <div class="qr-step-body">
                  <span class="qr-step-title">生成二维码</span>
                  <el-button size="small" type="primary" :loading="wxQrLoading" @click="emit('startWxLogin')" :disabled="wxQrLoading">获取二维码</el-button>
                </div>
              </div>
              <div class="qr-step-row">
                <div class="qr-step-num" :class="{ active: wxQrStep >= 1 }">2</div>
                <div class="qr-step-body">
                  <span class="qr-step-title">用微信扫码</span>
                  <span v-if="wxQrStep >= 1" class="qr-status">
                    <el-icon class="is-loading" v-if="wxScanning"><Loading /></el-icon>
                    {{ wxScanning ? '等待扫码中...' : '请扫描二维码' }}
                  </span>
                </div>
              </div>
              <div class="qr-step-row">
                <div class="qr-step-num" :class="{ active: wxConnected }">3</div>
                <div class="qr-step-body">
                  <span class="qr-step-title">确认连接</span>
                  <span v-if="wxConnected" class="qr-done">
                    <el-icon><CircleCheckFilled /></el-icon> 已连接
                  </span>
                </div>
              </div>
            </div>
            <div class="login-qr">
              <div class="qr-frame" v-if="wxQrCodeUrl">
                <img :src="wxQrCodeUrl" alt="二维码" />
              </div>
              <div class="qr-frame qr-empty" v-else>
                <el-icon :size="36"><Picture /></el-icon>
                <span>点击按钮获取二维码</span>
              </div>
            </div>
          </div>
          <div class="qr-tip" v-if="wxQrCodeUrl">打开微信，扫描二维码确认连接。</div>
        </el-card>
        <div v-if="wxError" style="color:#f56c6c;font-size:12px;margin-top:8px">{{ wxError }}</div>
        <el-alert type="warning" :closable="false" show-icon style="margin-top:12px">
          <template #title>主动推送须知</template>
          添加微信好友后，必须主动给机器人发一条消息，系统才能记录你的用户ID用于主动推送。用户ID每7天自动刷新，届时需重新发送一条消息。
        </el-alert>
      </template>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed } from "vue"
import { Loading, Picture, CircleCheckFilled } from "@element-plus/icons-vue"
const props = defineProps<{
  modelValue: boolean
  wxQrLoading: boolean
  wxQrCodeUrl: string
  wxQrStep: number
  wxScanning: boolean
  wxConnected: boolean
  wxAccountId: string
  wxMessageCount: number
  wxError: string
}>()
const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void
  (e: "startWxLogin"): void
}>()
const wechatModel = computed({ get: () => props.modelValue, set: (v) => emit("update:modelValue", v) })
</script>
