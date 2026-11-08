<template>
  <aside class="workspace-sidebar" data-amitia-ui="app.workspace-sidebar" @contextmenu.capture="handleBatchContextMenu">
    <div ref="chatSearchRoot" class="workspace-panel-header">
      <div class="workspace-chat-search" :class="{ 'is-open': showChatSearchResults }">
        <el-icon class="workspace-search-icon"><Search /></el-icon>
        <input
          ref="chatSearchInput"
          v-model="chatSearchQuery"
          class="workspace-search-input"
          type="search"
          autocomplete="off"
          aria-label="搜索对话"
          placeholder="搜索对话..."
          @focus="chatSearchOpen = true"
          @keydown="handleChatSearchKeydown"
        />
        <button
          v-if="chatSearchQuery"
          type="button"
          class="workspace-search-clear"
          aria-label="清空对话搜索"
          title="清空搜索"
          @click="clearChatSearch"
        >
          ×
        </button>
      </div>
      <button
        v-if="isChatPage"
        type="button"
        class="workspace-create-button"
        aria-label="新建"
        title="新建对话、群聊、团队或项目"
        data-amitia-ui="workspace.create-menu"
        @click.stop="openCreateMenu"
      >
        <el-icon><Plus /></el-icon>
      </button>
      <div ref="viewSwitchRef" class="workspace-view-switch" role="tablist" aria-label="侧栏内容">
        <button
          ref="chatViewTabRef"
          type="button"
          role="tab"
          :aria-selected="sidebarView === 'chat'"
          :tabindex="sidebarView === 'chat' ? 0 : -1"
          data-amitia-ui="workspace.view.chat"
          @click="switchSidebarView('chat')"
          @keydown.left.prevent="switchSidebarView('chat', true)"
          @keydown.right.prevent="switchSidebarView('project', true)"
        >
          <span>对话</span>
        </button>
        <button
          ref="projectViewTabRef"
          type="button"
          role="tab"
          :aria-selected="sidebarView === 'project'"
          :tabindex="sidebarView === 'project' ? 0 : -1"
          data-amitia-ui="workspace.view.projects"
          @click="switchSidebarView('project')"
          @keydown.left.prevent="switchSidebarView('chat', true)"
          @keydown.right.prevent="switchSidebarView('project', true)"
        >
          <span>项目</span>
        </button>
        <span class="workspace-view-indicator" :style="viewIndicatorStyle" aria-hidden="true"></span>
      </div>

      <div
        v-if="showChatSearchResults"
        class="workspace-chat-search-results"
        data-amitia-ui="workspace.chat-search-results"
        @mousedown.stop
      >
        <div class="workspace-search-status">
          <span>搜索结果</span>
          <span v-if="chatSearchLoading" class="is-loading">正在搜索</span>
          <span v-else>{{ chatSearchResults.length }} 项</span>
        </div>
        <div
          v-for="(item, index) in chatSearchResults"
          :key="item.id"
          class="workspace-search-result"
          :class="{ 'is-active': chatSearchActiveIndex === index }"
          role="option"
          tabindex="0"
          :aria-selected="chatSearchActiveIndex === index"
          @mouseenter="chatSearchActiveIndex = index"
          @click="selectChatSearchResult(item)"
          @keydown.enter.self.prevent="selectChatSearchResult(item)"
          @keydown.space.self.prevent="selectChatSearchResult(item)"
        >
          <button type="button" class="workspace-search-result-main" tabindex="-1">
            <el-icon><ChatDotRound /></el-icon>
            <span class="workspace-search-result-copy">
              <strong>{{ item.title || "新对话" }}</strong>
              <small>{{ chatSearchResultMeta(item) }}</small>
            </span>
          </button>
          <button
            v-if="item.archivedAt"
            type="button"
            class="workspace-search-restore"
            :disabled="restoringChatSearchId === item.id"
            @click.stop="restoreArchivedSearchResult(item)"
          >
            {{ restoringChatSearchId === item.id ? "撤销中" : "撤销归档" }}
          </button>
        </div>
        <div v-if="chatSearchError" class="workspace-search-message is-error">{{ chatSearchError }}</div>
        <div v-else-if="!chatSearchLoading && chatSearchResults.length === 0" class="workspace-search-message">
          没有找到相关对话
        </div>
      </div>
    </div>

    <div class="workspace-scroll">
      <div class="thread-sidebar">
        <div v-if="sidebarView === 'chat' && chatStore.sidebar.pinned.length" class="thread-section">
          <div class="section-caption">置顶</div>
          <SidebarConversationRow
            v-for="conversation in chatStore.sidebar.pinned"
            :key="conversation.id"
            :conversation="conversation"
            :active="conversation.id === activeConversationId"
            :projects="chatStore.sidebar.projects"
            :selecting="batchSelecting" :selected="selectedIds.includes(conversation.id)"
            @select="handleSelectConversation"
            @start-batch="startBatchSelection"
            @rename="handleConversationRename"
            @toggle-pin="handleToggleConversationPin"
            @archive="handleArchiveConversation"
            @move="handleMoveConversation"
            @open="handleSelectConversation"
            @mark-read="handleMarkConversationRead"
            @delete="handleDeleteConversation"
          />
        </div>
        <div v-if="sidebarView === 'project' && pinnedProjects.length" class="thread-section">
          <div class="section-caption">置顶</div>
          <SidebarProjectBlock
            v-for="project in pinnedProjects"
            :key="project.id"
            :project="project"
            :active="project.id === chatStore.currentProjectId"
            :active-conversation-id="activeConversationId"
            :projects="chatStore.sidebar.projects"
            :selecting="batchSelecting" :selected-ids="selectedIds"
            @select="handleSelectConversation"
            @start-batch="startBatchSelection"
            @create-conversation="handleCreateProjectConversation"
            @command="handleProjectCommand"
            @rename-conversation="handleConversationRename"
            @toggle-pin-conversation="handleToggleConversationPin"
            @archive-conversation="handleArchiveConversation"
            @move-conversation="handleMoveConversation"
            @open-conversation="handleSelectConversation"
            @mark-conversation-read="handleMarkConversationRead"
            @delete-conversation="handleDeleteConversation"
          />
        </div>

        <div v-if="sidebarView === 'chat'" class="thread-section">
          <div class="section-caption project-caption" @contextmenu="openSectionContextMenu($event, 'chat')">
            <span>聊天</span>
            <button type="button" class="section-add" :title="batchSelecting ? '退出多选' : '批量选择'" @click="toggleBatchSelection">☑</button>
            <button type="button" class="section-add" data-amitia-ui="app.sidebar.item" title="新建对话" aria-label="新建对话" @click="handleNewChat">
              <el-icon><Plus /></el-icon>
            </button>
          </div>
          <SidebarConversationRow
            v-for="conversation in visibleRecentConversations"
            :key="conversation.id"
            :conversation="conversation"
            :active="conversation.id === activeConversationId"
            :projects="chatStore.sidebar.projects"
            :selecting="batchSelecting" :selected="selectedIds.includes(conversation.id)"
            @select="handleSelectConversation"
            @start-batch="startBatchSelection"
            @rename="handleConversationRename"
            @toggle-pin="handleToggleConversationPin"
            @archive="handleArchiveConversation"
            @move="handleMoveConversation"
            @open="handleSelectConversation"
            @mark-read="handleMarkConversationRead"
            @delete="handleDeleteConversation"
          />
          <button
            v-if="canToggleRecentConversations"
            type="button"
            class="thread-expand"
            @click="toggleRecentConversations"
          >
            {{ hasMoreRecentConversations ? "展开显示" : "收起" }}
          </button>
          <div v-if="chatStore.sidebar.recent.length === 0" class="thread-empty">暂无对话</div>
        </div>

        <div v-if="sidebarView === 'project'" class="thread-section">
          <div class="section-caption project-caption" @contextmenu="openSectionContextMenu($event, 'project')">
            <span>项目</span>
            <button type="button" class="section-add" :title="batchSelecting ? '退出多选' : '批量选择'" @click="toggleBatchSelection">☑</button>
            <button type="button" class="section-add" data-amitia-ui="app.sidebar.item" title="新增项目" aria-label="新增项目" @click="handleAddProject">
              <el-icon><Plus /></el-icon>
            </button>
          </div>
          <SidebarProjectBlock
            v-for="project in regularProjects"
            :key="project.id"
            :project="project"
            :active="project.id === chatStore.currentProjectId"
            :active-conversation-id="activeConversationId"
            :projects="chatStore.sidebar.projects"
            :selecting="batchSelecting" :selected-ids="selectedIds"
            @select="handleSelectConversation"
            @start-batch="startBatchSelection"
            @create-conversation="handleCreateProjectConversation"
            @command="handleProjectCommand"
            @rename-conversation="handleConversationRename"
            @toggle-pin-conversation="handleToggleConversationPin"
            @archive-conversation="handleArchiveConversation"
            @move-conversation="handleMoveConversation"
            @open-conversation="handleSelectConversation"
            @mark-conversation-read="handleMarkConversationRead"
            @delete-conversation="handleDeleteConversation"
          />
          <div v-if="chatStore.sidebar.projects.length === 0" class="thread-empty">暂无项目</div>
        </div>
      </div>
    </div>
    <el-dialog
      v-model="projectDialogOpen"
      data-amitia-ui="project.create-dialog"
      title="新建项目"
      width="min(520px, calc(100vw - 32px))"
      align-center
      destroy-on-close
      :close-on-click-modal="!projectCreating && !projectSelectingFolder"
      :close-on-press-escape="!projectCreating && !projectSelectingFolder"
      :show-close="!projectCreating && !projectSelectingFolder"
      @closed="resetProjectDialog"
    >
      <el-form label-position="top" @submit.prevent="handleCreateProject">
        <el-form-item label="项目名称" required>
          <el-input
            v-model="projectDraftName"
            class="project-create-name"
            maxlength="120"
            placeholder="输入项目组名称"
            :disabled="projectCreating"
          />
        </el-form-item>
        <el-form-item label="项目文件夹" required>
          <div
            class="project-folder-dropzone"
            :class="{ 'is-dragging': projectDragActive }"
            @dragenter.prevent="handleProjectFolderDragEnter"
            @dragover.prevent="handleProjectFolderDragOver"
            @dragleave.prevent="handleProjectFolderDragLeave"
            @drop.prevent="handleProjectFolderDrop"
          >
            <button
              type="button"
              class="project-folder-picker"
              aria-label="选择项目文件夹"
              :disabled="projectCreating || projectSelectingFolder"
              @click="handleChooseProjectFolder"
            >
              选择文件夹
            </button>
            <span class="project-folder-drop-copy">{{ projectSelectingFolder ? "正在添加文件夹..." : "或将文件夹拖拽到这里" }}</span>
          </div>
          <div v-if="projectDraftWorkspaces.length" class="project-folder-list">
            <div
              v-for="(workspace, index) in projectDraftWorkspaces"
              :key="workspace.id"
              class="project-folder-item"
            >
              <span class="project-folder-role" :class="{ 'is-primary': index === 0 }">{{ index === 0 ? "主工作区" : "子工作区" }}</span>
              <el-icon><FolderOpened /></el-icon>
              <span class="project-folder-name">{{ workspace.name }}</span>
              <button
                type="button"
                class="project-folder-clear"
                :aria-label="`移除文件夹 ${workspace.name}`"
                :disabled="projectCreating || projectSelectingFolder"
                @click="removeProjectWorkspace(index)"
              >
                移除
              </button>
            </div>
          </div>
          <p class="project-folder-hint">每个项目只能有一个主工作区，第一个添加的文件夹作为主工作区，之后添加的均为子工作区。</p>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button :disabled="projectCreating || projectSelectingFolder" @click="projectDialogOpen = false">取消</el-button>
        <el-button type="primary" :loading="projectCreating" :disabled="projectCreating || projectSelectingFolder" @click="handleCreateProject">创建项目</el-button>
      </template>
    </el-dialog>
    <el-dialog
      v-model="projectConversationDialogVisible"
      data-amitia-ui="project.conversation-dialog"
      title="新建对话"
      width="min(480px, calc(100vw - 32px))"
      align-center
      destroy-on-close
      @closed="projectConversationTargetId = ''"
    >
      <p class="project-conversation-dialog-copy">选择要为哪个项目创建对话。</p>
      <div class="project-conversation-list" role="listbox" aria-label="选择项目">
        <button
          v-for="project in chatStore.sidebar.projects"
          :key="project.id"
          type="button"
          class="project-conversation-choice"
          :class="{ 'is-selected': projectConversationTargetId === project.id }"
          role="option"
          :aria-selected="projectConversationTargetId === project.id"
          :disabled="!project.available || project.readOnly"
          @click="projectConversationTargetId = project.id"
        >
          <el-icon><FolderOpened /></el-icon>
          <span class="project-conversation-choice-copy">
            <strong>{{ project.name }}</strong>
            <small>{{ project.readOnly ? "只读项目" : project.available ? `${project.workspaces.length} 个工作区` : project.statusReason || "项目不可用" }}</small>
          </span>
        </button>
        <div v-if="chatStore.sidebar.projects.length === 0" class="project-conversation-empty">暂无项目，请先新建项目。</div>
      </div>
      <template #footer>
        <el-button @click="projectConversationDialogVisible = false">取消</el-button>
        <el-button type="primary" :disabled="!projectConversationTargetId" @click="createConversationInSelectedProject">创建对话</el-button>
      </template>
    </el-dialog>
    <NewGroupChatDialog
      v-model:visible="newGroupDialogVisible"
      :characters="owned.roles.value"
      :default-role-id="newGroupDefaultRoleId"
      :project-required="sidebarView === 'project'"
      @created="handleGroupCreated"
    />
    <TeamCreateDialog v-model:visible="teamCreateDialogVisible" @created="handleTeamCreated" />
  </aside>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ChatDotRound, FolderOpened, Plus, Search } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  openContextMenu,
  projectContextMenuAccessKeys,
  type ContextMenuAction,
} from "./context-menu/contextMenuStore";
import { conversationChannel, useChatStore, type ConversationItem, type ProjectItem, type ProjectWorkspace } from "@/stores/chat";
import { useApi } from "@/composables/useApi";
import NewConversationIcon from "@/components/icons/NewConversationIcon.vue";
import NewGroupChatDialog from "@/components/NewGroupChatDialog.vue";
import TeamCreateDialog from "@/components/TeamCreateDialog.vue";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import type { OwnedGroupState } from "@/runtime/device-owned-chat";
import type { Team } from "@/api/role-types";
import { useConversationDisclosure } from "@/composables/useConversationDisclosure";
import { useConversationWorkspace } from "@/composables/useConversationWorkspace";
import { isDesktopShell } from "@/runtime/runtime-capabilities";
import { originalErrorMessage } from "@/runtime/original-error";
import { ownedProjectReference } from "@/runtime/owned-project-reference";
import SidebarConversationRow from "./SidebarConversationRow.vue";
import SidebarProjectBlock from "./SidebarProjectBlock.vue";

