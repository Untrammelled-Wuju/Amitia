<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="setup-wizard">
    <div class="sw-container">
      <header class="sw-header">
        <h1 class="sw-title">AI-Amitia System Setup</h1>
        <p class="sw-subtitle">Complete the following steps to configure your AI-Amitia companion.</p>
      </header>

      <div class="sw-steps">
        <div
          v-for="(s, i) in wizardSteps"
          :key="s.key"
          class="sw-step-dot"
          :class="{ active: i === currentStepIdx, done: s.done, current: i === currentStepIdx }"
        >
          <div class="sw-dot">
            <span v-if="s.done">&#10003;</span>
            <span v-else>{{ i + 1 }}</span>
          </div>
          <span class="sw-dot-label">{{ s.label }}</span>
        </div>
      </div>

      <div class="sw-content" :key="currentStepIdx">
        <template v-if="loading">
          <div class="sw-loading"><p>Checking...</p></div>
        </template>

        <template v-else-if="errorMsg">
          <div class="sw-error">
            <p class="sw-error-title">Error</p>
            <p class="sw-error-msg">{{ errorMsg }}</p>
            <p v-if="errorSuggestion" class="sw-error-suggestion">{{ errorSuggestion }}</p>
            <div class="sw-error-actions">
              <button class="sw-btn sw-btn-secondary" @click="handleRetry">Retry</button>
              <button v-if="currentStep.key !== 'finish'" class="sw-btn sw-btn-ghost" @click="handleSkip">Skip</button>
            </div>
          </div>
        </template>

        <template v-else>
          <div class="sw-step-body">
            <h2 class="sw-step-title">{{ currentStep.title }}</h2>
            <p class="sw-step-desc">{{ currentStep.description }}</p>

            <div class="sw-step-form">
              <DeployModeStep
                v-if="currentStep.key === 'detect-mode'"
                v-model="localData.deployMode"
              />
              <AdminPasswordStep
                v-if="currentStep.key === 'admin-password'"
                v-model:username="localData.username"
                v-model:password="localData.password"
                v-model:confirmPassword="localData.confirmPassword"
                v-model:skipAuth="skipAuth"
                :deployMode="deployMode"
              />
              <ModelConfigStep
                v-if="currentStep.key === 'model-config'"
                v-model:apiType="localData.apiType"
                v-model:baseUrl="localData.baseUrl"
                v-model:apiKey="localData.apiKey"
                v-model:modelName="localData.modelName"
              />
              <ModelTestStep
                v-if="currentStep.key === 'model-test'"
                :testResult="testResult"
              />
              <CharacterSelectStep
                v-if="currentStep.key === 'character-select'"
                v-model="localData.characterId"
                :characters="characters"
              />
              <WechatOptionStep
                v-if="currentStep.key === 'wechat-option'"
                v-model="localData.enableWechat"
              />
              <CloudDeployInfoStep
                v-if="currentStep.key === 'cloud-deploy-info'"
                :deployMode="deployMode"
              />
              <PrivacyBoundaryStep
                v-if="currentStep.key === 'privacy-boundary'"
                v-model:confirmed="localData.privacyConfirmed"
              />
              <FinishStep v-if="currentStep.key === 'finish'" />
            </div>
          </div>
        </template>
      </div>

      <div class="sw-actions">
        <button
          v-if="currentStepIdx > 0 && currentStep.key !== 'finish'"
          class="sw-btn sw-btn-secondary"
          :disabled="submitting"
          @click="goBack"
        >
          Back
        </button>
        <button
          v-if="currentStep.key !== 'finish'"
          class="sw-btn sw-btn-primary"
          :disabled="submitting || !canProceed"
          @click="handleNext"
        >
          {{ submitting ? 'Processing...' : currentStep.buttonLabel || 'Next' }}
        </button>
        <button
          v-if="currentStep.key === 'finish'"
          class="sw-btn sw-btn-primary"
          :disabled="submitting"
          @click="handleFinish"
        >
          {{ submitting ? 'Finishing...' : 'Enter Dashboard' }}
        </button>
      </div>

      <div class="sw-progress">
        <div class="sw-progress-bar">
          <div class="sw-progress-fill" :style="{ width: progressPercent + '%' }"></div>
        </div>
        <span class="sw-progress-text">{{ completedSteps }} / {{ totalSteps }} steps</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from "vue"
import DeployModeStep from "./components/DeployModeStep.vue"
import AdminPasswordStep from "./components/AdminPasswordStep.vue"
import ModelConfigStep from "./components/ModelConfigStep.vue"
import ModelTestStep from "./components/ModelTestStep.vue"
import CharacterSelectStep from "./components/CharacterSelectStep.vue"
import WechatOptionStep from "./components/WechatOptionStep.vue"
import CloudDeployInfoStep from "./components/CloudDeployInfoStep.vue"
import PrivacyBoundaryStep from "./components/PrivacyBoundaryStep.vue"
import FinishStep from "./components/FinishStep.vue"
import { useSetupWizard } from "./composables/useSetupWizard"

