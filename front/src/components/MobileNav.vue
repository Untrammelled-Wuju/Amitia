<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
﻿<template>
  <nav class="mobile-nav">
    <router-link to="/chat" class="tab-item" active-class="tab-active">
      <el-icon><ChatDotRound /></el-icon>
      <span>聊天</span>
    </router-link>
    <router-link to="/character" class="tab-item" active-class="tab-active">
      <el-icon><UserFilled /></el-icon>
      <span>角色</span>
    </router-link>
    <router-link to="/memory-manager" class="tab-item" active-class="tab-active">
      <el-icon><Collection /></el-icon>
      <span>记忆</span>
    </router-link>
    <router-link to="/logs" class="tab-item" active-class="tab-active">
      <el-icon><ChatLineSquare /></el-icon>
      <span>记录</span>
    </router-link>
    <router-link to="/settings" class="tab-item" active-class="tab-active">
      <el-icon><Setting /></el-icon>
      <span>设置</span>
    </router-link>
  </nav>
</template>

<script setup lang="ts">
import { ChatDotRound, UserFilled, Collection, ChatLineSquare, Setting } from "@element-plus/icons-vue"
</script>

<style scoped>
.mobile-nav {
  display: flex;
  align-items: center;
  justify-content: space-around;
  height: var(--ac-mobile-nav-height);
  background: var(--ac-color-surface);
  border-top: 1px solid var(--ac-color-border-light);
  flex-shrink: 0;
  user-select: none;
  -webkit-user-select: none;
  padding-bottom: var(--ac-safe-area-bottom);
  padding-bottom: max(0px, env(safe-area-inset-bottom, 0px));
}

.tab-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  padding: 4px 10px;
  border-radius: var(--ac-radius-sm);
  color: var(--ac-color-text-muted);
  text-decoration: none;
  font-size: 10px;
  transition: color var(--ac-transition-fast);
  min-width: 52px;
  -webkit-tap-highlight-color: transparent;
}

.tab-item .el-icon {
  font-size: 22px;
}

.tab-active {
  color: var(--ac-color-primary);
  font-weight: 500;
}
</style>
