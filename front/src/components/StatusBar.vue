<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <header class="status-bar">
    <div class="collapse-btn" @click="toggleSidebar">
      <el-icon :class="{ 'is-collapsed': isSidebarCollapsed }"><DArrowLeft /></el-icon>
    </div>
    <div class="status-search" @click="searchModal?.open()">
      <el-icon><Search /></el-icon>
      <span>搜索功能、角色、会话、记忆...</span>
    </div>
    <div class="status-center">
      <div class="status-indicators">
        <span class="status-dot status-on" :title="deployLabel">
          <span class="dot"></span>
          <span class="dot-label">核心服务正常</span>
        </span>
        <span class="status-dot" :class="wechatClass" :title="wechatLabel">
          <span class="dot"></span>
          <span class="dot-label">{{ wechatLabel }}</span>
        </span>
        <span class="status-dot" :class="qqClass" :title="qqLabel">
          <span class="dot"></span>
          <span class="dot-label">{{ qqLabel }}</span>
        </span>
        <span class="status-dot" :class="modelClass" :title="modelLabel">
          <span class="dot"></span>
          <span class="dot-label">模型服务{{ modelStatus === "configured" ? "正常" : "待配置" }}</span>
        </span>
      </div>
    </div>
    <div class="status-right">
      <el-button text circle class="icon-button" @click="$emit('toggleTheme')">
        <el-icon><component :is="themeIcon" /></el-icon>
      </el-button>
      <el-dropdown v-if="username" trigger="click">
        <button class="user-button">
          <span class="avatar"><el-icon><UserFilled /></el-icon></span>
          <span>{{ username }}</span>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item @click="$emit('logout')">退出登录</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <button v-else class="user-button">
        <span class="avatar"><el-icon><UserFilled /></el-icon></span>
        <span>管理员</span>
      </button>
    </div>
    <SearchModal ref="searchModal" />
  </header>
</template>

<script setup lang="ts">
import { computed, ref, inject } from "vue"
import type { Ref } from "vue"
import { DArrowLeft, Moon, Search, Sunny, UserFilled } from "@element-plus/icons-vue"
import SearchModal from "./SearchModal.vue"
const props = defineProps<{
  deployMode?: string
  wechatStatus?: string
  qqStatus?: string
  modelStatus?: string
  characterName?: string
  theme?: string
  username?: string
}>()

defineEmits<{
  toggleTheme: []
  logout: []
}>()

const isSidebarCollapsed = inject<Ref<boolean>>("isSidebarCollapsed", ref(false))
const toggleSidebar = inject<() => void>("toggleSidebar", () => {})

const searchModal = ref<InstanceType<typeof SearchModal> | null>(null)

const deployLabel = computed(() =>
  props.deployMode === "cloud-web" ? "私有云" : "本地"
)

const wechatClass = computed(() =>
  props.wechatStatus === "connected" ? "status-on" : "status-off"
)
const wechatLabel = computed(() =>
  props.wechatStatus === "connected" ? "微信已连接" : "微信未连接"
)

const qqClass = computed(() =>
  props.qqStatus === "connected" || props.qqStatus === "online" ? "status-on" : "status-off"
)
const qqLabel = computed(() =>
  props.qqStatus === "connected" || props.qqStatus === "online" ? "QQ已连接" : "QQ未连接"
)

const modelClass = computed(() =>
  props.modelStatus === "configured" ? "status-on" : "status-off"
)
const modelLabel = computed(() =>
  props.modelStatus === "configured" ? "模型已配" : "模型未配"
)

const themeIcon = computed(() => props.theme === "dark" ? Sunny : Moon)
</script>

<style scoped>
.status-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  height: 56px;
  padding: 0 12px;
  background: var(--console-topbar);
  border-bottom: 1px solid var(--console-border);
  flex-shrink: 0;
  user-select: none;
  backdrop-filter: blur(14px);
}

.collapse-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  cursor: pointer;
  opacity: 0.45;
  transition: opacity 0.2s;
  color: var(--console-text);
  border-radius: 7px;
  flex-shrink: 0;
}

.collapse-btn:hover {
  opacity: 0.8;
  background: var(--nav-hover-bg);
}

.collapse-btn .is-collapsed {
  transform: rotate(180deg);

}

.status-search {
  display: flex;
  align-items: center;
  gap: 10px;
  width: min(348px, 28vw);
  height: 32px;
  padding: 0 10px;
  border: 1px solid var(--console-border);
  border-radius: 10px;
  background: var(--console-search-bg);
  cursor: pointer;
  color: var(--console-search-text);
  box-shadow: inset 0 1px 1px rgba(15, 23, 42, 0.02);
  font-size: 12px;
}

.status-search span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.status-center {
  flex: 1;
  display: flex;
  justify-content: flex-start;
}

.status-indicators {
  display: flex;
  gap: 10px;
}

.status-dot {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 30px;
  padding: 0 10px;
  border: 1px solid rgba(34, 197, 94, 0.16);
  border-radius: 9px;
  background: var(--console-status-ok-bg);
  font-size: 12px;
  color: var(--console-text-secondary);
  cursor: default;
}

.status-dot .dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-on .dot { background: var(--status-ok-color); }

.status-off .dot { background: var(--status-off-color); }

.status-off {
  border-color: rgba(245, 158, 11, 0.22);
  background: var(--console-status-warn-bg);
}

.dot-label {
  white-space: nowrap;
}

.status-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.icon-button {
  color: var(--console-text);
  font-size: 16px;
}

.user-button {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--console-text-secondary);
  font-size: 12px;
  cursor: pointer;
}

.avatar {
  display: inline-flex;
  align-items: center;
  justify-content: flex-start;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  color: var(--console-text-muted);
  background: var(--console-avatar-bg);
}

@media (max-width: 768px) {
  .status-indicators {
    gap: 10px;
  }
  .dot-label {
    display: none;
  }
}
</style>
