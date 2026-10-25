<template>
  <aside class="workspace-sidebar" data-amitia-ui="app.workspace-sidebar">
    <div class="workspace-panel-header">
      <button
        type="button"
        class="workspace-new-chat"
        aria-label="新建对话"
        title="新建对话"
        data-amitia-ui="workspace.new-chat"
        @click="handleNewChat"
      >
        <el-icon><NewConversationIcon /></el-icon>
        <span>新对话</span>
      </button>
      <button
        type="button"
        class="workspace-new-group"
        aria-label="新建群聊"
        title="新建群聊"
        @click="handleNewGroupChat"
      >
        <el-icon><ChatDotRound /></el-icon>
        <span>新群聊</span>
      </button>
      <button
        type="button"
        class="workspace-search"
        aria-label="搜索"
        title="搜索"
        @click="searchModal?.open()"
      >
        <el-icon><Search /></el-icon>
      </button>
    </div>

    <div class="workspace-scroll">
      <div class="thread-sidebar">
        <div v-if="chatStore.sidebar.pinned.length || pinnedProjects.length" class="thread-section">
          <div class="section-caption">置顶</div>
          <SidebarConversationRow
            v-for="conversation in chatStore.sidebar.pinned"
            :key="conversation.id"
            :conversation="conversation"
            :active="conversation.id === activeConversationId"
            :projects="chatStore.sidebar.projects"
            @select="handleSelectConversation"
            @rename="handleConversationRename"
            @toggle-pin="handleToggleConversationPin"
            @archive="handleArchiveConversation"
            @move="handleMoveConversation"
          />
          <SidebarProjectBlock
            v-for="project in pinnedProjects"
            :key="project.id"
            :project="project"
            :active="project.id === chatStore.currentProjectId"
            :active-conversation-id="activeConversationId"
            :projects="chatStore.sidebar.projects"
            @select="handleSelectConversation"
            @create-conversation="handleCreateProjectConversation"
            @command="handleProjectCommand"
            @rename-conversation="handleConversationRename"
            @toggle-pin-conversation="handleToggleConversationPin"
            @archive-conversation="handleArchiveConversation"
            @move-conversation="handleMoveConversation"
          />
        </div>

        <div class="thread-section">
          <div class="section-caption project-caption">
            <span>最近</span>
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
            @select="handleSelectConversation"
            @rename="handleConversationRename"
            @toggle-pin="handleToggleConversationPin"
            @archive="handleArchiveConversation"
            @move="handleMoveConversation"
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

        <div class="thread-section">
          <div class="section-caption project-caption">
            <span>项目</span>
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
            @select="handleSelectConversation"
            @create-conversation="handleCreateProjectConversation"
            @command="handleProjectCommand"
            @rename-conversation="handleConversationRename"
            @toggle-pin-conversation="handleToggleConversationPin"
            @archive-conversation="handleArchiveConversation"
            @move-conversation="handleMoveConversation"
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
    <NewGroupChatDialog
      v-model:visible="newGroupDialogVisible"
      :characters="owned.roles.value"
      :default-role-id="newGroupDefaultRoleId"
      @created="handleGroupCreated"
    />
    <SearchModal ref="searchModal" />
  </aside>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ChatDotRound, FolderOpened, Plus, Search } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useChatStore, type ConversationItem, type ProjectItem, type ProjectWorkspace } from "@/stores/chat";
import { useApi } from "@/composables/useApi";
import NewConversationIcon from "@/components/icons/NewConversationIcon.vue";
import NewGroupChatDialog from "@/components/NewGroupChatDialog.vue";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import type { OwnedGroupState } from "@/runtime/device-owned-chat";
import { useConversationDisclosure } from "@/composables/useConversationDisclosure";
import { useConversationWorkspace } from "@/composables/useConversationWorkspace";
import SearchModal from "./SearchModal.vue";
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
const searchModal = ref<InstanceType<typeof SearchModal> | null>(null);
const projectDialogOpen = ref(false);
const newGroupDialogVisible = ref(false);
const projectCreating = ref(false);
const projectSelectingFolder = ref(false);
const projectDraftName = ref("");
const projectDraftWorkspaces = ref<ProjectWorkspace[]>([]);
const projectNameInitialized = ref(false);
const projectDragActive = ref(false);
const projectDragDepth = ref(0);
const activeConversationId = computed(() => String(route.query.conversationId || ""));
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
const hasGeneratingConversations = computed(() => {
  const rows = [
    ...chatStore.sidebar.pinned,
    ...chatStore.sidebar.recent,
    ...chatStore.sidebar.projects.flatMap((project) => project.conversations),
  ];
  return rows.some((conversation) => Boolean(conversation.isGenerating));
});
let conversationActivityTimer: number | null = null;
let conversationActivityRefreshing = false;

function syncConversationActivityPolling() {
  if (!hasGeneratingConversations.value) {
    if (conversationActivityTimer !== null) {
      window.clearInterval(conversationActivityTimer);
      conversationActivityTimer = null;
    }
    return;
  }
  if (conversationActivityTimer !== null) return;
  conversationActivityTimer = window.setInterval(async () => {
    if (conversationActivityRefreshing) return;
    conversationActivityRefreshing = true;
    try {
      await chatStore.fetchSidebar();
    } catch {
    } finally {
      conversationActivityRefreshing = false;
    }
  }, 2000);
}