const route = useRoute();
const router = useRouter();
const chatStore = useChatStore();
const owned = useDeviceOwnedConversation();
const { get } = useApi();
const {
  startDraftConversation,
  pickProjectWorkspace,
  registerProjectWorkspaces,
  createProjectFromWorkspaces,
  applySnapshotWorkspace,
  addWorkspaceToProject,
  removeWorkspaceFromProject,
  setPrimaryWorkspace,
} = useConversationWorkspace();
const chatSearchRoot = ref<HTMLElement | null>(null);
const chatSearchInput = ref<HTMLInputElement | null>(null);
const viewSwitchRef = ref<HTMLElement | null>(null);
const chatViewTabRef = ref<HTMLButtonElement | null>(null);
const projectViewTabRef = ref<HTMLButtonElement | null>(null);
const viewIndicatorStyle = ref<Record<string, string>>({});
const chatSearchQuery = ref("");
const chatSearchOpen = ref(false);
const chatSearchLoading = ref(false);
const chatSearchError = ref("");
const chatSearchActiveIndex = ref(0);
const restoringChatSearchId = ref("");
const remoteChatSearchResults = ref<ConversationItem[]>([]);
const remoteChatSearchIds = ref(new Set<string>());
type SidebarView = "chat" | "project";
const SIDEBAR_VIEW_STORAGE_KEY = "uai-workspace-sidebar-view";
function readSidebarView(): SidebarView {
  try {
    return localStorage.getItem(SIDEBAR_VIEW_STORAGE_KEY) === "project" ? "project" : "chat";
  } catch (error) {
    console.error("[original-error]", error);
    ElMessage.error(originalErrorMessage(error));
    return "chat";
  }
}
const sidebarView = ref<SidebarView>(readSidebarView());
let viewSwitchResizeObserver: ResizeObserver | undefined;
let chatSearchTimer: number | null = null;
let chatSearchRequest = 0;
const projectDialogOpen = ref(false);
const projectConversationDialogVisible = ref(false);
const projectConversationTargetId = ref("");
const newGroupDialogVisible = ref(false);
const teamCreateDialogVisible = ref(false);
const projectCreating = ref(false);
const projectSelectingFolder = ref(false);
const projectDraftName = ref("");
const projectDraftWorkspaces = ref<ProjectWorkspace[]>([]);
const projectNameInitialized = ref(false);
const projectDragActive = ref(false);
const projectDragDepth = ref(0);
const activeConversationId = computed(() => String(route.query.conversationId || ""));
const isChatPage = computed(() => route.path === "/chat");
const newGroupDefaultRoleId = computed(() => String(route.query.characterId || localStorage.getItem(`webchat-char-id:${owned.coreId.value}:${owned.roleOwnerId.value}`) || owned.selectInitialRole()));
const {
  visible: visibleRecentConversations,
  hasMore: hasMoreRecentConversations,
  canToggle: canToggleRecentConversations,
  toggle: toggleRecentConversations,
} = useConversationDisclosure(() => chatStore.sidebar.recent);
const pinnedProjects = computed(() =>
  chatStore.sidebar.projects.filter((project) => Boolean(project.pinnedAt)),
);
const regularProjects = computed(() =>
  chatStore.sidebar.projects.filter((project) => !project.pinnedAt),
);
const sidebarConversationRows = computed(() => {
  const rows = new Map<string, ConversationItem>();
  for (const conversation of [
    ...chatStore.sidebar.pinned,
    ...chatStore.sidebar.recent,
    ...chatStore.sidebar.projects.flatMap((project) => project.conversations),
  ]) {
    if (conversation.id) rows.set(conversation.id, conversation);
  }
  return [...rows.values()];
});
const projectNames = computed(() => new Map(chatStore.sidebar.projects.map((project) => [project.id, project.name])));
const showChatSearchResults = computed(() => chatSearchOpen.value && Boolean(chatSearchQuery.value.trim()));
const chatSearchResults = computed(() => {
  const query = chatSearchQuery.value.trim().toLocaleLowerCase();
  if (!query) return [];
  const rows = new Map<string, ConversationItem>();
  for (const conversation of sidebarConversationRows.value) {
    const projectName = projectNames.value.get(conversation.projectId) || "";
    const lastMessage = conversation.lastMessagePreview || conversation.lastMessage || "";
    if (![conversation.title, projectName, lastMessage].some((value) => String(value || "").toLocaleLowerCase().includes(query))) continue;
    rows.set(conversation.id, conversation);
  }
  for (const conversation of remoteChatSearchResults.value) rows.set(conversation.id, conversation);
  return [...rows.values()]
    .sort((left, right) => {
      const archivedOrder = Number(Boolean(left.archivedAt)) - Number(Boolean(right.archivedAt));
      if (archivedOrder !== 0) return archivedOrder;
      const leftTitle = String(left.title || "").toLocaleLowerCase();
      const rightTitle = String(right.title || "").toLocaleLowerCase();
      const leftStarts = leftTitle.startsWith(query) ? 0 : 1;
      const rightStarts = rightTitle.startsWith(query) ? 0 : 1;
      return leftStarts - rightStarts || String(right.updatedAt || "").localeCompare(String(left.updatedAt || ""));
    })
    .slice(0, 30);
});

