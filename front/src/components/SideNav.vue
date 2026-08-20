<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <nav class="side-nav" :class="{ 'is-collapsed': appStore.sidebarCollapsed }">
    <div class="brand-row">
      <div class="brand">
        <img class="brand-mark" :src="logoUrl" alt="Amitia" />
        <div v-show="!appStore.sidebarCollapsed" class="brand-name">Amitia</div>
      </div>
      <div class="brand-actions">
        <button
          v-show="!appStore.sidebarCollapsed"
          type="button"
          class="icon-btn"
          aria-label="搜索"
          title="搜索"
          @click="searchModal?.open()"
        >
          <el-icon><Search /></el-icon>
        </button>
        <button
          type="button"
          class="icon-btn"
          :aria-label="appStore.sidebarCollapsed ? '展开导航' : '收起导航'"
          :title="appStore.sidebarCollapsed ? '展开导航' : '收起导航'"
          @click="appStore.toggleSidebar"
        >
          <el-icon><DArrowRight v-if="appStore.sidebarCollapsed" /><DArrowLeft v-else /></el-icon>
        </button>
      </div>
    </div>

    <el-menu
      :default-active="activeIndex"
      :collapse="appStore.sidebarCollapsed"
      unique-opened
      router
      class="side-menu"
    >
      <template v-for="group in navigationGroups" :key="group.id">
        <el-sub-menu v-if="group.label && group.items.length > 1" :index="group.id">
          <template #title>
            <el-icon><component :is="group.icon" /></el-icon>
            <span>{{ group.label }}</span>
          </template>
          <el-menu-item v-for="item in group.items" :key="item.id" :index="item.route" :style="navigationItemStyle">
            <el-icon><component :is="item.icon" /></el-icon>
            <span>{{ item.label }}</span>
          </el-menu-item>
        </el-sub-menu>
        <el-menu-item v-for="item in group.label && group.items.length > 1 ? [] : group.items" :key="item.id" :index="item.route" :style="navigationItemStyle">
          <el-icon><component :is="item.icon" /></el-icon>
          <span>{{ item.label }}</span>
        </el-menu-item>
      </template>
    </el-menu>

    <div class="side-nav-bottom">
      <div
        v-if="profileMenuOpen"
        class="profile-menu"
        role="menu"
        aria-label="账户选项"
        @click.stop
      >
        <button type="button" role="menuitem" class="profile-menu__item" @click="openSettings">
          <el-icon><Setting /></el-icon>
          <span>设置</span>
        </button>
        <button type="button" role="menuitem" class="profile-menu__item" @click="openUserProfile">
          <el-icon><UserFilled /></el-icon>
          <span>用户信息</span>
        </button>
        <button
          type="button"
          role="menuitem"
          class="profile-menu__item"
          :aria-label="theme === 'dark' ? '切换为亮色模式' : '切换为暗色模式'"
          @click="toggleTheme"
        >
          <el-icon><Moon v-if="theme === 'light'" /><Sunny v-else /></el-icon>
          <span>{{ theme === 'dark' ? '切换为亮色模式' : '切换为暗色模式' }}</span>
        </button>
        <button
          type="button"
          role="menuitem"
          class="profile-menu__item profile-menu__item--logout"
          :disabled="logoutLoading"
          @click="handleLogout"
        >
          <el-icon><SwitchButton /></el-icon>
          <span>{{ logoutLoading ? '正在退出…' : '退出登录' }}</span>
        </button>
      </div>
      <button
        class="user-profile"
        type="button"
        :title="username || '管理员'"
        :aria-expanded="profileMenuOpen"
        aria-haspopup="menu"
        @click.stop="profileMenuOpen = !profileMenuOpen"
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
    </div>
    <SearchModal ref="searchModal" />
  </nav>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  DArrowLeft,
  DArrowRight,
  Moon,
  Search,
  Setting,
  Sunny,
  SwitchButton,
  UserFilled,
} from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useBrandLogo } from "@/composables/useBrandLogo";
import { apiClient, useApi } from "@/composables/useApi";
import { forceCleanupSession } from "@/stores/refresh-coordinator";
import SearchModal from "./SearchModal.vue";
import { isUINavigationItemActive, useUINavigationRegistry } from "@/ui-runtime/navigationRegistry";
import { useUIComponentVariant } from "@/ui-runtime/componentRegistry";

const route = useRoute();
const router = useRouter();
const appStore = useAppStore();
const { groups: navigationGroups, items: navigationItems } = useUINavigationRegistry();
const { style: navigationItemStyle } = useUIComponentVariant("navigationItem");
const { logoUrl } = useBrandLogo();
const { get, post } = useApi();
defineProps<{
  username?: string;
  avatar?: string;
  theme?: "light" | "dark";
}>();
const emit = defineEmits<{ toggleTheme: [] }>();
const searchModal = ref<InstanceType<typeof SearchModal> | null>(null);
const recentConversations = ref<any[]>([]);
const activeConversationId = ref(localStorage.getItem("webchat-conv-id") || "");
const profileMenuOpen = ref(false);
const logoutLoading = ref(false);
let isMounted = false;

