<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <nav class="side-nav">
    <div class="nav-brand">
      <div class="brand-mark">A</div>
      <div class="brand-copy">
        <strong>Amitia</strong>
        <span>Desktop</span>
      </div>
    </div>

    <div class="nav-profile">
      <div class="profile-avatar">{{ userAvatarText }}</div>
      <div class="profile-name">{{ displayUsername }}</div>
      <div class="profile-subtitle">{{ displayRole }}</div>
      <div class="profile-character">{{ displayCharacter }}</div>
    </div>

    <div class="nav-scroll">
      <div v-for="group in desktopNavGroups" :key="group.key" class="nav-section">
        <router-link
          v-for="item in group.items"
          :key="item.to"
          :to="item.to"
          class="nav-item"
          :class="{ 'nav-active': isActive(item) }"
          :aria-current="isActive(item) ? 'page' : undefined"
        >
          <el-icon>
            <component :is="item.icon" />
          </el-icon>
          <span>{{ item.label }}</span>
        </router-link>
      </div>
    </div>

    <div class="nav-version">
      <span>U-Ai</span>
      <span>local</span>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed, inject, type Ref } from "vue"
import { useRoute } from "vue-router"
import { desktopNavGroups, isNavItemActive, type AppNavItem } from "@/navigation/app-nav"

const route = useRoute()
const currentCharName = inject<Ref<string>>("currentCharName")
const authUsername = inject<Ref<string>>("authUsername")

const displayUsername = computed(() => authUsername?.value || "本地用户")
const displayRole = computed(() => authUsername?.value ? "已登录账号" : "离线模式")
const displayCharacter = computed(() => currentCharName?.value || "未选择角色")
const userAvatarText = computed(() => displayUsername.value.slice(0, 1).toUpperCase())

function isActive(item: AppNavItem) {
  return isNavItemActive(route.path, item)
}
</script>

<style scoped>
.side-nav {
  width: var(--ac-sidebar-width);
  height: 100%;
  background: var(--ac-color-bg-secondary);
  display: flex;
  flex-direction: column;
  padding: 16px 0;
  overflow: hidden;
  user-select: none;
  flex-shrink: 0;
}

.nav-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  height: 34px;
  flex-shrink: 0;
}

.brand-mark {
  width: 24px;
  height: 24px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--ac-color-primary);
  color: #fff;
  font-size: 14px;
  font-weight: 700;
}

.brand-copy {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.brand-copy strong {
  color: var(--ac-color-text);
  font-size: 14px;
  line-height: 17px;
  font-weight: 700;
}

.brand-copy span {
  color: var(--ac-color-text-muted);
  font-size: 11px;
  line-height: 14px;
}

.nav-profile {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 38px 18px 30px;
  flex-shrink: 0;
}

.profile-avatar {
  width: 50px;
  height: 50px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--ac-color-surface);
  color: var(--ac-color-primary);
  border: 1px solid var(--ac-color-primary-border);
  box-shadow: var(--ac-shadow-sm);
  font-size: 20px;
  font-weight: 700;
}

.profile-name {
  width: 100%;
  margin-top: 12px;
  color: var(--ac-color-text);
  font-size: 14px;
  line-height: 20px;
  font-weight: 600;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.profile-subtitle {
  margin-top: 3px;
  color: var(--ac-color-text-muted);
  font-size: 11px;
  line-height: 16px;
}

.profile-character {
  width: 100%;
  margin-top: 8px;
  padding: 8px 10px;
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-surface);
  color: var(--ac-color-text-secondary);
  font-size: 12px;
  line-height: 18px;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  box-shadow: var(--ac-shadow-sm);
}

.nav-scroll {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding-bottom: 8px;
}

.nav-section {
  display: flex;
  flex-direction: column;
  padding: 0;
}

.nav-section + .nav-section {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--ac-color-border-light);
}

.nav-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 11px;
  height: 50px;
  padding: 0 28px 0 42px;
  color: var(--ac-color-text-secondary);
  text-decoration: none;
  font-size: var(--ac-font-size-sm);
  font-weight: 400;
  transition: color var(--ac-transition-fast), background-color var(--ac-transition-fast);
}

.nav-item::before {
  content: "";
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 5px;
  background: var(--ac-color-primary);
  opacity: 0;
  transition: opacity var(--ac-transition-fast);
}

.nav-item .el-icon {
  width: 18px;
  height: 18px;
  font-size: 18px;
  flex-shrink: 0;
}

.nav-item span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nav-item:hover {
  background: var(--ac-color-surface-hover);
  color: var(--ac-color-text);
}

.nav-active {
  background: var(--ac-color-primary-bg);
  color: var(--ac-color-text);
  font-weight: 600;
}

.nav-active::before {
  opacity: 1;
}

.nav-active:hover {
  background: var(--ac-color-primary-bg);
  color: var(--ac-color-text);
}

.nav-version {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 30px;
  margin: 10px 16px 0;
  color: var(--ac-color-text-muted);
  font-size: 11px;
  flex-shrink: 0;
}
</style>
