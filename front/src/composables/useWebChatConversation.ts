// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, type Ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "./useApi";
import { useCachedApi } from "./useCachedApi";
import { useDeviceOwnedConversation } from "./useDeviceOwnedConversation";
import { useChatStore } from "@/stores/chat";
import { useConversationWorkspace } from "./useConversationWorkspace";
import { ownedProjectReference } from "@/runtime/owned-project-reference";

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
  const owned = useDeviceOwnedConversation();
  const chatStore = useChatStore();
  const { applySnapshotWorkspace } = useConversationWorkspace();
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
    localStorage.setItem(owned.enabled.value ? `webchat-char-id:${owned.coreId.value}:${owned.roleOwnerId.value}` : "webchat-char-id", c.id);
  }

  async function handleSwitchChar(c: any) {
    try {
      await ElMessageBox.confirm(
        owned.enabled.value ? "切换角色后将打开新对话，已有对话保持原记录。" : "切换后，当前对话的后续消息将使用新角色，历史消息保持不变。",
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
    if (owned.enabled.value) { owned.stopLocal("调用角色已切换，原回复已中断，将使用新角色开始对话"); disconnectSSE(); convId.value = ""; messages.value = []; convTitle.value = ""; }
    showCharPicker.value = false;
    ElMessage.success("已切换角色: " + c.name);
    await fetchConversations();
    if (owned.enabled.value) await chatStore.fetchSidebar();
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
    if (owned.enabled.value) {
      const selectedRole = characterId.value;
      const result = await owned.query(conversationID, selectedRole);
      if (convId.value !== conversationID || characterId.value !== selectedRole) return;
      messages.value = owned.messages(result);
      const conversation = result.snapshot.resources.find((resource) => resource.kind === "conversation")?.body;
      const historicalConversation = result.historicalSnapshot?.resources.find((resource) => resource.kind === "conversation")?.body;
      const grouped = conversation || historicalConversation;
      const owner = conversation ? result.snapshot.ownerId : result.historicalSnapshot?.ownerId;
      applySnapshotWorkspace(null, grouped?.projectId && owner ? ownedProjectReference(owner, grouped.projectId) : "");
      convTitle.value = conversation?.title || "历史对话";
      hasMoreHistory.value = owned.hasMore(conversationID, characterId.value);
      return;
    }
    connectSSE();
  }

  async function fetchConversations() {
    try {
      if (owned.enabled.value) {
        if (!characterId.value) { conversations.value = []; return; }
        const selectedRole = characterId.value;
        const rows = await owned.conversations(selectedRole);
        if (characterId.value !== selectedRole) return;
        conversations.value = rows;
        return;
      }
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
      if (owned.enabled.value) {
        return;
      }
      const r = await get<any>("/api/memories", { page: 1, pageSize: 10 });
      memories.value = r?.items || [];
    } catch {}
  }

  async function fetchWebMsgCount() {
    if (owned.enabled.value) return;
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
      if (owned.enabled.value) {
        await owned.refresh();
        characters.value = owned.roles.value;
        return;
      }
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
      if (owned.enabled.value) {
        const result = await owned.conversationSummary(id, characterId.value);
        return String(result?.summaryText || result?.summary_text || "").trim();
      }
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
