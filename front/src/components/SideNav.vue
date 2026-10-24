<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <nav class="side-nav" data-amitia-ui="app.sidebar">
    <div class="brand-row">
      <img class="brand-mark" :src="logoUrl" alt="Amitia" />
    </div>

    <div class="sidebar-scroll">
      <el-menu
        :default-active="activeIndex"
        :collapse="true"
        :collapse-transition="false"
        unique-opened
        router
        class="side-menu"
      >
        <template v-for="group in navigationGroups" :key="group.id">
          <el-sub-menu v-if="group.label && group.items.length > 1" :index="group.id">
            <template
              #title
            >
              <span
                class="side-nav-tooltip-anchor"
                :aria-label="`${group.label}：${groupDescription(group)}`"
                aria-describedby="side-nav-tooltip"
                @mouseenter="showTooltip($event, group.label!, groupDescription(group))"
                @mouseleave="hideTooltip"
                @focus="showTooltip($event, group.label!, groupDescription(group))"
                @blur="hideTooltip"
              >
              <el-icon><component :is="group.icon" /></el-icon>
              <span>{{ group.label }}</span>
              </span>
            </template>
            <el-menu-item
              v-for="item in group.items"
              :key="item.id"
              :index="item.route"
              :style="navigationItemStyle"
              data-amitia-ui="app.sidebar.item"
              :aria-label="`${item.label}：${itemDescription(item)}`"
              aria-describedby="side-nav-tooltip"
              @mouseenter="handleItemEnter($event, item)"
              @mouseleave="hideTooltip"
              @focus="handleItemFocus($event, item)"
              @blur="hideTooltip"
            >
              <el-icon><component :is="item.icon" /></el-icon>
              <span>{{ item.label }}</span>
            </el-menu-item>
          </el-sub-menu>
          <el-menu-item
            v-for="item in group.label && group.items.length > 1 ? [] : group.items"
            :key="item.id"
            :index="item.route"
            :style="navigationItemStyle"
            data-amitia-ui="app.sidebar.item"
            :aria-label="`${item.label}：${itemDescription(item)}`"
            aria-describedby="side-nav-tooltip"
            @mouseenter="handleItemEnter($event, item)"
            @mouseleave="hideTooltip"
            @focus="handleItemFocus($event, item)"
            @blur="hideTooltip"
          >
            <el-icon><component :is="item.icon" /></el-icon>
            <span>{{ item.label }}</span>
          </el-menu-item>
        </template>
      </el-menu>
    </div>

    <div class="side-nav-bottom">
      <div
        v-if="profileMenuOpen"
        class="profile-menu"
        role="menu"
        aria-label="个人空间选项"
        @click.stop
      >
        <button type="button" role="menuitem" class="profile-menu__item" data-amitia-ui="app.sidebar.item" @click="openArchivedConversations">
          <el-icon><Box /></el-icon>
          <span>归档对话</span>
        </button>
        <button type="button" role="menuitem" class="profile-menu__item" data-amitia-ui="app.sidebar.item" @click="openSettings">
          <el-icon><Setting /></el-icon>
          <span>设置</span>
        </button>
        <button type="button" role="menuitem" class="profile-menu__item" data-amitia-ui="app.sidebar.item" @click="openMySpace">
          <el-icon><UserFilled /></el-icon>
          <span>我的空间</span>
        </button>
        <button
          type="button"
          role="menuitem"
          class="profile-menu__item"
          data-amitia-ui="app.sidebar.item"
          :aria-label="theme === 'dark' ? '切换为亮色模式' : '切换为暗色模式'"
          @click="toggleTheme"
        >
          <el-icon><Moon v-if="theme === 'light'" /><Sunny v-else /></el-icon>
          <span>{{ theme === 'dark' ? '切换为亮色模式' : '切换为暗色模式' }}</span>
        </button>
      </div>
      <button
        class="user-profile"
        type="button"
        data-amitia-ui="app.sidebar.item"
        :aria-label="username || '个人空间'"
        aria-describedby="side-nav-tooltip"
        :aria-expanded="profileMenuOpen"
        aria-haspopup="menu"
        @mouseenter="showTooltip($event, username || '我的空间', '管理个人资料、空间身份、运行模式和设备配对。')"
        @mouseleave="hideTooltip"
        @focus="showTooltip($event, username || '我的空间', '管理个人资料、空间身份、运行模式和设备配对。')"
        @blur="hideTooltip"
        @click.stop="profileMenuOpen = !profileMenuOpen"
      >
        <span class="user-avatar">
          <img v-if="avatar" :src="avatar" alt="个人头像" />
          <el-icon v-else><UserFilled /></el-icon>
        </span>
      </button>
    </div>

    <Teleport to="body">
      <Transition name="side-nav-tooltip">
        <div
          v-if="tooltip"
          id="side-nav-tooltip"
          class="side-nav-tooltip"
          role="tooltip"
          :style="{ left: `${tooltip.left}px`, top: `${tooltip.top}px` }"
        >
          <strong>{{ tooltip.title }}</strong>
          <span>{{ tooltip.description }}</span>
        </div>
      </Transition>
    </Teleport>
  </nav>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Box, Moon, Setting, Sunny, UserFilled } from "@element-plus/icons-vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { useBrandLogo } from "@/composables/useBrandLogo";
