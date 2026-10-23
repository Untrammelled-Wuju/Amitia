// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { type Ref, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { createRequestEnvelope } from "../utils/requestEnvelope";
import { notifyDesktopPetChatState } from "@/runtime/desktop-pet-chat-state";
import { useConversationWorkspace } from "./useConversationWorkspace";
import { useChatAppearancePreference } from "./useChatAppearancePreference";
import { useDeviceOwnedConversation } from "./useDeviceOwnedConversation";
import { ownedImageAttachment, ownedAudioAttachment, ownedAudioURL, ownedFileAttachment, ownedAttachmentURL, type OwnedAttachment } from "@/runtime/device-owned-attachments";
import { sameOwnedAuthority } from "@/runtime/owned-speech-result";
import { parseConversationReference } from "@/runtime/device-owned-conversation-reference";
import { ownedChatRuntimeRequest } from "@/runtime/owned-chat-management";

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
  beginPendingAssistant?: (requestId: string, characterId?: string) => void,
  failPendingAssistant?: (requestId: string) => void,
) {
  const owned = useDeviceOwnedConversation();
  const { messageStyle } = useChatAppearancePreference();
  const { currentWorkspace, getWorkspaceRequestFields } = useConversationWorkspace();
  const isSubmitting = ref(false);
  const generating = sending;
  const pendingVideoFile = ref<File | null>(null);


  function onImageAttached(file: File, base64: string) {
    currentImageFile.value = file;
    currentImageBase64.value = base64;
  }

  function onImageRemoved() {
    currentImageFile.value = null;
    currentImageBase64.value = null;
  }

  function onVideoAttached(file: File, videoUrl: string) {
    pendingVideoFile.value = file;
    pendingVideoUrl.value = videoUrl;
  }

  function onVideoRemoved() {
    pendingVideoFile.value = null;
    pendingVideoUrl.value = null;
  }

  async function handleVoiceAudio(blob: Blob, transcript?: string, duration?: number) {
    try {
			if (await owned.refresh()) {
				const attachment = await ownedAudioAttachment(blob, blob.type.startsWith("audio/wav") ? "voice.wav" : "voice.webm");
				await doActualSend("[语音]", undefined, true, undefined, attachment);
				return;
			}
      throw new Error(owned.notice.value || "设备对话服务尚未就绪");
    } catch (err: any) {
      console.error("[Voice] upload failed:", err);
      ElMessage.error(err?.message || "语音发送失败");
    }
  }

  function handleVoiceText(text: unknown) {
    if (typeof text === "string" && text.trim()) inputRef.value?.setText?.(text);
  }

  async function handleImageSend(text: string, imageBase64: string) {
    if (await owned.refresh()) {
      await ownedImageAttachment(imageBase64, currentImageFile.value?.name || "image.png");
      currentImageBase64.value = null;
      currentImageFile.value = null;
      pendingImageBase64.value = imageBase64;
      await doActualSend(text.trim() || "[图片]");
      return;
    }
    throw new Error(owned.notice.value || "设备对话服务尚未就绪");
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

  async function doActualSend(text: unknown, audioUrl?: string, voiceMessage?: boolean, videoUrl?: string, ownedAudio?: OwnedAttachment, ownedExtra?: OwnedAttachment) {
    const originalConversation = convId.value;
    const originalRole = characterId.value;
    const originalReply = replyTarget?.value ? JSON.parse(JSON.stringify(replyTarget.value)) : undefined;
    let activeConversation = originalConversation;
    const draftProjectId = !convId.value ? String(currentWorkspace.value?.projectId || "").trim() : "";
    const safeText = typeof text === "string" ? text : "";
    if (isSubmitting.value || sending.value) return;
    isSubmitting.value = true;
    const requestEnvelope = createRequestEnvelope();
    const userMsgLocalId = `client:${requestEnvelope.requestId}`;
    const imgUrl = pendingImageBase64.value;
    const finalAudioUrl = audioUrl || pendingAudioUrl.value;
    const finalVideoUrl = videoUrl || pendingVideoUrl.value;
    const videoFile = pendingVideoFile.value;
    pendingVideoFile.value = null;
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
      role: "user",
      content: sendContent,
      imageUrl: imgUrl || undefined,
      audioUrl: ownedAudio ? ownedAudioURL([ownedAudio]) : finalAudioUrl || undefined,
      videoUrl: finalVideoUrl || undefined,
      ...(ownedExtra ? { attachments: [{ ...ownedExtra, url: ownedAttachmentURL(ownedExtra), downloadUrl: ownedAttachmentURL(ownedExtra) }] } : {}),
      status: "sending",
      conversationId: convId.value,
      createdAt: new Date().toISOString(),
      replyToMessageId: replyTarget?.value?.id || undefined,
      replyToRole: replyTarget?.value?.role || undefined,
      replyToExcerpt: replyTarget?.value?.content || undefined,
    });
    beginPendingAssistant?.(requestEnvelope.requestId, originalRole);
    scrollToBottom(true);
    sending.value = true;
    modelError.value = "";
    notifyDesktopPetChatState("assistant_thinking", requestEnvelope.requestId);

    try {
      if (await owned.refresh()) {
        if (!originalRole) throw new Error("请选择调用角色后发送消息");
        const prepared = await owned.data("working", originalRole);
        const expectedExecutionScope = prepared.executionScope;
        if (expectedExecutionScope.roleId !== originalRole || convId.value !== originalConversation || characterId.value !== originalRole) throw new Error("对话或角色已变化，原消息未发送");
        if (finalAudioUrl) throw new Error("请重新录制语音，以便当前数据所有者保存原始音频");
				let video: OwnedAttachment | undefined;
        if (finalVideoUrl) {
          if (videoFile) video = await ownedFileAttachment(videoFile, videoFile.name, "video");
          else { const match = /^data:(video\/(?:mp4|webm|quicktime));base64,([A-Za-z0-9+/]+={0,2})$/.exec(finalVideoUrl); if (!match || match[2].length > Math.ceil(1048576 / 3) * 4) throw new Error("请重新选择视频，当前 Core 需要校验并保存原始视频"); const bytes = Uint8Array.from(atob(match[2]), (value) => value.charCodeAt(0)); video = await ownedFileAttachment(new Blob([bytes], { type: match[1] }), "video", "video"); }
        }
				if (ownedAudio && imgUrl) throw new Error("语音消息不能同时携带图片");
        const attachments = ownedAudio ? [ownedAudio] : [ownedExtra, video, imgUrl ? await ownedImageAttachment(imgUrl) : undefined].filter((item): item is OwnedAttachment => !!item);
        if (attachments.length > 2) throw new Error("单条消息最多携带两个附件");
        const assistantId = `${requestEnvelope.requestId}/assistant`;
        const previousCore = [...messages.value].reverse().find((message) => message.executionScope?.coreId && message.executionScope.coreId !== owned.coreId.value)?.executionScope?.coreId;
        const context = previousCore && convId.value ? {
          previousCoreId: previousCore, conversationId: convId.value,
          summary: owned.previousSummary(previousCore, convId.value),
          messages: messages.value.filter((message) => message.id !== userMsgLocalId && ["user", "assistant"].includes(message.role) && !["sending", "failed"].includes(message.status)).slice(-128).map((message) => ({ id: message.id, ownerId: message.ownerId || message.executionScope?.resourceOwnerId, role: message.role, content: String(message.content || ""), status: message.status })),
        } : undefined;
        let quote;
        if (originalReply) {
          if (!originalReply.executionScope || !sameOwnedAuthority(originalReply.executionScope, expectedExecutionScope) || !originalReply.ownerId || !originalReply.characterId || !originalReply.sourceConversationId || typeof originalReply.fullContent !== "string") throw new Error("引用消息的服务、角色或归属已变化，请重新选择");
          const source = parseConversationReference(originalReply.sourceConversationId);
          const contentHash = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(originalReply.fullContent))), (value) => value.toString(16).padStart(2, "0")).join("");
          quote = { ownerId: originalReply.ownerId, characterId: originalReply.characterId, conversationId: source?.id || originalReply.sourceConversationId, messageId: originalReply.id, expectedRevision: originalReply.sourceRevision, contentHash, expectedExecutionScope: originalReply.executionScope };
        }
        const response = await owned.send({ requestId: requestEnvelope.requestId, conversationId: convId.value || undefined, ...(draftProjectId ? { projectId: draftProjectId } : {}), characterId: expectedExecutionScope.roleId, permissionMode: permissionMode?.value || undefined, modelConfigId: modelConfigId?.value, reasoningEffort: reasoningEffort?.value, reasoningEnabled: reasoningEnabled?.value, messageStyle: messageStyle.value, expectedExecutionScope, message: sendContent, context, attachments, quote }, (event) => {
          if (convId.value !== activeConversation || characterId.value !== originalRole) return;
          if (event.type === "started") {
            const userIndex = messages.value.findIndex((message) => message.id === userMsgLocalId);
            if (userIndex >= 0) messages.value[userIndex] = { ...messages.value[userIndex], id: `${requestEnvelope.requestId}/user`, ownerId: event.executionScope?.resourceOwnerId, sourceRevision: 1, status: "completed", executionScope: event.executionScope, conversationId: event.conversationId };
            if (event.conversationId && !convId.value) { convId.value = event.conversationId; activeConversation = event.conversationId; localStorage.setItem("webchat-conv-id", event.conversationId); }
            messages.value.push({ id: assistantId, uiKey: `request:${requestEnvelope.requestId}`, ownerId: event.executionScope?.resourceOwnerId, requestId: requestEnvelope.requestId, role: "assistant", content: "", reasoningContent: "", status: "streaming", reasoningComplete: false, outputComplete: false, createdAt: new Date().toISOString(), characterId: event.executionScope?.roleId, conversationId: event.conversationId, executionScope: event.executionScope });
          }
					if (event.type === "transcribed") {
						const userIndex = messages.value.findIndex((message) => message.id === `${requestEnvelope.requestId}/user` && message.ownerId === event.executionScope?.resourceOwnerId);
						if (userIndex >= 0) messages.value[userIndex] = { ...messages.value[userIndex], content: event.text, sourceRevision: 2 };
					}
          const index = messages.value.findIndex((message) => message.id === assistantId);
          if (index >= 0 && event.type === "delta") {
            const field = event.reasoning ? "reasoningContent" : "content";
            messages.value[index] = { ...messages.value[index], [field]: (messages.value[index][field] || "") + (event.text || ""), reasoningComplete: !event.reasoning, outputComplete: false };
          }
          if (index >= 0 && event.type === "reasoning-completed") messages.value[index] = { ...messages.value[index], reasoningComplete: true };
          if (index >= 0 && event.type === "output-completed") messages.value[index] = { ...messages.value[index], reasoningComplete: true, outputComplete: true };
          if (index >= 0 && (event.type === "interrupted" || event.type === "failed")) messages.value[index] = { ...messages.value[index], status: "interrupted", saved: event.data?.saved === true };
          scrollToBottom(true);
        });
        if (convId.value !== activeConversation || characterId.value !== originalRole) throw new Error("对话或角色已变化，原回复已丢弃");
        const index = messages.value.findIndex((message) => message.id === assistantId);
				const userIndex = messages.value.findIndex((message) => message.id === `${requestEnvelope.requestId}/user` && message.ownerId === response.executionScope.resourceOwnerId);
				if (userIndex >= 0) messages.value[userIndex] = { ...messages.value[userIndex], content: response.transcription || messages.value[userIndex].content, sourceRevision: response.userRevision || 1, sourceConversationId: response.resourceConversationId || parseConversationReference(response.conversationId)?.id || response.conversationId, characterId: response.executionScope.roleId };
        const assistant = { id: assistantId, uiKey: `request:${requestEnvelope.requestId}`, ownerId: response.executionScope.resourceOwnerId, sourceRevision: response.saved ? 1 : undefined, requestId: response.requestId, role: "assistant", content: response.reply, reasoningContent: response.reasoning, citationSnapshot: response.citationSnapshot || [], status: "completed", conversationId: response.conversationId, sourceConversationId: response.resourceConversationId || parseConversationReference(response.conversationId)?.id || response.conversationId, characterId: response.executionScope.roleId, executionScope: response.executionScope, saved: response.saved, memoryStatus: response.memoryStatus, createdAt: new Date().toISOString() };
        if (index >= 0) messages.value[index] = { ...messages.value[index], ...assistant };
        else messages.value.push(assistant);
        convId.value = response.conversationId;
        sending.value = false;
        notifyDesktopPetChatState("assistant_finished", requestEnvelope.requestId);
        if (replyTarget) replyTarget.value = null;
        try {
          await onConversationCreated?.(response.conversationId);
        } catch {
          if (convId.value === response.conversationId && characterId.value === originalRole) ElMessage.warning("回复已保存，会话列表刷新失败，请稍后刷新");
        }
        return;
      }
      throw new Error(owned.notice.value || "设备对话服务尚未就绪");
    } catch (err: any) {
      const errMsg = err?.message || "发送失败";
      failPendingAssistant?.(requestEnvelope.requestId);
      modelError.value = errMsg;
      const index = messages.value.findIndex((message) => message.id === userMsgLocalId);
      if (index >= 0) messages.value[index] = { ...messages.value[index], status: "failed" };
      if (owned.enabled.value) {
        const assistantIndex = messages.value.findIndex((message) => message.id === `${requestEnvelope.requestId}/assistant`);
        if (assistantIndex >= 0) messages.value[assistantIndex] = { ...messages.value[assistantIndex], status: "interrupted" };
      }
      sending.value = false;
      notifyDesktopPetChatState("assistant_error", requestEnvelope.requestId, errMsg);
    } finally {
      isSubmitting.value = false;
      fetchWebMsgCount?.();
    }
  }

  async function handleFileSend(file: File) { const attachment = await ownedFileAttachment(file, file.name); await doActualSend(`[文件] ${file.name}`, undefined, undefined, undefined, undefined, attachment); }

  async function handleStop() {
    try {
      if (activeTurnId?.value && convId.value) await ownedChatRuntimeRequest("interrupt", { conversationId: convId.value, turnId: activeTurnId.value }, characterId.value);
      else await owned.interrupt();
    } catch (error: any) { ElMessage.error(error?.message || "停止失败"); }
  }

  async function handleRetry(msg: any) {
    if (sending.value) return;
    const turnId = msg.assistantTurn?.id || msg.turnId;
    if (!turnId || !convId.value) {
      if (msg.role === "user" && msg.status === "failed" && !msg.imageUrl && !msg.audioUrl && !msg.videoUrl && !msg.attachments?.length) await doActualSend(msg.content);
      else ElMessage.error("当前消息缺少原执行记录，请重新加载对话后再重试");
      return;
    }
    try {
      await ownedChatRuntimeRequest("retry", { conversationId: convId.value, turnId }, characterId.value);
      window.dispatchEvent(new CustomEvent("amitia:owned-runtime-refresh", { detail: { conversationId: convId.value } }));
    } catch (error: any) { ElMessage.error(error?.message || "重试失败"); }
  }

  async function handleClear() {
    const conversationId = convId.value;
    const roleId = characterId.value;
    try {
      await ElMessageBox.confirm("确定清空当前会话的所有消息？", "提示", {
        type: "warning",
        confirmButtonText: "清空",
      });
      if (convId.value !== conversationId || characterId.value !== roleId) return;
      disconnectRuntime();
      if (conversationId) {
        await owned.edit("conversation", conversationId, {}, { clear: true, characterId: roleId });
      }
      if (convId.value !== conversationId || characterId.value !== roleId) return;
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
    handleFileSend,
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
