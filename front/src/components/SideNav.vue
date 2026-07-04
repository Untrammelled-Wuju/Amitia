<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <nav class="side-nav">
    <div class="brand">
      <div class="brand-mark">A</div>
      <div class="brand-name">Amitia</div>
    </div>
    <div class="nav-section">
      <router-link to="/dashboard" class="nav-item" active-class="nav-active">
        <el-icon><Odometer /></el-icon>
        <span>概览</span>
      </router-link>
      <router-link to="/chat" class="nav-item" active-class="nav-active">
        <el-icon><ChatDotRound /></el-icon>
        <span>聊天</span>
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
  background: var(--console-sidebar);
  border-right: 1px solid var(--console-border);
  display: flex;
  flex-direction: column;
  padding: 24px 14px 18px;
  overflow-y: auto;
  user-select: none;
  flex-shrink: 0;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 16px 24px;
}

.brand-mark {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  font-weight: 800;
  font-size: 22px;
  border-radius: 10px;
  background: linear-gradient(135deg, #4f7cff 12%, #7c3aed 48%, #28d8b8 100%);
  box-shadow: 0 12px 24px rgba(79, 124, 255, 0.22);
}

.brand-name {
  color: var(--console-text);
  font-size: 24px;
  font-weight: 700;
  letter-spacing: 0;
}

.nav-section { display: flex; flex-direction: column; gap: 2px; }
.nav-divider { height: 1px; background: var(--console-border-soft); margin: 8px 16px; }
.nav-item {
  display: flex; align-items: center; gap: 10px; padding: 9px 14px;
  border-radius: 7px; color: var(--console-text-secondary);
  text-decoration: none; font-size: var(--ac-font-size-base); transition: all var(--ac-transition-fast);
}
.nav-item:hover { background: var(--console-card-soft); color: #4f8df7; }
.nav-active { background: rgba(79, 141, 247, 0.12); color: #2f6fff; font-weight: 600; }
.nav-active:hover { background: rgba(79, 141, 247, 0.12); color: #2f6fff; }

</style>
