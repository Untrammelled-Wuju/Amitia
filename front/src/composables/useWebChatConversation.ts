// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, type Ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "./useApi";
import { useCachedApi } from "./useCachedApi";
import {
  compareChatMessages,
  getClientMessageId,
  getMessageUIKey,
  normalizeRealtimeMessage,
} from "@/utils/message-order";

export function useWebChatConversation(
  messages: Ref<any[]>,
  convId: Ref<string>,
  characterId: Ref<string>,
  convTitle: Ref<string>,
  charName: Ref<string>,
  charIdentity: Ref<string>,
  charAvatar: Ref<string>,
  hasMoreHistory: Ref<boolean>,
  msgPage: Ref<number>,
  scrollToBottom: (smooth?: boolean) => void,
  disconnectSSE: () => void,
  connectSSE: () => void,
  setLastPolledMsgId: (id: string | null) => void,
  setCurrentCharName: (name: string) => void,
) {
  const { get, del } = useApi();
  const { cachedGet, saveCache, invalidateCache } = useCachedApi();

  const characters = ref<any[]>([]);
  const conversations = ref<any[]>([]);
  const memories = ref<any[]>([]);

  const webMsgCount = ref(0);

  const showCharPicker = ref(false);
  const showMemories = ref(false);

  const HISTORY_PAGE_SIZE = 50;
  let messagesVersion = 0;

  function isLocalMessage(m: any) {
    const id = String(m.id || "");
    return id.startsWith("user-") || id.startsWith("failed-");
  }

  async function conversationExistsOnServer(conversationID: string): Promise<boolean> {
    try {
      const page = await get<any>("/api/web-chat/conversations", {
        page: 1,
        pageSize: 100,
      });
      const items = page?.conversations || page?.items || [];
      const total = Number(page?.total ?? items.length);
      if (total > items.length) return true;
      return items.some((item: any) => String(item?.id || "") === conversationID);
    } catch {
      return true;
    }
  }

  async function fetchLatestMessagesPage(conversationID: string) {
    const url = `/api/web-chat/conversations/${encodeURIComponent(conversationID)}/messages`;
    const first = await get<any>(url, { page: 1, pageSize: HISTORY_PAGE_SIZE });
    const totalPages = Math.max(1, Number(first?.totalPages || 1));
    if (totalPages <= 1) {
      return { response: first, page: 1, totalPages };
    }
    const latest = await get<any>(url, {
      page: totalPages,
      pageSize: HISTORY_PAGE_SIZE,
    });
    return { response: latest, page: totalPages, totalPages };
  }

  function mergeMessages(serverItems: any[]) {
    const serverMap = new Map<string, any>();
    for (const item of serverItems) {
      if (item.id) serverMap.set(String(item.id), item);
    }
    const localOnly = messages.value.filter((m) => isLocalMessage(m));
    const pendingKey = `uai-pending-msg:${convId.value}`;
    let pendingMsg: any = null;
    try {
      const raw = sessionStorage.getItem(pendingKey);
      if (raw) pendingMsg = JSON.parse(raw);
    } catch {}
    if (pendingMsg && !serverMap.has(String(pendingMsg.id)) && !localOnly.some((m: any) => m.id === pendingMsg.id)) {
      localOnly.push(pendingMsg);
    }
    const currentById = new Map<string, any>();
    const currentByClientMessageId = new Map<string, any>();
    for (const current of messages.value) {
      const id = String(current?.id || "");
      const clientMessageId = getClientMessageId(current);
      if (id) currentById.set(id, current);
      if (clientMessageId) currentByClientMessageId.set(clientMessageId, current);
    }
    const merged = serverItems.map((raw: any) => {
      const m = normalizeRealtimeMessage(raw);
      const existing =
        currentById.get(String(m.id || "")) ||
        currentByClientMessageId.get(getClientMessageId(m));
      const next = {
        ...existing,
        ...m,
        clientMessageId:
          getClientMessageId(m) || getClientMessageId(existing) || undefined,
        uiKey: existing?.uiKey || getMessageUIKey(m),
        animateIn: existing?.animateIn ?? false,
      };
      if (next.imageUrl && next.content === "[图片]") return { ...next, content: "" };
      return next;
    });
    for (const local of localOnly) {
      if (!serverMap.has(String(local.id))) {
        merged.push(local);
      }
    }
    merged.sort(compareChatMessages);
    messages.value = merged;
  }

  function selectCharacter(c: any) {
    characterId.value = c.id;
    charName.value = c.name;
    charIdentity.value = c.identity || c.personality || "";
    charAvatar.value = c.avatar || "";
    localStorage.setItem("webchat-char-id", c.id);
  }

  async function handleSwitchChar(c: any) {
    try {
      await ElMessageBox.confirm(
        "切换后，当前对话的后续消息将使用新角色，历史消息保持不变。",
        "选择角色",
        {
          confirmButtonText: "确认切换",
          cancelButtonText: "取消",
          type: "warning",
        },
      );
    } catch {
      return;
    }
    selectCharacter(c);
    showCharPicker.value = false;
    ElMessage.success("已切换角色: " + c.name);
    await fetchConversations();
  }

  async function loadCharacterConversation() {
    let conversationID = String(convId.value || "").trim();
    if (conversationID && !(await conversationExistsOnServer(conversationID))) {
      conversationID = "";
    }
    if (!conversationID) {
      disconnectSSE();
      convId.value = "";
      convTitle.value = "";
      messages.value = [];
      setLastPolledMsgId(null);
      return;
    }
    disconnectSSE();
    convId.value = conversationID;
    if (!convTitle.value) convTitle.value = "新对话";
    const version = ++messagesVersion;
    try {
      const latestPage = await fetchLatestMessagesPage(conversationID);
      if (version !== messagesVersion) return;
      const r = latestPage.response;
      const items = r?.messages || r?.items || [];
      msgPage.value = latestPage.page;
      hasMoreHistory.value = latestPage.page > 1;
      if (items.length) {
        mergeMessages(items);
        scrollToBottom();
      } else {
        messages.value = [];
      }
      setLastPolledMsgId(messages.value[messages.value.length - 1]?.id || null);
      connectSSE();
    } catch {
      if (version !== messagesVersion) return;
      if (messages.value.length === 0) messages.value = [];
    }
  }

  async function fetchConversations() {
    try {
      const r = await get<any>("/api/web-chat/conversations", {
        pageSize: 100,
      });
      const items = r?.conversations || r?.items || [];
      conversations.value = items;
      const webConv = items.find((x: any) => x.id === convId.value);
      if (webConv) webMsgCount.value = webConv?.messageCount || 0;
    } catch {
      conversations.value = [];
    }
  }

  async function handleViewMemories() {
    showMemories.value = true;
    try {
      const r = await get<any>("/api/memories", { page: 1, pageSize: 10 });
      memories.value = r?.items || [];
    } catch {}
  }

  async function fetchWebMsgCount() {
    if (!convId.value) return;
    try {
      const convs = await get<any>("/api/web-chat/conversations", {
        pageSize: 50,
      });
      const items = convs?.conversations || convs?.items || [];
      const wc = items.find((x: any) => x.id === convId.value);
      if (wc) webMsgCount.value = wc?.messageCount || 0;
    } catch {}
  }

  async function refreshCharacters() {
    try {
      const chars = await get<any[]>("/api/characters");
      if (Array.isArray(chars)) {
        characters.value = chars;
        saveCache("/api/characters", chars);
        if (characterId.value) {
          const current = chars.find((c: any) => c.id === characterId.value);
          if (current) selectCharacter(current);
        }
      }
    } catch {}
  }

  async function fetchConvSummary(conversationID = convId.value): Promise<string> {
    const id = String(conversationID || "").trim();
    if (!id) return "";
    try {
      const response = await get<any>(
        `/api/chats/conversations/${encodeURIComponent(id)}/summary`,
      );
      return String(
        response?.summaryText ??
          response?.summary_text ??
          response?.summary ??
          "",
      ).trim();
    } catch {
      return "";
    }
  }

  return {
    characters,
    conversations,
    memories,
    webMsgCount,
    showCharPicker,
    showMemories,
    selectCharacter,
    handleSwitchChar,
    loadCharacterConversation,
    fetchConversations,
    handleViewMemories,
    fetchWebMsgCount,
    refreshCharacters,
    fetchConvSummary,
  };
}