function normalizeSearchResult(value: Record<string, any>): ConversationItem | null {
  const id = String(value.id || "").trim();
  if (!id) return null;
  const channel = conversationChannel(value);
  return {
    ...value,
    id,
    title: String(value.title || value.name || "新对话"),
    projectId: value.projectId && value.ownerId ? ownedProjectReference(String(value.ownerId), String(value.projectId)) : "",
    channel,
    source: String(value.source || "device-mesh"),
    messageCount: Number(value.messageCount || 0),
    createdAt: String(value.createdAt || ""),
    updatedAt: String(value.updatedAt || ""),
  };
}

async function loadRemoteChatSearch(query: string) {
  const ticket = ++chatSearchRequest;
  const role = String(route.query.characterId || localStorage.getItem(`webchat-char-id:${owned.coreId.value}:${owned.roleOwnerId.value}`) || owned.selectInitialRole());
  if (!role) {
    remoteChatSearchResults.value = [];
    remoteChatSearchIds.value = new Set();
    chatSearchLoading.value = false;
    return;
  }
  try {
    if (!owned.enabled.value && !(await owned.refresh())) throw new Error(owned.notice.value || "设备对话服务尚未就绪");
    const values = await owned.conversations(role, query);
    if (ticket !== chatSearchRequest || query !== chatSearchQuery.value.trim()) return;
    const normalized = values
      .map((value) => normalizeSearchResult(value))
      .filter((value): value is ConversationItem => Boolean(value));
    remoteChatSearchResults.value = normalized;
    remoteChatSearchIds.value = new Set(normalized.map((item) => item.id));
    chatSearchActiveIndex.value = Math.min(chatSearchActiveIndex.value, Math.max(0, chatSearchResults.value.length - 1));
  } catch (error) {
    if (ticket !== chatSearchRequest) return;
    console.error("[original-error]", error);
    chatSearchError.value = originalErrorMessage(error);
    remoteChatSearchResults.value = [];
    remoteChatSearchIds.value = new Set();
  } finally {
    if (ticket === chatSearchRequest) chatSearchLoading.value = false;
  }
}

