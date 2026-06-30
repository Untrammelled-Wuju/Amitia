// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, type Ref, nextTick } from "vue"
import { useApi } from "./useApi"

export function useChatScroll(
  msgAreaRef: Ref<any>,
  messages: Ref<any[]>,
  convId: Ref<string>,
  showScrollBtn: Ref<boolean>,
  sending: Ref<boolean>,
) {
  const { get } = useApi()
  const userScrolledUp = ref(false)
  const isPulling = ref(false)
  const pullReady = ref(false)
  const pullLoading = ref(false)
  const pullText = ref("Pull down to load earlier messages")
  const pullStartY = ref(0)
  const isLoadingHistory = ref(false)
  const hasMoreHistory = ref(true)
  const msgPage = ref(1)
  const HISTORY_PAGE_SIZE = 50

  function scrollToBottom(smooth = false) {
    if (!smooth && userScrolledUp.value) return
    userScrolledUp.value = false
    nextTick(() => {
      requestAnimationFrame(() => {
        const el = msgAreaRef.value?.rootEl
        if (!el) return
        el.scrollTo({
          top: el.scrollHeight,
          behavior: smooth ? "smooth" : "auto",
        })
      })
    })
  }

  function onScroll() {
    const el = msgAreaRef.value?.rootEl
    if (!el) return
    const distFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
    const threshold = 200
    showScrollBtn.value = distFromBottom > threshold
    userScrolledUp.value = distFromBottom > 100

    if (el.scrollTop <= 50 && hasMoreHistory.value && !isLoadingHistory.value && convId.value) {
      loadOlderMessages()
    }
  }

  function onWheel(e: WheelEvent) {
    const el = msgAreaRef.value?.rootEl
    if (!el) return
    if (e.deltaY >= 0) return
    const noOverflow = el.scrollHeight <= el.clientHeight
    const atTop = el.scrollTop <= 0
    if ((noOverflow || atTop) && hasMoreHistory.value && !isLoadingHistory.value && convId.value) {
      e.preventDefault()
      loadOlderMessages()
    }
  }

  async function loadOlderMessages() {
    if (isLoadingHistory.value || !hasMoreHistory.value || !convId.value) return
    isLoadingHistory.value = true
    try {
      const r = await get<any>(`/api/web-chat/conversations/${convId.value}/messages`, {
        page: msgPage.value + 1,
        pageSize: HISTORY_PAGE_SIZE,
      })
      const older = r?.items || []
      if (older.length === 0) {
        hasMoreHistory.value = false
      } else {
        const el = msgAreaRef.value?.rootEl
        const prevHeight = el?.scrollHeight || 0
        messages.value = [...older, ...messages.value]
        msgPage.value++
        nextTick(() => {
          if (el) {
            el.scrollTop = el.scrollHeight - prevHeight
          }
        })
      }
    } catch { } finally {
      isLoadingHistory.value = false
    }
  }

  function onMsgTouchStart(e: TouchEvent) {
    const el = msgAreaRef.value?.rootEl
    if (!el || el.scrollTop > 5) return
    pullStartY.value = e.touches[0].clientY
    isPulling.value = true
    pullText.value = "Pull down to load earlier messages"
    pullReady.value = false
  }

  function onMsgTouchMove(e: TouchEvent) {
    if (!isPulling.value) return
    const dy = e.touches[0].clientY - pullStartY.value
    if (dy > 60) {
      pullReady.value = true
      pullText.value = "Release to load"
    } else {
      pullReady.value = false
      pullText.value = "Pull down to load earlier messages"
    }
  }

  async function onMsgTouchEnd() {
    if (!isPulling.value) return
    if (pullReady.value && hasMoreHistory.value && !isLoadingHistory.value) {
      pullLoading.value = true
      pullText.value = "Loading..."
      await loadOlderMessages()
      pullLoading.value = false
    }
    isPulling.value = false
    pullReady.value = false
    pullText.value = "Pull down to load earlier messages"
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
    msgPage,
  }
}
