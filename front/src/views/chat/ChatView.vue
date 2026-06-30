<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="chat-view">
    <div class="chat-header">
      <el-select v-model="selectedCharacterId" placeholder="选择角色" @change="onCharacterChange" style="width: 240px">
        <el-option v-for="char in characters" :key="char.id" :label="char.name" :value="char.id" />
      </el-select>
      <el-select v-model="currentConversationId" placeholder="选择或新建对话" clearable @change="onConversationChange" @clear="onNewConversation" style="width: 280px; margin-left: 12px">
        <el-option v-for="conv in filteredConversations" :key="conv.id" :label="conv.title" :value="conv.id" />
      </el-select>
      <el-button type="primary" text @click="onNewConversation" style="margin-left: 8px">新对话</el-button>
    </div>

    <div v-if="timingStatus && timingStatus !== 'none'" class="timing-status" :class="'timing-' + timingStatus">
      <span class="timing-dot"></span>
      <span class="timing-text">{{ timingStatusText }}</span>
      <template v-if="timingStatus === 'paused'">
        <el-button size="small" text type="primary" @click="onResumeReply" style="margin-left: 8px">现在回复</el-button>
      </template>
    </div>

    <div class="chat-body" v-if="selectedCharacter">
      <ChatPanel
        :messages="displayMessages"
        :loading="loading"
        :sending="loading"
        :call-active="callActive"
        :character-name="selectedCharacter.name"
        :character-avatar="selectedCharacter.avatar"
        @send="onSend"
        @toggle-call="toggleCall"
        @voiceAudio="handleVoiceAudio"
        @voiceText="handleVoiceText"
      />
    </div>
    <div class="chat-empty" v-else>
      <el-empty description="请先选择或创建一个角色" />
    </div>

    <RealtimeCallWidget
      v-if="callActive && showRealtimeCall"
      :visible="callActive"
      :api-key="ttsApiKey"
      :voice-type="callVoiceType"
      :resource-id="ttsResourceId"
      :conversation-id="currentConversationId"
      :dialog-id="currentConversationId"
      @state-change="onCallStateChange"
    />

    <div v-if="selectedCharacter && timingStatus && timingStatus !== 'none'" class="timing-actions">
      <el-button size="small" @click="onHoldReply" :disabled="timingStatus === 'paused'">先别回</el-button>
      <el-button size="small" type="primary" @click="onForceReply">现在回复</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from "vue"
import axios from "axios"
import type { Character, Conversation, Message, ApiResponse } from "@/types"
import { ChatPanel } from "../../ui-index"
import RealtimeCallWidget from "../../components/RealtimeCallWidget.vue"
import { ElMessage } from "element-plus"

const API = "http://127.0.0.1:8899"

async function handleVoiceAudio(blob: Blob, transcript?: string, duration?: number) {
  try {
    const formData = new FormData()
    formData.append("audio", blob, "voice.webm")
    const token = localStorage.getItem("ai-companion-token") || ""
    const res = await fetch(API + "/api/voice/upload", {
      method: "POST",
      headers: { Authorization: "Bearer " + token },
      body: formData,
    })
    if (!res.ok) throw new Error("Voice upload failed")
    const data = await res.json()
    const audioUrl = (data as any)?.data?.audioUrl || (data as any)?.audioUrl || ""
    if (!audioUrl) throw new Error("No audioUrl returned")
    const sendText = transcript || "[语音]"
    await onSend(sendText, undefined, undefined, audioUrl)
  } catch (err: any) {
    console.error("[Voice] upload failed:", err)
    ElMessage.error("语音发送失败")
  }
}

function handleVoiceText(text: string) {
  if (text) {
    onSend(text)
  }
}


const characters = ref<Character[]>([])
const conversations = ref<Conversation[]>([])
const messages = ref<Message[]>([])
const selectedCharacterId = ref("")
const currentConversationId = ref("")
const loading = ref(false)
const callActive = ref(false)

const timingStatus = ref<string | null>(null)
const timingBufferId = ref<number | null>(null)
let pollingTimer: ReturnType<typeof setInterval> | null = null
let eventSource: EventSource | null = null

const ttsApiKey = ref("")
const ttsResourceId = ref("volc.speech.dialog")
const callVoiceType = ref("zh_female_vv_jupiter_bigtts")

const selectedCharacter = computed(() => characters.value.find(c => c.id === selectedCharacterId.value))
const currentConversation = computed(() => conversations.value.find(c => c.id === currentConversationId.value))

const filteredConversations = computed(() =>
  conversations.value.filter(c =>
    c.characterId === selectedCharacterId.value &&
    c.channel !== "wechat" &&
    c.channel !== "qq"
  )
)

