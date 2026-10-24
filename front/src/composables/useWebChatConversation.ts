// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, watch, type Ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "./useApi";
import { useCachedApi } from "./useCachedApi";
import { useDeviceOwnedConversation } from "./useDeviceOwnedConversation";
import { useChatStore } from "@/stores/chat";
import { useConversationWorkspace } from "./useConversationWorkspace";
import { ownedProjectReference } from "@/runtime/owned-project-reference";

const minimumConversationLoadingMs = 600;

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
  connectSSE: (withSnapshot?: boolean) => unknown,
  loadSnapshot?: () => Promise<boolean>,
  minimumLoadingMs = minimumConversationLoadingMs,
) {
  const { get, del } = useApi();
  const owned = useDeviceOwnedConversation();
  const chatStore = useChatStore();
  const { applySnapshotWorkspace } = useConversationWorkspace();
  const { saveCache } = useCachedApi();

  const characters = ref<any[]>([]);
  const conversations = ref<any[]>([]);
  const memories = ref<any[]>([]);
  const conversationLoading = ref(false);
  let conversationLoadGeneration = 0;

  const webMsgCount = ref(0);

  const showCharPicker = ref(false);
  const showMemories = ref(false);

  function knownConversationTitle(id: string): string {
    const rows = [
      ...chatStore.sidebar.pinned,
      ...chatStore.sidebar.recent,
      ...chatStore.sidebar.projects.flatMap((project) => project.conversations),
    ];
    return String(rows.find((conversation) => conversation.id === id)?.title || "").trim();
  }

  function updateCharacterPresentation(c: any) {
    charName.value = c.name;
    charIdentity.value = c.identity || c.personality || "";
    charAvatar.value = c.avatar || "";
  }

  watch(
    () => characters.value.find((character) => character.id === characterId.value),
    (character) => {
      if (character) updateCharacterPresentation(character);
    },
    { deep: true },
  );

  function selectCharacter(c: any) {
    characterId.value = c.id;
    updateCharacterPresentation(c);
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
    if (owned.enabled.value) await chatStore.fetchSidebar().catch(() => undefined);
  }

  async function loadCharacterConversation() {
    const conversationID = String(convId.value || "").trim();
    const generation = ++conversationLoadGeneration;
    const loadingStartedAt = Date.now();
    disconnectSSE();
    if (!conversationID) {
      if (generation === conversationLoadGeneration) conversationLoading.value = false;
      convTitle.value = "";
      messages.value = [];
      return;
    }
    convId.value = conversationID;
    const titleHint = knownConversationTitle(conversationID);
    if (titleHint) convTitle.value = titleHint;
    messages.value = [];
    hasMoreHistory.value = true;
    conversationLoading.value = true;
    try {
      if (owned.enabled.value) {
        const selectedRole = characterId.value;
        const result = await owned.query(conversationID, selectedRole);
        if (generation !== conversationLoadGeneration || convId.value !== conversationID || characterId.value !== selectedRole) return;
        messages.value = owned.messages(result);
        const conversation = result.snapshot.resources.find((resource) => resource.kind === "conversation")?.body;
        const historicalConversation = result.historicalSnapshot?.resources.find((resource) => resource.kind === "conversation")?.body;
        const grouped = conversation || historicalConversation;
        const owner = conversation ? result.snapshot.ownerId : result.historicalSnapshot?.ownerId;
        applySnapshotWorkspace(null, grouped?.projectId && owner ? ownedProjectReference(owner, grouped.projectId) : "");
        convTitle.value = conversation?.title || "历史对话";
        hasMoreHistory.value = owned.hasMore(conversationID, characterId.value);
        await chatStore.markConversationRead(conversationID, Number(grouped?.lastTerminalTurnSequence || 0)).catch(() => undefined);
        if (loadSnapshot) {
          const snapshotLoaded = await loadSnapshot();
          if (generation !== conversationLoadGeneration || convId.value !== conversationID || characterId.value !== selectedRole) return;
          void connectSSE(!snapshotLoaded);
        } else {
          void connectSSE();
        }
        return;
      }
      throw new Error(owned.notice.value || "设备对话服务尚未就绪");
    } finally {
      const remaining = minimumLoadingMs - (Date.now() - loadingStartedAt);
      if (remaining > 0) await new Promise<void>((resolve) => setTimeout(resolve, remaining));
      if (generation === conversationLoadGeneration && convId.value === conversationID) conversationLoading.value = false;
    }
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
      throw new Error(owned.notice.value || "设备对话服务尚未就绪");
    } catch {
      conversations.value = [];
    }
  }

  async function handleViewMemories() {
    showMemories.value = true;
    await owned.refresh();
  }

  async function fetchWebMsgCount() {
    webMsgCount.value = messages.value.filter((message) => ["user", "assistant"].includes(message.role)).length;
  }

  async function refreshCharacters() {
    try {
      if (owned.enabled.value) {
        await owned.refresh();
        characters.value = owned.roles.value;
        return;
      }
      throw new Error(owned.notice.value || "设备对话服务尚未就绪");
    } catch {}
  }

  async function fetchConvSummary(conversationID = convId.value): Promise<string> {
    const id = String(conversationID || "").trim();
    if (!id) return "";
    try {
      if (owned.enabled.value) {
        const result = await owned.conversationSummary(id, characterId.value);
        return String(result?.summaryText || (result as any)?.summary_text || "").trim();
      }
      throw new Error(owned.notice.value || "设备对话服务尚未就绪");
    } catch {
      return "";
    }
  }

  return {
    characters,
    conversations,
    memories,
    conversationLoading,
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
