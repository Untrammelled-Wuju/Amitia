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
import { useChatAppearancePreference } from "./useChatAppearancePreference";
import { useDeviceOwnedConversation } from "./useDeviceOwnedConversation";
import { ownedImageAttachment, ownedAudioAttachment, ownedAudioURL, ownedFileAttachment, ownedAttachmentURL, type OwnedAttachment } from "@/runtime/device-owned-attachments";
import { parseOwnedProjectReference } from "@/runtime/owned-project-reference";
import { sameOwnedAuthority } from "@/runtime/owned-speech-result";
import { parseConversationReference } from "@/runtime/device-owned-conversation-reference";

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
  const { post, del } = useApi();
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
    let resourceUri = imageBase64;
    let file = currentImageFile.value;
    if (!file && imageBase64.startsWith("data:")) {
      try {
        const blob = await fetch(imageBase64).then((response) => response.blob());
        file = new File([blob], "image.png", {
          type: blob.type || "image/png",
        });
      } catch {
        file = null;
      }
    }
    if (file) {
      try {
        const form = new FormData();
        form.append("kind", "image");
        form.append("source", "chat_upload");
        form.append("file", file, file.name);
        const response = await post<any>("/api/artifacts/v1", form);
        const artifact = response?.artifact ?? response;
        const artifactId = String(artifact?.artifactId ?? artifact?.id ?? "").trim();
        if (!artifactId) throw new Error("图片上传结果缺少 artifactId");
        resourceUri = `amitia://artifacts/${artifactId}`;
      } catch (error: any) {
        ElMessage.error(error?.response?.data?.message || error?.message || "图片上传失败");
        throw error;
      }
    }
    currentImageBase64.value = null;
    currentImageFile.value = null;
    pendingImageBase64.value = resourceUri;
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

  async function doActualSend(text: unknown, audioUrl?: string, voiceMessage?: boolean, videoUrl?: string, ownedAudio?: OwnedAttachment, ownedExtra?: OwnedAttachment) {
    const originalConversation = convId.value;
    const originalRole = characterId.value;
    const originalReply = replyTarget?.value ? JSON.parse(JSON.stringify(replyTarget.value)) : undefined;
    let activeConversation = originalConversation;
    const draftProject = !convId.value && currentWorkspace.value?.workspaceKind === "logical" ? { ...currentWorkspace.value } : null;
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
    if (!owned.enabled.value) beginPendingAssistant?.(requestEnvelope.requestId, characterId.value);
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
        const response = await owned.send({ requestId: requestEnvelope.requestId, conversationId: convId.value || undefined, characterId: expectedExecutionScope.roleId, expectedExecutionScope, message: sendContent, context, attachments, quote }, (event) => {
          if (convId.value !== activeConversation || characterId.value !== originalRole) return;
          if (event.type === "started") {
            const userIndex = messages.value.findIndex((message) => message.id === userMsgLocalId);
            if (userIndex >= 0) messages.value[userIndex] = { ...messages.value[userIndex], id: `${requestEnvelope.requestId}/user`, ownerId: event.executionScope?.resourceOwnerId, sourceRevision: 1, status: "completed", executionScope: event.executionScope, conversationId: event.conversationId };
            if (event.conversationId && !convId.value) { convId.value = event.conversationId; activeConversation = event.conversationId; localStorage.setItem("webchat-conv-id", event.conversationId); }
            messages.value.push({ id: assistantId, uiKey: `${event.executionScope?.resourceOwnerId}:${assistantId}`, ownerId: event.executionScope?.resourceOwnerId, requestId: requestEnvelope.requestId, role: "assistant", content: "", reasoningContent: "", status: "streaming", createdAt: new Date().toISOString(), characterId: event.executionScope?.roleId, conversationId: event.conversationId, executionScope: event.executionScope });
          }
					if (event.type === "transcribed") {
						const userIndex = messages.value.findIndex((message) => message.id === `${requestEnvelope.requestId}/user` && message.ownerId === event.executionScope?.resourceOwnerId);
						if (userIndex >= 0) messages.value[userIndex] = { ...messages.value[userIndex], content: event.text, sourceRevision: 2 };
					}
          const index = messages.value.findIndex((message) => message.id === assistantId);
          if (index >= 0 && event.type === "delta") {
            const field = event.reasoning ? "reasoningContent" : "content";
            messages.value[index] = { ...messages.value[index], [field]: (messages.value[index][field] || "") + (event.text || "") };
          }
          if (index >= 0 && (event.type === "interrupted" || event.type === "failed")) messages.value[index] = { ...messages.value[index], status: "interrupted", saved: event.data?.saved === true };
          scrollToBottom(true);
        });
        if (convId.value !== activeConversation || characterId.value !== originalRole) throw new Error("对话或角色已变化，原回复已丢弃");
        const index = messages.value.findIndex((message) => message.id === assistantId);
				const userIndex = messages.value.findIndex((message) => message.id === `${requestEnvelope.requestId}/user` && message.ownerId === response.executionScope.resourceOwnerId);
				if (userIndex >= 0) messages.value[userIndex] = { ...messages.value[userIndex], content: response.transcription || messages.value[userIndex].content, sourceRevision: response.userRevision || 1, sourceConversationId: response.resourceConversationId || parseConversationReference(response.conversationId)?.id || response.conversationId, characterId: response.executionScope.roleId };
        const assistant = { id: assistantId, uiKey: `${response.executionScope.resourceOwnerId}:${assistantId}`, ownerId: response.executionScope.resourceOwnerId, sourceRevision: response.saved ? 1 : undefined, requestId: response.requestId, role: "assistant", content: response.reply, reasoningContent: response.reasoning, status: "completed", conversationId: response.conversationId, sourceConversationId: response.resourceConversationId || parseConversationReference(response.conversationId)?.id || response.conversationId, characterId: response.executionScope.roleId, executionScope: response.executionScope, saved: response.saved, memoryStatus: response.memoryStatus, createdAt: new Date().toISOString() };
        if (index >= 0) messages.value[index] = { ...messages.value[index], ...assistant };
        else messages.value.push(assistant);
        convId.value = response.conversationId;
        sending.value = false;
        notifyDesktopPetChatState("assistant_finished", requestEnvelope.requestId);
        if (replyTarget) replyTarget.value = null;
        if (draftProject?.projectId && draftProject.executionScope) {
          try {
            const project = parseOwnedProjectReference(draftProject.projectId);
            if (!project || project.ownerId !== response.executionScope.resourceOwnerId) throw new Error("项目数据所有者已变化，请重新选择项目");
            await owned.query(response.conversationId, response.executionScope.roleId);
            await owned.edit("conversation", response.conversationId, { projectId: project.id }, { characterId: response.executionScope.roleId, expectedExecutionScope: draftProject.executionScope, expectedOwnerId: response.executionScope.resourceOwnerId });
          } catch (error: any) {
            ElMessage.warning(error?.message || "对话已保存，但尚未确认移入项目，请重新加载后处理");
          }
        }
        await onConversationCreated?.(response.conversationId);
        return;
      }
			if (ownedAudio || ownedExtra) throw new Error("设备服务状态已变化，附件未发送");
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
        messageStyle: messageStyle.value,
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
    if (owned.enabled.value) { await owned.interrupt().catch((error) => ElMessage.error(error?.message || "停止失败")); return; }
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
    if (owned.enabled.value) { ElMessage.info("请继续发送新消息；已发起的请求不会自动重放。记忆保存任务会单独重试。"); return; }
    const turnId = String(msg?.turnId || msg?.assistantTurn?.id || "").trim();
    const conversationId = String(convId.value || "").trim();
    if (!conversationId || !turnId) {
      ElMessage.warning("该消息缺少可重试的 Turn 信息");
      return;
    }
    try {
      await post(`/api/web-chat/conversations/${encodeURIComponent(conversationId)}/turns/${encodeURIComponent(turnId)}/retry`, { messageStyle: messageStyle.value });
    } catch (err: any) {
      ElMessage.error(err?.message || "重试失败");
    }
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
        if (owned.enabled.value) await owned.edit("conversation", conversationId, {}, { clear: true, characterId: roleId });
        else await del(`/api/web-chat/conversations/${conversationId}/messages`);
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
