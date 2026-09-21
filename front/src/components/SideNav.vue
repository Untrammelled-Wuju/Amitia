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
          v-if="!isDesktopShell() && !isOnboardingPage"
          type="button"
          class="icon-btn"
          :aria-label="appStore.sidebarCollapsed ? '展开导航' : '收起导航'"
          :title="appStore.sidebarCollapsed ? '展开导航' : '收起导航'"
          @click="appStore.toggleSidebar"
        >
          <svg v-if="appStore.sidebarCollapsed" class="toggle-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="3.5" y="4" width="17" height="16" rx="3" stroke="currentColor" stroke-width="1.25"/>
            <path d="M9 4.8V19.2" stroke="currentColor" stroke-width="1.25" stroke-linecap="round"/>
            <path d="M12.6 8.9L15.4 12L12.6 15.1" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <svg v-else class="toggle-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="3.5" y="4" width="17" height="16" rx="3" stroke="currentColor" stroke-width="1.25"/>
            <path d="M9 4.8V19.2" stroke="currentColor" stroke-width="1.25" stroke-linecap="round"/>
            <path d="M15.4 8.9L12.6 12L15.4 15.1" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </button>
      </div>
    </div>

    <div class="sidebar-scroll">
      <el-menu
        :default-active="activeIndex"
        :collapse="appStore.sidebarCollapsed"
        unique-opened
        router
        class="side-menu"
      >
        <el-menu-item index="/chat" :style="navigationItemStyle" @click="handleNewChat">
          <el-icon><Plus /></el-icon>
          <span>新对话</span>
        </el-menu-item>
        <template v-for="group in navigationGroups" :key="group.id">
          <el-sub-menu v-if="group.label && group.items.length > 1" :index="group.id">
            <template #title>
              <el-icon><component :is="group.icon" /></el-icon>
              <span>{{ group.label }}</span>
            </template>
            <el-menu-item
              v-for="item in group.items"
              :key="item.id"
              :index="item.route"
              :style="navigationItemStyle"
              @mouseenter="prewarmItem(item)"
              @focus="prewarmItem(item)"
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
            @mouseenter="prewarmItem(item)"
            @focus="prewarmItem(item)"
          >
            <el-icon><component :is="item.icon" /></el-icon>
            <span>{{ item.label }}</span>
          </el-menu-item>
        </template>
      </el-menu>

      <div v-show="!appStore.sidebarCollapsed" class="thread-sidebar">
      <div v-if="chatStore.sidebar.pinned.length || pinnedProjects.length" class="thread-section">
        <div class="section-caption">置顶</div>
        <SidebarConversationRow
          v-for="conversation in chatStore.sidebar.pinned"
          :key="conversation.id"
          :conversation="conversation"
          :active="conversation.id === activeConversationId"
          @select="handleSelectConversation"
          @rename="handleConversationRename"
          @toggle-pin="handleToggleConversationPin"
          @archive="handleArchiveConversation"
        />
        <SidebarProjectBlock
          v-for="project in pinnedProjects"
          :key="project.id"
          :project="project"
          :active="project.id === chatStore.currentProjectId"
          :active-conversation-id="activeConversationId"
          @select="handleSelectConversation"
          @create-conversation="handleCreateProjectConversation"
          @command="handleProjectCommand"
          @rename-conversation="handleConversationRename"
          @toggle-pin-conversation="handleToggleConversationPin"
          @archive-conversation="handleArchiveConversation"
        />
      </div>

      <div class="thread-section">
        <div class="section-caption project-caption">
          <span>最近</span>
          <button type="button" class="section-add" title="新建对话" aria-label="新建对话" @click="handleNewChat">
            <el-icon><Plus /></el-icon>
          </button>
        </div>
        <SidebarConversationRow
          v-for="conversation in visibleRecentConversations"
          :key="conversation.id"
          :conversation="conversation"
          :active="conversation.id === activeConversationId"
          @select="handleSelectConversation"
          @rename="handleConversationRename"
          @toggle-pin="handleToggleConversationPin"
          @archive="handleArchiveConversation"
        />
        <button
          v-if="chatStore.sidebar.recent.length > 5"
          type="button"
          class="thread-expand"
          @click="expandAllRecent = !expandAllRecent"
        >
          {{ expandAllRecent ? "收起" : "展开显示" }}
        </button>
        <div v-if="chatStore.sidebar.recent.length === 0" class="thread-empty">暂无对话</div>
      </div>

      <div class="thread-section">
        <div class="section-caption project-caption">
          <span>项目</span>
          <button type="button" class="section-add" title="添加文件夹" aria-label="添加文件夹" @click="handleAddProject">
            <el-icon><Plus /></el-icon>
          </button>
        </div>
        <SidebarProjectBlock
          v-for="project in regularProjects"
          :key="project.id"
          :project="project"
          :active="project.id === chatStore.currentProjectId"
          :active-conversation-id="activeConversationId"
          @select="handleSelectConversation"
          @create-conversation="handleCreateProjectConversation"
          @command="handleProjectCommand"
          @rename-conversation="handleConversationRename"
          @toggle-pin-conversation="handleToggleConversationPin"
          @archive-conversation="handleArchiveConversation"
        />
      </div>
      </div>
    </div>

    <div class="side-nav-bottom">
      <div
        v-if="profileMenuOpen"
        class="profile-menu"
        role="menu"
        aria-label="个人空间选项"
        @click.stop
      >
        <button type="button" role="menuitem" class="profile-menu__item" @click="openArchivedConversations">
          <el-icon><Box /></el-icon>
          <span>归档对话</span>
        </button>
        <button type="button" role="menuitem" class="profile-menu__item" @click="openSettings">
          <el-icon><Setting /></el-icon>
          <span>设置</span>
        </button>
        <button type="button" role="menuitem" class="profile-menu__item" @click="openUserProfile">
          <el-icon><UserFilled /></el-icon>
          <span>个人资料</span>
        </button>
        <button type="button" role="menuitem" class="profile-menu__item" @click="openDevices">
          <el-icon><Connection /></el-icon>
          <span>我的设备</span>
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
      </div>
      <button
        class="user-profile"
        type="button"
        :title="username || '个人空间'"
        :aria-expanded="profileMenuOpen"
        aria-haspopup="menu"
        @click.stop="profileMenuOpen = !profileMenuOpen"
      >
        <span class="user-avatar">
          <img v-if="avatar" :src="avatar" alt="个人头像" />
          <el-icon v-else><UserFilled /></el-icon>
        </span>
        <span v-show="!appStore.sidebarCollapsed" class="user-copy">
          <strong>{{ username || "个人空间" }}</strong>
          <span>个人空间</span>
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
  Box,
  Connection,
  Moon,
  Plus,
  Search,
  Setting,
  Sunny,
  UserFilled,
} from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useChatStore, type ConversationItem, type ProjectItem } from "@/stores/chat";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { useBrandLogo } from "@/composables/useBrandLogo";
import { useApi } from "@/composables/useApi";
import { useConversationWorkspace } from "@/composables/useConversationWorkspace";
import { isDesktopShell } from "@/runtime/runtime-capabilities";
import SearchModal from "./SearchModal.vue";
import SidebarConversationRow from "./SidebarConversationRow.vue";
import SidebarProjectBlock from "./SidebarProjectBlock.vue";
import { isUINavigationItemActive, useUINavigationRegistry, type UINavigationItem } from "@/ui-runtime/navigationRegistry";
import { prewarmNavigationItem } from "@/ui-runtime/navigationPrewarm";
import { useUIComponentVariant } from "@/ui-runtime/componentRegistry";

