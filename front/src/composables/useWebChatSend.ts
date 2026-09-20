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
  mergeServerMessages,
  normalizeRealtimeMessage,
} from "@/utils/message-order";
import { notifyDesktopPetChatState } from "@/runtime/desktop-pet-chat-state";
import { useConversationWorkspace } from "./useConversationWorkspace";

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
  fetchWebMsgCount?: () => void,
  replyTarget?: Ref<any>,
  onConversationCreated?: (conversationId: string) => void | Promise<void>,
  modelConfigId?: Ref<number>,
  reasoningEffort?: Ref<string>,
) {
  const { post, del, get } = useApi();
  const { currentWorkspace, getWorkspaceRequestFields, bindCurrentWorkspaceToConversation } =
    useConversationWorkspace();
  let lastPolledMsgId: string | null = null;
  const isSubmitting = ref(false);
  const generating = ref(false);
  let sendingTimer: ReturnType<typeof setTimeout> | null = null;
  let generationPhaseTimer: ReturnType<typeof setTimeout> | null = null;
  let generationSettlementEpoch = 0;

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

  async function reconcilePersistedMessages(conversationId: string) {
    const path = `/api/web-chat/conversations/${encodeURIComponent(conversationId)}/messages`;
    const first = await get<any>(path, { page: 1, pageSize: 50 });
    const totalPages = Math.max(1, Number(first?.totalPages || 1));
    const latest =
      totalPages > 1
        ? await get<any>(path, { page: totalPages, pageSize: 50 })
        : first;
    const items = latest?.items || latest?.messages || [];
    if (!Array.isArray(items) || items.length === 0) return;
    messages.value = mergeServerMessages(messages.value, items);
    lastPolledMsgId = messages.value[messages.value.length - 1]?.id || lastPolledMsgId;
    scrollToBottom(true);
  }

  function startGenerationPhaseTracking(mergeWindowMs: unknown) {
    clearGenerationPhaseTimer();
    const epoch = ++generationSettlementEpoch;
    const parsedWindow = Number(mergeWindowMs);
    const fallbackDelay =
      Number.isFinite(parsedWindow) && parsedWindow >= 0 ? parsedWindow : 5000;
    const startedAt = Date.now();
    const deadline = startedAt + 120000;
    let sawActiveState = false;

    const schedule = (delay: number) => {
      if (epoch !== generationSettlementEpoch) return;
      generationPhaseTimer = setTimeout(poll, delay);
    };

    const poll = async () => {
      generationPhaseTimer = null;
      const conversationId = String(convId.value || "").trim();
      if (
        epoch !== generationSettlementEpoch ||
        !sending.value ||
        !conversationId
      ) {
        return;
      }
      if (Date.now() >= deadline) {
        try {
          await reconcilePersistedMessages(conversationId);
        } finally {
          if (epoch === generationSettlementEpoch) {
            clearSendingTimer();
            sending.value = false;
            generating.value = false;
          }
        }
        return;
      }
      try {
        const result = await get<{ status?: string }>(
          `/api/web-chat/conversations/${conversationId}/generations/current/status`,
        );
        const status = String(result?.status || "").toLowerCase();
        if (status === "collecting" || status === "processing") {
          sawActiveState = true;
          generating.value = true;
          schedule(500);
          return;
        }
        if (
          status === "completed" ||
          status === "failed" ||
          status === "cancelled" ||
          (status === "idle" && sawActiveState)
        ) {
          try {
            await reconcilePersistedMessages(conversationId);
          } finally {
            if (epoch === generationSettlementEpoch) {
              clearSendingTimer();
              sending.value = false;
              generating.value = false;
            }
          }
          return;
        }
      } catch {
        schedule(Date.now() - startedAt >= fallbackDelay ? 750 : 150);
        return;
      }
      if (!sawActiveState && Date.now() - startedAt < fallbackDelay) {
        schedule(150);
        return;
      }
      try {
        await reconcilePersistedMessages(conversationId);
      } finally {
        if (epoch === generationSettlementEpoch) {
          clearSendingTimer();
          sending.value = false;
          generating.value = false;
        }
      }
    };
    generationPhaseTimer = setTimeout(poll, 0);
  }

  watch(sending, (active) => {
    if (active) return;
    generationSettlementEpoch++;
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
    const clientMessageId = requestEnvelope.requestId;
    const uiKey = `client:${clientMessageId}`;
    const assistantPlaceholderId = `generating:${requestEnvelope.requestId}`;
    const assistantUiKey = `generation:${requestEnvelope.requestId}`;
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
      clientMessageId,
      uiKey,
      animateIn: true,
      role: "user",
      content: sendContent,
      imageUrl: imgUrl || undefined,
      audioUrl: finalAudioUrl || undefined,
      audioDuration: 0,
      videoUrl: finalVideoUrl || undefined,
      status: "sending",
      conversationId: convId.value,
      createdAt: new Date(Math.floor(Date.now() / 1000) * 1000).toISOString(),
      replyToMessageId: replyTarget?.value?.id || undefined,
      replyToRole: replyTarget?.value?.role || undefined,
      replyToExcerpt: replyTarget?.value?.content || undefined,
    });
    messages.value.push({
      id: assistantPlaceholderId,
      requestId: requestEnvelope.requestId,
      uiKey: assistantUiKey,
      role: "assistant",
      content: "",
      status: "streaming",
      conversationId: convId.value,
      createdAt: new Date().toISOString(),
      generationPending: true,
      animateIn: false,
    });
    if (convId.value) {
      try {
        sessionStorage.setItem(
          `uai-pending-msg:${convId.value}`,
          JSON.stringify({
            id: userMsgLocalId,
            requestId: requestEnvelope.requestId,
            clientMessageId,
            uiKey,
            animateIn: false,
            role: "user",
            content: sendContent,
            imageUrl: imgUrl || undefined,
            audioUrl: finalAudioUrl || undefined,
            audioDuration: 0,
            videoUrl: finalVideoUrl || undefined,
            status: "sending",
            conversationId: convId.value,
            createdAt: new Date(Math.floor(Date.now() / 1000) * 1000).toISOString(),
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
        clientMessageId,
        conversationId: convId.value || undefined,
        projectId: currentWorkspace.value?.projectId || undefined,
        characterId: characterId.value || undefined,
        message: sendContent,
        imageUrl: imgUrl || "",
        audioUrl: finalAudioUrl || "",
        voiceMessage: !!finalAudioUrl,
        videoUrl: finalVideoUrl || "",
        replyToMessageId: replyTarget?.value?.id || undefined,
        modelConfigId: modelConfigId?.value || undefined,
        reasoningEffort: reasoningEffort?.value || undefined,
        ...getWorkspaceRequestFields(),
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
      if (!res.ok) {
        const errorBody = await res.json().catch(() => null);
        throw new Error(
          errorBody?.msg || errorBody?.message || `HTTP ${res.status}`,
        );
      }
      const data = await res.json();
      const result = data?.data || data;
      const uIdx = messages.value.findIndex(
        (m: any) => m.id === userMsgLocalId,
      );
      if (uIdx >= 0 && result.userMessageId) {
        messages.value[uIdx].id = result.userMessageId;
        messages.value[uIdx].clientMessageId =
          result.clientMessageId || clientMessageId;
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
      const createdConversationId =
        result.conversationId && !convId.value ? result.conversationId : "";
      if (createdConversationId)
        convId.value = result.conversationId;
      if (result.conversationId) {
        localStorage.setItem("webchat-conv-id", result.conversationId);
        bindCurrentWorkspaceToConversation(result.conversationId).catch(() => {});
        if (createdConversationId) {
          await onConversationCreated?.(result.conversationId);
        }
      }
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
          status: "failed",
        };
      }
      const aIdx = messages.value.findIndex(
        (m: any) =>
          m.uiKey === assistantUiKey || m.id === assistantPlaceholderId,
      );
      if (aIdx >= 0) {
        messages.value[aIdx] = {
          ...messages.value[aIdx],
          content: errMsg,
          status: "failed",
          generationPending: false,
          rawError: errMsg,
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
      if (
        lastMsg?.id &&
        lastMsg.id !== "streaming" &&
        !String(lastMsg.id).startsWith("generating:")
      )
        lastPolledMsgId = lastMsg.id;
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
    handleClear,
    getLastPolledMsgId,
    setLastPolledMsgId,
    isSubmitting,
    generating,
  };
}