import {
  isUINavigationItemActive,
  useUINavigationRegistry,
  type UINavigationGroup,
  type UINavigationItem,
} from "@/ui-runtime/navigationRegistry";
import { prewarmNavigationItem } from "@/ui-runtime/navigationPrewarm";
import { useUIComponentVariant } from "@/ui-runtime/componentRegistry";

defineProps<{
  username?: string;
  avatar?: string;
  theme?: "light" | "dark";
}>();
const emit = defineEmits<{ toggleTheme: [] }>();
const route = useRoute();
const router = useRouter();
const extensionUIStore = useExtensionUIStore();
const { groups: navigationGroups, items: navigationItems } = useUINavigationRegistry();
const { style: navigationItemStyle } = useUIComponentVariant("navigationItem");
const { logoUrl } = useBrandLogo();
const profileMenuOpen = ref(false);
const tooltip = ref<{ title: string; description: string; left: number; top: number } | null>(null);
let tooltipTimer: number | null = null;
const groupDescriptions: Record<string, string> = {
  home: "对话、最近会话和日常入口。",
  character: "角色、用户画像和世界书。",
  memory: "知识库、记忆总览、图谱和时间线。",
  tasks: "定时消息和角色主动任务。",
  continuity: "长期目标、承诺和进展。",
  workshop: "游戏模式、角色卡和桌宠。",
  extensions: "扩展包、工作流、MCP 与服务。",
  channels: "渠道插件和宿主消息。",
};
const activeIndex = computed(() => {
  const path = route.path;
  const active = navigationItems.value.find((item) => isUINavigationItemActive(path, item));
  return active?.route ?? path;
});

function openMySpace() {
  profileMenuOpen.value = false;
  router.push("/my-space");
}

function openArchivedConversations() {
  profileMenuOpen.value = false;
  router.push("/logs");
}

function openSettings() {
  profileMenuOpen.value = false;
  router.push("/settings");
}

function toggleTheme() {
  emit("toggleTheme");
}

function prewarmItem(item: UINavigationItem) {
  prewarmNavigationItem(extensionUIStore, item);
}

function itemDescription(item: UINavigationItem) {
  return item.description || groupDescriptions[item.group] || `打开${item.label}。`;
}

function groupDescription(group: UINavigationGroup) {
  return groupDescriptions[group.id] || `查看${group.label || "该分组"}相关功能。`;
}

function showTooltip(event: MouseEvent | FocusEvent, title: string, description: string) {
  hideTooltip();
  const anchor = event.currentTarget as HTMLElement | null;
  if (!anchor) return;
  const rect = anchor.getBoundingClientRect();
  const width = 232;
  const left = Math.min(rect.right + 12, window.innerWidth - width - 12);
  const top = Math.min(Math.max(rect.top + rect.height / 2, 54), window.innerHeight - 54);
  tooltipTimer = window.setTimeout(() => {
    tooltip.value = { title, description, left, top };
    tooltipTimer = null;
  }, 100);
}

function hideTooltip() {
  if (tooltipTimer !== null) {
    window.clearTimeout(tooltipTimer);
    tooltipTimer = null;
  }
  tooltip.value = null;
}

function handleItemEnter(event: MouseEvent, item: UINavigationItem) {
  prewarmItem(item);
  showTooltip(event, item.label, itemDescription(item));
}

function handleItemFocus(event: FocusEvent, item: UINavigationItem) {
  prewarmItem(item);
  showTooltip(event, item.label, itemDescription(item));
}

function closeProfileMenu() {
  profileMenuOpen.value = false;
  hideTooltip();
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === "Escape") closeProfileMenu();
}

onMounted(() => {
  window.addEventListener("click", closeProfileMenu);
  window.addEventListener("keydown", handleKeydown);
});

onUnmounted(() => {
  window.removeEventListener("click", closeProfileMenu);
  window.removeEventListener("keydown", handleKeydown);
  hideTooltip();
});
</script>