const {
  wizardSteps,
  currentStepIdx,
  completedSteps,
  totalSteps,
  loading,
  submitting,
  errorMsg,
  errorSuggestion,
  testResult,
  skipAuth,
  deployMode,
  characters,
  localData,
  currentStep,
  progressPercent,
  canProceed,
  goBack,
  skipStep,
  retryStep,
  handleNext,
  handleFinish,
  init,
  runCurrentAutoStep: runAutoStep,
  fetchCharacters,
} = useSetupWizard()

function handleSkip() {
  skipStep()
  runAutoStep()
}

function handleRetry() {
  retryStep()
  handleNext()
}

onMounted(async () => {
  await init()
})
</script>

<style scoped>
.setup-wizard {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--ac-color-bg, #f5f5f5);
  padding: 20px;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
}

.sw-container {
  width: 100%;
  max-width: 560px;
  background: var(--ac-color-surface, #fff);
  border: 1px solid var(--ac-color-border-light, #e5e5e5);
  border-radius: 8px;
  padding: 32px 28px 24px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.06);
}

.sw-header {
  text-align: center;
  margin-bottom: 24px;
}

.sw-title {
  font-size: 22px;
  font-weight: 600;
  color: var(--ac-color-text, #1a1a1a);
  margin: 0 0 6px;
}

.sw-subtitle {
  font-size: 13px;
  color: var(--ac-color-text-muted, #888);
  margin: 0;
}

.sw-steps {
  display: flex;
  justify-content: center;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 28px;
  flex-wrap: wrap;
  padding: 0 8px;
}

.sw-step-dot {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  opacity: 0.35;
  transition: opacity 0.2s;
  cursor: default;
  max-width: 60px;
}

.sw-step-dot.active,
.sw-step-dot.done {
  opacity: 1;
}

.sw-dot {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  border: 2px solid var(--ac-color-border, #ccc);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
  color: var(--ac-color-text-muted, #999);
  background: var(--ac-color-surface, #fff);
  transition: all 0.2s;
}

.sw-step-dot.active .sw-dot {
  border-color: var(--ac-color-primary, #409eff);
  background: var(--ac-color-primary, #409eff);
  color: #fff;
}

.sw-step-dot.done .sw-dot {
  border-color: var(--ac-color-success, #67c23a);
  background: var(--ac-color-success, #67c23a);
  color: #fff;
}

.sw-dot-label {
  font-size: 10px;
  color: var(--ac-color-text-muted, #999);
  text-align: center;
  white-space: nowrap;
}

.sw-content {
  min-height: 160px;
}

.sw-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 0;
  color: var(--ac-color-text-muted, #888);
}

.sw-error {
  background: var(--ac-color-surface-alt, #fff5f5);
  border: 1px solid var(--ac-color-danger, #f56c6c);
  border-radius: 6px;
  padding: 16px;
  text-align: center;
}

.sw-error-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--ac-color-danger, #f56c6c);
  margin: 0 0 8px;
}

.sw-error-msg {
  font-size: 13px;
  color: var(--ac-color-text, #333);
  margin: 0 0 6px;
}

.sw-error-suggestion {
  font-size: 12px;
  color: var(--ac-color-text-muted, #888);
  margin: 0 0 12px;
}

.sw-error-actions {
  display: flex;
  gap: 8px;
  justify-content: center;
}

.sw-step-body {
  min-height: 120px;
}

.sw-step-title {
  font-size: 17px;
  font-weight: 600;
  margin: 0 0 4px;
  color: var(--ac-color-text, #1a1a1a);
}

.sw-step-desc {
  font-size: 13px;
  color: var(--ac-color-text-muted, #888);
  margin: 0 0 16px;
}

.sw-step-form {
  margin-top: 8px;
}

.sw-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--ac-color-border-light, #eee);
}

.sw-btn {
  padding: 8px 20px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.2s;
}

.sw-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.sw-btn-primary {
  background: var(--ac-color-primary, #409eff);
  color: #fff;
  border-color: var(--ac-color-primary, #409eff);
}

.sw-btn-primary:hover:not(:disabled) {
  background: var(--ac-color-primary-hover, #66b1ff);
}

.sw-btn-secondary {
  background: var(--ac-color-surface, #fff);
  color: var(--ac-color-text, #333);
  border-color: var(--ac-color-border, #ddd);
}

.sw-btn-secondary:hover:not(:disabled) {
  background: var(--ac-color-surface-alt, #f5f5f5);
}

.sw-btn-ghost {
  background: transparent;
  color: var(--ac-color-text-muted, #888);
  border-color: transparent;
}

.sw-btn-ghost:hover:not(:disabled) {
  color: var(--ac-color-text, #333);
}

.sw-progress {
  margin-top: 20px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.sw-progress-bar {
  flex: 1;
  height: 4px;
  background: var(--ac-color-border-light, #eee);
  border-radius: 2px;
  overflow: hidden;
}

.sw-progress-fill {
  height: 100%;
  background: var(--ac-color-primary, #409eff);
  border-radius: 2px;
  transition: width 0.3s;
}

.sw-progress-text {
  font-size: 12px;
  color: var(--ac-color-text-muted, #888);
  white-space: nowrap;
}
</style>

<style>
.sw-options {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sw-option-card {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--ac-color-border-light, #e5e5e5);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.sw-option-card:hover {
  border-color: var(--ac-color-primary, #409eff);
}

.sw-option-card.selected {
  border-color: var(--ac-color-primary, #409eff);
  background: var(--ac-color-primary-bg, #ecf5ff);
}

.sw-option-card input[type="radio"] {
  margin-top: 2px;
}

.sw-option-body strong {
  display: block;
  font-size: 14px;
  margin-bottom: 4px;
  color: var(--ac-color-text, #1a1a1a);
}

.sw-option-body p {
  font-size: 12px;
  color: var(--ac-color-text-muted, #888);
  margin: 0;
}

.sw-char-card {
  align-items: center;
}

.sw-char-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.sw-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.sw-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.sw-field label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ac-color-text-secondary, #666);
}

.sw-field input,
.sw-field select {
  padding: 8px 12px;
  border: 1px solid var(--ac-color-border-light, #ddd);
  border-radius: 6px;
  font-size: 13px;
  font-family: inherit;
  background: var(--ac-color-surface, #fff);
  color: var(--ac-color-text, #1a1a1a);
  transition: border-color 0.2s;
}

.sw-field input:focus,
.sw-field select:focus {
  outline: none;
  border-color: var(--ac-color-primary, #409eff);
}

.sw-notice {
  padding: 10px 12px;
  background: var(--ac-color-surface-alt, #f0f9ff);
  border-left: 3px solid var(--ac-color-primary, #409eff);
  border-radius: 4px;
  font-size: 12px;
  color: var(--ac-color-text-secondary, #666);
  line-height: 1.5;
}

.sw-notice strong {
  color: var(--ac-color-text, #333);
}

.sw-notice-warn {
  background: var(--ac-color-surface-alt, #fff7e6);
  border-left-color: var(--ac-color-warning, #e6a23c);
}

.sw-skip-option {
  padding: 12px 0;
  border-top: 1px solid var(--ac-color-border-light, #eee);
}

.sw-skip-option label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--ac-color-text-muted, #888);
  cursor: pointer;
}

.sw-test-result {
  padding: 12px;
  border-radius: 6px;
  font-size: 13px;
}

.sw-test-result.success {
  background: var(--ac-color-surface-alt, #f0f9eb);
  border: 1px solid var(--ac-color-success, #67c23a);
  color: var(--ac-color-success, #67c23a);
}

.sw-test-result.failed {
  background: var(--ac-color-surface-alt, #fef0f0);
  border: 1px solid var(--ac-color-danger, #f56c6c);
  color: var(--ac-color-danger, #f56c6c);
}

.sw-test-detail {
  font-size: 12px;
  margin-top: 4px;
  opacity: 0.8;
}

.sw-info {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.sw-info-card {
  padding: 14px;
  background: var(--ac-color-surface-alt, #f8f9fa);
  border-radius: 6px;
  border: 1px solid var(--ac-color-border-light, #eee);
}

.sw-info-card h3 {
  font-size: 14px;
  font-weight: 600;
  margin: 0 0 8px;
  color: var(--ac-color-text, #1a1a1a);
}

.sw-info-card ul {
  margin: 0;
  padding-left: 18px;
}

.sw-info-card li {
  font-size: 12px;
  color: var(--ac-color-text-secondary, #555);
  line-height: 1.8;
}

.sw-confirm {
  display: flex;
  align-items: center;
}

.sw-confirm label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--ac-color-text, #333);
  cursor: pointer;
}

.sw-hint {
  font-size: 12px;
  color: var(--ac-color-text-muted, #888);
  margin: 0 0 12px;
}

.sw-finish {
  text-align: center;
  padding: 32px 0;
}

.sw-finish-icon {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background: var(--ac-color-success, #67c23a);
  color: #fff;
  font-size: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 16px;
}

.sw-finish h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 8px;
  color: var(--ac-color-text, #1a1a1a);
}

.sw-finish p {
  font-size: 13px;
  color: var(--ac-color-text-muted, #888);
  margin: 0;
}
</style>