const route = useRoute();
const router = useRouter();
const isOnboardingPage = computed(
  () => route.path === "/onboarding" || route.path.startsWith("/onboarding/"),
);
const appStore = useAppStore();
const chatStore = useChatStore();
const extensionUIStore = useExtensionUIStore();
const { groups: navigationGroups, items: navigationItems } = useUINavigationRegistry();
const { style: navigationItemStyle } = useUIComponentVariant("navigationItem");
const { logoUrl } = useBrandLogo();
const { get, post } = useApi();
const { startDraftConversation } = useConversationWorkspace();
defineProps<{
  username?: string;
  avatar?: string;
  theme?: "light" | "dark";
}>();
const emit = defineEmits<{ toggleTheme: [] }>();
const searchModal = ref<InstanceType<typeof SearchModal> | null>(null);
const activeConversationId = computed(() => String(route.query.conversationId || ""));
const expandAllRecent = ref(false);
const profileMenuOpen = ref(false);
let isMounted = false;
const visibleRecentConversations = computed(() =>
  expandAllRecent.value
    ? chatStore.sidebar.recent
    : chatStore.sidebar.recent.slice(0, 5),
);
const pinnedProjects = computed(() =>
  chatStore.sidebar.projects.filter((project) => Boolean(project.pinnedAt)),
);
const regularProjects = computed(() =>
  chatStore.sidebar.projects.filter((project) => !project.pinnedAt),
);