async function fetchRecentConversations() {
  try {
    const result = await get<any>("/api/web-chat/conversations", { page: 1, pageSize: 7, channel: "web" });
    if (isMounted) {
      recentConversations.value = (result?.items || result?.conversations || []).slice(0, 7);
    }
  } catch {
    if (isMounted) {
      recentConversations.value = [];
    }
  }
}

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
  if (!isMounted) return;
  const newConvId = await createNewConversation();
  if (!newConvId || !isMounted) return;
  activeConversationId.value = newConvId;
  localStorage.setItem("webchat-conv-id", newConvId);
  localStorage.setItem("webchat-last-conv", "char");
  await fetchRecentConversations();
  window.dispatchEvent(new CustomEvent("amitia:conversation-list-changed"));
  if (route.path === "/chat") {
    window.dispatchEvent(new CustomEvent("amitia:new-chat", { detail: { conversationId: newConvId } }));
  } else {
    await router.push("/chat");
  }
}

async function handleSelectRecent(conversation: any) {
  if (!conversation?.id || !isMounted) return;
  activeConversationId.value = conversation.id;
  localStorage.setItem("webchat-conv-id", conversation.id);
  localStorage.setItem("webchat-last-conv", "char");
  const charId = conversation.characterId || conversation.character_id;
  if (charId) localStorage.setItem("webchat-char-id", charId);
  if (route.path === "/chat") {
    window.dispatchEvent(new CustomEvent("amitia:select-conversation", { detail: { conversation } }));
  } else {
    await router.push("/chat");
  }
}

function handleConversationListChanged() {
  if (!isMounted) return;
  activeConversationId.value = localStorage.getItem("webchat-conv-id") || "";
  void fetchRecentConversations();
}

const activeIndex = computed(() => {
  const path = route.path;
  const active = navigationItems.value.find((item) => isUINavigationItemActive(path, item));
  return active?.route ?? path;
});

function openUserProfile() {
  profileMenuOpen.value = false;
  router.push("/user-settings");
}

function openSettings() {
  profileMenuOpen.value = false;
  router.push("/settings");
}

function toggleTheme() {
  emit("toggleTheme");
}

async function handleLogout() {
  try {
    await ElMessageBox.confirm("退出后需要重新登录，确定继续吗？", "退出登录", {
      confirmButtonText: "退出登录",
      cancelButtonText: "取消",
      type: "warning",
      confirmButtonClass: "el-button--danger",
    });
  } catch {
    return;
  }
  profileMenuOpen.value = false;
  logoutLoading.value = true;
  try {
    await apiClient.post("/api/auth/logout");
  } catch {}
  forceCleanupSession();
  await router.replace("/login");
  logoutLoading.value = false;
}

function closeProfileMenu() {
  profileMenuOpen.value = false;
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === "Escape") closeProfileMenu();
}

onMounted(() => {
  isMounted = true;
  void fetchRecentConversations();
  window.addEventListener("amitia:conversation-list-changed", handleConversationListChanged);
  window.addEventListener("click", closeProfileMenu);
  window.addEventListener("keydown", handleKeydown);
});

onUnmounted(() => {
  isMounted = false;
  window.removeEventListener("amitia:conversation-list-changed", handleConversationListChanged);
  window.removeEventListener("click", closeProfileMenu);
  window.removeEventListener("keydown", handleKeydown);
});
</script>

