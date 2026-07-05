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
    <el-menu
      :default-active="activeIndex"
      router
      class="side-menu"
    >
      <el-sub-menu index="/dashboard">
        <template #title>
          <el-icon><Odometer /></el-icon>
          <span>概览</span>
        </template>
        <el-menu-item index="/dashboard/run">运行</el-menu-item>
        <el-menu-item index="/dashboard/data">数据</el-menu-item>
      </el-sub-menu>

      <el-menu-item index="/chat">
        <el-icon><ChatDotRound /></el-icon>
        <span>聊天</span>
      </el-menu-item>

      <div class="menu-divider"></div>

      <el-menu-item index="/wechat">
        <el-icon><Connection /></el-icon>
        <span>微信连接</span>
      </el-menu-item>
      <el-menu-item index="/qq">
        <el-icon><ChatDotSquare /></el-icon>
        <span>QQ 连接</span>
      </el-menu-item>

      <el-sub-menu index="char-memory">
        <template #title>
          <el-icon><UserFilled /></el-icon>
          <span>角色与记忆</span>
        </template>
        <el-menu-item index="/character">角色管理</el-menu-item>
        <el-menu-item index="/reminders">日程提醒</el-menu-item>
        <el-menu-item index="/memory-manager">记忆总览</el-menu-item>
        <el-menu-item index="/episodic">情景记忆</el-menu-item>
        <el-menu-item index="/graph">记忆图谱</el-menu-item>
        <el-menu-item index="/memory-timeline">时间线</el-menu-item>
        <el-menu-item index="/profiles">用户画像</el-menu-item>
        <el-menu-item index="/world-book">世界书</el-menu-item>
        <el-menu-item index="/logs">聊天记录</el-menu-item>
        <el-menu-item index="/import">导入记录</el-menu-item>
      </el-sub-menu>

      <div class="menu-divider"></div>

      <el-menu-item index="/settings">
        <el-icon><Setting /></el-icon>
        <span>设置</span>
      </el-menu-item>
    </el-menu>
  </nav>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useRoute } from "vue-router"
import {
  ChatDotRound, Odometer, Connection, UserFilled,
  ChatDotSquare, Setting,
} from "@element-plus/icons-vue"

const route = useRoute()

const CHAR_PATHS = [
  "/character", "/reminders", "/memory-manager", "/episodic",
  "/graph", "/memory-timeline", "/profiles", "/world-book",
  "/logs", "/import",
]

const activeIndex = computed(() => {
  const path = route.path
  if (CHAR_PATHS.some((p) => path.startsWith(p))) {
    return path
  }
  if (path.startsWith("/dashboard")) {
    return path
  }
  return path
})
</script>

<style scoped>
.side-nav {
  width: var(--ac-sidebar-width);
  height: 100%;
  background: var(--console-sidebar);
  border-right: 1px solid var(--console-border);
  display: flex;
  flex-direction: column;
  padding: 24px 0 18px;
  overflow-y: auto;
  user-select: none;
  flex-shrink: 0;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 24px 24px;
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

.side-menu {
  border-right: none;
  background: transparent;
  flex: 1;
}

.side-menu :deep(.el-menu-item),
.side-menu :deep(.el-sub-menu__title) {
  height: 40px;
  line-height: 40px;
  font-size: var(--ac-font-size-sm);
  margin: 0 12px;
  border-radius: 7px;
}

.side-menu :deep(.el-menu-item:hover),
.side-menu :deep(.el-sub-menu__title:hover) {
  background: var(--nav-hover-bg);
  color: var(--nav-hover-color);
}

.side-menu :deep(.el-menu-item.is-active) {
  background: var(--nav-active-bg);
  color: var(--nav-active-color);
  font-weight: 600;
}

.side-menu :deep(.el-sub-menu .el-menu) {
  background: transparent;
}

.side-menu :deep(.el-sub-menu .el-menu-item) {
  padding-left: 52px !important;
  height: 36px;
  line-height: 36px;
  font-size: var(--ac-font-size-xs);
}

.menu-divider {
  height: 1px;
  background: var(--console-border-soft);
  margin: 8px 28px;
}
</style>
