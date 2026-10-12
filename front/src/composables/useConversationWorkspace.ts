import { ref, computed, watch } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "./useApi";
import { useChatStore, type ProjectItem, type ProjectWorkspace } from "@/stores/chat";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";

export interface WorkspaceMountSummary {
  id: string;
  projectId?: string;
  name: string;
  kind: string;
  rootUri: string;
  executionScope?: OwnedExecutionScope;
  readOnly: boolean;
  available: boolean;
  status: string;
  statusReason?: string;
}

export interface ConversationWorkspaceBinding {
  projectId: string;
  workspaceId: string;
  primaryWorkspaceId: string;
  workspaces: ProjectWorkspace[];
  deviceId?: string;
  workspaceName?: string;
  workspaceKind: string;
  rootUri: string;
  executionScope?: OwnedExecutionScope;
}

const activeConversationId = ref("");
const currentWorkspace = ref<ConversationWorkspaceBinding | null>(null);
const recentWorkspaces = ref<WorkspaceMountSummary[]>([]);
const workspaceLoading = ref(false);

function projectToMount(project: ProjectItem): WorkspaceMountSummary {
  const workspaces = Array.isArray(project.workspaces) ? project.workspaces : [];
  const primary = workspaces.find((workspace) => workspace.id === project.primaryWorkspaceId) || workspaces[0];
  return {
    id: primary?.id || project.workspaceId,
    projectId: project.id,
    name: project.name,
    kind: primary?.kind || (primary?.rootUri.startsWith("content://") ? "saf" : "local"),
    rootUri: primary?.rootUri || project.rootUri,
    executionScope: project.executionScope,
    readOnly: project.readOnly || primary?.readOnly === true,
    available: project.available && workspaces.length > 0 && !project.readOnly,
    status: project.status,
    statusReason: project.statusReason,
  };
}

function projectToBinding(project: ProjectItem): ConversationWorkspaceBinding {
  const workspaces = Array.isArray(project.workspaces) ? project.workspaces : [];
  const primary = workspaces.find((workspace) => workspace.id === project.primaryWorkspaceId) || workspaces[0];
  return {
    projectId: project.id,
    workspaceId: primary?.id || project.workspaceId,
    primaryWorkspaceId: primary?.id || project.primaryWorkspaceId,
    workspaces,
    deviceId: primary?.deviceId || project.deviceId || undefined,
    workspaceName: project.name,
    workspaceKind: primary?.kind || "local",
    rootUri: primary?.rootUri || project.rootUri,
    executionScope: project.executionScope,
  };
}

function findProjectByWorkspace(projects: ProjectItem[], workspaceId: string) {
  return projects.find((project) => (project.workspaces || []).some((workspace) => workspace.id === workspaceId));
}

