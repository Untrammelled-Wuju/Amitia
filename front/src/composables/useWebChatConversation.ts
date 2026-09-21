// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, type Ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "./useApi";
import { useCachedApi } from "./useCachedApi";

export function useWebChatConversation(
  messages: Ref<any[]>,
  convId: Ref<string>,
  characterId: Ref<string>,
  convTitle: Ref<string>,
  charName: Ref<string>,
  charIdentity: Ref<string>,
  charAvatar: Ref<string>,
  hasMoreHistory: Ref<boolean>,
  disconnectSSE: () => void,
  connectSSE: () => void,
) {
  const { get, del } = useApi();
  const { saveCache } = useCachedApi();

  const characters = ref<any[]>([]);
  const conversations = ref<any[]>([]);
  const memories = ref<any[]>([]);

  const webMsgCount = ref(0);

  const showCharPicker = ref(false);
  const showMemories = ref(false);

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
    const conversationID = String(convId.value || "").trim();
    disconnectSSE();
    if (!conversationID) {
      convTitle.value = "";
      messages.value = [];
      return;
    }
    convId.value = conversationID;
    if (!convTitle.value) convTitle.value = "新对话";
    messages.value = [];
    hasMoreHistory.value = true;
    connectSSE();
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