function clearChatSearch() {
  chatSearchQuery.value = "";
  chatSearchOpen.value = false;
  chatSearchActiveIndex.value = 0;
  chatSearchLoading.value = false;
  chatSearchError.value = "";
  remoteChatSearchResults.value = [];
  remoteChatSearchIds.value = new Set();
  chatSearchInput.value?.focus();
}

function chatSearchResultMeta(conversation: ConversationItem) {
  const projectName = projectNames.value.get(conversation.projectId) || "";
  const type = String((conversation as any).conversationType || "").includes("group") ? "群聊" : "对话";
  const matchedInMessages = remoteChatSearchIds.value.has(conversation.id) && !String(conversation.title || "").toLocaleLowerCase().includes(chatSearchQuery.value.trim().toLocaleLowerCase());
  return [type, projectName, conversation.archivedAt ? "已归档" : matchedInMessages ? "匹配聊天记录" : conversation.lastMessagePreview || conversation.lastMessage || ""]
    .filter(Boolean)
    .join(" · ");
}

async function restoreArchivedSearchResult(conversation: ConversationItem) {
  if (restoringChatSearchId.value) return;
  restoringChatSearchId.value = conversation.id;
  try {
    await chatStore.restoreConversation(conversation.id);
    remoteChatSearchResults.value = remoteChatSearchResults.value.map((item) =>
      item.id === conversation.id ? { ...item, archivedAt: null } : item,
    );
    ElMessage.success("已撤销归档");
  } catch (error) {
    console.error("[original-error]", error);
    ElMessage.error(originalErrorMessage(error));
  } finally {
    restoringChatSearchId.value = "";
  }
}

async function selectChatSearchResult(conversation: ConversationItem) {
  clearChatSearch();
  await handleSelectConversation(conversation);
}

function handleChatSearchKeydown(event: KeyboardEvent) {
  if (event.key === "Escape") {
    event.preventDefault();
    clearChatSearch();
    return;
  }
  if (!chatSearchResults.value.length) return;
  if (event.key === "ArrowDown") {
    event.preventDefault();
    chatSearchActiveIndex.value = (chatSearchActiveIndex.value + 1) % chatSearchResults.value.length;
    return;
  }
  if (event.key === "ArrowUp") {
    event.preventDefault();
    chatSearchActiveIndex.value = (chatSearchActiveIndex.value - 1 + chatSearchResults.value.length) % chatSearchResults.value.length;
    return;
  }
  if (event.key === "Enter") {
    event.preventDefault();
    const conversation = chatSearchResults.value[chatSearchActiveIndex.value];
    if (conversation) void selectChatSearchResult(conversation);
  }
}

function handleDocumentPointerDown(event: PointerEvent) {
  if (!chatSearchRoot.value?.contains(event.target as Node)) chatSearchOpen.value = false;
}

function openCreateMenu(event: MouseEvent) {
  const anchor = event.currentTarget as HTMLElement | null;
  if (!anchor) return;
  const rect = anchor.getBoundingClientRect();
  openContextMenu({
    x: Math.max(8, rect.right - 188),
    y: rect.bottom + 6,
    items: [
      { id: "new-chat", label: "新对话", icon: NewConversationIcon, accessKey: "N", action: handleCreateConversation },
      { id: "new-group", label: "新群聊", icon: ChatDotRound, accessKey: "G", action: handleNewGroupChat },
      { id: "new-team", label: "新团队", icon: ChatDotRound, accessKey: "T", action: () => { teamCreateDialogVisible.value = true; } },
      { id: "new-project", label: "新项目", icon: FolderOpened, accessKey: "P", action: handleAddProject },
    ],
  });
}

function switchSidebarView(value: SidebarView, focus = false) {
  sidebarView.value = value;
  try {
    localStorage.setItem(SIDEBAR_VIEW_STORAGE_KEY, value);
  } catch (error) {
    console.error("[original-error]", error);
    ElMessage.error(originalErrorMessage(error));
  }
  if (focus) {
    requestAnimationFrame(() => {
      chatSearchRoot.value
        ?.closest(".workspace-sidebar")
        ?.querySelector<HTMLElement>(`[data-amitia-ui="workspace.view.${value === "chat" ? "chat" : "projects"}"]`)
        ?.focus();
    });
  }
}

function syncViewIndicator() {
  const container = viewSwitchRef.value;
  const activeTab = sidebarView.value === "chat" ? chatViewTabRef.value : projectViewTabRef.value;
  if (!container || !activeTab) return;
  const width = Math.max(18, Math.min(28, activeTab.offsetWidth - 24));
  const center = activeTab.offsetLeft + activeTab.offsetWidth / 2;
  viewIndicatorStyle.value = {
    left: `${center - width / 2}px`,
    width: `${width}px`,
  };
}

watch(chatSearchQuery, (value) => {
  if (chatSearchTimer !== null) window.clearTimeout(chatSearchTimer);
  chatSearchTimer = null;
  chatSearchError.value = "";
  chatSearchActiveIndex.value = 0;
  const query = value.trim();
  if (!query) {
    chatSearchRequest++;
    chatSearchLoading.value = false;
    remoteChatSearchResults.value = [];
    remoteChatSearchIds.value = new Set();
    return;
  }
  chatSearchOpen.value = true;
  remoteChatSearchResults.value = [];
  remoteChatSearchIds.value = new Set();
  chatSearchLoading.value = true;
  chatSearchTimer = window.setTimeout(() => {
    chatSearchTimer = null;
    void loadRemoteChatSearch(query);
  }, 220);
});
watch(sidebarView, () => {
  void nextTick(syncViewIndicator);
});

function refreshSidebarWhenVisible() {
  if (document.visibilityState === "visible") void chatStore.fetchSidebar().catch((error) => {
    console.error("[original-error]", error);
  });
}

