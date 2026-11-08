<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="legal-page">
    <el-card shadow="never" class="content-card">
      <section class="legal-section">
        <h3>数据收集与存储</h3>
        <ul>
          <li>Amitia 会保存维持个人空间、对话、角色、记忆、扩展与运行配置所需的数据。</li>
          <li><strong>设备本地数据：</strong>聊天会话、消息、项目、文件、提醒、主动任务、MCP、Skill、工作流和扩展包由对应设备保存。</li>
          <li><strong>统筹中心数据：</strong>角色、记忆、团队、世界书、用户画像和模型配置由当前选择的统筹中心保存。</li>
          <li>聊天、记忆、导入记录及相关业务数据可以通过应用提供的删除、清理、备份或恢复功能管理。</li>
        </ul>
      </section>

      <el-divider />

      <section class="legal-section">
        <h3>模型服务</h3>
        <p>当你配置第三方模型、语音、视觉或其它外部服务时，请求会按照对应配置发送给该服务提供方。外部服务的数据处理规则由你所选择的服务商决定。</p>
        <el-alert type="info" :closable="false" show-icon>
          <template #title>建议不要在模型请求中发送密码、验证码、银行卡号、身份证号等高敏感信息。</template>
        </el-alert>
      </section>

      <el-divider />

      <section class="legal-section">
        <h3>权限与工具调用</h3>
        <ul>
          <li>相机、麦克风、文件、通知等系统权限由操作系统授权，并可在应用的权限/安全设置中检查。</li>
          <li>扩展包、MCP、Agent Skills、工作流及其它工具能力受当前权限、运行时策略和运行模式约束。</li>
          <li>统筹中心不会把所有设备本地能力自动搬到中心；设备能力是否可见由该设备自己的能力开放设置决定。</li>
        </ul>
      </section>

      <el-divider />

      <section class="legal-section">
        <h3>设备与统筹协同</h3>
        <ul>
          <li>绑定设备后，Device Mesh 会使用设备凭据维护统筹中心与该设备之间的受控协同连接。</li>
          <li>你可以在“我的设备”中查看、管理或移除中心登记设备，并可在设备端解除本机中心凭据。</li>
          <li>设备凭据用于统筹中心认证；第三方服务凭据只属于对应 Service Connection。</li>
        </ul>
      </section>

      <el-divider />

      <section class="legal-section">
        <h3>你的控制权</h3>
        <ul>
          <li>可以删除聊天、记忆、导入记录以及其它应用提供删除入口的数据。</li>
          <li>可以使用存储管理、隐私扫描、备份恢复和设备管理功能检查数据范围。</li>
          <li>关闭或移除第三方模型/扩展配置后，对应外部能力将不再按该配置被调用。</li>
        </ul>
        <p class="update-note">最后更新：2026 年 9 月</p>
      </section>
    </el-card>
  </div>
</template>

<script setup lang="ts">
</script>

<style scoped>
.legal-page { padding: 20px; }
.page-header { display: flex; align-items: flex-start; gap: 8px; margin-bottom: 16px; }
.page-header h2 { margin: 0; font-size: 18px; font-weight: 600; }
.page-header p { margin: 4px 0 0; color: var(--ac-color-text-secondary); font-size: 13px; }
.back-btn { padding: 4px; }
.content-card { line-height: 1.75; }
.legal-section h3 { margin: 0 0 10px; font-size: 15px; font-weight: 600; }
.legal-section p, .legal-section li { font-size: 14px; color: var(--el-text-color-regular); }
.legal-section p { margin: 0 0 8px; }
.legal-section ul { margin: 4px 0; padding-left: 20px; }
.legal-section li { margin-bottom: 7px; }
.update-note { margin-top: 14px !important; color: var(--el-text-color-secondary) !important; font-size: 12px !important; }
@media (max-width: 768px) { .legal-page { padding: 12px; } }
</style>