async function handleNewChat() {
  await startDraftConversation();
  await router.push({ path: "/chat" });
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

async function handleGroupCreated(group: OwnedGroupState, baseRoleId: string) {
  chatStore.setConversationId(group.conversationId);
  await chatStore.fetchSidebar().catch(() => undefined);
  await router.push({ path: "/chat", query: { conversationId: group.conversationId, characterId: baseRoleId } });
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
  if (wasActive) await router.replace({ path: "/chat" });
}

async function handleMoveConversation(conversation: ConversationItem, projectId: string) {
  await chatStore.moveConversation(conversation.id, projectId);
  if (activeConversationId.value === conversation.id) applySnapshotWorkspace(null, projectId);
}

async function handleCreateProjectConversation(project: ProjectItem) {
  if (project.readOnly) {
    ElMessage.warning("该项目为只读");
    return;
  }
  try {
    await startDraftConversation(project.id);
    await router.push({ path: "/chat", query: { projectId: project.id } });
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "新建项目对话失败");
  }
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
    } catch {
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
  void chatStore.fetchSidebar().catch(() => undefined);
  syncConversationActivityPolling();
});

onUnmounted(() => {
  if (conversationActivityTimer !== null) {
    window.clearInterval(conversationActivityTimer);
    conversationActivityTimer = null;
  }
});

watch(hasGeneratingConversations, syncConversationActivityPolling, { immediate: true });
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
.workspace-panel-header { display: flex; align-items: center; gap: 8px; padding: 8px 8px 6px; }
.workspace-search {
  width: 36px;
  height: 36px;
  flex: 0 0 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  border-radius: 9px;
  background: var(--surface-bg-elevated);
  color: var(--text-muted);
  cursor: pointer;
  font: inherit;
}
.workspace-search:hover { color: var(--text-primary); }
.workspace-new-chat {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-width: 0;
  height: 36px;
  flex: 1 1 auto;
  padding: 0 10px;
  border: 0;
  border-radius: 9px;
  background: var(--surface-bg-elevated);
  color: var(--text-muted);
  cursor: pointer;
  font: inherit;
}
.workspace-new-chat span { min-width: 0; overflow: hidden; font-size: var(--ac-font-size-sm); text-overflow: ellipsis; white-space: nowrap; }
.workspace-new-chat .el-icon { flex: 0 0 auto; font-size: 16px; }
.workspace-new-chat:hover { color: var(--text-primary); }
.workspace-new-group { display: inline-flex; align-items: center; justify-content: center; gap: 5px; height: 36px; flex: 0 0 auto; padding: 0 9px; border: 1px solid color-mix(in srgb, var(--ac-color-primary) 24%, transparent); border-radius: 9px; background: color-mix(in srgb, var(--ac-color-primary) 8%, var(--surface-bg-elevated)); color: var(--ac-color-primary); cursor: pointer; font: inherit; font-size: 11px; }
.workspace-new-group:hover, .workspace-new-group:focus-visible { border-color: color-mix(in srgb, var(--ac-color-primary) 55%, transparent); color: var(--text-primary); outline: none; }
.workspace-new-group .el-icon { font-size: 15px; }
@media (max-width: 760px) { .workspace-new-group span { display: none; } .workspace-new-group { width: 36px; padding: 0; } }
.workspace-scroll { min-height: 0; flex: 1 1 auto; overflow-y: auto; overflow-x: hidden; padding: 6px 8px 8px; }
.thread-sidebar { display: flex; flex-direction: column; padding: 2px 0 8px; }
.section-caption { padding: 0 9px 5px; color: var(--text-muted); font-size: var(--ac-font-size-xs); font-weight: 550; }
.thread-row { display: flex; align-items: center; min-height: 30px; border-radius: 6px; color: var(--text-secondary); }
.thread-row:hover { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.thread-main { display: flex; align-items: center; gap: 7px; min-width: 0; flex: 1; min-height: 30px; padding: 0 6px 0 8px; border: 0; background: transparent; color: inherit; cursor: pointer; font: inherit; font-size: var(--ac-font-size-sm); text-align: left; }
.thread-main span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.thread-name-input { min-width: 0; width: 100%; min-height: 28px; padding: 3px 6px; border: 1px solid var(--ac-color-primary); border-radius: 5px; background: var(--surface-bg); color: var(--text-primary); font: inherit; outline: none; }
.thread-delete { display: grid; place-items: center; width: 26px; height: 26px; margin-right: 2px; padding: 0; border: 0; border-radius: 5px; background: transparent; color: var(--text-muted); cursor: pointer; opacity: 0; }
.thread-row:hover .thread-delete { opacity: 1; }
.thread-actions { opacity: 0; }
.thread-row:hover .thread-actions, .thread-row:focus-within .thread-actions { opacity: 1; }
.thread-delete:hover { background: var(--control-hover-bg); color: var(--danger-color, #d9534f); }
.thread-expand { align-self: flex-start; margin: 2px 0; padding: 3px 8px; border: 0; border-radius: 5px; background: transparent; color: var(--ac-color-primary); cursor: pointer; font: inherit; font-size: var(--ac-font-size-xs); }
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
.workspace-sidebar :deep(button), .thread-name-input, .section-caption { line-height: 1.45; }
.workspace-sidebar :deep(button:not(:disabled):focus-visible) { outline: 2px solid var(--ac-color-primary); outline-offset: -2px; }
.thread-row:focus-within .thread-delete { opacity: 1; }
</style>
