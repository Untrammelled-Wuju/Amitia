<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <header class="status-bar">
    <div class="collapse-btn" @click="appStore.toggleSidebar">
      <el-icon :class="{ 'is-collapsed': appStore.sidebarCollapsed }"
        ><DArrowLeft
      /></el-icon>
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
      </div>
    </div>
    <div class="status-right">
      <button
        class="theme-toggle"
        :class="{ 'is-light': theme === 'light' }"
        type="button"
        role="switch"
        :aria-checked="theme === 'light'"
        :aria-label="theme === 'dark' ? '切换为亮色主题' : '切换为暗色主题'"
        title="切换主题"
        @click="$emit('toggleTheme')"
      >
        <span class="toggle-icon moon">
          <Moon />
        </span>
        <span class="toggle-icon sun">
          <Sunny />
        </span>
      </button>
    </div>
    <SearchModal ref="searchModal" />
  </header>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { DArrowLeft, Moon, Search, Sunny } from "@element-plus/icons-vue";
import SearchModal from "./SearchModal.vue";
import { useAppStore } from "@/stores/app";

const props = defineProps<{
  deployMode?: string;
  modelStatus?: string;
  characterName?: string;
  theme?: string;
}>();

defineEmits<{
  toggleTheme: [];
}>();

const appStore = useAppStore();

const searchModal = ref<InstanceType<typeof SearchModal> | null>(null);

const deployLabel = computed(() =>
  props.deployMode === "cloud-web" ? "云端统筹" : "本地模式",
);

</script>

<style scoped>
.status-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  height: 56px;
  padding: 0 12px;
  background: var(--tp-glass-bg);
  border-bottom: 1px solid var(--tp-glass-border);
  flex-shrink: 0;
  user-select: none;
  -webkit-backdrop-filter: blur(var(--tp-glass-blur))
    saturate(var(--tp-glass-saturate));
  backdrop-filter: blur(var(--tp-glass-blur)) saturate(var(--tp-glass-saturate));
  box-shadow: none;
}

:global(html[data-theme="dark"]) .status-bar {
  background: var(--console-topbar);
  border-bottom-color: var(--console-border);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: none;
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
  box-shadow: none;
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
  border: 1px solid color-mix(in srgb, var(--tp-success) 18%, transparent);
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

.status-on .dot {
  background: var(--status-ok-color);
}

.status-off .dot {
  background: var(--status-off-color);
}

.status-off {
  border-color: color-mix(in srgb, var(--tp-warning) 22%, transparent);
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

.theme-toggle {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 72px;
  height: 34px;
  padding: 3px 4px;
  border: 1px solid var(--tp-border);
  border-radius: 999px;
  background: var(--tp-panel-soft);
  color: var(--tp-text-muted);
  cursor: pointer;
  transition:
    border-color 180ms cubic-bezier(0.2, 0.8, 0.2, 1),
    background 180ms cubic-bezier(0.2, 0.8, 0.2, 1),
    transform 180ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.theme-toggle:hover {
  border-color: var(--tp-border-strong);
  background: var(--tp-control-hover);
}

.theme-toggle:focus-visible {
  outline: 2px solid var(--tp-primary);
  outline-offset: 2px;
}

.theme-toggle::before {
  content: "";
  position: absolute;
  top: 3px;
  left: 4px;
  width: 26px;
  height: 26px;
  border: 1px solid color-mix(in srgb, var(--tp-primary) 46%, transparent);
  border-radius: 50%;
  background: var(--tp-primary);
  transform: translateX(0);
  transition:
    transform 240ms cubic-bezier(0.2, 0.85, 0.2, 1),
    background 180ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.theme-toggle.is-light::before {
  transform: translateX(38px);
}

.toggle-icon {
  position: relative;
  z-index: 1;
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  border-radius: 50%;
}

.theme-toggle:not(.is-light) .moon,
.theme-toggle.is-light .sun {
  color: var(--tp-text-on-primary);
}

@media (prefers-reduced-motion: reduce) {
  .theme-toggle,
  .theme-toggle::before {
    transition-duration: 0.001ms;
  }
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