async function handleNewChat() {
  await startDraftConversation();
  await router.push({ path: "/chat" });
}

async function handleSelectConversation(conversation: ConversationItem) {
  chatStore.currentProjectId = conversation.projectId || "";
  chatStore.setConversationId(conversation.id);
  await router.push({ path: "/chat", query: { conversationId: conversation.id } });
}

async function handleConversationRename(conversation: ConversationItem, title: string) {
  await chatStore.renameConversation(conversation.id, title);
}

async function handleToggleConversationPin(conversation: ConversationItem) {
  await chatStore.setConversationPinned(conversation.id, !conversation.pinnedAt);
}

async function handleArchiveConversation(conversation: ConversationItem) {
  const wasActive = activeConversationId.value === conversation.id;
  await chatStore.archiveConversation(conversation.id);
  if (wasActive) {
    await router.replace({ path: "/chat" });
  }
}

async function handleCreateProjectConversation(project: ProjectItem) {
  await startDraftConversation(project.id);
  await router.push({ path: "/chat", query: { projectId: project.id } });
}

async function handleAddProject() {
  if (!window.amitiaDesktop?.selectWorkspaceDirectory) {
    ElMessage.warning("当前环境不支持直接选择本机目录");
    return;
  }
  const selection = await window.amitiaDesktop.selectWorkspaceDirectory();
  if (!selection?.path) return;
  const mount = await post<any>("/api/workspaces/local", {
    name: selection.name || "项目",
    localRoot: selection.path,
    readOnly: false,
  });
  if (!mount?.id) throw new Error("项目根目录注册失败");
  await chatStore.fetchSidebar();
  const existing = chatStore.sidebar.projects.find((item) => item.workspaceId === mount.id);
  if (!existing) {
    await chatStore.createProject({
      name: mount.name || selection.name || "项目",
      workspaceId: mount.id,
      rootUri: mount.rootUri,
    });
  }
  ElMessage.success("项目已添加");
}

