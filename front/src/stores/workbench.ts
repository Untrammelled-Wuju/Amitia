import { defineStore } from "pinia";
import { computed, ref, watch } from "vue";

export type WorkbenchViewKind = "files" | "file" | "git" | "diff" | "browser" | "terminal" | "tasks";
export type WorkbenchFocusPolicy = "activate" | "background";

export interface WorkbenchTab {
  id: string;
  kind: WorkbenchViewKind;
  title: string;
  resourceUri: string;
  workspaceUri: string;
  targetDeviceId: string;
  browserRef?: string;
  taskId?: string;
  createdAt: number;
  pinned: boolean;
}

export interface WorkbenchOpenRequest {
  kind: WorkbenchViewKind;
  title?: string;
  resourceUri?: string;
  workspaceUri?: string;
  targetDeviceId?: string;
  browserRef?: string;
  taskId?: string;
  focus?: WorkbenchFocusPolicy;
  forceNew?: boolean;
  operationId?: string;
}

export interface WorkbenchOpenCommand extends WorkbenchOpenRequest {
  protocolVersion: 1;
  conversationId: string;
}

interface WorkbenchSession {
  tabs: WorkbenchTab[];
  activeTabId: string;
  visible: boolean;
  width: number;
  operations: string[];
}

const storageKey = "amitia:workbench:layout:v1";
const defaultWidth = 550;
const maxTabs = 40;
const allowedKinds: readonly WorkbenchViewKind[] = ["files", "file", "git", "diff", "browser", "terminal", "tasks"];
const emptySession = (): WorkbenchSession => ({ tabs: [], activeTabId: "", visible: false, width: defaultWidth, operations: [] });
const normalizeConversation = (value: string) => value.trim() || "__draft__";
const clampWidth = (value: number) => Number.isFinite(value) ? Math.min(1000, Math.max(350, value)) : defaultWidth;
const clampText = (value: unknown, max: number) => typeof value === "string" ? value.trim().slice(0, max) : "";

function parseSession(value: unknown): WorkbenchSession {
  if (!value || typeof value !== "object") return emptySession();
  const input = value as Record<string, unknown>;
  const tabs: WorkbenchTab[] = [];
  const seen = new Set<string>();
  if (Array.isArray(input.tabs)) {
    for (const row of input.tabs.slice(0, maxTabs)) {
      if (!row || typeof row !== "object") continue;
      const tab = row as Record<string, unknown>;
      const id = clampText(tab.id, 100);
      if (!id || seen.has(id) || !allowedKinds.includes(tab.kind as WorkbenchViewKind)) continue;
      seen.add(id);
      tabs.push({
        id,
        kind: tab.kind as WorkbenchViewKind,
        title: clampText(tab.title, 140) || String(tab.kind),
        resourceUri: clampText(tab.resourceUri, 4096),
        workspaceUri: clampText(tab.workspaceUri, 4096),
        targetDeviceId: clampText(tab.targetDeviceId, 140),
        browserRef: clampText(tab.browserRef, 64),
        taskId: tab.kind === "terminal" ? clampText(tab.taskId, 128) : "",
        createdAt: Number.isFinite(tab.createdAt) ? Number(tab.createdAt) : Date.now(),
        pinned: tab.pinned === true,
      });
    }
  }
  const activeTabId = tabs.some((tab) => tab.id === input.activeTabId) ? String(input.activeTabId) : (tabs[0]?.id || "");
  const operations = Array.isArray(input.operations)
    ? [...new Set(input.operations.filter((row): row is string => typeof row === "string" && row.length > 0 && row.length <= 140))].slice(-200)
    : [];
  return { tabs, activeTabId, visible: input.visible === true && tabs.length > 0, width: clampWidth(Number(input.width)), operations };
}

function loadSessions(): Record<string, WorkbenchSession> {
  if (typeof localStorage === "undefined") return {};
  try {
    const raw = JSON.parse(localStorage.getItem(storageKey) || "{}");
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {};
    const result: Record<string, WorkbenchSession> = Object.create(null);
    for (const [key, value] of Object.entries(raw).slice(0, 120)) {
      if (!key || key.length > 200 || key === '__proto__' || key === 'constructor' || key === 'prototype') continue;
      result[key] = parseSession(value);
    }
    return result;
  } catch {
    return {};
  }
}

