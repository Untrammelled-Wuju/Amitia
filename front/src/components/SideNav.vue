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
      <el-menu-item index="/chat">
        <el-icon><ChatDotRound /></el-icon>
        <span>聊天</span>
      </el-menu-item>
      <el-sub-menu index="character">
        <template #title>
          <el-icon><UserFilled /></el-icon>
          <span>角色</span>
        </template>
        <el-menu-item index="/character"><el-icon><User /></el-icon><span>角色管理</span></el-menu-item>
        <el-menu-item index="/reminders"><el-icon><Calendar /></el-icon><span>日程提醒</span></el-menu-item>
        <el-menu-item index="/profiles"><el-icon><Avatar /></el-icon><span>用户画像</span></el-menu-item>
        <el-menu-item index="/world-book"><el-icon><Collection /></el-icon><span>世界书</span></el-menu-item>
      </el-sub-menu>
      <el-sub-menu index="memory">
        <template #title>
          <el-icon><Grid /></el-icon>
          <span>记忆</span>
        </template>
        <el-menu-item index="/memory-manager"><el-icon><List /></el-icon><span>记忆总览</span></el-menu-item>
        <el-menu-item index="/episodic"><el-icon><Film /></el-icon><span>情景记忆</span></el-menu-item>
        <el-menu-item index="/graph"><el-icon><Share /></el-icon><span>记忆图谱</span></el-menu-item>
        <el-menu-item index="/memory-timeline"><el-icon><Timer /></el-icon><span>时间线</span></el-menu-item>
        <el-menu-item index="/logs"><el-icon><ChatLineRound /></el-icon><span>聊天记录</span></el-menu-item>
        <el-menu-item index="/import"><el-icon><Download /></el-icon><span>导入记录</span></el-menu-item>
      </el-sub-menu>
      <el-sub-menu index="more">
        <template #title><el-icon><Odometer /></el-icon><span>更多</span></template>
        <el-menu-item index="/dashboard/run"><el-icon><DataLine /></el-icon><span>运行概览</span></el-menu-item>
        <el-menu-item index="/dashboard/data"><el-icon><DataAnalysis /></el-icon><span>运行数据</span></el-menu-item>
        <el-menu-item index="/wechat"><el-icon><Connection /></el-icon>微信连接</el-menu-item>
        <el-menu-item index="/qq"><el-icon><ChatDotSquare /></el-icon>QQ 连接</el-menu-item>
        <el-menu-item index="/emotes"><el-icon><StarFilled /></el-icon><span>表情包管理</span></el-menu-item>
        <el-menu-item index="/extensions"><el-icon><Menu /></el-icon><span>扩展中心</span></el-menu-item>
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
import { computed, onMounted, onUnmounted, ref } from "vue";
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
  Clock,
  User,
  Calendar,
  Avatar,
  Collection,
  List,
  Film,
  Share,
  Timer,
  ChatLineRound,
  Download,
  DataLine,
  DataAnalysis,
  StarFilled,
  Menu,
} from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useBrandLogo } from "@/composables/useBrandLogo";
import { useApi } from "@/composables/useApi";
import SearchModal from "./SearchModal.vue";

const route = useRoute();
const router = useRouter();
const appStore = useAppStore();
const { logoUrl } = useBrandLogo();
const { get, post } = useApi();
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
const recentConversations = ref<any[]>([]);
const activeConversationId = ref(localStorage.getItem("webchat-conv-id") || "");
let isMounted = false;

const statusTitle = computed(() => {
  if (props.modelStatus === "not_configured" || props.modelStatus === "unconfigured") return "模型未配置";
  if (props.modelStatus === "error") return "核心服务异常";
  const channels = [
    props.wechatStatus === "connected" ? "微信" : "",
    props.qqStatus === "connected" || props.qqStatus === "online" ? "QQ" : "",
  ].filter(Boolean);
  return channels.length ? `核心服务正常 · ${channels.join(" / ")} 已连接` : "核心服务正常";
});

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

onMounted(() => {
  isMounted = true;
  void fetchRecentConversations();
  window.addEventListener("amitia:conversation-list-changed", handleConversationListChanged);
});

onUnmounted(() => {
  isMounted = false;
  window.removeEventListener("amitia:conversation-list-changed", handleConversationListChanged);
});
</script>

<style scoped>
.side-nav {
  width: var(--ac-sidebar-width);
  height: 100%;
  background: var(--workbench-sidebar-bg);
  border-right: 1px solid var(--surface-border);
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
.side-menu { border-right: none; background: transparent; flex: 0 1 auto; width: 100%; overflow: visible; }
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
.side-nav-bottom { flex: 0 0 auto; border-top: 1px solid var(--surface-border); padding: 7px 0 8px; }
.side-status { display: flex; align-items: center; gap: 7px; min-height: 25px; padding: 0 9px 5px; color: var(--text-muted); font-size: 10px; overflow: hidden; white-space: nowrap; }
.side-status__dot { width: 6px; height: 6px; flex: 0 0 auto; border-radius: 50%; background: var(--ac-color-success); }
.side-status__dot.is-off { background: var(--ac-color-warning); }
.user-profile, .theme-entry { display: flex; align-items: center; width: 100%; border: 0; border-radius: 7px; background: transparent; color: var(--text-secondary); cursor: pointer; text-align: left; }
.user-profile { gap: 9px; min-height: 38px; padding: 4px 7px; }
.user-profile:hover, .theme-entry:hover { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.user-avatar { display: grid; place-items: center; width: 28px; height: 28px; flex: 0 0 auto; border-radius: 50%; background: color-mix(in srgb, var(--tp-primary) 72%, var(--surface-bg)); color: var(--tp-text-on-primary); font-size: 12px; overflow: hidden; }
.user-avatar img { width: 100%; height: 100%; object-fit: cover; }
.user-copy { min-width: 0; }
.user-copy strong, .user-copy span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.user-copy strong { max-width: 132px; color: var(--text-primary); font-size: 12px; font-weight: 550; }
.user-copy span { margin-top: 1px; color: var(--text-muted); font-size: 10px; }
.theme-entry { gap: 8px; min-height: 31px; margin-top: 2px; padding: 0 9px; font: inherit; font-size: 11px; }
.side-nav.is-collapsed .side-status { justify-content: center; padding-inline: 0; }
.side-nav.is-collapsed .user-profile { justify-content: center; padding-inline: 0; }
</style>