async function handleProjectCommand(project: ProjectItem, command: string | number | object) {
  if (command === "newChat") {
    await handleCreateProjectConversation(project);
    return;
  }
  if (command === "rename") {
    const result = await ElMessageBox.prompt("输入新的项目名称", "重命名项目", {
      inputValue: project.name,
      confirmButtonText: "保存",
      cancelButtonText: "取消",
      inputValidator: (value) => Boolean(String(value || "").trim()) || "项目名称不能为空",
    });
    await chatStore.updateProject(project.id, { name: result.value.trim() });
    return;
  }
  if (command === "changeRoot") {
    if (!window.amitiaDesktop?.selectWorkspaceDirectory) {
      ElMessage.warning("当前环境不支持直接选择本机文件夹");
      return;
    }
    const selection = await window.amitiaDesktop.selectWorkspaceDirectory();
    if (!selection?.path) return;
    const mount = await post<any>("/api/workspaces/local", {
      name: selection.name || project.name,
      localRoot: selection.path,
      readOnly: false,
    });
    if (!mount?.id) throw new Error("项目根目录注册失败");
    await chatStore.updateProject(project.id, {
      workspaceId: mount.id,
      rootUri: mount.rootUri,
    });
    ElMessage.success("项目根目录已更新");
    return;
  }
  if (command === "pin") {
    await chatStore.updateProject(project.id, { pinned: !project.pinnedAt });
    return;
  }
  if (command === "open") {
    await handleOpenProject(project);
    return;
  }
  if (command === "remove") {
    await ElMessageBox.confirm("移除项目后，项目内对话会移回最近，磁盘文件夹不会被删除。", "移除项目", {
      type: "warning",
      confirmButtonText: "移除",
      confirmButtonClass: "el-button--danger",
    });
    await chatStore.deleteProject(project.id);
    ElMessage.success("项目已移除，对话已移至最近");
  }
}