<style scoped>
.side-nav {
  width: 64px;
  min-width: 64px;
  max-width: 64px;
  flex: 0 0 64px;
  box-sizing: border-box;
  height: 100%;
  background: var(--workbench-sidebar-bg);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 8px 8px 0;
  user-select: none;
  flex-shrink: 0;
}
.brand-row { width: 100%; min-height: 36px; display: grid; place-items: center; padding-bottom: 4px; }
.brand-mark { width: 24px; height: 24px; border-radius: 7px; object-fit: contain; }
.sidebar-scroll { width: 100%; min-height: 0; flex: 1 1 auto; overflow-y: auto; overflow-x: hidden; }
.side-menu { border-right: none; background: transparent; width: 100%; margin-bottom: 12px; }
.side-menu :deep(.el-menu-item), .side-menu :deep(.el-sub-menu__title) { height: 44px; min-height: 44px; margin: 4px 0; padding: 0 !important; border-radius: 10px; display: flex; align-items: center; justify-content: center; color: var(--text-secondary); }
.side-menu :deep(.el-icon) { width: 19px; height: 19px; margin: 0; font-size: 18px; }
.side-menu :deep(.el-menu-item:hover), .side-menu :deep(.el-sub-menu__title:hover) { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.side-menu :deep(.el-menu-item.is-active), .side-menu :deep(.el-sub-menu.is-active > .el-sub-menu__title) { background: var(--workbench-sidebar-active); color: var(--text-primary); font-weight: 550; }
.side-menu :deep(.el-sub-menu .el-menu) { background: transparent; }
.side-nav-bottom { position: relative; width: 100%; flex: 0 0 auto; margin-top: auto; display: flex; flex-direction: column; align-items: center; gap: 6px; border-top: 1px solid var(--surface-border); padding: 8px 0; }
.profile-menu { position: absolute; left: calc(100% + 8px); bottom: 0; display: grid; gap: 2px; width: 188px; padding: 5px; border: 1px solid var(--surface-border); border-radius: 10px; background: var(--ac-color-surface); z-index: 30; }
.profile-menu__item { display: flex; align-items: center; gap: 9px; min-height: 34px; width: 100%; padding: 0 8px; border: 0; border-radius: 7px; background: transparent; color: var(--text-secondary); cursor: pointer; font: inherit; font-size: var(--ac-font-size-sm); text-align: left; transition: background-color 0.18s ease, color 0.18s ease; }
.profile-menu__item:hover, .profile-menu__item:focus-visible { background: var(--workbench-sidebar-hover); color: var(--text-primary); outline: none; }
.profile-menu__item .el-icon { font-size: 15px; }
.profile-menu__item:disabled { cursor: wait; opacity: 0.7; }
.user-profile { display: grid; place-items: center; width: 38px; height: 38px; padding: 0; border: 0; border-radius: 50%; background: transparent; color: var(--text-secondary); cursor: pointer; }
.user-profile:hover { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.user-avatar { display: grid; place-items: center; width: 28px; height: 28px; flex: 0 0 auto; border-radius: 50%; background: var(--tp-selection); color: var(--text-primary); font-size: var(--ac-font-size-sm); overflow: hidden; }
.user-avatar img { width: 100%; height: 100%; object-fit: cover; }
.side-nav :deep(button:not(:disabled):focus-visible), .side-menu :deep(.el-menu-item:focus-visible), .side-menu :deep(.el-sub-menu__title:focus-visible) { outline: 2px solid var(--ac-color-primary); outline-offset: -2px; }
.side-nav-tooltip-anchor { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; }
.side-nav-tooltip { position: fixed; z-index: 4000; width: max-content; max-width: 232px; padding: 9px 11px; border: 1px solid var(--surface-border); border-radius: 10px; background: var(--surface-bg-elevated); color: var(--text-primary); box-shadow: 0 10px 28px color-mix(in srgb, #000 24%, transparent); pointer-events: none; transform: translateY(-50%); }
.side-nav-tooltip strong { display: block; font-size: 12px; font-weight: 620; line-height: 1.35; }
.side-nav-tooltip span { display: block; margin-top: 3px; color: var(--text-secondary); font-size: 11px; line-height: 1.45; }
.side-nav-tooltip-enter-active, .side-nav-tooltip-leave-active { transition: opacity 0.12s ease, transform 0.12s ease; }
.side-nav-tooltip-enter-from, .side-nav-tooltip-leave-to { opacity: 0; transform: translateY(-50%) translateX(-4px); }
</style>