export function useConversationWorkspace() {
  const { post } = useApi();
  const chatStore = useChatStore();
  watch(() => chatStore.currentProjectId, (id) => {
    if (!id || (currentWorkspace.value?.projectId && currentWorkspace.value.projectId !== id)) currentWorkspace.value = null;
  }, { flush: "sync" });

  async function refreshRecentWorkspaces(): Promise<void> {
    await chatStore.fetchSidebar();
    recentWorkspaces.value = chatStore.sidebar.projects.map(projectToMount);
  }

  async function pickWorkspaceMount(): Promise<any | null> {
    if (window.amitiaDesktop?.selectWorkspaceDirectory) {
      const selection = await window.amitiaDesktop.selectWorkspaceDirectory();
      if (!selection?.path) return null;
      const mount = await post<any>("/api/workspaces/local", { name: selection.name || "项目", localRoot: selection.path, readOnly: false });
      if (!mount?.id) throw new Error("项目根目录注册失败");
      return mount;
    }
    const mount = await post<any>("/api/workspaces/pick", {});
    if (mount?.cancelled) return null;
    if (!mount?.id) throw new Error("当前设备代理未返回工作区");
    return mount;
  }

  async function moveConversationToProject(project: ProjectItem): Promise<void> {
    const conversationId = activeConversationId.value.trim();
    if (conversationId) throw new Error("现有对话不能手动移入或移出项目");
    chatStore.currentProjectId = project.id;
    currentWorkspace.value = projectToBinding(project);
  }

  async function bindCurrentWorkspaceToConversation(conversationId: string): Promise<void> {
    if (String(conversationId || "").trim()) throw new Error("现有对话不能手动移入或移出项目");
  }

  async function selectWorkspaceMount(mount: WorkspaceMountSummary): Promise<void> {
    if (!mount.available) {
      throw new Error(mount.statusReason || "该文件夹当前不可用");
    }
    workspaceLoading.value = true;
    try {
      const project = findProjectByWorkspace(chatStore.sidebar.projects, mount.id);
      if (!project) throw new Error("项目不存在");
      if (activeConversationId.value.trim()) throw new Error("现有对话不能手动切换项目");
      await moveConversationToProject(project);
      await refreshRecentWorkspaces();
    } finally {
      workspaceLoading.value = false;
    }
  }

  async function chooseWorkspaceDirectory(): Promise<ProjectItem | null> {
    workspaceLoading.value = true;
    try {
      const mount = await pickWorkspaceMount();
      if (!mount) return null;
      await chatStore.fetchSidebar();
      const allProjects = [...chatStore.sidebar.projects, ...(chatStore.sidebar.removedProjects || [])];
      let project = findProjectByWorkspace(allProjects, mount.id);
      if (project?.hiddenAt) {
        await chatStore.restoreProject(project.id, project);
        project = chatStore.sidebar.projects.find((item) => item.id === project?.id) || project;
      }
      project ??= await chatStore.createProject({
          name: mount.name || "项目",
          workspaceId: mount.id,
          deviceId: chatStore.projectIntent?.targetDeviceId,
          rootUri: mount.rootUri,
          workspaces: [{ id: mount.id, deviceId: chatStore.projectIntent?.targetDeviceId || "", name: mount.name || "项目", kind: mount.rootUri?.startsWith("content://") ? "saf" : "local", rootUri: mount.rootUri }],
          primaryWorkspaceId: mount.id,
        });
      if (!activeConversationId.value.trim()) await moveConversationToProject(project);
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
      if (conversationId) throw new Error("现有对话不能手动移出项目");
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
    if (!project || project.readOnly || project.hiddenAt || !(project.workspaces || []).length) throw new Error("项目不存在、已移除或工作区不可用");
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
      chatStore.currentProjectId = String(conversationProjectId || "").trim();
      const project = chatStore.sidebar.projects.find((row) => row.id === chatStore.currentProjectId);
      currentWorkspace.value = project ? projectToBinding(project) : null;
      return;
  }

  function getWorkspaceRequestFields(): Record<string, string> {
    const workspace = currentWorkspace.value;
    if (!workspace?.workspaceId || workspace.workspaceId === workspace.projectId) return {};
    return {
      workspaceId: workspace.workspaceId,
      ...(workspace.deviceId ? { workspaceDeviceId: workspace.deviceId } : {}),
    };
  }

  async function addWorkspaceToProject(project: ProjectItem): Promise<ProjectWorkspace | null> {
    const mount = await pickWorkspaceMount();
    if (!mount) return null;
    if ((project.workspaces || []).some((workspace) => workspace.id === mount.id)) throw new Error("该工作区已经属于当前项目");
    const workspace: ProjectWorkspace = { id: mount.id, deviceId: chatStore.projectIntent?.targetDeviceId || "", name: mount.name || "工作区", kind: mount.rootUri?.startsWith("content://") ? "saf" : "local", rootUri: mount.rootUri };
    await chatStore.updateProject(project.id, { workspaces: [...(project.workspaces || []), workspace] }, project);
    return workspace;
  }

  async function removeWorkspaceFromProject(project: ProjectItem, workspaceId: string): Promise<void> {
    if ((project.workspaces || []).length <= 1) throw new Error("项目至少需要保留一个工作区");
    const workspaces = (project.workspaces || []).filter((workspace) => workspace.id !== workspaceId);
    const primaryWorkspaceId = project.primaryWorkspaceId === workspaceId ? workspaces[0].id : project.primaryWorkspaceId;
    await chatStore.updateProject(project.id, { workspaces, primaryWorkspaceId }, project);
  }

  async function setPrimaryWorkspace(project: ProjectItem, workspaceId: string): Promise<void> {
    if (!(project.workspaces || []).some((workspace) => workspace.id === workspaceId)) throw new Error("工作区不存在");
    await chatStore.updateProject(project.id, { primaryWorkspaceId: workspaceId }, project);
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
    addWorkspaceToProject,
    removeWorkspaceFromProject,
    setPrimaryWorkspace,
    ownedProjects: computed(() => chatStore.ownedProjects),
  };
}
