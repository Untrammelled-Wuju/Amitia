// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "./useApi";
import { getDeploymentConfig } from "@/runtime/runtime-adapter";

export interface WorkspaceMountSummary {
  id: string;
  name: string;
  kind: string;
  rootUri: string;
  readOnly: boolean;
  available: boolean;
  status: string;
  statusReason?: string;
  createdAt?: string;
  updatedAt?: string;
  lastUsedAt?: string;
}

export interface ConversationWorkspaceBinding {
  conversationId?: string;
  workspaceId: string;
  deviceId?: string;
  workspaceName?: string;
  workspaceKind: string;
  rootUri: string;
  updatedAt?: string;
}

const activeConversationId = ref("");
const currentWorkspace = ref<ConversationWorkspaceBinding | null>(null);
const recentWorkspaces = ref<WorkspaceMountSummary[]>([]);
const workspaceLoading = ref(false);
let loadEpoch = 0;

function normalizeMount(raw: any): WorkspaceMountSummary | null {
  const id = String(raw?.id || "").trim();
  if (!id) return null;
  return {
    id,
    name: String(raw?.name || id).trim() || id,
    kind: String(raw?.kind || "local").trim() || "local",
    rootUri:
      String(raw?.rootUri || "").trim() || `amitia://workspace/@${id}/`,
    readOnly: Boolean(raw?.readOnly),
    available: raw?.available !== false,
    status: String(raw?.status || "ready"),
    statusReason: String(raw?.statusReason || "").trim() || undefined,
    createdAt: String(raw?.createdAt || "").trim() || undefined,
    updatedAt: String(raw?.updatedAt || "").trim() || undefined,
    lastUsedAt: String(raw?.lastUsedAt || raw?.updatedAt || "").trim() || undefined,
  };
}

function normalizeBinding(raw: any): ConversationWorkspaceBinding | null {
  const workspaceId = String(raw?.workspaceId || "").trim();
  if (!workspaceId) return null;
  return {
    conversationId: String(raw?.conversationId || "").trim() || undefined,
    workspaceId,
    deviceId: String(raw?.deviceId || "").trim() || undefined,
    workspaceName: String(raw?.workspaceName || "").trim() || undefined,
    workspaceKind: String(raw?.workspaceKind || "local").trim() || "local",
    rootUri:
      String(raw?.rootUri || "").trim() ||
      `amitia://workspace/@${workspaceId}/`,
    updatedAt: String(raw?.updatedAt || "").trim() || undefined,
  };
}

async function resolveExecutionDeviceId(): Promise<string> {
  try {
    const identity = await window.amitiaDesktop?.getMeshIdentity?.();
    return String(identity?.deviceId || "").trim();
  } catch {
    return "";
  }
}

async function createBindingFromMount(
  mount: WorkspaceMountSummary,
): Promise<ConversationWorkspaceBinding> {
  const deployment = await getDeploymentConfig();
  const deviceId = await resolveExecutionDeviceId();
  if (deployment.mode === "cloud" && !deviceId) {
    throw new Error("本机 Device Agent 尚未就绪，云端模式无法绑定本地工作目录");
  }
  return {
    conversationId: activeConversationId.value || undefined,
    workspaceId: mount.id,
    deviceId: deviceId || undefined,
    workspaceName: mount.name,
    workspaceKind: mount.kind || "local",
    rootUri: mount.rootUri || `amitia://workspace/@${mount.id}/`,
  };
}

