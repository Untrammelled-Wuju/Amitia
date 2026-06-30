<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="onboarding-shell">
    <div class="onb-container">
      <header class="onb-header">
        <h1 class="onb-brand">AI-Amitia</h1>
        <p class="onb-sub">本地部署的 AI 虚拟陪伴智能体，守护你的每一次对话</p>
      </header>

      <div class="onb-steps">
        <div
          v-for="(s, idx) in steps"
          :key="idx"
          class="step-dot"
          :class="{ active: idx === current, done: idx < current }"
          @click="idx < current ? current = idx : null"
        >
          <span class="dot-icon">{{ idx < current ? '\u2713' : (idx + 1) }}</span>
          <span class="dot-label">{{ s.label }}</span>
        </div>
      </div>

      <div class="onb-content">
        <WelcomeStep v-if="current === 0" />

        <DeployModeStep
          v-if="current === 1"
          v-model="form.deployMode"
        />

        <AdminSetupStep
          v-if="current === 2"
          v-model:username="form.username"
          v-model:password="form.password"
          v-model:password2="form.password2"
          :hasAdmin="hasAdmin"
        />

        <ModelConfigStep
          v-if="current === 3"
          v-model:apiType="form.apiType"
          v-model:baseUrl="form.baseUrl"
          v-model:apiKey="form.apiKey"
          v-model:modelName="form.modelName"
          :detectingModels="detectingModels"
          :detectedModels="detectedModels"
          :detectError="detectError"
          @detect="detectModels"
          @pickModel="pickModel"
        />

        <CharacterStep
          v-if="current === 4"
          v-model:charName="form.charName"
          v-model:charIdentity="form.charIdentity"
          v-model:charPersonality="form.charPersonality"
        />

        <ProfileStep
          v-if="current === 5"
          :profileList="profileList"
        />

        <WebChatStep
          v-if="current === 6"
          v-model="form.webChatEnabled"
        />

        <WechatStep
          v-if="current === 7"
          v-model="form.wechatEnabled"
          :wxQrLoading="wxQrLoading"
          :wxQrCodeUrl="wxQrCodeUrl"
          :wxQrStep="wxQrStep"
          :wxScanning="wxScanning"
          :wxConnected="wxConnected"
          :wxAccountId="wxAccountId"
          :wxMessageCount="wxMessageCount"
          :wxError="wxError"
          @startWxLogin="startWxLogin"
        />

        <QQStep
          v-if="current === 8"
          v-model="form.qqEnabled"
          :qqConnected="qqConnected"
          :qqConnecting="qqConnecting"
          :qqAccountId="qqAccountId"
          :qqMessageCount="qqMessageCount"
          :qqError="qqError"
          :qqAppId="form.qqAppId"
          :qqToken="form.qqToken"
          @update:qqAppId="form.qqAppId = $event"
          @update:qqToken="form.qqToken = $event"
          @connectQQ="connectQQ"
          @resetQQ="resetQQConnection"
        />

        <PrivacyStep v-if="current === 9" />
      </div>

      <div class="onb-actions">
        <el-button v-if="current > 0" @click="current--">上一步</el-button>
        <el-button v-if="current < steps.length - 1" type="primary" @click="handleNext">下一步</el-button>
        <el-button v-if="current === steps.length - 1" type="primary" @click="handleFinish">完成设置</el-button>
      </div>

      <div v-if="stepError" class="step-error">{{ stepError }}</div>

      <SetupSummaryPanel
        v-if="current === steps.length - 1"
        :deployMode="form.deployMode"
        :webChatEnabled="form.webChatEnabled"
        :profileCount="profileList.filter(p=>p.attributeName).length"
        :wechatEnabled="form.wechatEnabled"
        :qqEnabled="form.qqEnabled"
        :username="form.username"
        :apiType="form.apiType"
        :modelName="form.modelName"
        :charName="form.charName"
        :charPersonality="form.charPersonality"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from "vue"
import WelcomeStep from "./components/WelcomeStep.vue"
import DeployModeStep from "./components/DeployModeStep.vue"
import AdminSetupStep from "./components/AdminSetupStep.vue"
import ModelConfigStep from "./components/ModelConfigStep.vue"
import CharacterStep from "./components/CharacterStep.vue"
import ProfileStep from "./components/ProfileStep.vue"
import WebChatStep from "./components/WebChatStep.vue"
import WechatStep from "./components/WechatStep.vue"
import QQStep from "./components/QQStep.vue"
import PrivacyStep from "./components/PrivacyStep.vue"
import SetupSummaryPanel from "./components/SetupSummaryPanel.vue"
import { useOnboardingWizard, steps } from "./composables/useOnboardingWizard"
import { useApi } from "../../ui-index"

const { get } = useApi()
const {
  current,
  stepError,
  detectingModels,
  detectedModels,
  detectError,
  hasAdmin,
  form,
  profileList,
  wxQrLoading,
  wxQrCodeUrl,
  wxQrStep,
  wxScanning,
  wxConnected,
  wxAccountId,
  wxMessageCount,
  wxError,
  qqConnected,
  qqConnecting,
  qqAccountId,
  qqMessageCount,
  qqError,
  detectModels,
  pickModel,
  startWxLogin,
  startWxPolling,
  stopWxPolling,
  stopQQPoll,
  resetQQConnection,
  connectQQ,
  refreshQQStatus,
  handleNext,
  handleFinish,
  cleanup,
} = useOnboardingWizard()

onMounted(async () => {
  try {
    const res = await get<any>("/api/auth/status")
    hasAdmin.value = !!res?.hasAdmin
  } catch { }
})

onUnmounted(() => {
  cleanup()
})
</script>

<style scoped>
.onboarding-shell {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--ac-color-bg);
  padding: 20px;
}

