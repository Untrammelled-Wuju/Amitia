<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog
    v-model="visible"
    title="欢迎使用阿米提亚"
    width="520px"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :show-close="false"
    center
  >
    <div class="consent-body">
      <el-alert type="info" :closable="false" show-icon class="consent-alert">
        <template #title>
          请阅读并确认以下说明
        </template>
        使用本产品前，请了解隐私保护和使用边界。
      </el-alert>

      <div class="consent-sections">
        <div class="cs-item">
          <el-icon color="#409eff"><Lock /></el-icon>
          <div class="csi-text">
            <div class="csi-title">数据由你自己掌控</div>
            <div class="csi-desc">
              本产品由你自行部署，所有数据保存在你自己的设备或服务器上。
              没有任何第三方可以访问你的数据。
            </div>
          </div>
        </div>

        <div class="cs-item">
          <el-icon color="#e6a23c"><WarningFilled /></el-icon>
          <div class="csi-text">
            <div class="csi-title">模型 API 数据提醒</div>
            <div class="csi-desc">
              使用 AI 聊天时，对话内容会发送给你配置的模型服务商（如 OpenAI）。
              请不要发送验证码、密码、银行卡号等敏感信息。
            </div>
          </div>
        </div>

        <div class="cs-item">
          <el-icon color="#67c23a"><ChatDotRound /></el-icon>
          <div class="csi-text">
            <div class="csi-title">AI 是虚拟角色</div>
            <div class="csi-desc">
              AI 陪伴角色由大语言模型驱动，不是真人。
              请理性看待 AI 的回复，不要将情感完全寄托于 AI。
            </div>
          </div>
        </div>

        <div class="cs-item">
          <el-icon color="#909399"><Delete /></el-icon>
          <div class="csi-text">
            <div class="csi-title">你可以随时删除数据</div>
            <div class="csi-desc">
              聊天记录、记忆、导入记录均可在设置中一键清空。
            </div>
          </div>
        </div>
      </div>

      <div class="consent-links">
        <el-link type="primary" @click="goPrivacy">查看完整隐私说明</el-link>
        <el-divider direction="vertical" />
        <el-link type="primary" @click="goBoundary">查看使用边界</el-link>
      </div>
    </div>

    <template #footer>
      <div class="consent-footer">
        <el-checkbox v-model="agreed" label="我已阅读并同意隐私说明和使用边界" size="large" />
        <el-button type="primary" :disabled="!agreed" @click="confirm" size="large">
          开始使用
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { useRouter } from "vue-router"
import { Lock, WarningFilled, ChatDotRound, Delete } from "@element-plus/icons-vue"

const STORAGE_KEY = "ai-companion-privacy-consent"

const router = useRouter()
const visible = ref(false)
const agreed = ref(false)

onMounted(() => {
  const consented = localStorage.getItem(STORAGE_KEY)
  if (!consented) {
    visible.value = true
  }
})

function confirm() {
  localStorage.setItem(STORAGE_KEY, "true")
  visible.value = false
}

function goPrivacy() {
  visible.value = false
  router.push("/privacy")
}

function goBoundary() {
  visible.value = false
  router.push("/usage-boundary")
}
</script>

<style scoped>
.consent-body {
  padding: 4px 0;
}

.consent-alert {
  margin-bottom: 16px;
}

.consent-sections {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 14px;
}

.cs-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--el-fill-color-light);
}

.cs-item > .el-icon {
  font-size: 20px;
  margin-top: 2px;
  flex-shrink: 0;
}

.csi-title {
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 2px;
}

.csi-desc {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
}

.consent-links {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  font-size: 13px;
}

.consent-footer {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}
</style>