const showRealtimeCall = computed(() => {
  if (!selectedCharacter.value || !currentConversationId.value) return false
  const conv = currentConversation.value
  if (!conv) return false
  return conv.channel !== "wechat" && conv.channel !== "qq"
})

const timingStatusText = computed(() => {
  const map: Record<string, string> = {
    waiting: "正在等你说完…",
    checking: "正在判断是否需要回复…",
    replying: "正在回复…",
    paused: "已暂停回复，点击“现在回复”继续",
  }
  return map[timingStatus.value || ""] || ""
})

const displayMessages = computed(() => {
  return messages.value
})

onMounted(async () => {
  const { data: c } = await axios.get<ApiResponse<Character[]>>(API + "/api/characters")
  if (c.code === 200 && c.data) characters.value = c.data
  fetchTtsConfig()
})

onUnmounted(() => {
  stopPolling()
  disconnectSSE()
})

async function fetchTtsConfig() {
  try {
    const { data } = await axios.get<ApiResponse<any[]>>("/api/tts/configs")
    const configs = (data as any)?.data || data || [] ; const arr = Array.isArray(configs) ? configs : (configs?.data || []) ; const cfg = arr.find((c: any) => c.isActive) || arr[0]
    
    if (cfg) {
      const full = await axios.get<ApiResponse<any>>("/api/tts/configs/" + cfg.id)
      const fullData = (full.data as any)?.data || full.data || {} ; ttsApiKey.value = fullData?.apiKey || ""
      ttsResourceId.value = fullData?.resourceId || "volc.speech.dialog"
      }
  } catch (e: any) { console.error("fetchTtsConfig failed", e) }
}

async function onCharacterChange() {
  currentConversationId.value = ""
  messages.value = []
  timingStatus.value = null
  callActive.value = false
  if (!selectedCharacterId.value) return
  const { data } = await axios.get<ApiResponse<Conversation[]>>(API + "/api/conversations")
  if (data.code === 200 && data.data) conversations.value = data.data
  updateCallVoiceType()
}

async function onConversationChange() {
  if (!currentConversationId.value) { messages.value = []; timingStatus.value = null; disconnectSSE(); callActive.value = false; return }
  const { data } = await axios.get<ApiResponse<Message[]>>(API + "/api/conversations/" + currentConversationId.value + "/messages")
  if (data.code === 200 && data.data) messages.value = data.data
  await checkTimingStatus()
  connectSSE()
  updateCallVoiceType()
}

function onNewConversation() {
  currentConversationId.value = ""
  messages.value = []
  timingStatus.value = null
  callActive.value = false
  stopPolling()
}

async function toggleCall() {
  if (!showRealtimeCall.value) {
    ElMessage.warning("当前对话不支持语音通话")
    return
  }
  if (callActive.value) {
    callActive.value = false
  } else {
    if (!ttsApiKey.value) { await fetchTtsConfig() }
    if (!ttsApiKey.value) { ElMessage.warning("请先在模型配置中设置语音API Key"); return }


    callActive.value = true
  }
}

function updateCallVoiceType() {
  const char = selectedCharacter.value
  if (char?.voiceType) {
    callVoiceType.value = char.voiceType
  } else if (char?.customVoiceId) {
    callVoiceType.value = char.customVoiceId
  }
}

function onCallStateChange(state: string) {
  if (state === "idle") {
    callActive.value = false
  }
  if (state === "connected") {
    timingStatus.value = null
    stopPolling()
  }
}

async function onSend(text: string, imageBase64?: string, videoBase64?: string, audioUrl?: string) {
  if (!selectedCharacterId.value) return
  loading.value = true
  try {
    const payload: any = {
      characterId: selectedCharacterId.value,
      message: text,
    }
    if (imageBase64) {
      payload.imageUrl = imageBase64
    }
    if (videoBase64) {
      loading.value = true
      try {
        const blob = await (await fetch(videoBase64)).blob()
        const formData = new FormData()
        formData.append("video", blob, "video.mp4")
        const uploadResp = await axios.post(API + "/api/video/upload", formData, {
          headers: { "Content-Type": "multipart/form-data" }
        })
        if (uploadResp.data?.data?.videoUrl) {
          payload.videoUrl = uploadResp.data.data.videoUrl
        }
      } catch (e: any) {
        ElMessage.error("视频上传失败: " + (e?.message || "未知错误"))
        loading.value = false
        return
      }
    }
    if (audioUrl) {
      payload.audioUrl = audioUrl
      payload.voiceMessage = true
    }
    if (currentConversationId.value) {
      payload.conversationId = currentConversationId.value
    }

    const { data } = await axios.post<ApiResponse<any>>(API + "/api/web-chat/send", payload)

    if (data.code === 200 && data.data) {
      if (data.data.conversationId) {
        currentConversationId.value = data.data.conversationId
        await onCharacterChange()
        connectSSE()
      }

      if (data.data.userMessage) {
        messages.value = [...messages.value, data.data.userMessage]
      }

      timingStatus.value = data.data.status || "waiting"
      timingBufferId.value = data.data.bufferId

      startPolling()
    } else {
      ElMessage.error(data.message || "发送失败")
    }
  } catch (err: any) {
    ElMessage.error("发送失败: " + (err?.response?.data?.message || err.message))
  } finally {
    loading.value = false
  }
}