async function handleNewChat() {
  try {
    await startDraftConversation();
    await router.push({ path: "/chat" });
    window.dispatchEvent(new CustomEvent("amitia:draft-created"));
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "新建对话失败");
  }
}

async function handleNewGroupChat() {
  try {
    if (!(await owned.refresh())) throw new Error("设备对话服务尚未就绪");
    if (owned.roles.value.length < 2) {
      ElMessage.warning("至少需要两个可用角色才能创建群聊");
      return;
    }
    newGroupDialogVisible.value = true;
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "无法准备新建群聊");
  }
}

function handleCreateConversation() {
  if (sidebarView.value === "chat") {
    void handleNewChat();
    return;
  }
  if (chatStore.sidebar.projects.length === 0) {
    ElMessage.warning("暂无项目，请先新建项目");
    handleAddProject();
    return;
  }
  projectConversationTargetId.value = "";
  projectConversationDialogVisible.value = true;
}

async function handleGroupCreated(group: OwnedGroupState, baseRoleId: string, projectId = "") {
  chatStore.setConversationId(group.conversationId);
  await chatStore.fetchSidebar().catch((error) => {
  console.error("[original-error]", error);
  return undefined;
});
  const project = projectId
    ? chatStore.sidebar.projects.find((item) => item.resourceId === projectId)
    : null;
  chatStore.currentProjectId = project?.id || "";
  await router.push({ path: "/chat", query: { conversationId: group.conversationId, characterId: baseRoleId } });
}

function handleTeamCreated(team: Team) {
  ElMessage.success(`团队「${team.name}」已创建`);
}

async function handleSelectConversation(conversation: ConversationItem) {
  if (batchSelecting.value) {
    selectedIds.value = selectedIds.value.includes(conversation.id) ? selectedIds.value.filter((id) => id !== conversation.id) : [...selectedIds.value, conversation.id];
    return;
  }
  chatStore.currentProjectId = conversation.projectId || "";
  chatStore.setConversationId(conversation.id);
  await router.push({ path: "/chat", query: { conversationId: conversation.id } });
}

const batchSelecting = ref(false);
const selectedIds = ref<string[]>([]);
const batchRows = computed(() => [...chatStore.sidebar.pinned, ...chatStore.sidebar.recent, ...chatStore.sidebar.projects.flatMap((project) => project.conversations)]);
function toggleBatchSelection() { batchSelecting.value = !batchSelecting.value; selectedIds.value = []; }
function startBatchSelection(conversation: ConversationItem) { batchSelecting.value = true; selectedIds.value = [conversation.id]; }
function openSectionContextMenu(event: MouseEvent, section: "chat" | "project") {
  if (!isDesktopShell()) return;
  event.preventDefault();
  event.stopPropagation();
  openContextMenu({ x: event.clientX, y: event.clientY, title: section === "chat" ? "聊天" : "项目", items: [
    { id: "select", label: batchSelecting.value ? "取消多选" : "多选", accessKey: "S", action: toggleBatchSelection },
    { id: "add", label: section === "chat" ? "新建对话" : "新增项目", accessKey: "N", action: section === "chat" ? handleNewChat : handleAddProject },
  ] });
}
async function batchOperation(action: string, projectId = "") {
  if (action === "delete") {
    const confirmed = await ElMessageBox.confirm(`确定删除选中的 ${selectedIds.value.length} 个对话吗？`, "批量删除", { type: "warning" }).then(() => true).catch(handleProjectDialogDismiss);
    if (!confirmed) return;
  }
  const failures: string[] = [];
  const succeeded: string[] = [];
  for (const id of selectedIds.value) {
    const row = batchRows.value.find((item) => item.id === id);
    if (!row) { failures.push(`${id}: 对话不存在`); continue; }
    try {
      const changes = action === "archive" ? { archived: true } : action === "pin" ? { pinned: true } : action === "read" ? { lastReadTurnSequence: Number(row.lastTerminalTurnSequence || 0) } : action === "move" ? { projectId } : {};
      await owned.edit("conversation", id, changes, { deleted: action === "delete", characterId: row.characterId });
      succeeded.push(id);
    } catch (error) { failures.push(`${row.title || id}: ${error instanceof Error ? error.stack || error.message : String(error)}`); }
  }
  selectedIds.value = selectedIds.value.filter((id) => !succeeded.includes(id));
  await chatStore.fetchSidebar();
  if ((action === "delete" || action === "archive") && succeeded.includes(activeConversationId.value)) await router.replace({ path: "/chat" });
  if (failures.length) await ElMessageBox.alert(failures.join("\n\n"), `已完成 ${succeeded.length} 项，失败 ${failures.length} 项`, { customClass: "batch-errors" });
  else ElMessage.success(`已完成 ${succeeded.length} 项`);
}
function handleBatchContextMenu(event: MouseEvent) {
  if (!batchSelecting.value || !(event.target as Element).closest(".thread-row")) return;
  event.preventDefault(); event.stopPropagation();
  const disabled = selectedIds.value.length === 0;
  const selectableIds = [...new Set(batchRows.value.map((row) => row.id))];
  const allSelected = selectableIds.length > 0 && selectableIds.every((id) => selectedIds.value.includes(id));
  const items: ContextMenuAction[] = [
    { id: "all", label: allSelected ? "取消全选" : "全选", accessKey: "A", disabled: selectableIds.length === 0, action: () => { selectedIds.value = allSelected ? selectedIds.value.filter((id) => !selectableIds.includes(id)) : [...new Set([...selectedIds.value, ...selectableIds])]; } },
    ...[["archive", "归档", "R"], ["read", "标为已读", "M"], ["pin", "置顶", "P"]].map(([id, label, accessKey]) => ({ id, label, accessKey, disabled, action: () => batchOperation(id) })),
    { id: "move", label: "移动到项目", accessKey: "V", disabled, children: [{ id: "ungrouped", label: "移出项目", accessKey: "H", action: () => batchOperation("move") }, ...chatStore.sidebar.projects.filter((project) => !project.readOnly).map((project, index) => ({ id: project.id, label: project.name, accessKey: projectContextMenuAccessKeys[index], action: () => batchOperation("move", project.id) }))] },
    { id: "delete", label: "删除", accessKey: "D", danger: true, disabled, action: () => batchOperation("delete") },
    { id: "cancel", label: "取消多选", accessKey: "C", action: toggleBatchSelection },
  ];
  openContextMenu({ x: event.clientX, y: event.clientY, title: `已选择 ${selectedIds.value.length} 个对话`, items });
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
  if (wasActive) await router.replace({ path: "/chat" });
}

async function handleMarkConversationRead(conversation: ConversationItem) {
  await chatStore.markConversationRead(
    conversation.id,
    Number(conversation.lastTerminalTurnSequence || 0),
  );
}

async function handleDeleteConversation(conversation: ConversationItem) {
  const result = await ElMessageBox.confirm(
    "删除后无法恢复，对话消息和工作区关联都会一并删除。",
    "删除对话",
    {
      type: "warning",
      confirmButtonText: "删除",
      cancelButtonText: "取消",
      confirmButtonClass: "el-button--danger",
    },
  ).catch(handleProjectDialogDismiss);
  if (!result) return;
  const wasActive = activeConversationId.value === conversation.id;
  await chatStore.deleteConversation(conversation.id);
  if (wasActive) await router.replace({ path: "/chat" });
  ElMessage.success("对话已删除");
}

