<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="onb-summary">
    <el-descriptions title="设置预览" :column="2" border size="small">
      <el-descriptions-item label="部署方式">{{ deployMode === 'desktop' ? '桌面本地' : '私有云' }}</el-descriptions-item>
      <el-descriptions-item label="Web 聊天">{{ webChatEnabled ? '已开启' : '未开启' }}</el-descriptions-item>
      <el-descriptions-item label="用户画像">{{ profileCount || '未填写' }}</el-descriptions-item>
      <el-descriptions-item label="微信接入">{{ wechatEnabled ? '已开启' : '未开启' }}</el-descriptions-item>
      <el-descriptions-item label="QQ 接入">{{ qqEnabled ? '已开启' : '未开启' }}</el-descriptions-item>
      <el-descriptions-item label="管理员">{{ username || '-' }}</el-descriptions-item>
      <el-descriptions-item label="API 类型">{{ apiType || '-' }}</el-descriptions-item>
      <el-descriptions-item label="模型">{{ modelName || '-' }}</el-descriptions-item>
      <el-descriptions-item label="角色名称">{{ charName || '-' }}</el-descriptions-item>
      <el-descriptions-item label="角色性格">{{ charPersonality || '-' }}</el-descriptions-item>
    </el-descriptions>
  </div>
</template>
<script setup lang="ts">
defineProps<{
  deployMode: string; webChatEnabled: boolean; profileCount: number
  wechatEnabled: boolean; qqEnabled: boolean; username: string
  apiType: string; modelName: string; charName: string; charPersonality: string
}>()
</script>
