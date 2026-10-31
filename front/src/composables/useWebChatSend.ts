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
import { parseOwnedProjectReference } from "@/runtime/owned-project-reference";
import { ownedChatRuntimeRequest } from "@/runtime/owned-chat-management";
import type { OwnedGroupChatResponse, OwnedGroupState } from "@/runtime/device-owned-chat";
import { resolveGroupMentions } from "./groupMentions";

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
  groupSending?: Ref<boolean>,
  groupDiscussion?: Ref<boolean>,
  groupConversation?: Ref<OwnedGroupState | null>,
) {
  const owned = useDeviceOwnedConversation();
  const { messageStyle } = useChatAppearancePreference();
  const { currentWorkspace, getWorkspaceRequestFields } = useConversationWorkspace();
  const isSubmitting = ref(false);
  const generating = sending;
  const pendingVideoFile = ref<File | null>(null);
  const activeSendConversations = new Map<string, string>();

  function currentSendKey() {
    return convId.value || "__draft__";
  }

  function syncSendState() {
    const key = currentSendKey();
    const active = [...activeSendConversations.values()].includes(key);
    sending.value = active;
    isSubmitting.value = active;
  }


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
    const draftProjectReference = !convId.value ? String(currentWorkspace.value?.projectId || "").trim() : "";
    let draftProjectId = "";
    const safeText = typeof text === "string" ? text : "";
    const requestEnvelope = createRequestEnvelope();
    if ([...activeSendConversations.values()].includes(currentSendKey())) return;
    activeSendConversations.set(requestEnvelope.requestId, currentSendKey());
    isSubmitting.value = true;
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

    const groupReply = Boolean(groupConversation?.value) || Boolean(groupDiscussion?.value);
    const createdAt = new Date().toISOString();
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
      createdAt,
      replyToMessageId: replyTarget?.value?.id || undefined,
      replyToRole: replyTarget?.value?.role || undefined,
      replyToExcerpt: replyTarget?.value?.content || undefined,
    });
    if (!groupReply) {
      messages.value.push({
        id: `${requestEnvelope.requestId}/assistant`,
        uiKey: `request:${requestEnvelope.requestId}`,
        requestId: requestEnvelope.requestId,
        role: "assistant",
        characterId: originalRole,
        content: "",
        status: "queued",
        conversationId: convId.value,
        createdAt,
      });
      beginPendingAssistant?.(requestEnvelope.requestId, originalRole);
    }
    scrollToBottom(true);
    sending.value = true;
    modelError.value = "";
    notifyDesktopPetChatState("assistant_thinking", requestEnvelope.requestId);

    try {
      if (await owned.refresh()) {
        if (!originalRole) throw new Error("请选择调用角色后发送消息");
        const prepared = await owned.data("working", originalRole);
        let expectedExecutionScope = prepared.executionScope;
        if (expectedExecutionScope.roleId !== originalRole || convId.value !== originalConversation || characterId.value !== originalRole) throw new Error("对话或角色已变化，原消息未发送");
        if (draftProjectReference) {
          const project = parseOwnedProjectReference(draftProjectReference);
          if (project && project.ownerId !== expectedExecutionScope.resourceOwnerId) throw new Error("项目数据归属已变化，请重新选择项目");
          draftProjectId = project?.id || draftProjectReference;
        }
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
        const group = originalConversation ? await owned.groupState(originalConversation, originalRole) : null;
        if (group) {
          failPendingAssistant?.(requestEnvelope.requestId);
          const pendingIndex = messages.value.findIndex((message) => message.id === `${requestEnvelope.requestId}/assistant` && message.status === "queued");
          if (pendingIndex >= 0) messages.value.splice(pendingIndex, 1);
          const groupAvatar = (roleId?: string) => group.members.find((member) => member.characterId === roleId)?.characterAvatar || "";
          if (groupSending) groupSending.value = true;
          const groupView = await owned.query(originalConversation, originalRole);
          expectedExecutionScope = groupView.executionScope;
          if (!sameOwnedAuthority(originalReply?.executionScope || expectedExecutionScope, expectedExecutionScope) && originalReply) throw new Error("引用消息的群会话权限已变化，请重新选择");
          const discussion = Boolean(groupDiscussion?.value);
          const activeMentions = discussion ? [] : resolveGroupMentions(sendContent, group.members);
          let response: OwnedGroupChatResponse | undefined;
          response = await owned.sendGroup({
            requestId: requestEnvelope.requestId,
            conversationId: originalConversation,
            characterId: expectedExecutionScope.roleId,
            permissionMode: permissionMode?.value || undefined,
            modelConfigId: modelConfigId?.value,
            reasoningEffort: reasoningEffort?.value,
            reasoningEnabled: reasoningEnabled?.value,
            messageStyle: messageStyle.value,
            expectedExecutionScope,
            message: sendContent,
            attachments,
            mentions: activeMentions,
            group: {
              expectedMembershipRevision: group.membershipRevision,
              expectedSettingsRevision: group.settingsRevision,
              mode: discussion ? "discussion" : "standard",
              ...(replyTarget?.value?.id ? { replyToMessageId: replyTarget.value.id } : {}),
            },
          }, (event) => {
            if (convId.value !== activeConversation || characterId.value !== originalRole) return;
            if (event.type === "group.delivery.started" && event.deliveryId) {
              const key = `group-delivery:${event.deliveryId}`;
              if (!messages.value.some((message) => message.uiKey === key)) messages.value.push({ id: event.childRequestId ? `${event.childRequestId}/assistant` : key, uiKey: key, requestId: event.childRequestId, role: "assistant", content: "", status: "streaming", createdAt: new Date().toISOString(), characterId: event.characterId, characterName: event.characterName, characterAvatar: groupAvatar(event.characterId), conversationId: event.conversationId, groupRoundId: event.groupRoundId, deliveryId: event.deliveryId, deliveryIndex: event.deliveryIndex ?? 0 });
            }
            if (event.type === "started" && event.executionScope && event.deliveryId) {
              const userIndex = messages.value.findIndex((message) => message.id === userMsgLocalId);
              if (userIndex >= 0) messages.value[userIndex] = { ...messages.value[userIndex], id: `${requestEnvelope.requestId}/user`, ownerId: event.executionScope.resourceOwnerId, sourceRevision: 1, status: "completed", executionScope: event.executionScope, conversationId: event.conversationId, sourceConversationId: event.conversationId };
              const assistantId = `${event.executionScope.requestId}/assistant`;
              const existing = messages.value.findIndex((message) => message.uiKey === `group-delivery:${event.deliveryId}`);
              const assistant = { id: assistantId, uiKey: `group-delivery:${event.deliveryId}`, ownerId: event.executionScope.resourceOwnerId, requestId: event.executionScope.requestId, role: "assistant", content: "", reasoningContent: "", status: "streaming", reasoningComplete: false, outputComplete: false, createdAt: new Date().toISOString(), characterId: event.characterId, characterName: event.characterName, characterAvatar: groupAvatar(event.characterId), conversationId: event.conversationId, sourceConversationId: event.conversationId, executionScope: event.executionScope, groupRoundId: event.groupRoundId, deliveryId: event.deliveryId, deliveryIndex: event.deliveryIndex ?? 0 };
              if (existing >= 0) {
                const current = messages.value[existing];
                messages.value[existing] = current.saved ? { ...assistant, ...current, executionScope: event.executionScope } : { ...current, ...assistant, createdAt: current.createdAt || assistant.createdAt };
              }
              else messages.value.push(assistant);
            }
            if (event.type === "delta" && event.deliveryId) {
              const index = messages.value.findIndex((message) => message.uiKey === `group-delivery:${event.deliveryId}`);
              if (index >= 0 && !messages.value[index].saved) {
                const field = event.reasoning ? "reasoningContent" : "content";
                messages.value[index] = { ...messages.value[index], [field]: (messages.value[index][field] || "") + (event.text || ""), reasoningComplete: !event.reasoning, outputComplete: false };
              }
            }
            if (event.type === "reasoning-completed" && event.deliveryId) {
              const index = messages.value.findIndex((message) => message.uiKey === `group-delivery:${event.deliveryId}`);
              if (index >= 0) messages.value[index] = { ...messages.value[index], reasoningComplete: true };
            }
            if (event.type === "output-completed" && event.deliveryId) {
              const index = messages.value.findIndex((message) => message.uiKey === `group-delivery:${event.deliveryId}`);
              if (index >= 0) messages.value[index] = { ...messages.value[index], reasoningComplete: true, outputComplete: true };
            }
            if (event.type === "group.delivery.completed" && event.deliveryId) {
              const index = messages.value.findIndex((message) => message.uiKey === `group-delivery:${event.deliveryId}`);
              if (index >= 0) messages.value[index] = { ...messages.value[index], status: "completed", saved: true, reasoningComplete: true, outputComplete: true };
            }
            if (event.type === "group.delivery.failed" && event.deliveryId) {
              const index = messages.value.findIndex((message) => message.uiKey === `group-delivery:${event.deliveryId}`);
              if (index >= 0) messages.value[index] = { ...messages.value[index], content: messages.value[index].content || event.text || "生成失败", status: "failed", reasoningComplete: true, outputComplete: true };
              else messages.value.push({ id: `group-delivery:${event.deliveryId}`, uiKey: `group-delivery:${event.deliveryId}`, role: "assistant", content: event.text || "生成失败", status: "failed", createdAt: new Date().toISOString(), characterId: event.characterId, characterName: event.characterName, groupRoundId: event.groupRoundId, deliveryId: event.deliveryId, deliveryIndex: event.deliveryIndex ?? 0 });
            }
            scrollToBottom(true);
          });
          if (convId.value !== activeConversation || characterId.value !== originalRole) {
            activeSendConversations.set(requestEnvelope.requestId, response.conversationId);
            notifyDesktopPetChatState("assistant_finished", requestEnvelope.requestId);
            void Promise.resolve().then(() => onConversationCreated?.(response.conversationId)).catch((error) => {
  console.error("[original-error]", error);
  return undefined;
});
            return;
          }
          const userIndex = messages.value.findIndex((message) => message.id === userMsgLocalId || message.id === `${requestEnvelope.requestId}/user`);
          if (userIndex >= 0) messages.value[userIndex] = { ...messages.value[userIndex], id: response.userMessageId, ownerId: expectedExecutionScope.resourceOwnerId, sourceRevision: 1, status: "completed", executionScope: expectedExecutionScope, conversationId: response.conversationId, sourceConversationId: parseConversationReference(response.conversationId)?.id || response.conversationId };
          for (const delivery of response.deliveries) {
            const index = messages.value.findIndex((message) => message.uiKey === `group-delivery:${delivery.deliveryId}`);
            const assistant = {
              id: `${delivery.requestId}/assistant`,
              uiKey: `group-delivery:${delivery.deliveryId}`,
              ownerId: delivery.executionScope.resourceOwnerId,
              sourceRevision: delivery.saved ? 1 : undefined,
              requestId: delivery.requestId,
              role: "assistant",
              content: delivery.reply,
              reasoningContent: delivery.reasoning,
              citationSnapshot: delivery.citationSnapshot || [],
              status: "completed",
              conversationId: response.conversationId,
              sourceConversationId: parseConversationReference(response.conversationId)?.id || response.conversationId,
              characterId: delivery.executionScope.roleId,
              characterName: delivery.characterName,
              characterAvatar: groupAvatar(delivery.executionScope.roleId),
              executionScope: delivery.executionScope,
              groupRoundId: response.groupRoundId,
              deliveryId: delivery.deliveryId,
              saved: delivery.saved,
              memoryStatus: delivery.memoryStatus,
              createdAt: index >= 0 ? messages.value[index].createdAt : new Date().toISOString(),
            };
            if (index >= 0) messages.value[index] = { ...messages.value[index], ...assistant };
            else messages.value.push(assistant);
          }
          convId.value = response.conversationId;
          activeSendConversations.set(requestEnvelope.requestId, response.conversationId);
          syncSendState();
          if (response.status === "completed_with_errors") modelError.value = "部分群成员回复失败，可在成员面板查看后重新点名";
          notifyDesktopPetChatState("assistant_finished", requestEnvelope.requestId);
          if (replyTarget) replyTarget.value = null;
          void Promise.resolve().then(() => onConversationCreated?.(response.conversationId)).catch((error) => {
  console.error("[original-error]", error);
if (convId.value === response.conversationId && characterId.value === originalRole) ElMessage.warning(error instanceof Error ? error.message : String(error));
          });
          return;
        }
        const response = await owned.send({ requestId: requestEnvelope.requestId, conversationId: convId.value || undefined, ...(draftProjectId ? { projectId: draftProjectId } : {}), characterId: expectedExecutionScope.roleId, permissionMode: permissionMode?.value || undefined, modelConfigId: modelConfigId?.value, reasoningEffort: reasoningEffort?.value, reasoningEnabled: reasoningEnabled?.value, messageStyle: messageStyle.value, expectedExecutionScope, message: sendContent, context, attachments, quote }, (event) => {
          if (convId.value !== activeConversation || characterId.value !== originalRole) return;
          if (event.type === "started") {
            const userIndex = messages.value.findIndex((message) => message.id === userMsgLocalId);
            if (userIndex >= 0) messages.value[userIndex] = { ...messages.value[userIndex], id: `${requestEnvelope.requestId}/user`, ownerId: event.executionScope?.resourceOwnerId, sourceRevision: 1, status: "completed", executionScope: event.executionScope, conversationId: event.conversationId };
            if (event.conversationId && !convId.value) {
              convId.value = event.conversationId;
              activeConversation = event.conversationId;
              activeSendConversations.set(requestEnvelope.requestId, event.conversationId);
              localStorage.setItem("webchat-conv-id", event.conversationId);
              window.dispatchEvent(new CustomEvent("amitia:owned-runtime-refresh", { detail: { conversationId: event.conversationId } }));
              syncSendState();
            }
            const assistantIndex = messages.value.findIndex((message) => message.role === "assistant" && message.requestId === requestEnvelope.requestId);
            const startedAssistant = { id: assistantId, uiKey: `request:${requestEnvelope.requestId}`, ownerId: event.executionScope?.resourceOwnerId, requestId: requestEnvelope.requestId, role: "assistant", status: "streaming", characterId: event.executionScope?.roleId || originalRole, conversationId: event.conversationId, executionScope: event.executionScope };
            if (assistantIndex >= 0) messages.value[assistantIndex] = { ...messages.value[assistantIndex], ...startedAssistant };
            else messages.value.push({ ...startedAssistant, content: "", reasoningContent: "", reasoningComplete: false, outputComplete: false, createdAt: new Date().toISOString() });
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
          if (index >= 0 && (event.type === "interrupted" || event.type === "failed")) messages.value[index] = { ...messages.value[index], status: event.type === "failed" ? "failed" : "interrupted", errorCode: event.errorCode, saved: event.data?.saved === true };
          scrollToBottom(true);
        });
        if (convId.value !== activeConversation || characterId.value !== originalRole) {
          activeSendConversations.set(requestEnvelope.requestId, response.conversationId);
          notifyDesktopPetChatState("assistant_finished", requestEnvelope.requestId);
          void Promise.resolve().then(() => onConversationCreated?.(response.conversationId)).catch((error) => {
  console.error("[original-error]", error);
  return undefined;
});
          return;
        }
        const index = messages.value.findIndex((message) => message.id === assistantId);
				const userIndex = messages.value.findIndex((message) => message.id === `${requestEnvelope.requestId}/user` && message.ownerId === response.executionScope.resourceOwnerId);
				if (userIndex >= 0) messages.value[userIndex] = { ...messages.value[userIndex], content: response.transcription || messages.value[userIndex].content, sourceRevision: response.userRevision || 1, sourceConversationId: response.resourceConversationId || parseConversationReference(response.conversationId)?.id || response.conversationId, characterId: response.executionScope.roleId };
        const assistant = { id: assistantId, uiKey: `request:${requestEnvelope.requestId}`, ownerId: response.executionScope.resourceOwnerId, sourceRevision: response.saved ? 1 : undefined, requestId: response.requestId, role: "assistant", content: response.reply, reasoningContent: response.reasoning, citationSnapshot: response.citationSnapshot || [], status: "completed", conversationId: response.conversationId, sourceConversationId: response.resourceConversationId || parseConversationReference(response.conversationId)?.id || response.conversationId, characterId: response.executionScope.roleId, executionScope: response.executionScope, saved: response.saved, memoryStatus: response.memoryStatus, createdAt: new Date().toISOString() };
        if (index >= 0) messages.value[index] = { ...messages.value[index], ...assistant };
        else messages.value.push(assistant);
        convId.value = response.conversationId;
        activeSendConversations.set(requestEnvelope.requestId, response.conversationId);
        syncSendState();
        notifyDesktopPetChatState("assistant_finished", requestEnvelope.requestId);
        if (replyTarget) replyTarget.value = null;
        try {
          await onConversationCreated?.(response.conversationId);
        } catch (error) {
  console.error("[original-error]", error);
if (convId.value === response.conversationId && characterId.value === originalRole) ElMessage.warning(error instanceof Error ? error.message : String(error));
        }
        return;
      }
      throw new Error(owned.notice.value || "设备对话服务尚未就绪");
    } catch (err: any) {
      const errMsg = err?.message || "发送失败";
      failPendingAssistant?.(requestEnvelope.requestId);
      if (convId.value === activeConversation && characterId.value === originalRole) {
        modelError.value = errMsg;
        const index = messages.value.findIndex((message) => message.id === userMsgLocalId);
        if (index >= 0) messages.value[index] = { ...messages.value[index], status: "failed" };
        const assistantIndex = messages.value.findIndex((message) => message.id === `${requestEnvelope.requestId}/assistant`);
        if (assistantIndex >= 0) messages.value[assistantIndex] = { ...messages.value[assistantIndex], status: "interrupted" };
      }
      notifyDesktopPetChatState("assistant_error", requestEnvelope.requestId, errMsg);
    } finally {
      if (groupSending) groupSending.value = false;
      activeSendConversations.delete(requestEnvelope.requestId);
      syncSendState();
      fetchWebMsgCount?.();
    }
  }

  async function handleFileSend(file: File) { const attachment = await ownedFileAttachment(file, file.name); await doActualSend(`[文件] ${file.name}`, undefined, undefined, undefined, undefined, attachment); }

  async function handleStop() {
    try {
      const requestId = [...activeSendConversations.entries()].find(([, conversation]) => conversation === currentSendKey())?.[0] || "";
      if (groupSending?.value) await owned.interrupt(requestId);
      else if (activeTurnId?.value && convId.value) await ownedChatRuntimeRequest("interrupt", { conversationId: convId.value, turnId: activeTurnId.value }, characterId.value);
      else await owned.interrupt(requestId);
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
    } catch (error) {
  console.error("[original-error]", error);
}
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
