import { ref } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "./useApi";
import { useChatStore, type ProjectItem } from "@/stores/chat";

export interface WorkspaceMountSummary {
  id: string;
  projectId?: string;
  name: string;
  kind: string;
  rootUri: string;
  readOnly: boolean;
  available: boolean;
  status: string;
  statusReason?: string;
}

export interface ConversationWorkspaceBinding {
  projectId: string;
  workspaceId: string;
  deviceId?: string;
  workspaceName?: string;
  workspaceKind: string;
  rootUri: string;
}

const activeConversationId = ref("");
const currentWorkspace = ref<ConversationWorkspaceBinding | null>(null);
const recentWorkspaces = ref<WorkspaceMountSummary[]>([]);
const workspaceLoading = ref(false);

function projectToMount(project: ProjectItem): WorkspaceMountSummary {
  return {
    id: project.workspaceId,
    projectId: project.id,
    name: project.name,
    kind: project.rootUri.startsWith("content://") ? "saf" : "local",
    rootUri: project.rootUri,
    readOnly: project.status === "read_only",
    available: project.available,
    status: project.status,
    statusReason: project.statusReason,
  };
}

function projectToBinding(project: ProjectItem): ConversationWorkspaceBinding {
  return {
    projectId: project.id,
    workspaceId: project.workspaceId,
    deviceId: project.deviceId || undefined,
    workspaceName: project.name,
    workspaceKind: project.rootUri.startsWith("content://") ? "saf" : "local",
    rootUri: project.rootUri,
  };
}

function findProjectByWorkspace(projects: ProjectItem[], workspaceId: string) {
  return projects.find((project) => project.workspaceId === workspaceId);
}

export function useConversationWorkspace() {
  const { post } = useApi();
  const chatStore = useChatStore();

  async function refreshRecentWorkspaces(): Promise<void> {
    await chatStore.fetchSidebar();
    recentWorkspaces.value = chatStore.sidebar.projects.map(projectToMount);
  }

  async function moveConversationToProject(project: ProjectItem): Promise<void> {
    const conversationId = activeConversationId.value.trim();
    if (conversationId) {
      await chatStore.moveConversation(conversationId, project.id);
    }
    chatStore.currentProjectId = project.id;
    currentWorkspace.value = projectToBinding(project);
  }

  async function bindCurrentWorkspaceToConversation(conversationId: string): Promise<void> {
    const id = String(conversationId || "").trim();
    const projectId = currentWorkspace.value?.projectId || "";
    if (!id || !projectId) return;
    await chatStore.moveConversation(id, projectId);
  }

  async function selectWorkspaceMount(mount: WorkspaceMountSummary): Promise<void> {
    if (!mount.available) {
      throw new Error(mount.statusReason || "该文件夹当前不可用");
    }
    workspaceLoading.value = true;
    try {
      const project = findProjectByWorkspace(chatStore.sidebar.projects, mount.id);
      if (!project) throw new Error("项目不存在");
      await moveConversationToProject(project);
      await refreshRecentWorkspaces();
    } finally {
      workspaceLoading.value = false;
    }
  }

  async function chooseWorkspaceDirectory(): Promise<ProjectItem | null> {
    if (!window.amitiaDesktop?.selectWorkspaceDirectory) {
      ElMessage.warning("当前环境不支持直接选择本机文件夹");
      return null;
    }
    workspaceLoading.value = true;
    try {
      const selection = await window.amitiaDesktop.selectWorkspaceDirectory();
      if (!selection?.path) return null;
      const mount = await post<any>("/api/workspaces/local", {
        name: selection.name || "项目",
        localRoot: selection.path,
        readOnly: false,
      });
      if (!mount?.id) throw new Error("项目根目录注册失败");
      await chatStore.fetchSidebar();
      let project = findProjectByWorkspace(chatStore.sidebar.projects, mount.id);
      project ??= await chatStore.createProject({
          name: mount.name || selection.name || "项目",
          workspaceId: mount.id,
          rootUri: mount.rootUri,
        });
      await moveConversationToProject(project);
      await refreshRecentWorkspaces();
      return project;
    } catch (error) {
      const message = error instanceof Error ? error.message : "选择文件夹失败";
      ElMessage.error(message);
      throw error;
    } finally {
      workspaceLoading.value = false;
    }
  }

  async function clearWorkspace(): Promise<void> {
    const conversationId = activeConversationId.value.trim();
    workspaceLoading.value = true;
    try {
      if (conversationId) {
        await chatStore.moveConversation(conversationId, "");
      }
      currentWorkspace.value = null;
      chatStore.currentProjectId = "";
      await refreshRecentWorkspaces();
    } finally {
      workspaceLoading.value = false;
    }
  }

  async function startDraftConversation(projectId = ""): Promise<ProjectItem | null> {
    activeConversationId.value = "";
    chatStore.clearMessages();
    await chatStore.fetchSidebar();
    const normalizedProjectId = String(projectId || "").trim();
    if (!normalizedProjectId) {
      currentWorkspace.value = null;
      chatStore.currentProjectId = "";
      return null;
    }
    const project = chatStore.sidebar.projects.find(
      (item) => item.id === normalizedProjectId,
    );
    if (!project) throw new Error("项目不存在");
    currentWorkspace.value = projectToBinding(project);
    chatStore.currentProjectId = project.id;
    return project;
  }

  async function loadConversationWorkspace(
    conversationId: string,
    draftProjectId = "",
  ): Promise<void> {
    const id = String(conversationId || "").trim();
    activeConversationId.value = id;
    if (!id) {
      await startDraftConversation(draftProjectId);
      return;
    }
    currentWorkspace.value = null;
  }

  function applySnapshotWorkspace(
    workspace?: Record<string, any> | null,
    conversationProjectId = "",
  ): void {
    if (!workspace || !String(workspace.workspaceId || "").trim()) {
      currentWorkspace.value = null;
      chatStore.currentProjectId = String(conversationProjectId || "").trim();
      return;
    }
    const projectId = String(workspace.projectId || "").trim();
    currentWorkspace.value = {
      projectId,
      workspaceId: String(workspace.workspaceId || "").trim(),
      deviceId: String(workspace.deviceId || "").trim() || undefined,
      workspaceName: String(workspace.workspaceName || "").trim(),
      workspaceKind: String(workspace.workspaceKind || "local").trim() || "local",
      rootUri: String(workspace.rootUri || "").trim(),
    };
    chatStore.currentProjectId = projectId;
  }

  function getWorkspaceRequestFields(): Record<string, string> {
    const workspace = currentWorkspace.value;
    if (!workspace || workspace.projectId) return {};
    return {
      workspaceId: workspace.workspaceId,
      ...(workspace.deviceId ? { workspaceDeviceId: workspace.deviceId } : {}),
    };
  }

  return {
    activeConversationId,
    currentWorkspace,
    recentWorkspaces,
    workspaceLoading,
    refreshRecentWorkspaces,
    loadConversationWorkspace,
    applySnapshotWorkspace,
    startDraftConversation,
    chooseWorkspaceDirectory,
    selectWorkspaceMount,
    clearWorkspace,
    bindCurrentWorkspaceToConversation,
    getWorkspaceRequestFields,
  };
}
