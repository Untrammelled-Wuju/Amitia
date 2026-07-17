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
      <div class="version-bar" @click="handleCheckUpdate">
        <span v-show="!appStore.sidebarCollapsed" class="version-label">v{{ version }}</span>
        <el-icon v-if="checking" class="version-spinner"><Loading /></el-icon>
      </div>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from "vue"

import { useRoute } from "vue-router"
import { ElMessage } from "element-plus"
import {
  ChatDotRound, Odometer, Connection, UserFilled,
  ChatDotSquare, Setting, Loading, Grid,
} from "@element-plus/icons-vue"
import { useAppStore } from "@/stores/app"

import logoUrl from "../../public/logo.png"
const route = useRoute()

const version = ref("1.0.0")
const checking = ref(false)

const appStore = useAppStore()

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
  if (path.startsWith("/extensions")) {
    return "/extensions"
  }
  return path
})

async function handleCheckUpdate() {
  if (checking.value) return
  checking.value = true
  try {
    if (window.amitiaDesktop) {
      await window.amitiaDesktop.checkUpdate()
    } else {
      ElMessage.info("当前为浏览器环境，无法检查更新")
    }
  } catch (e) {
    ElMessage.error("检查更新失败")
  } finally {
    checking.value = false
  }
}

onMounted(async () => {
  if (window.amitiaDesktop) {
    try {
      version.value = await window.amitiaDesktop.getVersion()
    } catch {
    }
  }
})
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
  background: var(--console-sidebar);
  border-right-color: var(--console-border);
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
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

.version-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  opacity: 0.45;
  transition: opacity 0.2s;
  padding: 6px 0 4px;
  justify-content: center;
}

.version-bar:hover {
  opacity: 0.8;
}

.version-label {
  font-size: 12px;
  color: var(--console-text);
  letter-spacing: 0;
  line-height: 1;
  white-space: nowrap;
  overflow: hidden;
}

.version-spinner {
  font-size: 12px;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