async function handleOpenProject(project: ProjectItem) {
  const location = await get<{ kind: string; path?: string; uri?: string }>(
    `/api/web-chat/projects/${encodeURIComponent(project.id)}/location`,
  );
  if (location?.path && window.amitiaDesktop?.openPath) {
    await window.amitiaDesktop.openPath(location.path);
    return;
  }
  ElMessage.warning("当前环境不支持在资源管理器中打开该项目");
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

function openArchivedConversations() {
  profileMenuOpen.value = false;
  router.push("/logs");
}

function openDevices() {
  profileMenuOpen.value = false;
  router.push("/devices");
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

function closeProfileMenu() {
  profileMenuOpen.value = false;
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === "Escape") closeProfileMenu();
}

onMounted(() => {
  isMounted = true;
  void chatStore.fetchSidebar();
  window.addEventListener("click", closeProfileMenu);
  window.addEventListener("keydown", handleKeydown);
});

onUnmounted(() => {
  isMounted = false;
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
.toggle-icon { width: 20px; height: 20px; display: block; color: var(--text-muted); }
.side-nav.is-collapsed .brand-row { justify-content: center; flex-direction: column; padding-bottom: 8px; }
.side-nav.is-collapsed .brand-actions { width: 100%; justify-content: center; }
.sidebar-scroll { min-height: 0; flex: 1 1 auto; overflow-y: auto; overflow-x: hidden; }
.side-menu { border-right: none; background: transparent; width: 100%; margin-bottom: 12px; }
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
.thread-sidebar { display: flex; flex-direction: column; padding: 2px 0 8px; }
.thread-row { display: flex; align-items: center; min-height: 30px; border-radius: 6px; color: var(--text-secondary); }
.thread-row:hover, .thread-row.active { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.thread-row.active { background: var(--workbench-sidebar-active); }
.thread-main { display: flex; align-items: center; gap: 7px; min-width: 0; flex: 1; height: 30px; padding: 0 6px 0 8px; border: 0; background: transparent; color: inherit; cursor: pointer; font: inherit; font-size: 12px; text-align: left; }
.thread-main span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.thread-name-input { min-width: 0; width: 100%; height: 24px; padding: 0 6px; border: 1px solid var(--ac-color-primary); border-radius: 5px; background: var(--surface-bg); color: var(--text-primary); font: inherit; outline: none; }
.thread-delete { display: grid; place-items: center; width: 26px; height: 26px; margin-right: 2px; padding: 0; border: 0; border-radius: 5px; background: transparent; color: var(--text-muted); cursor: pointer; opacity: 0; }
.thread-row:hover .thread-delete, .thread-row.active .thread-delete { opacity: 1; }
.thread-actions { opacity: 0; }
.thread-row:hover .thread-actions, .thread-row.active .thread-actions, .thread-row:focus-within .thread-actions { opacity: 1; }
.thread-delete:hover { background: var(--control-hover-bg); color: var(--danger-color, #d9534f); }
.thread-expand { align-self: flex-start; margin: 2px 0 2px 30px; padding: 3px 7px; border: 0; border-radius: 5px; background: transparent; color: var(--ac-color-primary); cursor: pointer; font: inherit; font-size: 11px; }
.thread-expand:hover { background: var(--workbench-sidebar-hover); }
.thread-section { display: flex; flex-direction: column; gap: 1px; padding: 4px 0; }
.thread-section + .thread-section { margin-top: 6px; border-top: 1px solid var(--surface-border); padding-top: 10px; }
.project-caption { display: flex; align-items: center; justify-content: space-between; }
.section-add, .project-action { display: grid; place-items: center; width: 24px; height: 24px; padding: 0; border: 0; border-radius: 6px; background: transparent; color: var(--text-muted); cursor: pointer; }
.section-add:hover, .project-action:hover { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.section-add { opacity: 0; pointer-events: none; transition: opacity 0.16s ease, background-color 0.16s ease, color 0.16s ease; }
.project-caption:hover .section-add, .project-caption:focus-within .section-add { opacity: 1; pointer-events: auto; }
.project-action:disabled { cursor: not-allowed; opacity: 0.35; }
.project-action-wrap { opacity: 0; }
.project-row:hover .project-action-wrap, .project-row:focus-within .project-action-wrap { opacity: 1; }
.project-block { display: grid; gap: 1px; }
.project-row { display: flex; align-items: center; gap: 2px; min-height: 32px; padding: 0 3px 0 7px; border-radius: 7px; color: var(--text-secondary); }
.project-row:hover, .project-row.active { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.project-main { display: flex; align-items: center; gap: 8px; min-width: 0; flex: 1; height: 32px; padding: 0; border: 0; background: transparent; color: inherit; cursor: pointer; font: inherit; font-size: 12px; text-align: left; }
.project-main span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.project-main small { margin-left: auto; color: var(--text-muted); font-size: 9px; }
.project-threads { display: grid; gap: 1px; padding-left: 18px; }
.thread-item { display: flex; align-items: center; gap: 7px; width: 100%; min-height: 30px; padding: 0 8px; border: 0; border-radius: 6px; background: transparent; color: var(--text-secondary); cursor: pointer; font: inherit; font-size: 12px; text-align: left; }
.thread-item span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.thread-item:hover, .thread-item.active { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.thread-item.active { background: var(--workbench-sidebar-active); }
.project-thread { min-height: 27px; font-size: 11px; }
.thread-empty { padding: 5px 9px; color: var(--text-muted); font-size: 10px; }
.side-nav-bottom { position: relative; flex: 0 0 auto; margin-top: auto; border-top: 1px solid var(--surface-border); padding: 7px 0 8px; }
.profile-menu { position: absolute; right: 0; bottom: calc(100% + 8px); display: grid; gap: 2px; width: 100%; padding: 5px; border: 1px solid var(--surface-border); border-radius: 10px; background: var(--ac-color-surface); }
.profile-menu__item { display: flex; align-items: center; gap: 9px; min-height: 34px; width: 100%; padding: 0 8px; border: 0; border-radius: 7px; background: transparent; color: var(--text-secondary); cursor: pointer; font: inherit; font-size: 12px; text-align: left; transition: background-color 0.18s ease, color 0.18s ease; }
.profile-menu__item:hover, .profile-menu__item:focus-visible { background: var(--workbench-sidebar-hover); color: var(--text-primary); outline: none; }
.profile-menu__item .el-icon { font-size: 15px; }
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