export function useConversationWorkspace() {
  const { get, post, put, del } = useApi();

  async function refreshRecentWorkspaces(): Promise<void> {
    if (!window.amitiaDesktop) {
      recentWorkspaces.value = [];
      return;
    }
    try {
      const raw = await get<any[]>("/api/local/workspaces");
      recentWorkspaces.value = (Array.isArray(raw) ? raw : [])
        .map(normalizeMount)
        .filter((item): item is WorkspaceMountSummary => Boolean(item))
        .filter((item) => item.kind === "local")
        .sort((a, b) => {
          const at = a.lastUsedAt ? Date.parse(a.lastUsedAt) : 0;
          const bt = b.lastUsedAt ? Date.parse(b.lastUsedAt) : 0;
          return bt - at;
        })
        .slice(0, 10);
    } catch {
      recentWorkspaces.value = [];
    }
  }

  async function bindCurrentWorkspaceToConversation(
    conversationId: string,
  ): Promise<void> {
    const id = String(conversationId || "").trim();
    const binding = currentWorkspace.value;
    if (!id || !binding) return;
    const saved = await put<ConversationWorkspaceBinding>(
      `/api/web-chat/conversations/${encodeURIComponent(id)}/workspace`,
      {
        workspaceId: binding.workspaceId,
        deviceId: binding.deviceId || "",
        workspaceName: binding.workspaceName || "",
        workspaceKind: binding.workspaceKind || "local",
        rootUri: binding.rootUri,
      },
    );
    const normalized = normalizeBinding(saved);
    if (normalized && activeConversationId.value === id) {
      currentWorkspace.value = normalized;
    }
  }

  async function selectWorkspaceMount(
    mount: WorkspaceMountSummary,
  ): Promise<void> {
    if (!mount.available) {
      throw new Error(mount.statusReason || "该工作目录当前不可用");
    }
    workspaceLoading.value = true;
    try {
      const binding = await createBindingFromMount(mount);
      currentWorkspace.value = binding;
      try {
        await post(`/api/local/workspaces/${encodeURIComponent(mount.id)}/touch`);
      } catch {
        // Touch is only an MRU hint; the binding itself remains valid.
      }
      if (activeConversationId.value) {
        await bindCurrentWorkspaceToConversation(activeConversationId.value);
      }
      await refreshRecentWorkspaces();
    } finally {
      workspaceLoading.value = false;
    }
  }

  async function chooseWorkspaceDirectory(): Promise<void> {
    if (!window.amitiaDesktop?.selectWorkspaceDirectory) {
      ElMessage.warning("当前环境不支持直接选择本机工作目录");
      return;
    }
    workspaceLoading.value = true;
    try {
      const selection = await window.amitiaDesktop.selectWorkspaceDirectory();
      if (!selection?.path) return;
      const created = await post<any>("/api/local/workspaces/local", {
        name: String(selection.name || "").trim() || "工作目录",
        localRoot: selection.path,
        readOnly: false,
      });
      const mount = normalizeMount(created);
      if (!mount) throw new Error("工作目录注册失败：后端未返回有效 Workspace");
      const binding = await createBindingFromMount(mount);
      currentWorkspace.value = binding;
      if (activeConversationId.value) {
        await bindCurrentWorkspaceToConversation(activeConversationId.value);
      }
      await refreshRecentWorkspaces();
    } catch (error) {
      const message = error instanceof Error ? error.message : "选择工作目录失败";
      ElMessage.error(message);
      throw error;
    } finally {
      workspaceLoading.value = false;
    }
  }

  async function clearWorkspace(): Promise<void> {
    const id = activeConversationId.value;
    workspaceLoading.value = true;
    try {
      if (id) {
        await del(`/api/web-chat/conversations/${encodeURIComponent(id)}/workspace`);
      }
      currentWorkspace.value = null;
    } finally {
      workspaceLoading.value = false;
    }
  }

  async function loadConversationWorkspace(conversationId: string): Promise<void> {
    const id = String(conversationId || "").trim();
    activeConversationId.value = id;
    const epoch = ++loadEpoch;
    if (!id) {
      currentWorkspace.value = null;
      await refreshRecentWorkspaces();
      return;
    }
    workspaceLoading.value = true;
    try {
      const raw = await get<any>(
        `/api/web-chat/conversations/${encodeURIComponent(id)}/workspace`,
      );
      if (epoch !== loadEpoch || activeConversationId.value !== id) return;
      currentWorkspace.value = normalizeBinding(raw);
      await refreshRecentWorkspaces();
    } catch {
      if (epoch === loadEpoch && activeConversationId.value === id) {
        currentWorkspace.value = null;
      }
    } finally {
      if (epoch === loadEpoch) workspaceLoading.value = false;
    }
  }

  function getWorkspaceRequestFields(): Record<string, string> {
    const binding = currentWorkspace.value;
    if (!binding) return {};
    return {
      workspaceId: binding.workspaceId,
      workspaceDeviceId: binding.deviceId || "",
      workspaceName: binding.workspaceName || "",
      workspaceKind: binding.workspaceKind || "local",
      workspaceRootUri: binding.rootUri,
    };
  }

  return {
    activeConversationId,
    currentWorkspace,
    recentWorkspaces,
    workspaceLoading,
    refreshRecentWorkspaces,
    loadConversationWorkspace,
    chooseWorkspaceDirectory,
    selectWorkspaceMount,
    clearWorkspace,
    bindCurrentWorkspaceToConversation,
    getWorkspaceRequestFields,
  };
}
