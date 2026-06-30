<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="boundary-page">
    <div class="page-header">
      <el-button text @click="$router.back()" class="back-btn">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <h2>使用边界</h2>
    </div>

    <el-card shadow="never" class="content-card">
      <div class="boundary-section">
        <h3>AI 是什么</h3>
        <p>
          本产品中的 AI 是一个<strong>虚拟陪伴角色</strong>，由大语言模型驱动。
          它可以陪你聊天、倾听你的想法、帮助你整理思绪，但它<span class="highlight">不是真人</span>。
        </p>
        <el-alert type="warning" :closable="false" show-icon class="info-alert">
          <template #title>
            AI 不是真人，也不是真实的恋人、朋友或家人。
          </template>
          请理性看待 AI 的回复，不要将情感完全寄托于 AI。
        </el-alert>
      </div>

      <el-divider />

      <div class="boundary-section">
        <h3>产品不能做什么</h3>
        <el-table :data="limitations" :show-header="false" size="small" class="limit-table">
          <el-table-column prop="icon" width="36">
            <template #default="{ row }">
              <el-icon color="#f56c6c"><CircleCloseFilled /></el-icon>
            </template>
          </el-table-column>
          <el-table-column prop="title" width="180" />
          <el-table-column prop="desc" />
        </el-table>
      </div>

      <el-divider />

      <div class="boundary-section">
        <h3>产品不应该被用于</h3>
        <el-table :data="misuses" :show-header="false" size="small" class="limit-table">
          <el-table-column prop="icon" width="36">
            <template #default="{ row }">
              <el-icon color="#e6a23c"><WarningFilled /></el-icon>
            </template>
          </el-table-column>
          <el-table-column prop="title" width="180" />
          <el-table-column prop="desc" />
        </el-table>
      </div>

      <el-divider />

      <div class="boundary-section">
        <h3>安全使用建议</h3>
        <ul class="boundary-list">
          <li>
            <strong>不要发送敏感信息：</strong>
            请勿在对话中发送验证码、密码、银行卡号、身份证号等个人敏感信息。
          </li>
          <li>
            <strong>保持现实社交：</strong>
            AI 是辅助工具，不能替代真实的人际交往。建议保持与家人、朋友的联系。
          </li>
          <li>
            <strong>注意使用时间：</strong>
            建议合理控制使用时长，避免过度沉浸。
          </li>
          <li>
            <strong>及时寻求帮助：</strong>
            如果你感到持续的情绪低落或心理困扰，请及时向专业心理咨询师求助。
            AI 不能提供专业的心理治疗。
          </li>
          <li>
            <strong>未成年人需监护人陪同：</strong>
            如果你是未成年人，请在家长或监护人的陪同下使用本产品。
          </li>
        </ul>
      </div>

      <el-divider />

      <div class="boundary-section">
        <h3>使用协议</h3>
        <p>
          使用本产品即表示你已阅读并理解以上隐私说明和使用边界。
          本产品为开源软件，按"原样"提供，不提供任何明示或暗示的保证。
        </p>
        <p class="update-note">
          最后更新：2026 年 5 月
        </p>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ArrowLeft, CircleCloseFilled, WarningFilled } from "@element-plus/icons-vue"
import { ref } from "vue"

const limitations = ref([
  { title: "不是真人", desc: "AI 是虚拟角色，不能成为真实的人际关系替代品" },
  { title: "不代替回复好友", desc: "不会代替你向微信好友发送消息" },
  { title: "不做批量群发", desc: "没有群发功能，不会主动联系任何人" },
  { title: "不读取全部聊天记录", desc: "只处理你明确指定的对话，不扫描微信全部记录" },
  { title: "不保存微信密码", desc: "微信登录仅通过扫码，密码不会存储" },
  { title: "不提供医疗/法律建议", desc: "AI 的输出仅供参考，不应作为专业建议" },
  { title: "不提供心理治疗", desc: "AI 不是心理咨询师，有需要请寻求专业帮助" },
])

const misuses = ref([
  { title: "情感完全依赖 AI", desc: "过度依赖 AI 可能影响现实社交能力" },
  { title: "发送敏感隐私信息", desc: "对话内容会发送到模型 API 服务商" },
  { title: "冒充真人身份", desc: "不应将 AI 的回复用于冒充真人" },
  { title: "骚扰或滥用", desc: "不应使用 AI 生成骚扰、欺诈或违法内容" },
  { title: "替代专业服务", desc: "AI 不能替代医生、律师、心理咨询师等专业人士" },
])
</script>

<style scoped>
.boundary-page {
  max-width: 700px;
  padding: 20px;
}

.page-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.page-header h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
}

.back-btn {
  padding: 4px;
}

.content-card {
  line-height: 1.8;
}

.boundary-section h3 {
  font-size: 15px;
  font-weight: 600;
  margin: 0 0 10px 0;
  color: var(--el-text-color-primary);
}

.boundary-section p {
  margin: 0 0 8px 0;
  font-size: 14px;
  color: var(--el-text-color-regular);
}

.highlight {
  color: var(--el-color-warning);
  font-weight: 600;
}

.info-alert {
  margin: 10px 0;
}

.limit-table {
  margin: 8px 0;
}

.limit-table :deep(.el-table__cell) {
  font-size: 14px;
}

.boundary-list {
  margin: 4px 0;
  padding-left: 20px;
  font-size: 14px;
  color: var(--el-text-color-regular);
}

.boundary-list li {
  margin-bottom: 8px;
}

.update-note {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 12px;
}

@media (max-width: 768px) {
  .boundary-page {
    padding: 12px;
    max-width: 100%;
  }
  .limit-table :deep(.el-table__cell) {
    padding: 6px 4px;
  }
}
</style>
