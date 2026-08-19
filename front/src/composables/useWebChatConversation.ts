// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, type Ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "./useApi";
import { useCachedApi } from "./useCachedApi";
import {
  compareChatMessages,
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
  const { get, post, del } = useApi();
  const { cachedGet, saveCache, invalidateCache } = useCachedApi();

  const characters = ref<any[]>([]);
  const conversations = ref<any[]>([]);
  const importBatches = ref<any[]>([]);
  const memories = ref<any[]>([]);

  const isWechatActive = ref(false);
  const wechatOnline = ref(false);
  const wechatMsgCount = ref(0);
  const isQQActive = ref(false);
  const qqOnline = ref(false);
  const qqMsgCount = ref(0);
  const webMsgCount = ref(0);

  const showDrawer = ref(false);
  const showCharPicker = ref(false);
  const showMemories = ref(false);

  const HISTORY_PAGE_SIZE = 50;
  let messagesVersion = 0;

  let __fsLast = 0;
  let __wcfLast = 0;

  function isLocalMessage(m: any) {
    const id = String(m.id || "");
    return id.startsWith("user-") || id.startsWith("failed-");
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
    const merged = serverItems.map((raw: any) => {
      const m = normalizeRealtimeMessage(raw);
      if (m.imageUrl && m.content === "[图片]") return { ...m, content: "" };
      return m;
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
        "切换角色后，将加载新角色的对话记录。",
        "切换角色",
        {
          confirmButtonText: "确认切换",
          cancelButtonText: "取消",
          type: "warning",
        },
      );
    } catch {
      return;
    }
    isWechatActive.value = false;
    isQQActive.value = false;
    localStorage.setItem("webchat-last-conv", "char");
    localStorage.removeItem("webchat-conv-id");
    selectCharacter(c);
    showCharPicker.value = false;
    ElMessage.success("已切换角色: " + c.name);
    await loadCharacterConversation();
    if (!convId.value) messages.value = [];
    fetchConversations();
  }

  async function loadCharacterConversation() {
    if (!characterId.value) return;
    const c = characters.value.find((x: any) => x.id === characterId.value);
    let dedicatedConvId = localStorage.getItem("webchat-conv-id") || c?.conversationId;
    if (!dedicatedConvId) {
      try {
        const created = await post<any>("/api/web-chat/conversations", {
          characterId: characterId.value,
          title: "",
        });
        if (created?.id) dedicatedConvId = created.id;
      } catch {}
    }
    if (!dedicatedConvId) {
      disconnectSSE();
      convId.value = "";
      convTitle.value = "";
      messages.value = [];
      setLastPolledMsgId(null);
      return;
    }
    disconnectSSE();
    convId.value = dedicatedConvId;
    convTitle.value = c?.name ? `${c.name} 的对话` : "";
    const version = ++messagesVersion;
    try {
      const latestPage = await fetchLatestMessagesPage(dedicatedConvId);
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
    if (!characterId.value) {
      conversations.value = [];
      return;
    }
    try {
      const r = await get<any>("/api/web-chat/conversations", {
        pageSize: 100,
      });
      const items = r?.conversations || r?.items || [];
      conversations.value = items;
      const wc =
        items.find(
          (x: any) =>
            x.channel === "wechat" && (x.messageCount > 0 || x.msgCount > 0),
        ) ||
        items.find(
          (x: any) => x.id.startsWith("conv-") && x.channel === "wechat",
        ) ||
        items.find((x: any) => x.channel === "wechat");
      wechatMsgCount.value = wc?.messageCount || wc?.msgCount || 0;
      const qc =
        items.find(
          (x: any) =>
            x.channel === "qq" && (x.messageCount > 0 || x.msgCount > 0),
        ) ||
        items.find(
          (x: any) => x.id.startsWith("conv-") && x.channel === "qq",
        ) ||
        items.find((x: any) => x.channel === "qq");
      qqMsgCount.value = qc?.messageCount || qc?.msgCount || 0;
      const webConv = items.find((x: any) => x.id === convId.value);
      if (webConv) webMsgCount.value = webConv?.messageCount || 0;
    } catch {
      conversations.value = [];
    }
  }

  async function handleSelectConv(conv: any) {
    showDrawer.value = false;
    const convIdStr = String(conv?.id || "");
    isWechatActive.value = conv?.channel === "wechat";
    isQQActive.value = conv?.channel === "qq";
    disconnectSSE();
    convId.value = conv.id;
    convTitle.value =
      conv?.channel === "qq"
        ? "QQ聊天"
        : conv?.channel === "wechat"
          ? "微信聊天"
          : conv.title || "";
    msgPage.value = 1;
    hasMoreHistory.value = false;
    const version = ++messagesVersion;
    try {
      const latestPage = await fetchLatestMessagesPage(String(conv.id));
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
      messages.value = [];
    }
  }

  async function handleSelectWechat(skipConfirm = false) {
    console.log("[handleSelectWechat] called, skipConfirm =", skipConfirm);
    if (!skipConfirm) {
      try {
        await ElMessageBox.confirm("将切换到微信对话。", "切换对话", {
          confirmButtonText: "确认切换",
          cancelButtonText: "取消",
          type: "info",
        });
      } catch (e: any) {
        if (e !== "cancel" && e !== "close")
      console.error("[handleSelectWechat] confirm error:", e);
        return;
      }
    }
    showDrawer.value = false;
    localStorage.removeItem("webchat-conv-id");
    try {
      const convs = await get<any>("/api/web-chat/conversations", {
        pageSize: 50,
      });
      const items = convs?.conversations || convs?.items || [];
      const wc =
        items.find(
          (x: any) =>
            x.channel === "wechat" && (x.messageCount > 0 || x.msgCount > 0),
        ) ||
        items.find(
          (x: any) => x.id.startsWith("conv-") && x.channel === "wechat",
        ) ||
        items.find((x: any) => x.channel === "wechat") ||
        items.find((x: any) => x.id === "channel-wechat");
      if (wc) {
        localStorage.setItem("webchat-last-conv", "wechat");
        const cid = wc.characterId || wc.character_id;
        if (cid) {
          const c = characters.value.find((x: any) => x.id === cid);
          if (c) selectCharacter(c);
        }
        if (!characterId.value || !charName.value) {
          const fallback =
            characters.value.find((x: any) => x.isDefault) ||
            characters.value.find((x: any) => x.isActive) ||
            characters.value[0];
          if (fallback) selectCharacter(fallback);
        }
        await handleSelectConv(wc);
        return;
      }
      const defaultChar = characters.value.find(
        (c: any) => c.isDefault || c.isActive,
      );
      const created = await post<any>("/api/web-chat/conversations", {
        title: "微信对话",
        channel: "wechat",
        characterId: defaultChar?.id || characterId.value || "",
      });
      if (created?.id) {
        await handleSelectConv(created);
        return;
      }
    } catch (e: any) {
      console.error("[handleSelectWechat]", e);
    }
    ElMessage.warning("未找到微信对话");
  }

  async function handleSelectQQ(skipConfirm = false) {
    console.log("[handleSelectQQ] called, skipConfirm =", skipConfirm);
    if (!skipConfirm) {
      try {
        await ElMessageBox.confirm("将切换到QQ对话。", "切换对话", {
          confirmButtonText: "确认切换",
          cancelButtonText: "取消",
          type: "info",
        });
      } catch (e: any) {
        if (e !== "cancel" && e !== "close")
      console.error("[handleSelectQQ] confirm error:", e);
        return;
      }
    }
    showDrawer.value = false;
    localStorage.removeItem("webchat-conv-id");
    try {
      if (!qqOnline.value) {
        ElMessage.warning("QQ未连接，仅展示历史消息");
      }
      const convs = await get<any>("/api/web-chat/conversations", {
        pageSize: 50,
      });
      const items = convs?.conversations || convs?.items || [];
      const qc =
        items.find(
          (x: any) =>
            x.channel === "qq" && (x.messageCount > 0 || x.msgCount > 0),
        ) ||
        items.find(
          (x: any) => x.id.startsWith("conv-") && x.channel === "qq",
        ) ||
        items.find((x: any) => x.channel === "qq") ||
        items.find((x: any) => x.id === "channel-qq");
      if (qc) {
        localStorage.setItem("webchat-last-conv", "qq");
        const cid = qc.characterId || qc.character_id;
        if (cid) {
          const c = characters.value.find((x: any) => x.id === cid);
          if (c) selectCharacter(c);
        }
        if (!characterId.value || !charName.value) {
          const fallback =
            characters.value.find((x: any) => x.isDefault) ||
            characters.value.find((x: any) => x.isActive) ||
            characters.value[0];
          if (fallback) selectCharacter(fallback);
        }
        await handleSelectConv(qc);
        return;
      }
      const defaultChar = characters.value.find(
        (c: any) => c.isDefault || c.isActive,
      );
      const created = await post<any>("/api/web-chat/conversations", {
        title: "QQ对话",
        channel: "qq",
        characterId: defaultChar?.id || characterId.value || "",
      });
      if (created?.id) {
        await handleSelectConv(created);
        return;
      }
    } catch (e: any) {
      console.error("[handleSelectQQ]", e);
    }
    ElMessage.warning("未找到QQ对话");
  }

  async function handleContinueImport(batch: any) {
    showDrawer.value = false;
    if (!batch?.id) return;
    try {
      await post<any>("/api/web-chat/conversations/from-import", {
        conversationId: batch.id,
      });
      await handleSelectConv({
        ...batch,
        id: batch.id,
        channel: batch.channel || "web",
        source: batch.source || "import",
        characterId: batch.characterId || batch.character_id || characterId.value,
      });
      ElMessage.success("已切换到导入记录对话");
    } catch (error: any) {
      ElMessage.error(error?.response?.data?.msg || "无法继续导入记录对话");
    }
  }

  async function handleViewMemories() {
    showMemories.value = true;
    try {
      const r = await get<any>("/api/memories", { page: 1, pageSize: 10 });
      memories.value = r?.items || [];
    } catch {}
  }

  async function fetchWechatMsgCount() {
    if (Date.now() - __wcfLast < 8000) return;
    __wcfLast = Date.now();
    try {
      const r = await get<any>("/api/wechat/status");
      const status = r?.data || r;
      wechatOnline.value =
        status?.status === "connected" || status?.accountId != null;
    } catch {}
    try {
      const convs = await get<any>("/api/web-chat/conversations", {
        pageSize: 50,
      });
      const items = convs?.conversations || convs?.items || [];
      const wc =
        items.find(
          (x: any) =>
            x.channel === "wechat" && (x.messageCount > 0 || x.msgCount > 0),
        ) ||
        items.find(
          (x: any) => x.id.startsWith("conv-") && x.channel === "wechat",
        ) ||
        items.find((x: any) => x.channel === "wechat");
      if (wc) wechatMsgCount.value = wc?.messageCount || wc?.msgCount || 0;
    } catch {}
  }

  async function fetchQQStatus() {
    if (Date.now() - __fsLast < 8000) return;
    __fsLast = Date.now();
    try {
      const r = await get<any>("/api/qq/status");
      const data = r?.data || r;
      qqOnline.value = data?.qqOnline || data?.status === "online";
    } catch {}
    try {
      const convs = await get<any>("/api/web-chat/conversations", {
        pageSize: 50,
      });
      const items = convs?.conversations || convs?.items || [];
      const qc =
        items.find(
          (x: any) =>
            x.channel === "qq" && (x.messageCount > 0 || x.msgCount > 0),
        ) ||
        items.find(
          (x: any) => x.id.startsWith("conv-") && x.channel === "qq",
        ) ||
        items.find((x: any) => x.channel === "qq");
      if (qc) qqMsgCount.value = qc?.messageCount || qc?.msgCount || 0;
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

  function fetchConvSummary() {}

  return {
    characters,
    conversations,
    importBatches,
    memories,
    isWechatActive,
    wechatOnline,
    wechatMsgCount,
    isQQActive,
    qqOnline,
    qqMsgCount,
    webMsgCount,
    showDrawer,
    showCharPicker,
    showMemories,
    selectCharacter,
    handleSwitchChar,
    loadCharacterConversation,
    fetchConversations,
    handleSelectConv,
    handleSelectWechat,
    handleSelectQQ,
    handleContinueImport,
    handleViewMemories,
    fetchWechatMsgCount,
    fetchQQStatus,
    fetchWebMsgCount,
    refreshCharacters,
    fetchConvSummary,
  };
}
