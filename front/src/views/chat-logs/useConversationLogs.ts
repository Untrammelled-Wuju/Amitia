// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, computed, nextTick } from "vue";
import { useRoute } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  fetchConvsApi,
  fetchMessagesApi,
  deleteMessageApi,
  clearConversationApi,
  deleteConversationApi,
  deleteAllConversationsApi,
  searchMessagesApi,
  exportConversationApi,
  fetchFeedbackApi,
  fetchMoodsApi,
  fetchSummaryApi,
  generateSummaryApi,
  updateSummaryApi,
  deleteSummaryApi,
  switchCharacterApi,
  fetchContextPreviewApi,
  continueChatApi,
  loadCharactersApi,
  type MessagePsycheSnapshot,
  fetchMessagePsycheApi,
  fetchMessageStatusApi,
} from "./api";

export function useConversationLogs() {
  const route = useRoute();
  const characters = ref<any[]>([]);

  const convs = ref<any[]>([]);
  const convKeyword = ref("");
  const characterFilter = ref("");

  if (route.query.characterId) {
    characterFilter.value = route.query.characterId as string;
  }
  const continueCharId = ref("");
  const channelFilter = ref("");
  const convPage = ref(1);
  const convTotal = ref(0);
  const selectedConv = ref<any>(null);
  const selectedConvId = ref("");

  const messages = ref<any[]>([]);
  const msgPage = ref(1);
  const msgTotal = ref(0);
  const messageKeywordFilter = ref("");
  const roleFilter = ref("");
  const msgListRef = ref<HTMLElement>();

  const filteredMessages = computed(() => {
    const keyword = messageKeywordFilter.value.trim().toLowerCase();
    return messages.value.filter((m) => {
      if (roleFilter.value && m.role !== roleFilter.value) return false;
      if (!keyword) return true;
      const haystack = [m.content, m.source, m.modelName]
        .filter(Boolean)
        .join(" ")
        .toLowerCase();
      return haystack.includes(keyword);
    });
  });

  async function fetchConvs() {
    const params: any = { page: convPage.value, pageSize: 20 };
    if (convKeyword.value) params.keyword = convKeyword.value;
    if (channelFilter.value) params.channel = channelFilter.value;
    if (characterFilter.value) params.characterId = characterFilter.value;
    try {
      const r = await fetchConvsApi(params);
      let items: any[] = Array.isArray(r) ? r : r?.items || [];
      const wechatItems = items.filter(
        (c: any) => c.channel === "wechat" || c.source === "wechat",
      );
      const otherItems = items.filter(
        (c: any) => c.channel !== "wechat" && c.source !== "wechat",
      );
      convs.value = [...wechatItems, ...otherItems];
      convTotal.value = r?.total || (Array.isArray(r) ? r.length : 0);
    } catch {}
  }

  async function selectConv(c: any) {
    selectedConv.value = c;
    selectedConvId.value = c.id;
    msgPage.value = 1;
    await fetchMessages();
    await fetchSummary();
    await fetchMoods();
    await fetchFeedback();
  }

  async function fetchMessages() {
    if (!selectedConvId.value) return;
    try {
      const r = await fetchMessagesApi(selectedConvId.value, {
        page: msgPage.value,
        pageSize: 50,
      });
      messages.value = Array.isArray(r) ? r : r?.items || [];
      msgTotal.value = r?.total || (Array.isArray(r) ? r.length : 0);
      nextTick(() => {
        if (msgListRef.value) msgListRef.value.scrollTop = 0;
      });
    } catch {}
  }

  async function delMsg(id: string) {
    await ElMessageBox.confirm("确定删除这条消息？", "提示", {
      type: "warning",
    });
    await deleteMessageApi(id);
    ElMessage.success("已删除");
    fetchMessages();
    fetchConvs();
  }

  const moodMap = ref<Record<string, string>>({});
  const feedbackMap = ref<Record<string, any[]>>({});

  async function fetchFeedback() {
    if (!selectedConvId.value) return;
    const map: Record<string, any[]> = {};
    try {
      const res = await fetchFeedbackApi();
      const items = res?.items || res || [];
      for (const f of items) {
        if (!map[f.messageId]) map[f.messageId] = [];
        map[f.messageId].push(f);
      }
      feedbackMap.value = map;
    } catch {
      feedbackMap.value = {};
    }
  }

  async function fetchMoods() {
    if (!selectedConvId.value) return;
    try {
      const r = await fetchMoodsApi(selectedConvId.value);
      const items = r?.items || [];
      const map: Record<string, string> = {};
      for (const m of items) {
        if (m.messageId) map[m.messageId] = m.moodLabel;
      }
      moodMap.value = map;
    } catch {
      moodMap.value = {};
    }
  }

  async function clearConv() {
    await ElMessageBox.confirm("确定清空本会话所有消息？", "确认", {
      type: "warning",
    });
    await clearConversationApi(selectedConvId.value);
    messages.value = [];
    ElMessage.success("已清空");
    fetchConvs();
  }

  const messageSearchVisible = ref(false);
  const messageSearchKeyword = ref("");
  const messageSearchResults = ref<any[]>([]);
  const messageSearchLoading = ref(false);

  async function searchMessagesGlobal() {
    const keyword = messageSearchKeyword.value.trim();
    if (!keyword) {
      messageSearchResults.value = [];
      return;
    }
    messageSearchLoading.value = true;
    try {
      const result = await searchMessagesApi(keyword, 1, 100);
      messageSearchResults.value = Array.isArray(result)
        ? result
        : result?.items || [];
    } catch (e: any) {
      messageSearchResults.value = [];
      ElMessage.error(e?.message || "消息搜索失败");
    } finally {
      messageSearchLoading.value = false;
    }
  }

  async function deleteAllConversations() {
    await ElMessageBox.confirm(
      "确定删除全部会话和聊天消息？此操作不可撤销。",
      "删除全部聊天记录",
      {
        type: "warning",
        confirmButtonText: "全部删除",
        confirmButtonClass: "el-button--danger",
      },
    );
    await deleteAllConversationsApi();
    selectedConv.value = null;
    selectedConvId.value = "";
    messages.value = [];
    convs.value = [];
    convTotal.value = 0;
    ElMessage.success("全部聊天记录已删除");
    await fetchConvs();
  }

  async function delConv() {
    const boundChar = characters.value.find(
      (c: any) => c.conversationId === selectedConvId.value,
    );
    const confirmMsg = boundChar
      ? `该对话与角色「${boundChar.name}」永久绑定，删除对话将一同删除角色「${boundChar.name}」。此操作不可撤销。`
      : "确定删除整个会话及其所有消息？此操作不可撤销。";
    await ElMessageBox.confirm(confirmMsg, "警告", {
      type: "warning",
      confirmButtonText: "删除",
      confirmButtonClass: "el-button--danger",
    });
    await deleteConversationApi(selectedConvId.value);
    selectedConv.value = null;
    selectedConvId.value = "";
    messages.value = [];
    ElMessage.success("已删除");
    fetchConvs();
    loadCharacters();
  }

  async function exportConv(format: string) {
    try {
      await exportConversationApi(format, [selectedConvId.value]);
      ElMessage.success("已导出到 data/exports 目录");
    } catch {}
  }

  const currentSummary = ref<any>(null);
  const summaryVisible = ref(false);
  const genSummaryLoading = ref(false);

  async function fetchSummary() {
    if (!selectedConvId.value) return;
    try {
      const r = await fetchSummaryApi(selectedConvId.value);
      currentSummary.value = r?.summaryText ? r : null;
    } catch {
      currentSummary.value = null;
    }
  }

  async function genSummary() {
    if (!selectedConvId.value) return;
    genSummaryLoading.value = true;
    try {
      await generateSummaryApi(selectedConvId.value);
      ElMessage.success("摘要已生成");
      await fetchSummary();
    } catch (err: any) {}
    genSummaryLoading.value = false;
  }

  function viewSummary() {
    summaryVisible.value = true;
  }

  async function editSummary() {
    if (!selectedConvId.value) return;
    const { value } = await ElMessageBox.prompt(
      "可直接修改当前会话摘要，保存后后续上下文压缩将使用新内容。",
      "编辑会话摘要",
      {
        inputType: "textarea",
        inputValue: currentSummary.value?.summaryText || "",
        inputValidator: (value) => String(value || "").trim().length > 0 || "摘要不能为空",
        confirmButtonText: "保存",
        cancelButtonText: "取消",
      },
    );
    const text = String(value || "").trim();
    if (!text) return;
    const updated = await updateSummaryApi(selectedConvId.value, text);
    currentSummary.value = updated || { ...(currentSummary.value || {}), summaryText: text };
    ElMessage.success("摘要已更新");
  }

  async function delSummary() {
    await ElMessageBox.confirm("确定删除此会话的摘要?", "确认", {
      type: "warning",
    });
    if (!selectedConvId.value) return;
    await deleteSummaryApi(selectedConvId.value);
    currentSummary.value = null;
    ElMessage.success("已删除");
  }

  const messageStatusMap = ref<Record<string, any>>({});
  const messageStatusLoadingMap = ref<Record<string, boolean>>({});

  async function toggleMessageStatus(messageId: string) {
    if (messageStatusMap.value[messageId]) {
      const next = { ...messageStatusMap.value };
      delete next[messageId];
      messageStatusMap.value = next;
      return;
    }
    if (messageStatusLoadingMap.value[messageId]) return;
    messageStatusLoadingMap.value = { ...messageStatusLoadingMap.value, [messageId]: true };
    try {
      const data = await fetchMessageStatusApi(messageId);
      messageStatusMap.value = { ...messageStatusMap.value, [messageId]: data || {} };
    } catch {
      ElMessage.error("读取消息状态失败");
    } finally {
      messageStatusLoadingMap.value = { ...messageStatusLoadingMap.value, [messageId]: false };
    }
  }

  const psycheMap = ref<Record<string, MessagePsycheSnapshot>>({});
  const psycheLoadingMap = ref<Record<string, boolean>>({});

  async function loadMessagePsyche(messageId: string) {
    if (psycheMap.value[messageId] || psycheLoadingMap.value[messageId]) return;
    psycheLoadingMap.value[messageId] = true;
    try {
      const data = await fetchMessagePsycheApi(messageId);
      if (data) psycheMap.value[messageId] = data;
    } catch {
    } finally {
      psycheLoadingMap.value[messageId] = false;
    }
  }

  function toggleMessagePsyche(messageId: string) {
    if (psycheMap.value[messageId]) {
      const next = { ...psycheMap.value };
      delete next[messageId];
      psycheMap.value = next;
      return;
    }
    loadMessagePsyche(messageId);
  }
  const devMode = ref(false);

  const ctxPreviewVisible = ref(false);
  const ctxPreviewLoading = ref(false);
  const ctxPreview = ref<any>(null);

  async function fetchContextPreview() {
    if (!selectedConvId.value) return;
    ctxPreviewVisible.value = true;
    ctxPreviewLoading.value = true;
    ctxPreview.value = null;
    try {
      ctxPreview.value = await fetchContextPreviewApi(selectedConvId.value);
    } catch (err: any) {
      ElMessage.error(err?.message || "Failed to load context preview");
      ctxPreviewVisible.value = false;
    } finally {
      ctxPreviewLoading.value = false;
    }
  }

  async function switchCharacter(charId: string) {
    if (!selectedConvId.value) return;
    try {
      await ElMessageBox.confirm(
        "切换角色后，该会话的后续回复将按新角色风格生成，历史消息保持不变。",
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

    try {
      await switchCharacterApi(selectedConvId.value, charId);
      ElMessage.success("角色已切换");
      selectedConv.value.characterId = charId;
      const char = characters.value.find((c: any) => c.id === charId);
      if (char) selectedConv.value.characterName = char.name;
    } catch (e: any) {
      ElMessage.error(
        "切换失败: " + (e?.response?.data?.message || e?.message || ""),
      );
    }
  }

  async function continueChat() {
    if (!selectedConv.value) return;
    try {
      const result = await continueChatApi({
        importBatchId:
          selectedConv.value.importBatchId || selectedConv.value.id,
        characterId: continueCharId.value || undefined,
      });
      if (result?.id) {
        ElMessage.success("Conversation created! Redirecting...");
        window.open(`/chat/${result.id}`, "_self");
      }
    } catch (err: any) {
      ElMessage.error(err?.message || "Failed to create conversation");
    }
  }

  async function loadCharacters() {
    try {
      characters.value = (await loadCharactersApi()) || [];
    } catch {}
  }

  return {
    characters,
    convs,
    convKeyword,
    characterFilter,
    continueCharId,
    channelFilter,
    convPage,
    convTotal,
    selectedConv,
    selectedConvId,
    messages,
    msgPage,
    msgTotal,
    messageKeywordFilter,
    roleFilter,
    msgListRef,
    filteredMessages,
    fetchConvs,
    selectConv,
    fetchMessages,
    delMsg,
    moodMap,
    feedbackMap,
    fetchFeedback,
    fetchMoods,
    clearConv,
    delConv,
    deleteAllConversations,
    messageSearchVisible,
    messageSearchKeyword,
    messageSearchResults,
    messageSearchLoading,
    searchMessagesGlobal,
    exportConv,
    currentSummary,
    summaryVisible,
    genSummaryLoading,
    fetchSummary,
    genSummary,
    viewSummary,
    editSummary,
    delSummary,
    devMode,
    ctxPreviewVisible,
    ctxPreviewLoading,
    ctxPreview,
    fetchContextPreview,
    switchCharacter,
    continueChat,
    loadCharacters,
    messageStatusMap,
    messageStatusLoadingMap,
    toggleMessageStatus,
    psycheMap,
    psycheLoadingMap,
    loadMessagePsyche,
    toggleMessagePsyche,
  };
}