async function handleMoveConversation(conversation: ConversationItem, projectId: string) {
  await chatStore.moveConversation(conversation.id, projectId);
  if (activeConversationId.value === conversation.id) applySnapshotWorkspace(null, projectId);
}

async function handleCreateProjectConversation(project: ProjectItem): Promise<boolean> {
  if (project.readOnly) {
    ElMessage.warning("该项目为只读");
    return false;
  }
  try {
    await startDraftConversation(project.id);
    await router.push({ path: "/chat", query: { projectId: project.id } });
    window.dispatchEvent(new CustomEvent("amitia:draft-created"));
    return true;
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "新建项目对话失败");
    return false;
  }
}

async function createConversationInSelectedProject() {
  const project = chatStore.sidebar.projects.find((item) => item.id === projectConversationTargetId.value);
  if (!project) return;
  if (await handleCreateProjectConversation(project)) projectConversationDialogVisible.value = false;
}

function handleProjectDialogDismiss(error: unknown): undefined {
  if (error === "cancel" || error === "close") return;
  throw error;
}

function resetProjectDialog() {
  projectDraftName.value = "";
  projectDraftWorkspaces.value = [];
  projectCreating.value = false;
  projectSelectingFolder.value = false;
  projectNameInitialized.value = false;
  projectDragActive.value = false;
  projectDragDepth.value = 0;
}

function handleAddProject() {
  resetProjectDialog();
  projectDialogOpen.value = true;
}

async function handleChooseProjectFolder() {
  projectSelectingFolder.value = true;
  try {
    const workspace = await pickProjectWorkspace();
    if (!workspace) return;
    appendProjectWorkspaces([workspace]);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "选择文件夹失败");
  } finally {
    projectSelectingFolder.value = false;
  }
}

function appendProjectWorkspaces(workspaces: ProjectWorkspace[]) {
  let changed = false;
  for (const workspace of workspaces) {
    if (!workspace.id || !workspace.rootUri || projectDraftWorkspaces.value.some((item) => item.id === workspace.id)) continue;
    projectDraftWorkspaces.value.push(workspace);
    changed = true;
  }
  if (!changed || projectNameInitialized.value) return;
  if (!projectDraftName.value.trim()) projectDraftName.value = projectDraftWorkspaces.value[0].name;
  projectNameInitialized.value = true;
}

function removeProjectWorkspace(index: number) {
  projectDraftWorkspaces.value.splice(index, 1);
}

function handleProjectFolderDragEnter() {
  projectDragDepth.value += 1;
  projectDragActive.value = true;
}

function handleProjectFolderDragOver(event: DragEvent) {
  if (event.dataTransfer) event.dataTransfer.dropEffect = "copy";
  projectDragActive.value = true;
}

function handleProjectFolderDragLeave() {
  projectDragDepth.value = Math.max(0, projectDragDepth.value - 1);
  projectDragActive.value = projectDragDepth.value > 0;
}

function projectFolderPathsFromDrop(event: DragEvent): string[] {
  const files = Array.from(event.dataTransfer?.files || []);
  const paths = files.flatMap((file) => {
    try {
      const path = window.amitiaDesktop?.getPathForFile?.(file);
      return path ? [path] : [];
    } catch (error) {
  console.error("[original-error]", error);
return [];
    }
  });
  if (paths.length) return [...new Set(paths)];
  const uriList = event.dataTransfer?.getData("text/uri-list") || "";
  return [...new Set(uriList
    .split(/\r?\n/)
    .filter((item) => item.startsWith("file://"))
    .map((item) => decodeURIComponent(new URL(item).pathname)))];
}

async function handleProjectFolderDrop(event: DragEvent) {
  projectDragDepth.value = 0;
  projectDragActive.value = false;
  const paths = projectFolderPathsFromDrop(event);
  if (!paths.length) {
    ElMessage.warning("请拖拽本地文件夹");
    return;
  }
  projectSelectingFolder.value = true;
  try {
    const workspaces = await registerProjectWorkspaces(paths);
    appendProjectWorkspaces(workspaces);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "添加文件夹失败");
  } finally {
    projectSelectingFolder.value = false;
  }
}

async function handleCreateProject() {
  if (projectCreating.value || projectSelectingFolder.value) return;
  if (!projectDraftWorkspaces.value.length) {
    ElMessage.warning("请选择一个文件夹");
    return;
  }
  if (!projectDraftName.value.trim()) {
    ElMessage.warning("请输入项目名称");
    return;
  }
  projectCreating.value = true;
  try {
    await createProjectFromWorkspaces(projectDraftName.value, projectDraftWorkspaces.value);
    projectDialogOpen.value = false;
    switchSidebarView("project");
    ElMessage.success("项目已添加");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "创建项目失败");
  } finally {
    projectCreating.value = false;
  }
}

async function handleProjectCommand(project: ProjectItem, command: string | number | object) {
  if (project.readOnly) {
    ElMessage.warning("旧项目为只读，请在原设备管理");
    return;
  }
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
    }).catch(handleProjectDialogDismiss);
    if (!result) return;
    await chatStore.updateProject(project.id, { name: result.value.trim() }, project);
    return;
  }
  if (command === "addWorkspace") {
    try {
      const workspace = await addWorkspaceToProject(project);
      if (workspace) ElMessage.success(project.workspaces.length ? "子工作区已添加" : "主工作区已添加");
    } catch (error) {
      ElMessage.error(error instanceof Error ? error.message : "添加工作区失败");
    }
    return;
  }
  if (typeof command === "string" && command.startsWith("primary:")) {
    try {
      await setPrimaryWorkspace(project, command.slice("primary:".length));
      ElMessage.success("主工作区已切换");
    } catch (error) {
      ElMessage.error(error instanceof Error ? error.message : "切换主工作区失败");
    }
    return;
  }
  if (typeof command === "string" && command.startsWith("removeWorkspace:")) {
    try {
      const workspaceId = command.slice("removeWorkspace:".length);
      const workspace = project.workspaces.find((item) => item.id === workspaceId);
      await removeWorkspaceFromProject(project, workspaceId);
      ElMessage.success(`已移除工作区${workspace ? `：${workspace.name}` : ""}`);
    } catch (error) {
      ElMessage.error(error instanceof Error ? error.message : "移除工作区失败");
    }
    return;
  }
  if (command === "pin") {
    await chatStore.updateProject(project.id, { pinned: !project.pinnedAt }, project);
    return;
  }
  if (command === "open") {
    await handleOpenProject(project);
    return;
  }
  if (command === "remove") {
    const result = await ElMessageBox.confirm("项目只会从侧边栏隐藏，项目、工作区绑定、对话和消息都会保留。", "从侧边栏移除", {
      type: "warning",
      confirmButtonText: "移除",
    }).catch(handleProjectDialogDismiss);
    if (!result) return;
    await chatStore.hideProject(project.id, project);
    ElMessage.success("项目已从侧边栏移除");
  }
}

