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
    <button v-show="appStore.sidebarCollapsed" class="side-collapse side-collapse--collapsed" type="button" aria-label="展开导航" @click="appStore.toggleSidebar"><el-icon><DArrowRight /></el-icon></button>
    <button v-show="!appStore.sidebarCollapsed" class="side-collapse" type="button" aria-label="收起导航" @click="appStore.toggleSidebar"><el-icon><DArrowLeft /></el-icon></button>
    <button v-show="!appStore.sidebarCollapsed" type="button" class="nav-search" @click="searchModal?.open()"><el-icon><Search /></el-icon><span>搜索功能、角色、会话、记忆</span></button>
    <button class="new-chat" type="button" @click="handleNewChat"><el-icon><Plus /></el-icon><span v-show="!appStore.sidebarCollapsed">新对话</span></button>
    <el-menu
      :default-active="activeIndex"
      :collapse="appStore.sidebarCollapsed"
      unique-opened
      router
      class="side-menu"
    >
      <el-menu-item index="/chat">
        <el-icon><ChatDotRound /></el-icon>
        <span>聊天</span>
      </el-menu-item>
      <el-sub-menu index="character">
        <template #title>
          <el-icon><UserFilled /></el-icon>
          <span>角色</span>
        </template>
        <el-menu-item index="/character">角色管理</el-menu-item>
        <el-menu-item index="/reminders">日程提醒</el-menu-item>
        <el-menu-item index="/profiles">用户画像</el-menu-item>
        <el-menu-item index="/world-book">世界书</el-menu-item>
      </el-sub-menu>
      <el-sub-menu index="memory">
        <template #title>
          <el-icon><Grid /></el-icon>
          <span>记忆</span>
        </template>
        <el-menu-item index="/memory-manager">记忆总览</el-menu-item>
        <el-menu-item index="/episodic">情景记忆</el-menu-item>
        <el-menu-item index="/graph">记忆图谱</el-menu-item>
        <el-menu-item index="/memory-timeline">时间线</el-menu-item>
        <el-menu-item index="/logs">聊天记录</el-menu-item>
        <el-menu-item index="/import">导入记录</el-menu-item>
      </el-sub-menu>
      <el-sub-menu index="more">
        <template #title><el-icon><Odometer /></el-icon><span>更多</span></template>
        <el-menu-item index="/dashboard/run">运行概览</el-menu-item>
        <el-menu-item index="/dashboard/data">运行数据</el-menu-item>
        <el-menu-item index="/wechat"><el-icon><Connection /></el-icon>微信连接</el-menu-item>
        <el-menu-item index="/qq"><el-icon><ChatDotSquare /></el-icon>QQ 连接</el-menu-item>
        <el-menu-item index="/emotes">表情包管理</el-menu-item>
        <el-menu-item index="/extensions">扩展中心</el-menu-item>
        <el-menu-item index="/creative-workshop"><el-icon><MagicStick /></el-icon>创意工坊</el-menu-item>
        <el-menu-item index="/settings"><el-icon><Setting /></el-icon>设置</el-menu-item>
      </el-sub-menu>
    </el-menu>

    <div class="side-nav-bottom">
      <div class="side-status" :title="statusTitle">
        <span class="side-status__dot" :class="{ 'is-off': modelStatus !== 'configured' }"></span>
        <span v-show="!appStore.sidebarCollapsed">{{ statusTitle }}</span>
      </div>
      <button
        class="user-profile"
        type="button"
        :title="username || '管理员'"
        @click="openUserProfile"
      >
        <span class="user-avatar">
          <img v-if="avatar" :src="avatar" alt="用户头像" />
          <el-icon v-else><UserFilled /></el-icon>
        </span>
        <span v-show="!appStore.sidebarCollapsed" class="user-copy">
          <strong>{{ username || "管理员" }}</strong>
          <span>当前用户</span>
        </span>
      </button>
      <button v-show="!appStore.sidebarCollapsed" type="button" class="theme-entry" :aria-label="theme === 'dark' ? '切换为亮色主题' : '切换为暗色主题'" @click="$emit('toggleTheme')"><el-icon><Moon v-if="theme === 'light'" /><Sunny v-else /></el-icon><span>{{ theme === 'dark' ? '暗色主题' : '亮色主题' }}</span></button>
    </div>
    <SearchModal ref="searchModal" />
  </nav>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";

import { useRoute, useRouter } from "vue-router";
import {
  ChatDotRound,
  Odometer,
  Connection,
  UserFilled,
  ChatDotSquare,
  Setting,
  Grid,
  MagicStick,
  Plus,
  DArrowLeft,
  DArrowRight,
  Search,
  Moon,
  Sunny,
} from "@element-plus/icons-vue";
import { useAppStore } from "@/stores/app";
import { useBrandLogo } from "@/composables/useBrandLogo";
import { useApi } from "@/composables/useApi";
import { ElMessage } from "element-plus";
import SearchModal from "./SearchModal.vue";

const route = useRoute();
const router = useRouter();

const appStore = useAppStore();
const { logoUrl } = useBrandLogo();
const { post } = useApi();
const props = defineProps<{
  username?: string;
  avatar?: string;
  wechatStatus?: string;
  qqStatus?: string;
  modelStatus?: string;
  theme?: "light" | "dark";
}>();
defineEmits<{ toggleTheme: [] }>();
const searchModal = ref<InstanceType<typeof SearchModal> | null>(null);
const statusTitle = computed(() => {
  if (props.modelStatus === "unconfigured") return "模型未配置";
  if (props.modelStatus === "error") return "核心服务异常";
  const channels = [props.wechatStatus === "connected" ? "微信" : "", props.qqStatus === "connected" || props.qqStatus === "online" ? "QQ" : ""].filter(Boolean);
  return channels.length ? `核心服务正常 · ${channels.join(" / ")} 已连接` : "核心服务正常";
});