<style scoped>
.side-nav {
  width: var(--ac-sidebar-width);
  height: 100%;
  background: var(--workbench-sidebar-bg);
  display: flex;
  flex-direction: column;
  padding: 8px 8px 0;
  user-select: none;
  flex-shrink: 0;
  transition: width 0.2s ease;
}
.side-nav.is-collapsed { width: 52px; padding-inline: 6px; }
.brand-row { display: flex; align-items: center; justify-content: space-between; min-height: 42px; padding: 0 4px 6px; gap: 6px; }
.brand { display: flex; align-items: center; min-width: 0; gap: 8px; }
.brand-mark { width: 26px; height: 26px; border-radius: 7px; object-fit: contain; flex: 0 0 auto; }
.brand-name { color: var(--text-primary); font-size: 14px; font-weight: 650; white-space: nowrap; }
.brand-actions { display: flex; align-items: center; gap: 2px; }
.icon-btn { display: grid; place-items: center; width: 28px; height: 28px; padding: 0; border: 0; border-radius: 7px; background: transparent; color: var(--text-muted); cursor: pointer; }
.icon-btn:hover, .icon-btn:focus-visible { background: var(--workbench-sidebar-hover); color: var(--text-primary); outline: none; }
.side-nav.is-collapsed .brand-row { justify-content: center; flex-direction: column; padding-bottom: 8px; }
.side-nav.is-collapsed .brand-actions { width: 100%; justify-content: center; }
.new-chat { display: flex; align-items: center; gap: 9px; min-height: 34px; width: 100%; margin: 2px 0 7px; padding: 0 9px; border: 0; border-radius: 7px; background: transparent; color: var(--text-primary); cursor: pointer; font: inherit; font-size: 13px; text-align: left; }
.new-chat:hover, .new-chat:focus-visible { background: var(--workbench-sidebar-hover); outline: none; }
.side-nav.is-collapsed .new-chat { justify-content: center; padding: 0; }
.side-menu { border-right: none; background: transparent; flex: 1 1 auto; min-height: 0; width: 100%; overflow-y: auto; overflow-x: visible; }
.side-menu :deep(.el-menu-item), .side-menu :deep(.el-sub-menu__title) { height: 34px; line-height: 34px; min-height: 34px; margin: 1px 0; padding: 0 9px !important; border-radius: 7px; font-size: 13px; color: var(--text-secondary); }
.side-menu :deep(.el-icon) { width: 18px; font-size: 15px; margin-right: 8px; }
.side-menu :deep(.el-menu-item:hover), .side-menu :deep(.el-sub-menu__title:hover) { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.side-menu :deep(.el-menu-item.is-active), .side-menu :deep(.el-sub-menu.is-active > .el-sub-menu__title) { background: var(--workbench-sidebar-active); color: var(--text-primary); font-weight: 550; }
.side-menu :deep(.el-sub-menu .el-menu) { background: transparent; }
.side-menu :deep(.el-sub-menu .el-menu-item) { padding-left: 34px !important; height: 31px; min-height: 31px; line-height: 31px; font-size: 12px; }
.recent-section { min-height: 0; flex: 1 1 auto; overflow-y: auto; padding: 12px 0 8px; }
.section-caption { padding: 0 9px 5px; color: var(--text-muted); font-size: 11px; font-weight: 550; }
.recent-item { display: flex; align-items: center; gap: 8px; width: 100%; min-height: 31px; padding: 0 9px; border: 0; border-radius: 7px; background: transparent; color: var(--text-secondary); cursor: pointer; font: inherit; font-size: 12px; text-align: left; }
.recent-item .el-icon { flex: 0 0 auto; font-size: 13px; color: var(--text-muted); }
.recent-item span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.recent-item:hover, .recent-item.active { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.recent-item.active { background: var(--workbench-sidebar-active); }
.side-nav-bottom { position: relative; flex: 0 0 auto; margin-top: auto; border-top: 1px solid var(--surface-border); padding: 7px 0 8px; }
.profile-menu { position: absolute; right: 0; bottom: calc(100% + 8px); display: grid; gap: 2px; width: 100%; padding: 5px; border: 1px solid var(--surface-border); border-radius: 10px; background: var(--ac-color-surface); }
.profile-menu__item { display: flex; align-items: center; gap: 9px; min-height: 34px; width: 100%; padding: 0 8px; border: 0; border-radius: 7px; background: transparent; color: var(--text-secondary); cursor: pointer; font: inherit; font-size: 12px; text-align: left; transition: background-color 0.18s ease, color 0.18s ease; }
.profile-menu__item:hover, .profile-menu__item:focus-visible { background: var(--workbench-sidebar-hover); color: var(--text-primary); outline: none; }
.profile-menu__item .el-icon { font-size: 15px; }
.profile-menu__item--logout { margin-top: 4px; border-top: 1px solid var(--surface-border); border-radius: 0 0 7px 7px; color: var(--el-color-danger); }
.profile-menu__item--logout:hover, .profile-menu__item--logout:focus-visible { background: color-mix(in srgb, var(--el-color-danger) 10%, transparent); color: var(--el-color-danger); }
.profile-menu__item:disabled { cursor: wait; opacity: 0.7; }
.user-profile { display: flex; align-items: center; width: 100%; border: 0; border-radius: 7px; background: transparent; color: var(--text-secondary); cursor: pointer; text-align: left; }
.user-profile { gap: 9px; min-height: 38px; padding: 4px 7px; }
.user-profile:hover { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.user-avatar { display: grid; place-items: center; width: 28px; height: 28px; flex: 0 0 auto; border-radius: 50%; background: color-mix(in srgb, var(--tp-primary) 72%, var(--surface-bg)); color: var(--tp-text-on-primary); font-size: 12px; overflow: hidden; }
.user-avatar img { width: 100%; height: 100%; object-fit: cover; }
.user-copy { min-width: 0; }
.user-copy strong, .user-copy span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.user-copy strong { max-width: 132px; color: var(--text-primary); font-size: 12px; font-weight: 550; }
.user-copy span { margin-top: 1px; color: var(--text-muted); font-size: 10px; }
.side-nav.is-collapsed .user-profile { justify-content: center; padding-inline: 0; }
.side-nav.is-collapsed .profile-menu { width: 188px; }

.side-menu .el-icon {
  font-size: var(--ui-component-icon-size, inherit);
}
</style>
