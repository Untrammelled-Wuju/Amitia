// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { type Ref, computed, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "./useApi";
import { resolveApiUrl } from "../runtime/runtime-adapter";
import { createAuthenticatedFetchInit } from "../runtime/request-auth";
import { createRequestEnvelope } from "../utils/requestEnvelope";
import {
  compareChatMessages,
  normalizeRealtimeMessage,
} from "@/utils/message-order";
import { notifyDesktopPetChatState } from "@/runtime/desktop-pet-chat-state";

export function useWebChatSend(
  messages: Ref<any[]>,
  convId: Ref<string>,
  characterId: Ref<string>,
  sending: Ref<boolean>,
  modelError: Ref<string>,
  modelMissing: Ref<boolean>,
  currentImageBase64: Ref<string | null>,
  currentImageFile: Ref<File | null>,
  pendingImageBase64: Ref<string | null>,
  pendingAudioUrl: Ref<string | null>,
  pendingVideoUrl: Ref<string | null>,
  scrollToBottom: (smooth?: boolean) => void,
  disconnectSSE: () => void,
  inputRef: Ref<any>,
  fetchWechatMsgCount: () => void,
  fetchQQStatus: () => void,
  fetchWebMsgCount?: () => void,
  replyTarget?: Ref<any>,
) {
  const { post, del, get } = useApi();
  let lastPolledMsgId: string | null = null;
  const isSubmitting = ref(false);
  const generating = ref(false);
  let sendingTimer: ReturnType<typeof setTimeout> | null = null;
  let generationPhaseTimer: ReturnType<typeof setTimeout> | null = null;

  function getLastPolledMsgId() {
    return lastPolledMsgId;
  }
  function setLastPolledMsgId(id: string | null) {
    lastPolledMsgId = id;
  }

  function clearSendingTimer() {
    if (sendingTimer) {
      clearTimeout(sendingTimer);
      sendingTimer = null;
    }
  }

  function clearGenerationPhaseTimer() {
    if (generationPhaseTimer) {
      clearTimeout(generationPhaseTimer);
      generationPhaseTimer = null;
    }
  }

  function startGenerationPhaseTracking(mergeWindowMs: unknown) {
    if (generating.value || generationPhaseTimer) return;
    const parsedWindow = Number(mergeWindowMs);
    const fallbackDelay =
      Number.isFinite(parsedWindow) && parsedWindow >= 0 ? parsedWindow : 5000;
    const startedAt = Date.now();
    const poll = async () => {
      generationPhaseTimer = null;
      if (!sending.value || generating.value || !convId.value) return;
      try {
        const result = await get<{ status?: string }>(
          `/api/web-chat/conversations/${convId.value}/generations/current/status`,
        );
        if (result?.status === "processing") {
          generating.value = true;
          return;
        }
      } catch {
        if (Date.now() - startedAt >= fallbackDelay) {
          generating.value = true;
          return;
        }
      }
      generationPhaseTimer = setTimeout(poll, 150);
    };
    generationPhaseTimer = setTimeout(poll, 0);
  }

  watch(sending, (active) => {
    if (active) return;
    clearGenerationPhaseTimer();
    generating.value = false;
  });

  function startSendingTimeout(userMsgId: string) {
    clearSendingTimer();
    sendingTimer = setTimeout(() => {
      if (!sending.value) return;
      const idx = messages.value.findIndex((m: any) => m.id === userMsgId);
      if (idx >= 0 && messages.value[idx].status === "queued") {
        messages.value[idx] = { ...messages.value[idx], status: "timeout" };
      }
      sending.value = false;
      if (!modelError.value) {
        modelError.value = "AI响应超时，请重试";
      }
      notifyDesktopPetChatState("assistant_error", convId.value || undefined);
    }, 60000);
  }

  const canRegenerate = computed(() => {
    if (!convId.value || messages.value.length === 0) return false;
    const last = messages.value[messages.value.length - 1];
    return last?.role === "assistant";
  });

  function onImageAttached(file: File, base64: string) {
    currentImageFile.value = file;
    currentImageBase64.value = base64;
  }

  function onImageRemoved() {
    currentImageFile.value = null;
    currentImageBase64.value = null;
  }

  function onVideoAttached(_file: File, videoUrl: string) {
    pendingVideoUrl.value = videoUrl;
  }

  function onVideoRemoved() {
    pendingVideoUrl.value = null;
  }

  async function handleVoiceAudio(
    blob: Blob,
    transcript?: string,
    duration?: number,
  ) {
    try {
      const formData = new FormData();
      formData.append("audio", blob, "voice.webm");
      const [url, init] = await Promise.all([
        resolveApiUrl("/api/voice/upload"),
        createAuthenticatedFetchInit("/api/voice/upload", { method: "POST", body: formData }),
      ]);
      const res = await fetch(url, init);
      if (!res.ok) throw new Error("Voice upload failed");
      const data = await res.json();
      const audioUrl = data?.data?.audioUrl || data?.audioUrl || "";
      if (!audioUrl) throw new Error("No audioUrl returned");
      pendingAudioUrl.value = audioUrl;
      const sendText =
        typeof transcript === "string" && transcript.trim()
          ? transcript
          : "[语音]";
      await doActualSend(sendText, audioUrl, true);
    } catch (err: any) {
      console.error("[Voice] upload failed:", err);
      ElMessage.error("语音发送失败");
    }
  }

  function handleVoiceText(text: unknown) {
    if (typeof text === "string" && text.trim()) {
      inputRef.value?.setText?.(text);
    }
  }

  async function handleImageSend(text: string, imageBase64: string) {
    currentImageBase64.value = null;
    currentImageFile.value = null;
    const hasUserText = !!(text && text.trim());
    const sendText = hasUserText ? text : "[图片]";
    pendingImageBase64.value = imageBase64;
    await doActualSend(sendText);
  }

  async function handleSend(
    text: string,
    imageBase64?: string,
    videoBase64?: string,
  ) {
    if (videoBase64 || pendingVideoUrl.value) {
      pendingVideoUrl.value = videoBase64 || pendingVideoUrl.value || "";
      const sendText = text.trim() || "[视频]";
      doActualSend(sendText, undefined, undefined, pendingVideoUrl.value);
      pendingVideoUrl.value = null;
      return;
    }
    if (imageBase64 || currentImageBase64.value) {
      handleImageSend(text, imageBase64 || currentImageBase64.value || "");
      return;
    }
    doActualSend(text);
  }

  function cancelActiveGeneration() {
    messages.value
      .filter((m: any) => m.status === "streaming")
      .forEach((m: any) => (m.status = "interrupted"));
    if (convId.value) {
      post(
        `/api/web-chat/conversations/${convId.value}/generations/current/cancel`,
      ).catch(() => {});
    }
  }

  async function doActualSend(
    text: unknown,
    audioUrl?: string,
    voiceMessage?: boolean,
    videoUrl?: string,
  ) {
    const safeText = typeof text === "string" ? text : "";
    if (isSubmitting.value) return;
    isSubmitting.value = true;
    const requestEnvelope = createRequestEnvelope();
    const userMsgLocalId = "user-" + Date.now();
    const imgUrl = pendingImageBase64.value;
    const finalAudioUrl = audioUrl || pendingAudioUrl.value;
    const finalVideoUrl = videoUrl || pendingVideoUrl.value;
    pendingImageBase64.value = null;
    pendingAudioUrl.value = null;
    pendingVideoUrl.value = null;
    const hasImage = !!imgUrl;
    const hasVoice = !!finalAudioUrl;
    const hasVideo = !!finalVideoUrl;
    const displayContent =
      hasVoice && !safeText.trim()
        ? "[语音]"
        : hasVideo && !safeText.trim()
          ? ""
          : hasImage && safeText === "[图片]"
            ? ""
            : safeText;
    const sendContent =
      hasVoice && !safeText.trim()
        ? "[语音]"
        : hasVideo && !safeText.trim()
          ? "[视频]"
          : hasImage && !safeText.trim()
            ? "[图片]"
            : safeText;
    messages.value.push({
      id: userMsgLocalId,
      requestId: requestEnvelope.requestId,
      role: "user",
      content: sendContent,
      imageUrl: imgUrl || undefined,
      audioUrl: finalAudioUrl || undefined,
      audioDuration: 0,
      videoUrl: finalVideoUrl || undefined,
      status: "sent",
      conversationId: convId.value,
      createdAt: new Date().toISOString(),
      replyToMessageId: replyTarget?.value?.id || undefined,
      replyToRole: replyTarget?.value?.role || undefined,
      replyToExcerpt: replyTarget?.value?.content || undefined,
    });
    if (convId.value) {
      try {
        sessionStorage.setItem(
          `uai-pending-msg:${convId.value}`,
          JSON.stringify({
            id: userMsgLocalId,
            requestId: requestEnvelope.requestId,
            role: "user",
            content: sendContent,
            imageUrl: imgUrl || undefined,
            audioUrl: finalAudioUrl || undefined,
            audioDuration: 0,
            videoUrl: finalVideoUrl || undefined,
            status: "sent",
            conversationId: convId.value,
            createdAt: new Date().toISOString(),
            replyToMessageId: replyTarget?.value?.id || undefined,
            replyToRole: replyTarget?.value?.role || undefined,
            replyToExcerpt: replyTarget?.value?.content || undefined,
          })
        );
      } catch {}
    }
    scrollToBottom(true);
    sending.value = true;
    modelError.value = "";
    notifyDesktopPetChatState("assistant_thinking", requestEnvelope.requestId);
    clearSendingTimer();
    try {
      const payload = {
        ...requestEnvelope,
        conversationId: convId.value || undefined,
        characterId: characterId.value || undefined,
        message: sendContent,
        imageUrl: imgUrl || "",
        audioUrl: finalAudioUrl || "",
        voiceMessage: !!finalAudioUrl,
        videoUrl: finalVideoUrl || "",
        replyToMessageId: replyTarget?.value?.id || undefined,
      };
      const [url, init] = await Promise.all([
        resolveApiUrl("/api/web-chat/messages"),
        createAuthenticatedFetchInit("/api/web-chat/messages", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        }),
      ]);
      const res = await fetch(url, init);
      isSubmitting.value = false;
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      const result = data?.data || data;
      const uIdx = messages.value.findIndex(
        (m: any) => m.id === userMsgLocalId,
      );
      if (uIdx >= 0 && result.userMessageId) {
        messages.value[uIdx].id = result.userMessageId;
        messages.value[uIdx].status = "queued";
        const duplicates = messages.value.filter(
          (m: any, i: number) => i !== uIdx && m.id === result.userMessageId,
        );
        if (duplicates.length > 0) {
          messages.value = messages.value.filter(
            (_m: any, i: number) =>
              i === uIdx || _m.id !== result.userMessageId,
          );
        }
      }
      if (convId.value) { try { sessionStorage.removeItem(`uai-pending-msg:${convId.value}`) } catch {} }
      if (result.conversationId && !convId.value)
        convId.value = result.conversationId;
      if (result.conversationId)
        localStorage.setItem("webchat-conv-id", result.conversationId);
      if (replyTarget) replyTarget.value = null;
      startGenerationPhaseTracking(result.mergeWindowMs);
      startSendingTimeout(result.userMessageId || userMsgLocalId);
    } catch (err: any) {
      console.error("[Send] Failed:", err);
      const errMsg = err?.message || "发送失败";
      modelError.value = errMsg;
      ElMessage.error(errMsg);
      const tIdx = messages.value.findIndex(
        (m: any) => m.id === userMsgLocalId,
      );
      if (tIdx >= 0) {
        messages.value[tIdx] = {
          ...messages.value[tIdx],
          id: "failed-" + Date.now(),
          status: "failed",
        };
      }
      notifyDesktopPetChatState(
        "assistant_error",
        requestEnvelope.requestId,
        errMsg,
      );
      sending.value = false;
      isSubmitting.value = false;
      if (convId.value) { try { sessionStorage.removeItem(`uai-pending-msg:${convId.value}`) } catch {} }
      clearSendingTimer();
    } finally {
      const lastMsg = messages.value[messages.value.length - 1];
      if (lastMsg?.id && lastMsg.id !== "streaming")
        lastPolledMsgId = lastMsg.id;
      fetchWechatMsgCount();
      fetchQQStatus();
      if (fetchWebMsgCount) fetchWebMsgCount();
    }
  }

  function handleStop() {
    clearSendingTimer();
    clearGenerationPhaseTimer();
    messages.value
      .filter((m: any) => m.status === "streaming")
      .forEach((m: any) => (m.status = "interrupted"));
    notifyDesktopPetChatState("assistant_finished", convId.value || undefined);
    sending.value = false;
    generating.value = false;
    if (convId.value) {
      post(
        `/api/web-chat/conversations/${convId.value}/generations/current/cancel`,
      ).catch(() => {});
    }
  }

  async function handleRetry(msg: any) {
    if (sending.value) return;
    const msgIdToRemove = msg.id;
    messages.value = messages.value.filter((m) => m.id !== msgIdToRemove);
    const lastAsst = [...messages.value]
      .reverse()
      .find(
        (m) =>
          m.role === "assistant" &&
          (m.status === "interrupted" || m.status === "failed"),
      );
    if (lastAsst) {
      messages.value = messages.value.filter((m) => m.id !== lastAsst.id);
    }
    const text = msg.content;
    if (text) {
      await handleSend(text);
    }
  }

  async function handleRegenerate() {
    if (!canRegenerate.value || !convId.value || sending.value) return;
    sending.value = true;
    generating.value = true;
    modelError.value = "";
    notifyDesktopPetChatState("assistant_thinking", convId.value);
    clearSendingTimer();
    clearGenerationPhaseTimer();
    let completed = false;
    try {
      const res = await post<any>(
        `/api/web-chat/conversations/${convId.value}/regenerate`,
      );
      const regenerated = (res?.assistantMessages || [])
        .map(normalizeRealtimeMessage)
        .filter((message: any) => message?.id);
      if (regenerated.length > 0) {
        let lastUserIndex = -1;
        for (let i = messages.value.length - 1; i >= 0; i -= 1) {
          if (messages.value[i]?.role === "user") {
            lastUserIndex = i;
            break;
          }
        }
        const prefix =
          lastUserIndex >= 0
            ? messages.value.slice(0, lastUserIndex + 1)
            : messages.value.filter((message: any) => message?.role !== "assistant");
        messages.value = [...prefix, ...regenerated].sort(compareChatMessages);
        lastPolledMsgId = regenerated[regenerated.length - 1]?.id || lastPolledMsgId;
      } else if (res?.reply) {
        const first = await get<any>(
          `/api/web-chat/conversations/${convId.value}/messages`,
          { page: 1, pageSize: 50 },
        );
        const totalPages = Math.max(1, Number(first?.totalPages || 1));
        const latest =
          totalPages > 1
            ? await get<any>(
                `/api/web-chat/conversations/${convId.value}/messages`,
                { page: totalPages, pageSize: 50 },
              )
            : first;
        messages.value = (latest?.items || latest?.messages || [])
          .map(normalizeRealtimeMessage)
          .sort(compareChatMessages);
        lastPolledMsgId = messages.value[messages.value.length - 1]?.id || null;
      }
      scrollToBottom(true);
      if (fetchWebMsgCount) fetchWebMsgCount();
      completed = true;
    } catch (err: any) {
      notifyDesktopPetChatState("assistant_error", convId.value, err?.message || "重新生成失败");
      ElMessage.error(err?.message || "重新生成失败");
    } finally {
      sending.value = false;
      generating.value = false;
      if (completed) {
        notifyDesktopPetChatState("assistant_finished", convId.value);
      }
    }
  }

  async function handleClear() {
    try {
      await ElMessageBox.confirm("确定清空当前会话的所有消息？", "提示", {
        type: "warning",
        confirmButtonText: "清空",
      });
      if (convId.value) {
        await del(`/api/web-chat/conversations/${convId.value}/messages`);
      }
      messages.value = [];
      ElMessage.success("已清空");
      if (convId.value) { try { sessionStorage.removeItem(`uai-pending-msg:${convId.value}`) } catch {} }
    } catch {}
  }

  return {
    canRegenerate,
    onImageAttached,
    onImageRemoved,
    onVideoAttached,
    onVideoRemoved,
    handleVoiceAudio,
    handleVoiceText,
    handleImageSend,
    handleSend,
    doActualSend,
    handleStop,
    handleRetry,
    handleRegenerate,
    handleClear,
    getLastPolledMsgId,
    setLastPolledMsgId,
    isSubmitting,
    generating,
  };
}
