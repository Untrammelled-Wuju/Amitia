<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="privacy-page">
    <div class="page-header">
      <el-button text @click="$router.back()" class="back-btn">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <h2>隐私说明</h2>
    </div>

    <el-card shadow="never" class="content-card">
      <div class="privacy-section">
        <h3>你的数据在哪里</h3>
        <p>
          本产品由<strong>你自己部署</strong>，所有聊天记录、角色设定、记忆数据均保存在
          <strong>你自己的设备或服务器</strong>上。数据库文件位于项目目录下的
          <code>data/app.db</code>。
        </p>
        <p>
          没有任何第三方可以访问你的数据，除非你主动将数据库文件分享给他人。
        </p>
      </div>

      <el-divider />

      <div class="privacy-section">
        <h3>模型 API 的数据处理</h3>
        <p>
          当你使用 AI 聊天功能时，你的对话内容会发送给你配置的
          <strong>模型 API 服务商</strong>（如 OpenAI、DeepSeek、阿里百炼等）。
        </p>
        <p>
          对话数据的隐私保护取决于你选择的模型服务商的隐私政策。建议选择你信赖的服务商，
          并了解其数据使用政策。
        </p>
        <el-alert
          title="建议"
          type="info"
          :closable="false"
          show-icon
          class="info-alert"
        >
          不要向 AI 发送验证码、密码、银行卡号、身份证号等敏感个人信息。
          虽然我们已在本地做了脱敏处理，但对话内容仍会发送到模型服务商。
        </el-alert>
      </div>

      <el-divider />

      <div class="privacy-section">
        <h3>微信相关</h3>
        <ul class="privacy-list">
          <li>
            <strong>不保存微信密码：</strong>
            微信桥接功能仅通过扫码登录，系统不会保存你的微信密码。
          </li>
          <li>
            <strong>不读取全部聊天记录：</strong>
            系统仅处理你与指定联系人的对话，不会扫描或读取你的全部微信聊天记录。
          </li>
          <li>
            <strong>不代替你回复好友：</strong>
            AI 仅供你个人使用，不会代替你向微信好友发送消息。
          </li>
          <li>
            <strong>不做批量群发：</strong>
            本产品不提供群发功能，也不会向你的联系人列表发送任何消息。
          </li>
        </ul>
      </div>

      <el-divider />

      <div class="privacy-section">
        <h3>你的控制权</h3>
        <ul class="privacy-list">
          <li>
            <strong>删除聊天记录：</strong>
            你可以在 Chat 页面或设置中随时删除任何对话记录。删除后数据不可恢复。
          </li>
          <li>
            <strong>删除记忆：</strong>
            你可以在 Memory 页面管理或删除 AI 对你的记忆。
          </li>
          <li>
            <strong>删除导入记录：</strong>
            导入的聊天记录同样可以被删除。
          </li>
          <li>
            <strong>一键清空：</strong>
            在设置页面可以一键清除所有数据（包括聊天记录、记忆、导入记录等）。
          </li>
        </ul>
      </div>

      <el-divider />

      <div class="privacy-section">
        <h3>Cookie 与追踪</h3>
        <p>
          本产品<strong>不使用任何 Cookie 追踪技术</strong>。登录状态通过浏览器本地存储（localStorage）
          中的 JWT 令牌维护。不包含任何第三方追踪脚本或分析工具。
        </p>
      </div>

      <el-divider />

      <div class="privacy-section">
        <h3>免责声明</h3>
        <p>
          本产品是开源软件，由用户自行部署和管理。开发者不对以下情况承担责任：
        </p>
        <ul class="privacy-list">
          <li>因服务器安全配置不当导致的数据泄露</li>
          <li>模型 API 服务商的数据处理行为</li>
          <li>用户主动分享数据库文件导致的信息泄露</li>
        </ul>
        <p class="update-note">
          最后更新：2026 年 5 月
        </p>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ArrowLeft } from "@element-plus/icons-vue"
</script>

<style scoped>
.privacy-page {
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

.privacy-section h3 {
  font-size: 15px;
  font-weight: 600;
  margin: 0 0 10px 0;
  color: var(--el-text-color-primary);
}

.privacy-section p {
  margin: 0 0 8px 0;
  font-size: 14px;
  color: var(--el-text-color-regular);
}

.privacy-section code {
  background: var(--el-fill-color);
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 13px;
}

.privacy-list {
  margin: 4px 0;
  padding-left: 20px;
  font-size: 14px;
  color: var(--el-text-color-regular);
}

.privacy-list li {
  margin-bottom: 6px;
}

.info-alert {
  margin: 10px 0;
}

.update-note {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 12px;
}

@media (max-width: 768px) {
  .privacy-page {
    padding: 12px;
    max-width: 100%;
  }
}
</style>
