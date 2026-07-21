<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <nav class="side-nav" :class="{ 'is-collapsed': appStore.sidebarCollapsed }">
    <div class="brand">
      <img class="brand-mark" :src="logoUrl" alt="Amitia" />
      <div v-show="!appStore.sidebarCollapsed" class="brand-name">Amitia</div>
    </div>
    <el-menu
      :default-active="activeIndex"
      :collapse="appStore.sidebarCollapsed"
      unique-opened
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
        <el-menu-item index="/emotes">表情包管理</el-menu-item>
        <el-menu-item index="/episodic">情景记忆</el-menu-item>
        <el-menu-item index="/graph">记忆图谱</el-menu-item>
        <el-menu-item index="/memory-timeline">时间线</el-menu-item>
        <el-menu-item index="/profiles">用户画像</el-menu-item>
        <el-menu-item index="/world-book">世界书</el-menu-item>
        <el-menu-item index="/logs">聊天记录</el-menu-item>
        <el-menu-item index="/import">导入记录</el-menu-item>
      </el-sub-menu>

      <el-menu-item index="/extensions">
        <el-icon><Grid /></el-icon>
        <span>扩展中心</span>
      </el-menu-item>

      <div class="menu-divider"></div>

      <el-menu-item index="/settings">
        <el-icon><Setting /></el-icon>
        <span>设置</span>
      </el-menu-item>
    </el-menu>

    <div class="side-nav-bottom">
      <button class="user-profile" type="button" :title="username || '管理员'" @click="openUserProfile">
        <span class="user-avatar">
          <img v-if="avatar" :src="avatar" alt="用户头像" />
          <el-icon v-else><UserFilled /></el-icon>
        </span>
        <span v-show="!appStore.sidebarCollapsed" class="user-copy">
          <strong>{{ username || "管理员" }}</strong>
          <span>当前用户</span>
        </span>
      </button>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from "vue"

import { useRoute, useRouter } from "vue-router"
import {
  ChatDotRound, Odometer, Connection, UserFilled,
  ChatDotSquare, Setting, Grid,
} from "@element-plus/icons-vue"
import { useAppStore } from "@/stores/app"

import logoUrl from "../../public/logo.png"
const route = useRoute()
const router = useRouter()

defineProps<{
  username?: string
  avatar?: string
}>()

const appStore = useAppStore()

const CHAR_PATHS = [
  "/character", "/reminders", "/memory-manager", "/emotes", "/episodic",
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
  if (path.startsWith("/extensions")) {
    return "/extensions"
  }
  return path
})

function openUserProfile() {
  router.push("/user-settings")
}
</script>

<style scoped>
.side-nav {
  width: var(--ac-sidebar-width);
  height: 100%;
  background: var(--tp-glass-bg-strong);
  border-right: 1px solid var(--tp-glass-border);
  backdrop-filter: blur(var(--tp-glass-blur)) saturate(var(--tp-glass-saturate));
  -webkit-backdrop-filter: blur(var(--tp-glass-blur)) saturate(var(--tp-glass-saturate));
  box-shadow: none;
  display: flex;
  flex-direction: column;
  padding: 24px 0 0;
  user-select: none;
  flex-shrink: 0;
  transition: width 0.3s ease;
}

:global(html[data-theme="dark"]) .side-nav {
  background: var(--tp-glass-bg-strong);
  border-right-color: var(--tp-glass-border);
  backdrop-filter: blur(var(--tp-glass-blur)) saturate(var(--tp-glass-saturate));
  -webkit-backdrop-filter: blur(var(--tp-glass-blur)) saturate(var(--tp-glass-saturate));
}

.side-nav.is-collapsed {
  width: 72px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 14px 24px;
  flex-shrink: 0;
}

.side-nav.is-collapsed .brand {
  justify-content: center;
}

.brand-mark {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  object-fit: contain;
  flex-shrink: 0;
}

.brand-name {
  color: var(--console-text);
  font-size: 24px;
  font-weight: 700;
  letter-spacing: 0;
  white-space: nowrap;
  overflow: hidden;
}

.side-menu {
  border-right: none;
  background: transparent;
  flex: 1;
  width: 100%;
  overflow-y: auto;
  overflow-x: hidden;
}

.side-menu :deep(.el-menu-item),
.side-menu :deep(.el-sub-menu__title) {
  height: 40px;
  line-height: 40px;
  font-size: calc(var(--ac-font-size-base) - 1px);
  margin: 0 12px;
  padding: 0 20px 0 12px !important;
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

.side-menu :deep(.el-sub-menu.is-active > .el-sub-menu__title) {
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
  font-size: var(--ac-font-size-sm);
}

.menu-divider {
  height: 1px;
  background: var(--console-border-soft);
  margin: 8px 28px;
}

.side-nav.is-collapsed .menu-divider {
  margin: 8px 14px;
}

.side-nav-bottom {
  flex-shrink: 0;
  border-top: 1px solid var(--console-border-soft);
  margin: 0 12px;
  padding: 8px 0;
}

.side-nav.is-collapsed .side-nav-bottom {
  margin: 0 8px;
}

.user-profile {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px;
  border: 1px solid var(--console-border);
  border-radius: 14px;
  background: var(--tp-profile-bg);
  color: var(--console-text);
  cursor: pointer;
  text-align: left;
}

.user-profile:focus-visible {
  outline: 2px solid var(--tp-primary);
  outline-offset: 2px;
}

.user-avatar {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  flex: 0 0 auto;
  border-radius: 11px;
  background: var(--tp-primary);
  color: var(--tp-text-on-primary);
  font-size: 13px;
  font-weight: 800;
  overflow: hidden;
}

.user-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.user-copy {
  min-width: 0;
}

.user-copy strong,
.user-copy span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-copy strong {
  max-width: 126px;
  font-size: 13px;
  font-weight: 650;
}

.user-copy span {
  margin-top: 2px;
  color: var(--console-text-muted);
  font-size: 11px;
}

.side-nav.is-collapsed .user-profile {
  justify-content: center;
  padding: 10px;
}
</style>
