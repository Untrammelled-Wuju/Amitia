// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { defineStore } from "pinia";
import { ref, watch } from "vue";
import type { Message } from "@/types";
import { apiClient } from "@/composables/useApi";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";
import { originalErrorText } from "@/runtime/original-error";
import { ownedProjectReference } from "@/runtime/owned-project-reference";

export function conversationChannel(value: Record<string, unknown>): string {
  const explicit = String(value.channel ?? value.channelId ?? "").trim();
  if (explicit && explicit !== "web") return explicit;
  const id = String(value.resourceId ?? value.id ?? "");
  if (id.startsWith("conv-qq-personal-")) return "qq_personal";
  if (id.startsWith("conv-group-")) return "qq";
  if (id.startsWith("wechat-ipad860-") || id.includes("@im.wechat") || id.includes("@chatroom")) return "wechat";
  return "web";
}

export interface ConversationItem {
  id: string;
  ownerId?: string;
  revision?: number;
  projectId: string;
  title: string;
  channel: string;
  source: string;
  messageCount: number;
  pinnedAt?: string | null;
  archivedAt?: string | null;
  lastReadTurnSequence?: number;
  lastTerminalTurnSequence?: number;
  isGenerating?: boolean;
  hasUnread?: boolean;
  unreadCompletedCount?: number;
  characterId?: string;
  lastMessage?: string;
  lastMessagePreview?: string;
  lastMessageType?: string;
  lastMessageRole?: string;
  activityLabel?: string;
  activeToolName?: string;
  activeTaskTitle?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ProjectItem {
  resourceId?: string;
  roleId?: string;
  readOnly?: boolean;
  ownerId?: string;
  revision?: number;
  executionScope?: OwnedExecutionScope;
  id: string;
  name: string;
  workspaceId: string;
  deviceId: string;
  rootUri: string;
  workspaces: ProjectWorkspace[];
  primaryWorkspaceId: string;
  hiddenAt?: string | null;
  available: boolean;
  status: string;
  statusReason?: string;
  conversationCount: number;
  conversations: ConversationItem[];
  pinnedAt?: string | null;
}

export interface ProjectWorkspace {
  id: string;
  deviceId: string;
  name: string;
  kind: string;
  rootUri: string;
  readOnly?: boolean;
}

export interface SidebarData {
  pinned: ConversationItem[];
  recent: ConversationItem[];
  projects: ProjectItem[];
  removedProjects?: ProjectItem[];
}

export const useChatStore = defineStore("chat", () => {
  const owned=useDeviceOwnedConversation();
  const messages = ref<Message[]>([]);
  const loading = ref(false);
  const currentConversationId = ref<string | null>(null);
  const currentProjectId = ref("");
  const sidebar = ref<SidebarData>({ pinned: [], recent: [], projects: [], removedProjects: [] });
  const archivedRevision = ref(0);
  const projectIntent = ref<OwnedExecutionScope | null>(null);
  const historicalProjectRoles = ref<Array<{ id: string; name: string }>>([]);
  const historicalProjectRole = ref("");
  type RolePage = Awaited<ReturnType<typeof owned.conversationPage>>;
  type ProjectConversationPage = { loading: boolean; loaded: boolean; cursor?: RolePage["cursor"]; error: string };
  const projectConversationPages = ref<Record<string, ProjectConversationPage>>({});
  const sidebarHasMore = ref(false);
  const sidebarLoading = ref(false);
  let sidebarRole = "";
  let sidebarPages = new Map<string, RolePage>();
  let sidebarGeneration = 0;
  let sidebarFetch: Promise<void> | null = null;
  watch([owned.coreId, () => owned.policy.value?.providerEpoch, () => owned.policy.value?.coordinated, () => owned.policy.value?.modeRevision, () => owned.policy.value?.permissionRevision], () => {
    sidebarGeneration++;
    sidebarFetch = null;
    sidebarLoading.value = false;
    sidebarHasMore.value = false;
    sidebarPages = new Map();
    projectConversationPages.value = {};
    sidebarRole = "";
    projectIntent.value = null;
    historicalProjectRoles.value = [];
    historicalProjectRole.value = "";
    sidebar.value = { pinned: [], recent: [], projects: [], removedProjects: [] };
    currentProjectId.value = "";
  }, { flush: "sync" });
  function projectRow(value: any, conversations: ConversationItem[] = []): ProjectItem {
    const rawWorkspaces: any[] = Array.isArray(value.workspaces) ? value.workspaces : [];
    const workspaces: ProjectWorkspace[] = rawWorkspaces.map((workspace: any): ProjectWorkspace => ({ id: String(workspace.id || ""), deviceId: String(workspace.deviceId || ""), name: String(workspace.name || ""), kind: String(workspace.kind || "local"), rootUri: String(workspace.rootUri || ""), readOnly: workspace.readOnly === true })).filter((workspace: ProjectWorkspace) => Boolean(workspace.id && workspace.rootUri));
    const primaryWorkspaceId = String(value.primaryWorkspaceId || workspaces[0]?.id || "");
    const primary = workspaces.find((workspace) => workspace.id === primaryWorkspaceId) || workspaces[0];
    return { id: ownedProjectReference(value.ownerId, value.id), resourceId: value.id, roleId: value.roleId, readOnly: value.readOnly, ownerId: value.ownerId, revision: value.revision, executionScope: value.executionScope, name: value.title, workspaces, primaryWorkspaceId, hiddenAt: value.hiddenAt || null, workspaceId: primary?.id || "", deviceId: primary?.deviceId || "", rootUri: primary?.rootUri || "", available: true, status: "available", pinnedAt: value.pinnedAt, conversationCount: conversations.length, conversations };
  }

  function normalizeProjectRow(project: ProjectItem): ProjectItem {
    const workspaces = Array.isArray(project.workspaces) ? project.workspaces : [];
    if (workspaces.length || !project.workspaceId || !project.rootUri) return { ...project, workspaces };
    const workspace = { id: project.workspaceId, deviceId: project.deviceId || "", name: project.name, kind: project.rootUri.startsWith("content://") ? "saf" : "local", rootUri: project.rootUri, readOnly: project.readOnly };
    return { ...project, workspaces: [workspace], primaryWorkspaceId: project.primaryWorkspaceId || workspace.id };
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

  function sidebarConversation(value: any): ConversationItem {
    return {
      ...value,
      channel: conversationChannel(value),
      projectId: value.projectId && value.ownerId ? ownedProjectReference(value.ownerId, value.projectId) : "",
      source: value.source || "device-mesh",
      messageCount: value.messageCount || 0,
    };
  }

  async function fetchProjectConversationPage(projectId: string) {
    const project = sidebar.value.projects.find((row) => row.id === projectId);
    if (!project?.resourceId || !project.roleId || !project.ownerId) return;

    projectConversationPages.value[projectId] ??= { loading: false, loaded: false, error: "" };
    const state = projectConversationPages.value[projectId];
    if (state.loading || state.loaded && !state.cursor) return;
    state.loading = true;
    state.error = "";
    try {
      const page = await owned.conversationPage(project.roleId, "", state.cursor, {
        archive: "active", channel: "web", projectId: project.resourceId, limit: 36,
      });
      if (projectConversationPages.value[projectId] !== state) return;
      const current = sidebar.value.projects.find((row) => row.id === projectId);
      if (!current) return;
      const rows = new Map((state.loaded ? current.conversations : []).map((row) => [row.id, row]));
      for (const value of page.rows) {
        if (!value?.id || value.ownerId !== project.ownerId || value.projectId !== project.resourceId) continue;
        const row = sidebarConversation(value);
        if (!row.archivedAt && row.channel === "web") rows.set(row.id, row);
      }
      current.conversations = [...rows.values()].sort((a, b) => String(b.updatedAt).localeCompare(String(a.updatedAt)));
      current.conversationCount = current.conversations.length;
      state.loaded = true;
      state.cursor = page.cursor;
    } catch (error) {
      console.error("[original-error]", error);
      if (projectConversationPages.value[projectId] === state) state.error = originalErrorText(error);
    } finally {
      state.loading = false;
    }
  }

  async function loadSidebar(append = false) {
    if (!owned.enabled.value && !await owned.refresh()) throw new Error(owned.notice.value || "设备对话服务尚未就绪");
    if (owned.enabled.value) {
      const generation = ++sidebarGeneration;
      const role=owned.selectInitialRole(localStorage.getItem(`webchat-char-id:${owned.coreId.value}:${owned.roleOwnerId.value}`) || "");
      if (!role) {
        sidebarPages.clear();
        projectConversationPages.value = {};
        sidebarHasMore.value = false;
        sidebar.value = { pinned: [], recent: [], projects: [], removedProjects: [] };
        return;
      }
      append = append && sidebarRole === role;
      const source = append ? sidebarPages : new Map<string, RolePage>();
      const roles = owned.roles.value.filter((item) => !append || source.get(item.id)?.cursor);
      const [roleConversations, projectPage] = await Promise.all([
        Promise.all(roles.map(async (item) => ({
          role: item.id,
          page: await owned.conversationPage(item.id, "", source.get(item.id)?.cursor, { archive: "active", channel: "web", limit: 36 }),
        }))),
        append ? Promise.resolve(null) : owned.projects(role, historicalProjectRole.value),
      ]);
      if (generation !== sidebarGeneration) return;
      const pages = new Map(source);
      for (const { role: roleId, page } of roleConversations) {
        const rows = new Map(source.get(roleId)?.rows.map((row) => [row.id, row]));
        for (const row of page.rows) rows.set(row.id, row);
        pages.set(roleId, { rows: [...rows.values()], cursor: page.cursor });
      }
      const conversations = [...pages].flatMap(([roleId, page]) => roleId === role
        ? page.rows
        : page.rows.filter((row) => row.source === "reminder" || row.source === "proactive"));
      sidebarPages = pages;
      sidebarRole = role;
      sidebarHasMore.value = [...pages.values()].some((page) => Boolean(page.cursor));
      if (projectPage) {
        projectConversationPages.value = {};
        projectIntent.value = projectPage.executionScope;
        historicalProjectRoles.value = projectPage.historicalRoles;
        historicalProjectRole.value = projectPage.selectedHistoricalRole;
      }
      const rows=new Map<string,ConversationItem>();
      for (const value of conversations) {
        if (!value?.id) continue;
        rows.set(value.id, sidebarConversation(value));
      }
      const visible=[...rows.values()].filter((row)=>!row.archivedAt && row.channel === "web").sort((a,b)=>String(b.updatedAt).localeCompare(String(a.updatedAt)));
      const projects = projectPage
        ? projectPage.projects.map((project) => projectRow(project, visible.filter((row) => row.projectId === ownedProjectReference(project.ownerId, project.id))))
        : [...sidebar.value.projects, ...(sidebar.value.removedProjects || [])].map((project) => ({
          ...project,
          conversations: projectConversationPages.value[project.id]?.loaded ? project.conversations : visible.filter((row) => row.projectId === project.id),
        }));
      const visibleProjects = projects.filter((project) => !project.hiddenAt);
      const removedProjects = projects.filter((project) => project.hiddenAt);
      const projectIds = new Set(projects.map((project) => project.id));
      const ungrouped = visible.filter((row) => !projectIds.has(row.projectId));
      sidebar.value={pinned:ungrouped.filter((row)=>Boolean(row.pinnedAt)),recent:ungrouped.filter((row)=>!row.pinnedAt),projects:visibleProjects,removedProjects};
      return;
    }
    throw new Error(owned.notice.value || "设备对话服务尚未就绪");
  }

  async function fetchSidebar(refreshAfterCurrent = false) {
    if (sidebarFetch) {
      if (refreshAfterCurrent) {
        await sidebarFetch;
        return fetchSidebar();
      }
      return sidebarFetch;
    }
    return fetchSidebarPage(false);
  }

  function fetchSidebarPage(append: boolean) {
    if (sidebarFetch) return sidebarFetch;
    if (append && !sidebarHasMore.value) return Promise.resolve();
    sidebarLoading.value = true;
    const wrapped = loadSidebar(append).finally(() => {
      if (sidebarFetch === wrapped) {
        sidebarFetch = null;
        sidebarLoading.value = false;
      }
    });
    sidebarFetch = wrapped;
    return wrapped;
  }


  async function createProject(input: {
    name: string;
    workspaceId?: string;
    deviceId?: string;
    rootUri?: string;
    workspaces?: ProjectWorkspace[];
    primaryWorkspaceId?: string;
    expectedExecutionScope?: OwnedExecutionScope;
  }): Promise<ProjectItem> {
    if (owned.enabled.value) {
      const expected = input.expectedExecutionScope || projectIntent.value;
      if (!expected) throw new Error("请先加载项目列表，确认数据所属设备");
      const workspaces = input.workspaces || (input.workspaceId && input.rootUri ? [{ id: input.workspaceId, deviceId: input.deviceId || expected.targetDeviceId, name: input.name, kind: input.rootUri.startsWith("content://") ? "saf" : "local", rootUri: input.rootUri }] : []);
      const created = await owned.createProject(input.name, expected, workspaces, input.primaryWorkspaceId || workspaces[0]?.id || "");
      await fetchSidebar();
      return projectRow(created);
    }
    throw new Error(owned.notice.value || "设备对话服务尚未就绪");
  }

  async function updateProject(projectId: string, input: Partial<Pick<ProjectItem, "name" | "workspaces" | "primaryWorkspaceId">> & { pinned?: boolean; hidden?: boolean }, intent?: ProjectItem) {
    if (owned.enabled.value) {
      const project = intent || sidebar.value.projects.find((row) => row.id === projectId);
      if (project?.readOnly && !owned.policy.value?.coordinated) throw new Error("旧项目为只读，请在原设备管理");
      if (!project?.executionScope || !project.ownerId || !project.revision || !project.resourceId) throw new Error("请重新加载项目后修改");
      await owned.edit("project", project.resourceId, { ...(input.name !== undefined ? { title: input.name } : {}), ...(input.pinned !== undefined ? { pinned: input.pinned } : {}), ...(input.hidden !== undefined ? { hidden: input.hidden } : {}), ...(input.workspaces !== undefined ? { workspaces: input.workspaces } : {}), ...(input.primaryWorkspaceId !== undefined ? { primaryWorkspaceId: input.primaryWorkspaceId } : {}) }, { characterId: project.executionScope.roleId, expectedExecutionScope: project.executionScope, expectedOwnerId: project.ownerId, expectedRevision: project.revision });
      await fetchSidebar();
      return;
    }
    throw new Error(owned.notice.value || "设备对话服务尚未就绪");
  }

  async function hideProject(projectId: string, intent?: ProjectItem) {
    await updateProject(projectId, { hidden: true }, intent);
    if (currentProjectId.value === projectId) currentProjectId.value = "";
  }

  async function restoreProject(projectId: string, intent?: ProjectItem) {
    await updateProject(projectId, { hidden: false }, intent);
  }

  async function moveConversation(conversationId: string, projectId: string) {
    await owned.edit("conversation", conversationId, { projectId: String(projectId || "").trim() });
    await fetchSidebar();
  }

  async function renameConversation(conversationId: string, title: string) {
    await owned.edit("conversation",conversationId,{title});
    await fetchSidebar();
  }

  async function setConversationPinned(conversationId: string, pinned: boolean) {
    await owned.edit("conversation",conversationId,{pinned});
    await fetchSidebar();
  }

  async function archiveConversation(conversationId: string) {
    await owned.edit("conversation",conversationId,{archived:true});
    if (currentConversationId.value === conversationId) {
      currentConversationId.value = null;
      currentProjectId.value = "";
    }
    await fetchSidebar();
    archivedRevision.value += 1;
  }

  async function restoreConversation(conversationId: string) {
    await owned.edit("conversation",conversationId,{archived:false});
    await fetchSidebar();
    archivedRevision.value += 1;
  }

  async function deleteConversation(conversationId: string) {
    await owned.edit("conversation",conversationId,{}, {deleted:true});
    if (currentConversationId.value === conversationId) {
      currentConversationId.value = null;
      currentProjectId.value = "";
    }
    await fetchSidebar();
  }

  async function markConversationRead(conversationId: string, lastTerminalTurnSequence = 0, characterId = "") {
    const rows = [sidebar.value.pinned, sidebar.value.recent, ...sidebar.value.projects.map((project) => project.conversations)];
    const conversation = rows.flat().find((row) => row.id === conversationId);
    let terminalSequence = Math.max(0, Number(lastTerminalTurnSequence || conversation?.lastTerminalTurnSequence || 0));
    const readSequence = Math.max(0, Number(conversation?.lastReadTurnSequence || 0));
    if (conversation && !conversation.hasUnread && readSequence >= terminalSequence) return;
    if (owned.enabled.value) {
      if (terminalSequence <= 0) {
        const role = characterId || conversation?.characterId || owned.selectInitialRole();
        if (!role) return;
        const result = await owned.query(conversationId, role);
        const body = result.snapshot.resources.find((resource) => resource.kind === "conversation")?.body as Record<string, any> | undefined;
        terminalSequence = Math.max(0, Number(body?.lastTerminalTurnSequence || 0));
      }
      await owned.edit("conversation",conversationId,{lastReadTurnSequence:terminalSequence}, { characterId: characterId || conversation?.characterId });
    } else {
      throw new Error(owned.notice.value || "设备对话服务尚未就绪");
    }
    const currentRows = [sidebar.value.pinned, sidebar.value.recent, ...sidebar.value.projects.map((project) => project.conversations)];
    const currentConversation = currentRows.flat().find((row) => row.id === conversationId);
    if (currentConversation) {
      currentConversation.lastReadTurnSequence = Math.max(Number(currentConversation.lastReadTurnSequence || 0), terminalSequence);
      if (Number(currentConversation.lastTerminalTurnSequence || 0) <= terminalSequence) {
        currentConversation.hasUnread = false;
        currentConversation.unreadCompletedCount = 0;
      }
    }
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
    fetchSidebarPage,
    fetchProjectConversationPage,
    projectConversationPages,
    sidebarHasMore,
    sidebarLoading,
    createProject,
    updateProject,
    hideProject,
    restoreProject,
    moveConversation,
    renameConversation,
    setConversationPinned,
    archiveConversation,
    restoreConversation,
    deleteConversation,
    markConversationRead,
  };
});
