<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
﻿<template>
  <nav class="mobile-nav">
    <router-link
      v-for="item in mobileNavItems"
      :key="item.key"
      :to="item.to"
      class="tab-item"
      :class="{ 'tab-active': isActive(item) }"
      :aria-current="isActive(item) ? 'page' : undefined"
    >
      <el-icon><component :is="item.icon" /></el-icon>
      <span>{{ mobileLabelMap[item.key] || item.label }}</span>
    </router-link>
  </nav>
</template>

<script setup lang="ts">
import { useRoute } from "vue-router"
import { isNavItemActive, mobileNavItems, type AppNavItem } from "@/navigation/app-nav"

const route = useRoute()

const mobileLabelMap: Record<string, string> = {
  character: "角色",
  memoryManager: "记忆",
  logs: "记录",
}

function isActive(item: AppNavItem) {
  return isNavItemActive(route.path, item)
}
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
