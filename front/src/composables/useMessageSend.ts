// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, type Ref } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "./useApi"

export function useMessageSend(
  messages: Ref<any[]>,
  convId: Ref<string>,
  characterId: Ref<string>,
  sending: Ref<boolean>,
  modelError: Ref<string>,
  pendingImageBase64: Ref<string | null>,
  pendingAudioUrl: Ref<string | null>,
  pendingVideoUrl: Ref<string | null>,
  currentImageBase64: Ref<string | null>,
  currentImageFile: Ref<File | null>,
  scrollToBottom: (smooth?: boolean) => void,
  disconnectSSE: () => void,
) {
  const { post } = useApi()
  let abortController: AbortController | null = null

  let lastPolledMsgId: string | null = null

  function getLastPolledMsgId() { return lastPolledMsgId }
  function setLastPolledMsgId(id: string | null) { lastPolledMsgId = id }

  function handleVoiceText(text: string) {
    return text
  }

  async function handleVoiceAudio(blob: Blob, transcript?: string) {
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
      const sendText = transcript || "[语音]"
      await doActualSend(sendText, audioUrl, true)
    } catch (err: any) {
      console.error("[Voice] upload failed:", err)
      ElMessage.error("语音发送失败")
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

  async function doActualSend(text: string, audioUrl?: string, voiceMessage?: boolean, videoUrl?: string) {
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
    const displayContent = (hasVoice && !text.trim()) ? "[语音]" : (hasVideo && !text.trim()) ? "" : (hasImage && text === "[图片]") ? "" : text
    const sendContent = (hasVoice && !text.trim()) ? "[语音]" : (hasVideo && !text.trim()) ? "[视频]" : (hasImage && !text.trim()) ? "[图片]" : text
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
        const lines = buffer.split("\\n")
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

  return {
    handleSend,
    handleImageSend,
    handleVoiceAudio,
    handleVoiceText,
    handleStop,
    handleRetry,
    getLastPolledMsgId,
    setLastPolledMsgId,
  }
}