async function createNewConversation() {
  try {
    const currentCharId = localStorage.getItem("webchat-char-id");
    const cachedChar = JSON.parse(localStorage.getItem("uai-default-char") || "{}");
    const charId = currentCharId || cachedChar?.id;
    if (!charId) {
      ElMessage.warning("请先选择角色");
      return null;
    }
    const created = await post<any>("/api/web-chat/conversations", {
      characterId: charId,
      title: "",
    });
    return created?.id || null;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg || "创建新对话失败");
    return null;
  }
}

async function handleNewChat() {
  const newConvId = await createNewConversation();
  if (!newConvId) return;
  localStorage.setItem("webchat-conv-id", newConvId);
  if (route.path === "/chat") {
    window.dispatchEvent(new CustomEvent("amitia:new-chat", { detail: { conversationId: newConvId } }));
  } else {
    await router.push("/chat");
  }
}

const CHAR_PATHS = [
  "/character",
  "/reminders",
  "/memory-manager",
  "/emotes",
  "/episodic",
  "/graph",
  "/memory-timeline",
  "/profiles",
  "/world-book",
  "/logs",
  "/import",
];

const activeIndex = computed(() => {
  const path = route.path;
  if (CHAR_PATHS.some((p) => path.startsWith(p))) {
    return path;
  }
  if (path.startsWith("/dashboard")) {
    return path;
  }
  if (path.startsWith("/extensions")) {
    if (route.query.from === "creative-workshop") {
      return "/creative-workshop";
    }
    return "/extensions";
  }
  if (path.startsWith("/kernel")) {
    return "/extensions";
  }
  if (path.startsWith("/creative-workshop")) {
    return "/creative-workshop";
  }
  return path;
});

function openUserProfile() {
  router.push("/user-settings");
}
</script>

<style scoped>
.side-nav {
  width: var(--ac-sidebar-width);
  height: 100%;
  background: var(--workbench-sidebar-bg);
  border-right: 1px solid var(--surface-border);
  box-shadow: none;
  display: flex;
  flex-direction: column;
  padding: 16px 0 0;
  user-select: none;
  flex-shrink: 0;
  transition: width 0.3s ease;
}

:global(html[data-theme="dark"]) .side-nav {
  background: var(--workbench-sidebar-bg);
  border-right-color: var(--surface-border);
  -webkit-backdrop-filter: none;
  backdrop-filter: none;
}

.side-nav.is-collapsed {
  width: 60px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 14px 14px;
  flex-shrink: 0;
}

.side-nav.is-collapsed .brand {
  justify-content: center;
  padding: 0 10px 14px;
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
  font-size: 18px;
  font-weight: 650;
  letter-spacing: 0;
  white-space: nowrap;
  overflow: hidden;
}
.side-collapse { display: grid; place-items: center; width: 28px; height: 28px; margin-left: auto; border: 0; border-radius: var(--radius-xs); background: transparent; color: var(--text-muted); cursor: pointer; }
.side-collapse:hover, .side-collapse:focus-visible { background: var(--workbench-sidebar-hover); color: var(--text-primary); outline: none; }
.side-nav.is-collapsed .side-collapse { margin-left: 0; }
.side-collapse--collapsed { width: 36px; height: 36px; margin: 0 auto 12px; border: 1px solid var(--surface-border); border-radius: var(--radius-sm); }
.side-collapse--collapsed:hover { border-color: var(--surface-border-hover); }
.new-chat { display: flex; align-items: center; justify-content: center; gap: 8px; min-height: 38px; margin: 0 12px 12px; border: 1px solid var(--surface-border); border-radius: var(--radius-sm); background: var(--surface-bg); color: var(--text-primary); cursor: pointer; font: inherit; }
.new-chat:hover, .new-chat:focus-visible { border-color: var(--surface-border-hover); background: var(--control-hover-bg); outline: none; }
.side-nav.is-collapsed .new-chat { width: 40px; margin: 0 auto 12px; }
.nav-search, .theme-entry { display: flex; align-items: center; gap: 8px; width: calc(100% - 24px); min-height: 34px; margin: 0 12px 10px; padding: 0 10px; border: 1px solid transparent; border-radius: var(--radius-sm); background: transparent; color: var(--text-muted); cursor: pointer; font: inherit; font-size: 12px; text-align: left; }
.nav-search:hover, .nav-search:focus-visible, .theme-entry:hover, .theme-entry:focus-visible { border-color: var(--surface-border); background: var(--control-hover-bg); color: var(--text-primary); outline: none; }

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
  border-radius: var(--radius-sm);
}

.side-menu :deep(.el-menu-item:hover),
.side-menu :deep(.el-sub-menu__title:hover) {
  background: var(--workbench-sidebar-hover);
  color: var(--nav-hover-color);
}

.side-menu :deep(.el-menu-item.is-active) {
  background: var(--workbench-sidebar-active);
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

.side-status { display: flex; align-items: center; gap: 7px; min-height: 28px; padding: 0 10px 8px; color: var(--text-muted); font-size: 11px; overflow: hidden; white-space: nowrap; }
.side-status__dot { width: 7px; height: 7px; flex: 0 0 auto; border-radius: 50%; background: var(--ac-color-success); }
.side-status__dot.is-off { background: var(--ac-color-warning); }
.theme-entry { margin: 0 0 4px; width: 100%; }

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
