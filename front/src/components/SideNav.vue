<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <nav class="side-nav">
    <div class="nav-section">
      <router-link to="/chat" class="nav-item" active-class="nav-active">
        <el-icon><ChatDotRound /></el-icon>
        <span>聊天</span>
      </router-link>
      <router-link to="/dashboard" class="nav-item" active-class="nav-active">
        <el-icon><Odometer /></el-icon>
        <span>概览</span>
      </router-link>
    </div>
    <div class="nav-divider"></div>
    <div class="nav-section">
      <router-link to="/wechat" class="nav-item" active-class="nav-active">
        <el-icon><Connection /></el-icon>
        <span>微信连接</span>
      </router-link>
      <router-link to="/qq" class="nav-item" active-class="nav-active">
        <el-icon><ChatDotSquare /></el-icon>
        <span>QQ 连接</span>
      </router-link>
      <router-link to="/model" class="nav-item" active-class="nav-active">
        <el-icon><Cpu /></el-icon>
        <span>模型配置</span>
      </router-link>
      <router-link to="/character" class="nav-item" active-class="nav-active">
        <el-icon><UserFilled /></el-icon>
        <span>角色管理</span>
      </router-link>
      <router-link to="/reminders" class="nav-item" active-class="nav-active">
        <el-icon><Bell /></el-icon>
        <span>日程提醒</span>
      </router-link>
    </div>
    <div class="nav-divider"></div>
    <div class="nav-section">
      <router-link to="/logs" class="nav-item" active-class="nav-active">
        <el-icon><ChatLineSquare /></el-icon>
        <span>聊天记录</span>
      </router-link>
      <router-link to="/import" class="nav-item" active-class="nav-active">
        <el-icon><Upload /></el-icon>
        <span>导入记录</span>
      </router-link>
    </div>
    <div class="nav-divider"></div>
    <div class="nav-section">
      <router-link to="/memory-manager" class="nav-item" active-class="nav-active">
        <el-icon><Collection /></el-icon>
        <span>记忆总览</span>
      </router-link>
      <router-link to="/profiles" class="nav-item" active-class="nav-active">
        <el-icon><User /></el-icon>
        <span>用户画像</span>
      </router-link>
      <router-link to="/episodic" class="nav-item" active-class="nav-active">
        <el-icon><Timer /></el-icon>
        <span>情景记忆</span>
      </router-link>
      <router-link to="/world-book" class="nav-item" active-class="nav-active">
        <el-icon><Notebook /></el-icon>
        <span>世界书</span>
      </router-link>
      <router-link to="/graph" class="nav-item" active-class="nav-active">
        <el-icon><Share /></el-icon>
        <span>记忆图谱</span>
      </router-link>
      <router-link to="/memory-timeline" class="nav-item" active-class="nav-active">
        <el-icon><Histogram /></el-icon>
        <span>时间线</span>
      </router-link>
    </div>
    <div class="nav-divider"></div>
    <div class="nav-section">
      <router-link to="/safety" class="nav-item" active-class="nav-active">
        <el-icon><Lock /></el-icon>
        <span>安全设置</span>
      </router-link>
      <router-link to="/long-running" class="nav-item" active-class="nav-active">
        <el-icon><Clock /></el-icon>
        <span>长期运行</span>
      </router-link>
      <router-link to="/maintenance" class="nav-item" active-class="nav-active">
        <el-icon><Monitor /></el-icon>
        <span>维护诊断</span>
      </router-link>
      <router-link to="/settings" class="nav-item" active-class="nav-active">
        <el-icon><Setting /></el-icon>
        <span>设置</span>
      </router-link>
    </div>
  </nav>
</template>

<script setup lang="ts">
import {
  ChatDotRound, Odometer, Connection, Cpu, UserFilled,
  Bell, ChatDotSquare, ChatLineSquare, Upload, Lock, Clock, Setting, Monitor,
  Collection, User, Timer, Notebook, Share, Histogram,
} from "@element-plus/icons-vue"
</script>

<style scoped>
.side-nav {
  width: var(--ac-sidebar-width);
  height: 100%;
  background: var(--ac-color-surface);
  border-right: 1px solid var(--ac-color-border-light);
  display: flex;
  flex-direction: column;
  padding: 10px 0;
  overflow-y: auto;
  user-select: none;
  flex-shrink: 0;
}
.nav-section { display: flex; flex-direction: column; gap: 2px; padding: 0 8px; }
.nav-divider { height: 1px; background: var(--ac-color-border-light); margin: 8px 16px; }
.nav-item {
  display: flex; align-items: center; gap: 10px; padding: 9px 14px;
  border-radius: var(--ac-radius-sm); color: var(--ac-color-text-secondary);
  text-decoration: none; font-size: var(--ac-font-size-sm); transition: all var(--ac-transition-fast);
}
.nav-item:hover { background: var(--ac-color-surface-hover); color: var(--ac-color-text); }
.nav-active { background: var(--ac-color-primary-bg); color: var(--ac-color-primary); font-weight: 500; }
.nav-active:hover { background: var(--ac-color-primary-bg); color: var(--ac-color-primary); }
</style>