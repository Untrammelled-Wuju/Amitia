// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, nextTick, watch } from "vue"

const DRAFT_KEY = "webchat_draft"

export function useTextInput(
  emit: (e: "send", ...args: any[]) => void,
  isDisabled: () => boolean,
  isSending: () => boolean,
) {
  const text = ref(localStorage.getItem(DRAFT_KEY) || "")
  const inputRef = ref<HTMLTextAreaElement>()

  function saveDraft() {
    if (text.value.trim()) {
      localStorage.setItem(DRAFT_KEY, text.value)
    } else {
      localStorage.removeItem(DRAFT_KEY)
    }
  }

  function handleSend(e?: KeyboardEvent) {
    if (e) e.preventDefault()
    const trimmed = text.value.trim()
    if (!trimmed || isDisabled() || isSending()) return
    emit("send", trimmed)
    text.value = ""
    localStorage.removeItem(DRAFT_KEY)
    nextTick(() => autoResize())
  }

  function sendWithImage(textStr: string, imageBase64: string) {
    emit("send", textStr, imageBase64)
    text.value = ""
    localStorage.removeItem(DRAFT_KEY)
    nextTick(() => autoResize())
  }

  function sendWithVideo(textStr: string, videoUrl: string) {
    emit("send", textStr, undefined, videoUrl)
    text.value = ""
    localStorage.removeItem(DRAFT_KEY)
    nextTick(() => autoResize())
  }

  function autoResize() {
    const el = inputRef.value
    if (!el) return
    el.style.height = "auto"
    el.style.height = Math.min(el.scrollHeight, 120) + "px"
  }

  watch(text, () => { saveDraft() }, { flush: "post" })

  function focus() {
    inputRef.value?.focus()
  }

  function setText(t: string) {
    text.value = t
    saveDraft()
    nextTick(() => autoResize())
  }

  function clear() {
    text.value = ""
    localStorage.removeItem(DRAFT_KEY)
  }

  return {
    text,
    inputRef,
    handleSend,
    sendWithImage,
    sendWithVideo,
    autoResize,
    focus,
    setText,
    clear,
    saveDraft,
  }
}

