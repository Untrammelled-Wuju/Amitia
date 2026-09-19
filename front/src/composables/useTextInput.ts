// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { computed, ref, nextTick, watch, type Ref } from "vue";

const DRAFT_PREFIX = "webchat_draft:";

function readDraft(key: string): string {
  try {
    return localStorage.getItem(`${DRAFT_PREFIX}${key}`) || "";
  } catch {
    return "";
  }
}

function writeDraft(key: string, value: string) {
  try {
    const storageKey = `${DRAFT_PREFIX}${key}`;
    if (value.trim()) localStorage.setItem(storageKey, value);
    else localStorage.removeItem(storageKey);
  } catch {}
}

export function useTextInput(
  emit: (e: "send", ...args: any[]) => void,
  isDisabled: () => boolean,
  draftKey?: Ref<string>,
) {
  const activeDraftKey = computed(() => draftKey?.value || "new:recent");
  const text = ref(readDraft(activeDraftKey.value));
  const inputRef = ref<HTMLTextAreaElement>();

  function saveDraft() {
    writeDraft(activeDraftKey.value, text.value);
  }

  function handleSend(e?: KeyboardEvent) {
    if (e) e.preventDefault();
    const trimmed = text.value.trim();
    if (!trimmed || isDisabled()) return;
    emit("send", trimmed);
    text.value = "";
    writeDraft(activeDraftKey.value, "");
    nextTick(() => autoResize());
  }

  function sendWithImage(textStr: string, imageBase64: string) {
    emit("send", textStr, imageBase64);
    text.value = "";
    writeDraft(activeDraftKey.value, "");
    nextTick(() => autoResize());
  }

  function sendWithVideo(textStr: string, videoUrl: string) {
    emit("send", textStr, undefined, videoUrl);
    text.value = "";
    writeDraft(activeDraftKey.value, "");
    nextTick(() => autoResize());
  }

  function autoResize() {
    const el = inputRef.value;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, 120) + "px";
  }

  watch(
    text,
    () => {
      saveDraft();
    },
    { flush: "post" },
  );

  watch(activeDraftKey, (nextKey, previousKey) => {
    writeDraft(previousKey, text.value);
    text.value = readDraft(nextKey);
    nextTick(() => autoResize());
  });

  function focus() {
    inputRef.value?.focus();
  }

  function setText(t: string) {
    text.value = t;
    saveDraft();
    nextTick(() => autoResize());
  }

  function clear() {
    text.value = "";
    writeDraft(activeDraftKey.value, "");
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
  };
}