async function checkTimingStatus() {
  if (!currentConversationId.value) return
  try {
    const { data } = await axios.get<ApiResponse<any>>(
      API + "/api/web-chat/conversations/" + currentConversationId.value + "/reply-timing/status"
    )
    if (data.code === 200 && data.data) {
      if (data.data.hasActiveBuffer) {
        timingStatus.value = data.data.status
        timingBufferId.value = data.data.bufferId
        startPolling()
      } else {
        timingStatus.value = "none"
        stopPolling()
      }
    }
  } catch {
    timingStatus.value = "none"
  }
}

function startPolling() {
  stopPolling()
  pollingTimer = setInterval(pollForMessages, 1500)
}

function stopPolling() {
  if (pollingTimer) {
    clearInterval(pollingTimer)
    pollingTimer = null
  }
}

function connectSSE() {
  disconnectSSE()
  const cid = currentConversationId.value
  if (!cid) return
  eventSource = new EventSource(API + "/api/messages/stream?conversationId=" + encodeURIComponent(cid))
  eventSource.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      if (data.type === "new_message" && data.message) {
        const exists = messages.value.some(m => m.id === data.message.id)
        if (!exists) {
          messages.value = [...messages.value, data.message]
        }
      }
    } catch {}
  }
  eventSource.onerror = () => {
    disconnectSSE()
    setTimeout(() => { if (currentConversationId.value) connectSSE() }, 3000)
  }
}

function disconnectSSE() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
}

async function pollForMessages() {
  if (!currentConversationId.value) return
  try {
    const { data } = await axios.get<ApiResponse<Message[]>>(
      API + "/api/conversations/" + currentConversationId.value + "/messages"
    )
    if (data.code === 200 && data.data) {
      const newMsgs = data.data
      const existingIds = new Set(messages.value.map((m: Message) => m.id))
      const added = newMsgs.filter((m: Message) => !existingIds.has(m.id))
      if (added.length > 0) {
        messages.value = [...messages.value, ...added]
      }
      const lastMsg = newMsgs[newMsgs.length - 1]
      if (lastMsg && lastMsg.role === "assistant") {
        timingStatus.value = "none"
        stopPolling()
      }
    }
    await checkTimingStatus()
  } catch {}
}

async function onForceReply() {
  if (!currentConversationId.value) return
  try {
    await axios.post(API + "/api/web-chat/conversations/" + currentConversationId.value + "/reply-timing/force")
    timingStatus.value = "replying"
    ElMessage.success("已触发回复")
  } catch (err: any) {
    ElMessage.error("操作失败")
  }
}

async function onHoldReply() {
  if (!currentConversationId.value) return
  try {
    await axios.post(API + "/api/web-chat/conversations/" + currentConversationId.value + "/reply-timing/hold")
    timingStatus.value = "paused"
    ElMessage.success("已暂停自动回复")
  } catch (err: any) {
    ElMessage.error("操作失败")
  }
}

async function onResumeReply() {
  if (!currentConversationId.value) return
  try {
    await axios.post(API + "/api/web-chat/conversations/" + currentConversationId.value + "/reply-timing/resume")
    timingStatus.value = "waiting"
    ElMessage.success("已恢复")
    startPolling()
  } catch (err: any) {
    ElMessage.error("操作失败")
  }
}
</script>

<style scoped>
.chat-view { display: flex; flex-direction: column; height: 100vh; }
.chat-header { display: flex; align-items: center; padding: 10px 16px; border-bottom: 1px solid var(--el-border-color-light); background: var(--el-bg-color); }
.chat-body { flex: 1; overflow: hidden; }
.chat-empty { flex: 1; display: flex; align-items: center; justify-content: center; }

.timing-status {
  display: flex;
  align-items: center;
  padding: 6px 16px;
  background: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color-lighter);
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.timing-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  margin-right: 8px;
  background: var(--el-color-warning);
  animation: pulse 1.5s ease-in-out infinite;
}
.timing-replying .timing-dot { background: var(--el-color-primary); }
.timing-paused .timing-dot { background: var(--el-color-danger); animation: none; }
@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.3; } }

.timing-actions {
  display: flex;
  gap: 8px;
  padding: 8px 16px;
  border-top: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
}
</style>
