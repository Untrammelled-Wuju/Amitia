// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { type Ref, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "./useApi";
import { resolveApiUrl } from "../runtime/runtime-adapter";
import { createAuthenticatedFetchInit } from "../runtime/request-auth";
import { createRequestEnvelope } from "../utils/requestEnvelope";
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
  disconnectRuntime: () => void,
  inputRef: Ref<any>,
  fetchWebMsgCount?: () => void,
  replyTarget?: Ref<any>,
  onConversationCreated?: (conversationId: string) => void | Promise<void>,
  modelConfigId?: Ref<number>,
  reasoningEffort?: Ref<string>,
  reasoningEnabled?: Ref<boolean>,
  permissionMode?: Ref<string>,
  activeTurnId?: Ref<string>,
) {
  const { post, del } = useApi();
  const { currentWorkspace, getWorkspaceRequestFields } = useConversationWorkspace();
  const isSubmitting = ref(false);
  const generating = sending;


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

  async function handleVoiceAudio(blob: Blob, transcript?: string, duration?: number) {
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
      await doActualSend(typeof transcript === "string" && transcript.trim() ? transcript : "[语音]", audioUrl, true);
    } catch (err: any) {
      console.error("[Voice] upload failed:", err);
      ElMessage.error("语音发送失败");
    }
  }

  function handleVoiceText(text: unknown) {
    if (typeof text === "string" && text.trim()) inputRef.value?.setText?.(text);
  }

  async function handleImageSend(text: string, imageBase64: string) {
    currentImageBase64.value = null;
    currentImageFile.value = null;
    pendingImageBase64.value = imageBase64;
    await doActualSend(text && text.trim() ? text : "[图片]");
  }

  async function handleSend(text: string, imageBase64?: string, videoBase64?: string) {
    if (videoBase64 || pendingVideoUrl.value) {
      pendingVideoUrl.value = videoBase64 || pendingVideoUrl.value || "";
      const sendText = text.trim() || "[视频]";
      await doActualSend(sendText, undefined, undefined, pendingVideoUrl.value);
      pendingVideoUrl.value = null;
      return;
    }
    if (imageBase64 || currentImageBase64.value) {
      await handleImageSend(text, imageBase64 || currentImageBase64.value || "");
      return;
    }
    await doActualSend(text);
  }

  async function doActualSend(text: unknown, audioUrl?: string, voiceMessage?: boolean, videoUrl?: string) {
    const safeText = typeof text === "string" ? text : "";
    if (isSubmitting.value || sending.value) return;
    isSubmitting.value = true;
    const requestEnvelope = createRequestEnvelope();
    const userMsgLocalId = `client:${requestEnvelope.requestId}`;
    const imgUrl = pendingImageBase64.value;
    const finalAudioUrl = audioUrl || pendingAudioUrl.value;
    const finalVideoUrl = videoUrl || pendingVideoUrl.value;
    pendingImageBase64.value = null;
    pendingAudioUrl.value = null;
    pendingVideoUrl.value = null;
    const sendContent = finalAudioUrl && !safeText.trim()
      ? "[语音]"
      : finalVideoUrl && !safeText.trim()
        ? "[视频]"
        : imgUrl && !safeText.trim()
          ? "[图片]"
          : safeText;

    messages.value.push({
      id: userMsgLocalId,
      requestId: requestEnvelope.requestId,
      clientMessageId: requestEnvelope.requestId,
      uiKey: userMsgLocalId,
      animateIn: true,
      role: "user",
      content: sendContent,
      imageUrl: imgUrl || undefined,
      audioUrl: finalAudioUrl || undefined,
      videoUrl: finalVideoUrl || undefined,
      status: "sending",
      conversationId: convId.value,
      createdAt: new Date().toISOString(),
      replyToMessageId: replyTarget?.value?.id || undefined,
      replyToRole: replyTarget?.value?.role || undefined,
      replyToExcerpt: replyTarget?.value?.content || undefined,
    });
    scrollToBottom(true);
    sending.value = true;
    modelError.value = "";
    notifyDesktopPetChatState("assistant_thinking", requestEnvelope.requestId);

    try {
      const result = await post<any>("/api/web-chat/messages", {
        requestId: requestEnvelope.requestId,
        sessionId: requestEnvelope.sessionId,
        deviceTimezone: requestEnvelope.deviceTimezone,
        clientMessageId: requestEnvelope.requestId,
        conversationId: convId.value || undefined,
        projectId: currentWorkspace.value?.projectId || undefined,
        characterId: characterId.value || undefined,
        content: sendContent,
        imageUrl: imgUrl || "",
        audioUrl: finalAudioUrl || "",
        voiceMessage: voiceMessage ?? !!finalAudioUrl,
        videoUrl: finalVideoUrl || "",
        replyToMessageId: replyTarget?.value?.id || undefined,
        modelConfigId: modelConfigId?.value || undefined,
        reasoningEffort: reasoningEffort?.value || undefined,
        reasoningEnabled: reasoningEnabled?.value,
        permissionMode: permissionMode?.value || undefined,
        ...getWorkspaceRequestFields(),
      });
      const authoritativeConversationId = String(result?.conversationId || convId.value || "");
      const localIndex = messages.value.findIndex((message) => message.id === userMsgLocalId);
      if (localIndex >= 0) {
        messages.value[localIndex] = {
          ...messages.value[localIndex],
          id: result?.userMessageId || userMsgLocalId,
          conversationId: authoritativeConversationId,
          clientMessageId: result?.clientMessageId || requestEnvelope.requestId,
          turnId: result?.turnId || undefined,
          executionId: result?.executionId || undefined,
          status: "queued",
        };
      }
      if (authoritativeConversationId && !convId.value) {
        convId.value = authoritativeConversationId;
        localStorage.setItem("webchat-conv-id", authoritativeConversationId);
        await onConversationCreated?.(authoritativeConversationId);
      }
      if (replyTarget) replyTarget.value = null;
    } catch (err: any) {
      const errMsg = err?.message || "发送失败";
      modelError.value = errMsg;
      ElMessage.error(errMsg);
      const index = messages.value.findIndex((message) => message.id === userMsgLocalId);
      if (index >= 0) messages.value[index] = { ...messages.value[index], status: "failed" };
      sending.value = false;
      notifyDesktopPetChatState("assistant_error", requestEnvelope.requestId, errMsg);
    } finally {
      isSubmitting.value = false;
      fetchWebMsgCount?.();
    }
  }

  async function handleStop() {
    const conversationId = String(convId.value || "").trim();
    const turnId = String(activeTurnId?.value || "").trim();
    if (!conversationId || !turnId) return;
    try {
      await post(`/api/web-chat/conversations/${encodeURIComponent(conversationId)}/turns/${encodeURIComponent(turnId)}/interrupt`);
    } catch (err: any) {
      ElMessage.error(err?.message || "停止失败");
    }
  }

  async function handleRetry(msg: any) {
    if (sending.value) return;
    const turnId = String(msg?.turnId || msg?.assistantTurn?.id || "").trim();
    const conversationId = String(convId.value || "").trim();
    if (!conversationId || !turnId) {
      ElMessage.warning("该消息缺少可重试的 Turn 信息");
      return;
    }
    try {
      await post(`/api/web-chat/conversations/${encodeURIComponent(conversationId)}/turns/${encodeURIComponent(turnId)}/retry`);
    } catch (err: any) {
      ElMessage.error(err?.message || "重试失败");
    }
  }

  async function handleClear() {
    try {
      await ElMessageBox.confirm("确定清空当前会话的所有消息？", "提示", {
        type: "warning",
        confirmButtonText: "清空",
      });
      disconnectRuntime();
      if (convId.value) await del(`/api/web-chat/conversations/${convId.value}/messages`);
      messages.value = [];
      sending.value = false;
      ElMessage.success("已清空");
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
    isSubmitting,
    generating,
  };
}
