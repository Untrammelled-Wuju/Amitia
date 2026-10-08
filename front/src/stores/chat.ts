// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { defineStore } from "pinia";
import { ref, watch } from "vue";
import type { Message } from "@/types";
import { apiClient } from "@/composables/useApi";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";
import { ownedProjectReference, parseOwnedProjectReference } from "@/runtime/owned-project-reference";

export interface ConversationItem {
  id: string;
  projectId: string;
  title: string;
  channel: string;
  source: string;
  messageCount: number;
  pinnedAt?: string | null;
  archivedAt?: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface ProjectItem {
  resourceId?: string;
  roleId?: string;
  readOnly?: boolean;
  logical?: boolean;
  ownerId?: string;
  revision?: number;
  executionScope?: OwnedExecutionScope;
  id: string;
  name: string;
  workspaceId: string;
  deviceId: string;
  rootUri: string;
  available: boolean;
  status: string;
  statusReason?: string;
  conversationCount: number;
  conversations: ConversationItem[];
  pinnedAt?: string | null;
}

export interface SidebarData {
  pinned: ConversationItem[];
  recent: ConversationItem[];
  projects: ProjectItem[];
}

export const useChatStore = defineStore("chat", () => {
  const owned=useDeviceOwnedConversation();
  const messages = ref<Message[]>([]);
  const loading = ref(false);
  const currentConversationId = ref<string | null>(null);
  const currentProjectId = ref("");
  const sidebar = ref<SidebarData>({ pinned: [], recent: [], projects: [] });
  const archivedRevision = ref(0);
  const projectIntent = ref<OwnedExecutionScope | null>(null);
  const historicalProjectRoles = ref<Array<{ id: string; name: string }>>([]);
  const historicalProjectRole = ref("");
  let sidebarGeneration = 0;
  watch([owned.coreId, () => owned.policy.value?.providerEpoch, () => owned.policy.value?.coordinated, () => owned.policy.value?.modeRevision, () => owned.policy.value?.permissionRevision], () => {
    sidebarGeneration++;
    projectIntent.value = null;
    historicalProjectRoles.value = [];
    historicalProjectRole.value = "";
    sidebar.value = { pinned: [], recent: [], projects: [] };
    currentProjectId.value = "";
  }, { flush: "sync" });
  function projectRow(value: any, conversations: ConversationItem[] = []): ProjectItem {
    return { id: ownedProjectReference(value.ownerId, value.id), resourceId: value.id, roleId: value.roleId, readOnly: value.readOnly, name: value.title, logical: true, ownerId: value.ownerId, revision: value.revision, executionScope: value.executionScope, workspaceId: "", deviceId: "", rootUri: "", available: true, status: "logical", pinnedAt: value.pinnedAt, conversationCount: conversations.length, conversations };
  }

  function setMessages(msgs: Message[]) {
    messages.value = msgs;
  }
  function addMessage(msg: Message) {
    messages.value.push(msg);
  }
  function clearMessages() {
    messages.value = [];
    currentConversationId.value = null;
  }
  function setConversationId(id: string) {
    currentConversationId.value = id;
  }

  async function fetchSidebar() {
    if (owned.enabled.value) {
      const generation = ++sidebarGeneration;
      const role=owned.selectInitialRole(localStorage.getItem(`webchat-char-id:${owned.coreId.value}:${owned.roleOwnerId.value}`) || "");
      if (!role) { sidebar.value={pinned:[],recent:[],projects:[]};return; }
      const conversations=await owned.conversations(role);
      const projectPage = await owned.projects(role, historicalProjectRole.value);
      if (generation !== sidebarGeneration) throw new Error("服务已变化，旧项目列表已丢弃");
      projectIntent.value = projectPage.executionScope;
      historicalProjectRoles.value = projectPage.historicalRoles;
      historicalProjectRole.value = projectPage.selectedHistoricalRole;
      const rows=new Map<string,ConversationItem>();
      for (const value of conversations) {
        if (value?.id) rows.set(value.id,{...value,channel:"web",projectId:value.projectId && value.ownerId ? ownedProjectReference(value.ownerId, value.projectId) : "",source:value.source || "device-mesh",messageCount:value.messageCount || 0});
      }
      const visible=[...rows.values()].filter((row)=>!row.archivedAt).sort((a,b)=>String(b.updatedAt).localeCompare(String(a.updatedAt)));
      const projects = [...projectPage.projects, ...projectPage.historicalProjects].map((project) => projectRow(project, visible.filter((row) => row.projectId === ownedProjectReference(project.ownerId, project.id))));
      const projectIds = new Set(projects.map((project) => project.id));
      const ungrouped = visible.filter((row) => !projectIds.has(row.projectId));
      sidebar.value={pinned:ungrouped.filter((row)=>Boolean(row.pinnedAt)),recent:ungrouped.filter((row)=>!row.pinnedAt),projects};
      return;
    }
    const response = await apiClient.get<SidebarData>("/api/web-chat/sidebar");
    const data = response.data || { pinned: [], recent: [], projects: [] };
    sidebar.value = {
      pinned: (data.pinned || []).filter((item) => item.channel === "web"),
      recent: (data.recent || []).filter((item) => item.channel === "web"),
      projects: data.projects || [],
    };
  }


  async function createProject(input: {
    name: string;
    workspaceId?: string;
    deviceId?: string;
    rootUri?: string;
    expectedExecutionScope?: OwnedExecutionScope;
  }): Promise<ProjectItem> {
    if (owned.enabled.value) {
      if (input.workspaceId || input.deviceId || input.rootUri) throw new Error("设备项目仅用于对话分组，文件访问需单独授权");
      const expected = input.expectedExecutionScope || projectIntent.value;
      if (!expected) throw new Error("请先加载项目列表，确认数据所属设备");
      const created = await owned.createProject(input.name, expected);
      await fetchSidebar();
      return projectRow(created);
    }
    const response = await apiClient.post<ProjectItem>("/api/web-chat/projects", input);
    await fetchSidebar();
    return response.data;
  }

  async function updateProject(projectId: string, input: Partial<Pick<ProjectItem, "name" | "workspaceId" | "deviceId" | "rootUri">> & { pinned?: boolean }, intent?: ProjectItem) {
    if (owned.enabled.value) {
      const project = intent || sidebar.value.projects.find((row) => row.id === projectId);
      if (project?.readOnly) throw new Error("旧项目为只读，请在原设备管理");
      if (!project?.executionScope || !project.ownerId || !project.revision || !project.resourceId) throw new Error("请重新加载项目后修改");
      if (input.workspaceId || input.deviceId || input.rootUri) throw new Error("对话分组不能绑定文件目录");
      await owned.edit("project", project.resourceId, { ...(input.name !== undefined ? { title: input.name } : {}), ...(input.pinned !== undefined ? { pinned: input.pinned } : {}) }, { characterId: project.executionScope.roleId, expectedExecutionScope: project.executionScope, expectedOwnerId: project.ownerId, expectedRevision: project.revision });
      await fetchSidebar();
      return;
    }
    await apiClient.patch(`/api/web-chat/projects/${encodeURIComponent(projectId)}`, input);
    await fetchSidebar();
  }

  async function deleteProject(projectId: string, intent?: ProjectItem) {
    if (owned.enabled.value) {
      const project = intent || sidebar.value.projects.find((row) => row.id === projectId);
      if (project?.readOnly) throw new Error("旧项目为只读，请在原设备管理");
      if (!project?.executionScope || !project.ownerId || !project.revision || !project.resourceId) throw new Error("请重新加载项目后删除");
      await owned.edit("project", project.resourceId, {}, { deleted: true, characterId: project.executionScope.roleId, expectedExecutionScope: project.executionScope, expectedOwnerId: project.ownerId, expectedRevision: project.revision });
      if (currentProjectId.value === projectId) currentProjectId.value = "";
      await fetchSidebar();
      return;
    }
    await apiClient.delete(`/api/web-chat/projects/${encodeURIComponent(projectId)}`);
    await fetchSidebar();
  }

  async function moveConversation(conversationId: string, projectId: string) {
    if (owned.enabled.value) {
      const origin = projectId ? parseOwnedProjectReference(projectId) : undefined;
      const project = projectId ? sidebar.value.projects.find((row) => row.id === projectId) : undefined;
      if (projectId && (!origin || !project || project.readOnly || origin.ownerId !== projectIntent.value?.resourceOwnerId)) throw new Error("只能移入当前角色和数据所有者的项目");
      await owned.edit("conversation", conversationId, { projectId: origin?.id || "" }, project?.executionScope ? { expectedExecutionScope: project.executionScope, expectedOwnerId: project.ownerId } : {});
    }
    else await apiClient.put(`/api/web-chat/conversations/${encodeURIComponent(conversationId)}`, { projectId });
    currentProjectId.value = projectId;
    await fetchSidebar();
  }

  async function renameConversation(conversationId: string, title: string) {
    if (owned.enabled.value) await owned.edit("conversation",conversationId,{title});
    else await apiClient.put(`/api/web-chat/conversations/${encodeURIComponent(conversationId)}`, { title });
    await fetchSidebar();
  }

  async function setConversationPinned(conversationId: string, pinned: boolean) {
    if (owned.enabled.value) await owned.edit("conversation",conversationId,{pinned});
    else await apiClient.put(`/api/web-chat/conversations/${encodeURIComponent(conversationId)}`, { pinned });
    await fetchSidebar();
  }

  async function archiveConversation(conversationId: string) {
    if (owned.enabled.value) await owned.edit("conversation",conversationId,{archived:true});
    else await apiClient.put(`/api/web-chat/conversations/${encodeURIComponent(conversationId)}`, { archived: true });
    if (currentConversationId.value === conversationId) {
      currentConversationId.value = null;
      currentProjectId.value = "";
    }
    await fetchSidebar();
    archivedRevision.value += 1;
  }

  async function restoreConversation(conversationId: string) {
    if (owned.enabled.value) await owned.edit("conversation",conversationId,{archived:false});
    else await apiClient.put(`/api/web-chat/conversations/${encodeURIComponent(conversationId)}`, { archived: false });
    await fetchSidebar();
    archivedRevision.value += 1;
  }

  async function deleteConversation(conversationId: string) {
    if (owned.enabled.value) await owned.edit("conversation",conversationId,{}, {deleted:true});
    else await apiClient.delete(`/api/web-chat/conversations/${encodeURIComponent(conversationId)}`);
    if (currentConversationId.value === conversationId) {
      currentConversationId.value = null;
      currentProjectId.value = "";
    }
    await fetchSidebar();
  }

  return {
    messages,
    loading,
    currentConversationId,
    setMessages,
    addMessage,
    clearMessages,
    setConversationId,
    currentProjectId,
    sidebar,
    archivedRevision,
    projectIntent,
    historicalProjectRoles,
    historicalProjectRole,
    ownedProjects: owned.enabled,
    fetchSidebar,
    createProject,
    updateProject,
    deleteProject,
    moveConversation,
    renameConversation,
    setConversationPinned,
    archiveConversation,
    restoreConversation,
    deleteConversation,
  };
});
