// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { defineStore } from "pinia";
import { ref } from "vue";
import type { Message } from "@/types";
import { apiClient } from "@/composables/useApi";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";

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
      const role=owned.selectInitialRole();
      if (!role) { sidebar.value={pinned:[],recent:[],projects:[]};return; }
      const conversations=await owned.conversations(role);
      const rows=new Map<string,ConversationItem>();
      for (const value of conversations) {
        if (value?.id) rows.set(value.id,{...value,channel:"web",projectId:value.projectId || "",source:value.source || "device-mesh",messageCount:value.messageCount || 0});
      }
      const visible=[...rows.values()].filter((row)=>!row.archivedAt).sort((a,b)=>String(b.updatedAt).localeCompare(String(a.updatedAt)));
      sidebar.value={pinned:visible.filter((row)=>Boolean(row.pinnedAt)),recent:visible.filter((row)=>!row.pinnedAt),projects:[]};
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
    workspaceId: string;
    deviceId?: string;
    rootUri?: string;
  }): Promise<ProjectItem> {
    const response = await apiClient.post<ProjectItem>("/api/web-chat/projects", input);
    await fetchSidebar();
    return response.data;
  }

  async function updateProject(projectId: string, input: Partial<Pick<ProjectItem, "name" | "workspaceId" | "deviceId" | "rootUri">> & { pinned?: boolean }) {
    await apiClient.patch(`/api/web-chat/projects/${encodeURIComponent(projectId)}`, input);
    await fetchSidebar();
  }

  async function deleteProject(projectId: string) {
    await apiClient.delete(`/api/web-chat/projects/${encodeURIComponent(projectId)}`);
    await fetchSidebar();
  }

  async function moveConversation(conversationId: string, projectId: string) {
    await apiClient.put(`/api/web-chat/conversations/${encodeURIComponent(conversationId)}`, { projectId });
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