async function handleOpenProject(project: ProjectItem) {
  const primary = project.workspaces.find((workspace) => workspace.id === project.primaryWorkspaceId) || project.workspaces[0];
  if (!primary) {
    ElMessage.warning("项目没有可用工作区");
    return;
  }
  const location = await get<{ kind: string; path?: string; uri?: string }>(
    `/api/workspaces/${encodeURIComponent(primary.id)}/location`,
  );
  if (location?.path && window.amitiaDesktop?.openPath) {
    await window.amitiaDesktop.openPath(location.path);
    return;
  }
  ElMessage.warning("当前环境不支持在资源管理器中打开该项目");
}

onMounted(() => {
  document.addEventListener("pointerdown", handleDocumentPointerDown);
  if (typeof ResizeObserver !== "undefined") {
    viewSwitchResizeObserver = new ResizeObserver(syncViewIndicator);
    if (viewSwitchRef.value) viewSwitchResizeObserver.observe(viewSwitchRef.value);
    if (chatViewTabRef.value) viewSwitchResizeObserver.observe(chatViewTabRef.value);
    if (projectViewTabRef.value) viewSwitchResizeObserver.observe(projectViewTabRef.value);
  }
  void nextTick(syncViewIndicator);
  void chatStore.fetchSidebar().catch((error) => {
  console.error("[original-error]", error);
  return undefined;
});
  window.addEventListener("focus", refreshSidebarWhenVisible);
  document.addEventListener("visibilitychange", refreshSidebarWhenVisible);
});

onUnmounted(() => {
  document.removeEventListener("pointerdown", handleDocumentPointerDown);
  viewSwitchResizeObserver?.disconnect();
  viewSwitchResizeObserver = undefined;
  if (chatSearchTimer !== null) window.clearTimeout(chatSearchTimer);
  chatSearchRequest++;
  window.removeEventListener("focus", refreshSidebarWhenVisible);
  document.removeEventListener("visibilitychange", refreshSidebarWhenVisible);
});

</script>

