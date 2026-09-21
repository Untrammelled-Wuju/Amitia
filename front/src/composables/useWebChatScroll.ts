// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, type Ref, nextTick, watch } from "vue";
import { useApi } from "./useApi";
import {
  mergeMessageCollections,
  normalizeRealtimeMessage,
} from "@/utils/message-order";

export function useWebChatScroll(
  msgAreaRef: Ref<any>,
  messages: Ref<any[]>,
  convId: Ref<string>,
  showScrollBtn: Ref<boolean>,
  loadOlderTurns?: () => Promise<unknown>,
) {
  const { get } = useApi();
  const userScrolledUp = ref(false);
  const isPulling = ref(false);
  const pullReady = ref(false);
  const pullLoading = ref(false);
  const pullText = ref("下拉加载更早消息");
  const pullStartY = ref(0);
  const isLoadingHistory = ref(false);
  const hasMoreHistory = ref(true);
  const historyBeforeSequence = ref(0);
  const HISTORY_PAGE_SIZE = 50;

  watch(convId, () => {
    historyBeforeSequence.value = 0;
    hasMoreHistory.value = true;
  });

  function attachLocalImages(_msgs: any[]) {}

  function applyHistorySnapshot(history?: { nextBefore?: number; hasMore?: boolean } | null) {
    historyBeforeSequence.value = Number(history?.nextBefore || 0);
    hasMoreHistory.value = history?.hasMore === true;
  }

  function scrollToBottom(smooth = false) {
    if (!smooth && userScrolledUp.value) return;
    userScrolledUp.value = false;
    nextTick(() => {
      requestAnimationFrame(() => {
        const el = msgAreaRef.value?.rootEl;
        if (!el) return;
        el.scrollTo({
          top: el.scrollHeight,
          behavior: smooth ? "smooth" : "auto",
        });
      });
    });
  }

  function onScroll() {
    const el = msgAreaRef.value?.rootEl;
    if (!el) return;
    const distFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    const threshold = 200;
    showScrollBtn.value = distFromBottom > threshold;
    userScrolledUp.value = distFromBottom > 100;
    if (
      el.scrollTop <= 50 &&
      hasMoreHistory.value &&
      !isLoadingHistory.value &&
      convId.value
    ) {
      loadOlderMessages();
    }
  }

  function onWheel(e: WheelEvent) {
    const el = msgAreaRef.value?.rootEl;
    if (!el) return;
    if (e.deltaY >= 0) return;
    const noOverflow = el.scrollHeight <= el.clientHeight;
    const atTop = el.scrollTop <= 0;
    if (
      (noOverflow || atTop) &&
      hasMoreHistory.value &&
      !isLoadingHistory.value &&
      convId.value
    ) {
      e.preventDefault();
      loadOlderMessages();
    }
  }

  async function loadOlderMessages() {
    if (isLoadingHistory.value || !hasMoreHistory.value || !convId.value) return;
    if (historyBeforeSequence.value <= 0) {
      const sequences = messages.value
        .map((message) => Number(message?.sequence || 0))
        .filter((sequence) => sequence > 0);
      if (sequences.length === 0) {
        hasMoreHistory.value = false;
        return;
      }
      historyBeforeSequence.value = Math.min(...sequences);
    }
    isLoadingHistory.value = true;
    try {
      const [r] = await Promise.all([
        get<any>(
          `/api/web-chat/conversations/${convId.value}/messages`,
          {
            beforeSequence: historyBeforeSequence.value,
            limit: HISTORY_PAGE_SIZE,
          },
        ),
        loadOlderTurns?.(),
      ]);
      const older = (r?.items || []).map(normalizeRealtimeMessage);
      const el = msgAreaRef.value?.rootEl;
      const prevHeight = el?.scrollHeight || 0;
      if (older.length > 0) {
        attachLocalImages(older);
        messages.value = mergeMessageCollections(messages.value, older);
        const responseCursor = Number(r?.nextBefore || 0);
        historyBeforeSequence.value = responseCursor > 0
          ? responseCursor
          : Math.min(...older.map((message: any) => Number(message?.sequence || 0)).filter((sequence: number) => sequence > 0));
        nextTick(() => {
          if (el) el.scrollTop = el.scrollHeight - prevHeight;
        });
      }
      hasMoreHistory.value = r?.hasMore === true;
    } catch {
    } finally {
      isLoadingHistory.value = false;
    }
  }

  function onMsgTouchStart(e: TouchEvent) {
    const el = msgAreaRef.value?.rootEl;
    if (!el || el.scrollTop > 5) return;
    pullStartY.value = e.touches[0].clientY;
    isPulling.value = true;
    pullText.value = "下拉加载更早消息";
    pullReady.value = false;
  }

  function onMsgTouchMove(e: TouchEvent) {
    if (!isPulling.value) return;
    const dy = e.touches[0].clientY - pullStartY.value;
    if (dy > 60) {
      pullReady.value = true;
      pullText.value = "松开加载";
    } else {
      pullReady.value = false;
      pullText.value = "下拉加载更早消息";
    }
  }

  async function onMsgTouchEnd() {
    if (!isPulling.value) return;
    if (pullReady.value && hasMoreHistory.value && !isLoadingHistory.value) {
      pullLoading.value = true;
      pullText.value = "加载中...";
      await loadOlderMessages();
      pullLoading.value = false;
    }
    isPulling.value = false;
    pullReady.value = false;
    pullText.value = "下拉加载更早消息";
  }

  return {
    scrollToBottom,
    onScroll,
    onWheel,
    loadOlderMessages,
    onMsgTouchStart,
    onMsgTouchMove,
    onMsgTouchEnd,
    userScrolledUp,
    isPulling,
    pullReady,
    pullLoading,
    pullText,
    hasMoreHistory,
    isLoadingHistory,
    applyHistorySnapshot,
  };
}