.onb-container {
  width: 100%;
  max-width: 600px;
}

.onb-header {
  text-align: center;
  margin-bottom: 24px;
}

.onb-brand {
  font-size: 28px;
  font-weight: 700;
  color: var(--ac-color-primary);
  margin: 0 0 4px;
}

.onb-sub {
  font-size: 14px;
  color: var(--ac-color-text-muted);
}

.onb-steps {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-bottom: 28px;
  flex-wrap: wrap;
}

.step-dot {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  opacity: 0.4;
  transition: opacity 0.2s;
}

.step-dot.active,
.step-dot.done {
  opacity: 1;
}

.dot-icon {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--ac-color-bg-secondary);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
}

.step-dot.active .dot-icon {
  background: var(--ac-color-primary);
  color: #fff;
}

.step-dot.done .dot-icon {
  background: var(--ac-color-success);
  color: #fff;
}

.dot-label {
  font-size: 10px;
  color: var(--ac-color-text-muted);
}

.onb-actions {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-top: 24px;
}

.step-error {
  color: var(--ac-color-danger);
  font-size: 13px;
  text-align: center;
  margin-top: 12px;
}

.onb-summary {
  margin-top: 20px;
}
</style>

<style>
.step-panel h2 {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--ac-color-text);
}

.step-desc {
  font-size: 13px;
  color: var(--ac-color-text-muted);
  margin-bottom: 16px;
}

.step-illustration {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px;
  background: var(--ac-color-surface-alt);
  border-radius: 6px;
  font-size: 13px;
  color: var(--ac-color-text-secondary);
}

.step-illustration span:first-child {
  font-weight: 600;
  color: var(--ac-color-primary);
}

.deploy-cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.deploy-card {
  display: flex;
  gap: 12px;
  padding: 14px;
  border: 1.5px solid var(--ac-color-border-light);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.deploy-card:hover {
  border-color: var(--ac-color-primary);
}

.deploy-card.selected {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-bg);
}

.dc-radio {
  display: flex;
  align-items: flex-start;
  padding-top: 2px;
}

.dc-dot {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 2px solid var(--ac-color-border);
  display: flex;
  align-items: center;
  justify-content: center;
}

.dc-dot.on {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary);
}

.dc-body {
  flex: 1;
}

.dc-title {
  font-weight: 600;
  font-size: 14px;
  margin-bottom: 4px;
  color: var(--ac-color-text);
}

.dc-desc {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  margin: 0;
}

.toggle-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 8px;
  background: var(--ac-color-surface-alt);
}

.tc-title {
  font-weight: 600;
  font-size: 14px;
  color: var(--ac-color-text);
}

.tc-desc {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  flex: 1;
}

.profile-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.profile-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.model-detect-wrap {
  width: 100%;
}

.model-detect-row {
  display: flex;
  gap: 8px;
}

.model-input {
  flex: 1;
}

.detect-error {
  color: var(--ac-color-danger);
  font-size: 12px;
  margin-top: 6px;
}

.detect-dropdown {
  margin-top: 8px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 6px;
  max-height: 200px;
  overflow-y: auto;
  background: var(--ac-color-surface);
}

.detect-hint {
  padding: 8px 12px;
  font-size: 12px;
  color: var(--ac-color-text-muted);
  border-bottom: 1px solid var(--ac-color-border-light);
}

.detect-option {
  padding: 8px 12px;
  cursor: pointer;
  font-size: 13px;
  transition: background 0.15s;
}

.detect-option:hover {
  background: var(--ac-color-surface-alt);
}

.detect-option.active {
  background: var(--ac-color-primary-bg);
  color: var(--ac-color-primary);
  font-weight: 600;
}

.privacy-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.pc-card {
  padding: 12px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 8px;
  background: var(--ac-color-surface-alt);
}

.pc-title {
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 4px;
  color: var(--ac-color-text);
}

.pc-desc {
  font-size: 12px;
  color: var(--ac-color-text-muted);
}

.section-card {
  border: 1px solid var(--ac-color-border-light);
}

.card-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header-title {
  font-weight: 600;
  font-size: 14px;
}

.status-main {
  padding: 8px 0;
}

.status-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.status-indicator.ok {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--ac-color-success);
}

.status-label {
  font-size: 14px;
  font-weight: 500;
}

.status-detail-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 8px;
}

.sd-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.sd-label {
  font-size: 11px;
  color: var(--ac-color-text-muted);
}

.sd-value {
  font-size: 13px;
  font-weight: 500;
}

.login-layout {
  display: flex;
  gap: 20px;
  align-items: flex-start;
}

.login-steps {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.login-qr {
  width: 140px;
  height: 140px;
}

.qr-frame {
  width: 100%;
  height: 100%;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.qr-frame img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.qr-empty {
  flex-direction: column;
  gap: 8px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
}

.qr-step-row {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}

.qr-step-num {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  border: 2px solid var(--ac-color-border-light);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
  color: var(--ac-color-text-muted);
  flex-shrink: 0;
}

.qr-step-num.active {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary);
  color: #fff;
}

.qr-step-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.qr-step-title {
  font-size: 13px;
  font-weight: 500;
}

.qr-status {
  font-size: 12px;
  color: var(--ac-color-text-muted);
}

.qr-done {
  font-size: 12px;
  color: var(--ac-color-success);
  display: flex;
  align-items: center;
  gap: 4px;
}

.qr-tip {
  margin-top: 12px;
  font-size: 12px;
  color: var(--ac-color-text-muted);
  text-align: center;
}

@media (max-width: 768px) {
  .privacy-cards {
    grid-template-columns: 1fr;
  }
  .login-layout {
    flex-direction: column;
    align-items: center;
  }
  .login-qr {
    width: 180px;
    height: 180px;
  }
}
</style>