export const useWorkbenchStore = defineStore("workbench", () => {
  const conversationId = ref("__draft__");
  const characterId = ref('');
  const sessions = ref<Record<string, WorkbenchSession>>(loadSessions());
  const session = computed(() => sessions.value[conversationId.value] || emptySession());
  const tabs = computed(() => session.value.tabs);
  const activeTab = computed(() => tabs.value.find((tab) => tab.id === session.value.activeTabId) || null);
  const visible = computed(() => session.value.visible);
  const width = computed(() => session.value.width);

  function mutateFor(key: string, update: (value: WorkbenchSession) => void) {
    const value = parseSession(sessions.value[key] || emptySession());
    update(value);
    sessions.value = { ...sessions.value, [key]: value };
  }

  function mutate(update: (value: WorkbenchSession) => void) {
    mutateFor(conversationId.value, update);
  }

  function setConversation(id: string, roleId = '') {
    characterId.value = roleId;
    const next = normalizeConversation(id);
    if (next === '__proto__' || next === 'constructor' || next === 'prototype') return;
    const previous = conversationId.value;
    if (previous === "__draft__" && next !== "__draft__" && !sessions.value[next] && sessions.value.__draft__?.tabs.length) {
      const migrated = parseSession(sessions.value.__draft__);
      const updated = { ...sessions.value };
      delete updated.__draft__;
      updated[next] = migrated;
      sessions.value = updated;
    }
    conversationId.value = next;
  }

  function makeTabId() {
    return typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
      ? crypto.randomUUID()
      : `tab-${Date.now()}-${Math.random().toString(36).slice(2)}`;
  }

  function openInConversation(key: string, request: WorkbenchOpenRequest): WorkbenchTab {
    if (!request || !allowedKinds.includes(request.kind)) throw new Error("不支持的工作区视图");
    const resourceUri = clampText(request.resourceUri, 4096);
    const workspaceUri = clampText(request.workspaceUri, 4096);
    const targetDeviceId = clampText(request.targetDeviceId, 140);
    const browserRef = request.kind === "browser" ? clampText(request.browserRef, 64) : "";
    const taskId = request.kind === "terminal" ? clampText(request.taskId, 128) : "";
    let result: WorkbenchTab | null = null;
    mutateFor(key, (value) => {
      const existing = request.forceNew ? undefined : value.tabs.find((tab) => tab.kind === request.kind && tab.resourceUri === resourceUri && tab.workspaceUri === workspaceUri && tab.targetDeviceId === targetDeviceId && (tab.browserRef || "") === browserRef && (tab.taskId || "") === taskId);
      if (existing) {
        result = existing;
      } else {
        if (value.tabs.length >= maxTabs) throw new Error(`单个对话最多打开 ${maxTabs} 个工作区标签`);
        const tab: WorkbenchTab = {
          id: makeTabId(), kind: request.kind,
          title: clampText(request.title, 140) || ({ files: "文件", file: "文件预览", git: "Git", diff: "代码变更", browser: "浏览器", terminal: "终端输出", tasks: "任务状态" }[request.kind]),
          resourceUri, workspaceUri, targetDeviceId, browserRef, taskId, createdAt: Date.now(), pinned: false,
        };
        value.tabs.push(tab);
        result = tab;
      }
      if (request.focus !== "background" || !value.activeTabId) {
        value.activeTabId = result!.id;
      }
      if (request.focus !== "background") value.visible = true;
    });
    return result!;
  }

  function openTab(request: WorkbenchOpenRequest): WorkbenchTab {
    return openInConversation(conversationId.value, request);
  }

  function openCommand(input: WorkbenchOpenCommand): WorkbenchTab | null {
    if (!input || input.protocolVersion !== 1 || typeof input.conversationId !== "string" || !/^[a-zA-Z0-9_:.-]{1,160}$/.test(input.conversationId)) return null;
    if (!allowedKinds.includes(input.kind) || (input.focus !== undefined && input.focus !== "activate" && input.focus !== "background")) return null;
    if (input.browserRef && (input.kind !== "browser" || !/^[A-Za-z0-9_-]{32}$/.test(input.browserRef))) return null;
    if (input.taskId && (input.kind !== "terminal" || !/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(input.taskId))) return null;
    if (input.kind === "tasks" && (input.resourceUri || input.workspaceUri || input.targetDeviceId || input.browserRef || input.taskId)) return null;
    if (input.kind === "browser" && input.resourceUri) {
      try {
        const parsed = new URL(input.resourceUri);
        if (!["http:", "https:"].includes(parsed.protocol) || parsed.username || parsed.password || !parsed.hostname) return null;
      } catch { return null; }
    }
    const key = normalizeConversation(input.conversationId);
    if (key === '__proto__' || key === 'constructor' || key === 'prototype') return null;
    const operationId = clampText(input.operationId, 140);
    if (operationId && sessions.value[key]?.operations.includes(operationId)) return null;
    const tab = openInConversation(key, input);
    if (operationId) mutateFor(key, (value) => {
      value.operations = [...value.operations.filter((id) => id !== operationId).slice(-199), operationId];
    });
    return tab;
  }

  function activateTab(id: string) {
    mutate((value) => {
      if (value.tabs.some((tab) => tab.id === id)) {
        value.activeTabId = id;
        value.visible = true;
      }
    });
  }

  function updateTab(id: string, update: Partial<Pick<WorkbenchTab, "title" | "resourceUri">>) {
    mutate((value) => {
      const tab = value.tabs.find((row) => row.id === id);
      if (!tab) return;
      if (update.title !== undefined) tab.title = clampText(update.title, 140) || tab.title;
      if (update.resourceUri !== undefined) tab.resourceUri = clampText(update.resourceUri, 4096);
    });
  }

  function setPinned(id: string, pinned: boolean) {
    mutate((value) => {
      const tab = value.tabs.find((row) => row.id === id);
      if (tab) tab.pinned = pinned;
    });
  }

  function moveTab(id: string, offset: number) {
    mutate((value) => {
      const from = value.tabs.findIndex((row) => row.id === id);
      if (from < 0 || !Number.isInteger(offset)) return;
      const to = Math.max(0, Math.min(value.tabs.length - 1, from + offset));
      if (to === from) return;
      const [tab] = value.tabs.splice(from, 1);
      value.tabs.splice(to, 0, tab);
    });
  }

  function closeTab(id: string, force = false) {
    mutate((value) => {
      const index = value.tabs.findIndex((row) => row.id === id);
      if (index < 0 || (value.tabs[index].pinned && !force)) return;
      value.tabs.splice(index, 1);
      if (value.activeTabId === id) value.activeTabId = value.tabs[Math.min(index, value.tabs.length - 1)]?.id || "";
      if (!value.tabs.length) value.visible = false;
    });
  }

  function closeAll(force = false) {
    mutate((value) => {
      value.tabs = force ? [] : value.tabs.filter((tab) => tab.pinned);
      if (!value.tabs.some((tab) => tab.id === value.activeTabId)) value.activeTabId = value.tabs[0]?.id || "";
      if (!value.tabs.length) value.visible = false;
    });
  }

  function toggleVisible(force?: boolean) {
    mutate((value) => {
      if (!value.tabs.length && force !== false) {
        value.tabs.push({ id: makeTabId(), kind: "files", title: "文件", resourceUri: "", workspaceUri: "", targetDeviceId: "", createdAt: Date.now(), pinned: false });
        value.activeTabId = value.tabs[0].id;
      }
      value.visible = typeof force === "boolean" ? (force && value.tabs.length > 0) : !value.visible;
    });
  }

  function setWidth(next: number) {
    mutate((value) => { value.width = clampWidth(next); });
  }

  let persistTimer: ReturnType<typeof setTimeout> | undefined;
  function persistSessions() {
    if (persistTimer !== undefined) clearTimeout(persistTimer);
    persistTimer = undefined;
    if (typeof localStorage === "undefined") return;
    try { localStorage.setItem(storageKey, JSON.stringify(sessions.value)); } catch {}
  }
  watch(sessions, () => {
    if (persistTimer !== undefined) clearTimeout(persistTimer);
    persistTimer = setTimeout(persistSessions, 180);
  }, { deep: true });
  if (typeof window !== "undefined") window.addEventListener("pagehide", persistSessions);

  return { conversationId, characterId, tabs, activeTab, visible, width, setConversation, openTab, openCommand, activateTab, updateTab, setPinned, moveTab, closeTab, closeAll, toggleVisible, setWidth, persistSessions };
});