<style scoped>
.workspace-sidebar {
  width: var(--ac-workspace-sidebar-width);
  flex: 0 0 var(--ac-workspace-sidebar-width);
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--workbench-bg);
  border-right: 1px solid var(--surface-border);
}
.workspace-panel-header {
  position: relative;
  z-index: 4;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 6px;
  padding: 8px 8px 6px;
}
.workspace-chat-search {
  display: flex;
  align-items: center;
  min-width: 0;
  flex: 1 1 auto;
  height: 31px;
  box-sizing: border-box;
  padding: 0 2px 0 8px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: var(--surface-bg-elevated);
  color: var(--text-muted);
  transition: border-color 0.16s ease, background-color 0.16s ease, box-shadow 0.16s ease;
}
.workspace-chat-search:focus-within,
.workspace-chat-search.is-open {
  border-color: color-mix(in srgb, var(--ac-color-primary) 42%, var(--surface-border));
  background: var(--surface-bg);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--ac-color-primary) 10%, transparent);
}
.workspace-search-icon { flex: 0 0 auto; font-size: 14px; }
.workspace-search-input {
  min-width: 0;
  flex: 1 1 auto;
  height: 100%;
  padding: 0 8px;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--text-primary);
  font: inherit;
  font-size: var(--ac-font-size-xs);
  appearance: none;
}
.workspace-search-input::-webkit-search-cancel-button { display: none; }
.workspace-search-input::placeholder { color: var(--text-muted); }
.workspace-search-clear,
.workspace-create-button {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font: inherit;
}
.workspace-search-clear { width: 24px; height: 24px; font-size: 16px; line-height: 1; }
.workspace-search-clear:hover { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.workspace-create-button { width: 28px; height: 31px; color: var(--text-muted); background: var(--surface-bg-elevated); }
.workspace-create-button:hover,
.workspace-create-button:focus-visible { color: var(--text-secondary); background: var(--workbench-sidebar-hover); }
.workspace-create-button .el-icon { font-size: 16px; }
.workspace-view-switch {
  position: relative;
  grid-column: 1 / -1;
  display: flex;
  align-items: baseline;
  gap: 2px;
  min-width: 0;
  padding: 0;
  background: transparent;
}
.workspace-view-switch button {
  position: relative;
  bottom: -1px;
  flex: 0 0 auto;
  min-width: 0;
  padding: 7px 14px;
  border: 0;
  border-radius: 0;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font: inherit;
  font-size: var(--ac-font-size-sm);
  font-weight: 600;
  white-space: nowrap;
  transition: color 0.2s ease;
}
.workspace-view-switch button:hover { color: var(--ac-color-primary); }
.workspace-view-switch button[aria-selected="true"] {
  color: var(--ac-color-primary);
}
.workspace-view-indicator {
  position: absolute;
  bottom: 0;
  height: 2px;
  border-radius: 999px;
  background: var(--ac-color-primary);
  pointer-events: none;
  transition: left 0.2s ease, width 0.2s ease;
}
.workspace-chat-search-results {
  position: absolute;
  top: calc(100% - 2px);
  right: 8px;
  left: 8px;
  max-height: min(420px, 56vh);
  overflow-y: auto;
  padding: 5px;
  border: 1px solid var(--surface-border);
  border-radius: 11px;
  background: var(--surface-bg-elevated);
  box-shadow: var(--tp-shadow-float);
}
.workspace-search-status {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  padding: 5px 7px 7px;
  color: var(--text-muted);
  font-size: var(--ac-font-size-xs);
}
.workspace-search-status .is-loading { color: var(--ac-color-primary); }
.workspace-search-result {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-height: 44px;
  padding: 4px 6px;
  border-radius: 8px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font: inherit;
  text-align: left;
}
.workspace-search-result:hover,
.workspace-search-result.is-active { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.workspace-search-result-main {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
  flex: 1 1 auto;
  padding: 3px 2px;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font: inherit;
  text-align: left;
}
.workspace-search-result-main > .el-icon { flex: 0 0 auto; font-size: 15px; }
.workspace-search-result-copy { min-width: 0; flex: 1 1 auto; }
.workspace-search-result-copy strong,
.workspace-search-result-copy small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.workspace-search-result-copy strong { font-size: var(--ac-font-size-sm); font-weight: 560; }
.workspace-search-result-copy small { margin-top: 2px; color: var(--text-muted); font-size: var(--ac-font-size-xs); }
.workspace-search-restore {
  flex: 0 0 auto;
  padding: 4px 7px;
  border: 1px solid var(--surface-border);
  border-radius: 6px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: var(--ac-font-size-xs);
  white-space: nowrap;
}
.workspace-search-restore:hover:not(:disabled) { background: var(--surface-bg); color: var(--text-primary); }
.workspace-search-restore:disabled { cursor: wait; opacity: 0.6; }
.workspace-search-message { padding: 14px 10px 16px; color: var(--text-muted); font-size: var(--ac-font-size-xs); text-align: center; }
.workspace-search-message.is-error { color: var(--ac-color-danger); text-align: left; white-space: pre-wrap; overflow-wrap: anywhere; user-select: text; }
.workspace-scroll { min-height: 0; flex: 1 1 auto; overflow-y: auto; overflow-x: hidden; padding: 6px 8px 8px; }
.thread-sidebar { display: flex; flex-direction: column; padding: 2px 0 8px; }
.section-caption { padding: 0 9px 5px; color: var(--text-muted); font-size: var(--ac-font-size-xs); font-weight: 550; }
.thread-row { display: flex; align-items: center; min-height: 30px; border-radius: 6px; color: var(--text-secondary); }
.thread-row:hover { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.thread-main { display: flex; align-items: center; gap: 7px; min-width: 0; flex: 1; min-height: 30px; padding: 0 6px 0 8px; border: 0; background: transparent; color: inherit; cursor: pointer; font: inherit; font-size: var(--ac-font-size-sm); text-align: left; }
.thread-main span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.thread-name-input { min-width: 0; width: 100%; min-height: 28px; padding: 3px 6px; border: 1px solid var(--ac-color-primary); border-radius: 5px; background: var(--surface-bg); color: var(--text-primary); font: inherit; outline: none; }
.thread-expand { align-self: flex-start; margin: 2px 0; padding: 3px 8px; border: 0; border-radius: 5px; background: transparent; color: var(--ac-color-primary); cursor: pointer; font: inherit; font-size: var(--ac-font-size-xs); }
.thread-expand:hover { background: var(--workbench-sidebar-hover); }
.thread-section { display: flex; flex-direction: column; gap: 1px; padding: 4px 0; }
.thread-section + .thread-section { margin-top: 6px; border-top: 1px solid var(--surface-border); padding-top: 10px; }
.project-caption { display: flex; align-items: center; gap: 2px; }
.project-caption > span { margin-right: auto; }
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
.project-main { display: flex; align-items: center; gap: 8px; min-width: 0; flex: 1; min-height: 32px; padding: 0; border: 0; background: transparent; color: inherit; cursor: pointer; font: inherit; font-size: var(--ac-font-size-sm); text-align: left; }
.project-main span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.project-main small { margin-left: auto; color: var(--text-muted); font-size: var(--ac-font-size-xs); }
.project-threads { display: grid; gap: 1px; padding-left: 18px; }
.thread-item { display: flex; align-items: center; gap: 7px; width: 100%; min-height: 30px; padding: 0 8px; border: 0; border-radius: 6px; background: transparent; color: var(--text-secondary); cursor: pointer; font: inherit; font-size: var(--ac-font-size-sm); text-align: left; }
.thread-item span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.thread-item:hover { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.project-thread { min-height: 27px; font-size: var(--ac-font-size-xs); }
.thread-empty { padding: 5px 9px; color: var(--text-muted); font-size: var(--ac-font-size-xs); }
.project-folder-dropzone {
  position: relative;
  display: grid;
  place-items: center;
  width: 100%;
  min-height: 132px;
  padding: 16px 16px 42px;
  border: 1px dashed var(--surface-border);
  border-radius: 10px;
  background: var(--surface-bg-elevated);
  color: var(--text-muted);
  text-align: center;
  transition: border-color 0.16s ease, background-color 0.16s ease, color 0.16s ease;
}
.project-folder-dropzone.is-dragging {
  border-color: var(--ac-color-primary);
  background: color-mix(in srgb, var(--ac-color-primary) 9%, var(--surface-bg-elevated));
  color: var(--text-primary);
}
.project-folder-drop-copy { position: absolute; right: 16px; bottom: 14px; left: 16px; font-size: var(--ac-font-size-xs); text-align: center; }
.project-folder-picker {
  min-height: 34px;
  padding: 0 16px;
  border: 1px solid color-mix(in srgb, var(--ac-color-primary) 45%, var(--surface-border));
  border-radius: 7px;
  background: transparent;
  color: var(--ac-color-primary);
  cursor: pointer;
  font: inherit;
  font-size: var(--ac-font-size-xs);
}
.project-folder-picker:hover { background: var(--workbench-sidebar-hover); }
.project-folder-list { display: grid; gap: 6px; width: 100%; margin-top: 10px; }
.project-folder-item {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  min-height: 38px;
  padding: 0 8px 0 10px;
  border: 1px solid var(--surface-border);
  border-radius: 8px;
  background: var(--surface-bg-elevated);
}
.project-folder-role {
  flex: 0 0 auto;
  padding: 2px 6px;
  border-radius: 999px;
  background: var(--workbench-sidebar-hover);
  color: var(--text-muted);
  font-size: var(--ac-font-size-xs);
}
.project-folder-role.is-primary {
  background: color-mix(in srgb, var(--ac-color-primary) 14%, transparent);
  color: var(--ac-color-primary);
}
.project-folder-name { min-width: 0; flex: 1 1 auto; overflow: hidden; color: var(--text-primary); font-size: var(--ac-font-size-sm); text-overflow: ellipsis; white-space: nowrap; }
.project-folder-clear {
  flex: 0 0 auto;
  padding: 6px 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font: inherit;
}
.project-folder-clear:hover { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.project-folder-hint { margin: 6px 0 0; color: var(--text-muted); font-size: var(--ac-font-size-xs); line-height: 1.5; }
.project-folder-picker:disabled, .project-folder-clear:disabled { cursor: not-allowed; opacity: 0.6; }
.project-conversation-dialog-copy { margin: 0 0 12px; color: var(--text-secondary); font-size: var(--ac-font-size-sm); }
.project-conversation-list { display: grid; gap: 6px; max-height: min(360px, 52vh); overflow-y: auto; }
.project-conversation-choice {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 52px;
  padding: 8px 10px;
  border: 1px solid var(--surface-border);
  border-radius: 9px;
  background: var(--surface-bg);
  color: var(--text-secondary);
  cursor: pointer;
  font: inherit;
  text-align: left;
}
.project-conversation-choice:hover,
.project-conversation-choice.is-selected {
  border-color: color-mix(in srgb, var(--ac-color-primary) 45%, var(--surface-border));
  background: var(--workbench-sidebar-hover);
  color: var(--text-primary);
}
.project-conversation-choice:disabled { cursor: not-allowed; opacity: 0.55; }
.project-conversation-choice > .el-icon { flex: 0 0 auto; font-size: 17px; }
.project-conversation-choice-copy { min-width: 0; flex: 1 1 auto; }
.project-conversation-choice-copy strong,
.project-conversation-choice-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.project-conversation-choice-copy strong { font-size: var(--ac-font-size-sm); font-weight: 560; }
.project-conversation-choice-copy small { margin-top: 3px; color: var(--text-muted); font-size: var(--ac-font-size-xs); }
.project-conversation-empty { padding: 20px 10px; color: var(--text-muted); font-size: var(--ac-font-size-sm); text-align: center; }
.workspace-sidebar :deep(button), .thread-name-input, .section-caption { line-height: 1.45; }
.workspace-sidebar :deep(button:not(:disabled):focus-visible) { outline: 2px solid var(--ac-color-primary); outline-offset: -2px; }
</style>
