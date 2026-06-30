// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { type Ref, computed } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { useApi } from "./useApi"

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
) {
  const { post, del, get } = useApi()
  let abortController: AbortController | null = null
  let lastPolledMsgId: string | null = null

  function getLastPolledMsgId() { return lastPolledMsgId }
  function setLastPolledMsgId(id: string | null) { lastPolledMsgId = id }

  const canRegenerate = computed(() => {
    if (!convId.value || messages.value.length === 0) return false
    const last = messages.value[messages.value.length - 1]
    return last?.role === "assistant"
  })

  function onImageAttached(file: File, base64: string) {
    currentImageFile.value = file
    currentImageBase64.value = base64
  }

  function onImageRemoved() {
    currentImageFile.value = null
    currentImageBase64.value = null
  }

  function onVideoAttached(_file: File, videoUrl: string) {
    pendingVideoUrl.value = videoUrl
  }

  function onVideoRemoved() {
    pendingVideoUrl.value = null
  }

  async function handleVoiceAudio(blob: Blob, transcript?: string, duration?: number) {
    try {
      const formData = new FormData()
      formData.append("audio", blob, "voice.webm")
      const token = localStorage.getItem("ai-companion-token") || ""
      const res = await fetch("/api/voice/upload", {
        method: "POST",
        headers: { Authorization: "Bearer " + token },
        body: formData,
      })
      if (!res.ok) throw new Error("Voice upload failed")
      const data = await res.json()
      const audioUrl = data?.data?.audioUrl || data?.audioUrl || ""
      if (!audioUrl) throw new Error("No audioUrl returned")
      pendingAudioUrl.value = audioUrl
      const sendText = typeof transcript === "string" && transcript.trim() ? transcript : "[语音]"
      await doActualSend(sendText, audioUrl, true)
    } catch (err: any) {
      console.error("[Voice] upload failed:", err)
      ElMessage.error("语音发送失败")
    }
  }

  function handleVoiceText(text: unknown) {
    if (typeof text === "string" && text.trim()) {
      inputRef.value?.setText?.(text)
    }
  }

  async function handleImageSend(text: string, imageBase64: string) {
    if (sending.value) return
    currentImageBase64.value = null
    currentImageFile.value = null
    const hasUserText = !!(text && text.trim())
    const sendText = hasUserText ? text : "[图片]"
    pendingImageBase64.value = imageBase64
    await doActualSend(sendText)
  }

  async function handleSend(text: string, imageBase64?: string, videoBase64?: string) {
    if (videoBase64 || pendingVideoUrl.value) {
      pendingVideoUrl.value = videoBase64 || pendingVideoUrl.value || ""
      const sendText = text.trim() || "[视频]"
      doActualSend(sendText, undefined, undefined, pendingVideoUrl.value)
      pendingVideoUrl.value = null
      return
    }
    if (imageBase64 || currentImageBase64.value) {
      handleImageSend(text, imageBase64 || currentImageBase64.value || "")
      return
    }
    if (sending.value) return
    doActualSend(text)
  }

  async function doActualSend(text: unknown, audioUrl?: string, voiceMessage?: boolean, videoUrl?: string) {
    const safeText = typeof text === "string" ? text : ""
    if (sending.value) return
    disconnectSSE()
    const userMsgLocalId = "user-" + Date.now()
    const imgUrl = pendingImageBase64.value
    const finalAudioUrl = audioUrl || pendingAudioUrl.value
    const finalVideoUrl = videoUrl || pendingVideoUrl.value
    pendingImageBase64.value = null
    pendingAudioUrl.value = null
    pendingVideoUrl.value = null
    const hasImage = !!(imgUrl)
    const hasVoice = !!(finalAudioUrl)
    const hasVideo = !!(finalVideoUrl)
    const displayContent = (hasVoice && !safeText.trim()) ? "[语音]" : (hasVideo && !safeText.trim()) ? "" : (hasImage && safeText === "[图片]") ? "" : safeText
    const sendContent = (hasVoice && !safeText.trim()) ? "[语音]" : (hasVideo && !safeText.trim()) ? "[视频]" : (hasImage && !safeText.trim()) ? "[图片]" : safeText
    messages.value.push({ id: userMsgLocalId, role: "user", content: displayContent, imageUrl: imgUrl || undefined, audioUrl: finalAudioUrl || undefined, audioDuration: 0, videoUrl: finalVideoUrl || undefined, status: "sent", conversationId: convId.value, createdAt: new Date().toISOString() })
    scrollToBottom(true)
    sending.value = true
    modelError.value = ""
    try {
      const token = localStorage.getItem("ai-companion-token") || ""
      const res = await fetch("/api/web-chat/send-stream", {
        method: "POST",
        headers: { "Content-Type": "application/json", "Authorization": `Bearer ${token}` },
        body: JSON.stringify({ conversationId: convId.value || undefined, characterId: characterId.value || undefined, message: sendContent, imageUrl: imgUrl || "", audioUrl: finalAudioUrl || "", voiceMessage: !!finalAudioUrl, videoUrl: finalVideoUrl || "" }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const reader = res.body?.getReader()
      if (!reader) throw new Error("No response stream")
      const decoder = new TextDecoder()
      let buffer = ""
      let eventType = ""
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split("\n")
        buffer = lines.pop() || ""
        for (const line of lines) {
          if (line.startsWith("event:")) {
            eventType = line.slice(6).trim()
            continue
          }
          if (!line.startsWith("data:")) continue
          try {
            const data = JSON.parse(line.slice(5).trim())
            if (data.conversationId && !convId.value) convId.value = data.conversationId
            if (eventType === "message_start") {
              const uIdx = messages.value.findIndex((m: any) => m.id === userMsgLocalId)
              if (uIdx >= 0 && data.userMessageId) {
                messages.value[uIdx].id = data.userMessageId
              }
              if (data.conversationId && !convId.value) convId.value = data.conversationId
              continue
            }
            if (eventType === "token" && data.content) {
              messages.value.push({
                id: data.id || ("msg-" + Date.now()), role: "assistant", content: data.content,
                status: "streaming", conversationId: data.conversationId || convId.value,
                createdAt: data.createdAt || new Date().toISOString()
              })
              scrollToBottom(true)
            }
            if (eventType === "voice_audio" && data.audioUrl) {
              messages.value.push({
                id: data.messageId || ("msg-" + Date.now()), role: "assistant", content: data.content || "",
                status: "streaming", conversationId: data.conversationId || convId.value,
                createdAt: data.createdAt || new Date().toISOString(), audioUrl: data.audioUrl, audioDuration: data.duration || 0,
              })
              scrollToBottom(true)
            }
            if (eventType === "done") {
              const lastStreaming = [...messages.value].reverse().find((m: any) => m.status === "streaming" && m.id !== "streaming")
              if (lastStreaming?.id) lastPolledMsgId = lastStreaming.id
              messages.value.forEach((m: any) => {
                if (m.status === "streaming") m.status = "sent"
              })
            }
          } catch { }
        }
      }
    } catch (err: any) {
      if (err?.name === "AbortError") {
        const streaming = messages.value.filter((m: any) => m.status === "streaming")
        for (const sm of streaming) {
          sm.status = "interrupted"
        }
      } else {
        console.error("[Stream] Failed:", err)
        const errMsg = err?.message || "连接失败"
        modelError.value = errMsg
        ElMessage.error(errMsg)
        const tIdx = messages.value.findIndex((m: any) => m.id === userMsgLocalId)
        if (tIdx >= 0) {
          messages.value[tIdx] = { ...messages.value[tIdx], id: "failed-" + Date.now(), status: "failed" }
        }
        const sIdx = messages.value.findIndex(m => m.id === "streaming")
        if (sIdx >= 0) messages.value.splice(sIdx, 1)
      }
    } finally {
      sending.value = false
      abortController = null
      const lastMsg = messages.value[messages.value.length - 1]
      if (lastMsg?.id && lastMsg.id !== "streaming") lastPolledMsgId = lastMsg.id
      fetchWechatMsgCount()
      fetchQQStatus()
    }
  }

  function handleStop() {
    if (abortController) {
      abortController.abort()
      abortController = null
    }
    messages.value.filter((m: any) => m.status === "streaming").forEach((m: any) => m.status = "interrupted")
    sending.value = false
  }

  async function handleRetry(msg: any) {
    if (sending.value) return
    messages.value = messages.value.filter(m => m.id !== msg.id)
    const lastAsst = [...messages.value].reverse().find(m => m.role === "assistant" && (m.status === "interrupted" || m.status === "failed"))
    if (lastAsst) {
      messages.value = messages.value.filter(m => m.id !== lastAsst.id)
    }
    try {
      await post("/api/web-chat/retry", { messageId: msg.id })
    } catch { }
    const text = msg.content
    if (text) {
      await handleSend(text)
    }
  }

  async function handleRegenerate() {
    if (!canRegenerate.value || !convId.value) return
    sending.value = true
    try {
      const res = await post<any>(`/api/web-chat/conversations/${convId.value}/regenerate`)
      if (res) {
        if (res.assistantMessage) {
          const lastIdx = messages.value.length - 1
          if (messages.value[lastIdx]?.role === "assistant") {
            messages.value[lastIdx] = res.assistantMessage
          } else {
            messages.value.push(res.assistantMessage)
          }
        }
        scrollToBottom(true)
      }
    } catch (err: any) {
      ElMessage.error(err?.message || "重新生成失败")
    } finally {
      sending.value = false
    }
  }

  async function handleClear() {
    try {
      await ElMessageBox.confirm("确定清空当前会话的所有消息？", "提示", {
        type: "warning",
        confirmButtonText: "清空",
      })
      if (convId.value) {
        await del(`/api/web-chat/conversations/${convId.value}/messages`)
      }
      messages.value = []
      ElMessage.success("已清空")
    } catch { }
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
  }
}
