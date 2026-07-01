<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="page">
    <h2 class="page-title">实时语音通话</h2>

    <el-alert type="info" :closable="false" show-icon style="margin-bottom:14px">
      <template #title>
        端到端实时语音大模型，支持低延迟语音对话。使用WebSocket双向流式传输，浏览器麦克风直接采集。
      </template>
    </el-alert>

    <el-card>
      <template #header>会话配置</template>
      <el-form label-position="top" :inline="true">
        <el-form-item label="API Key">
          <el-input v-model="config.apiKey" type="password" show-password placeholder="火山引擎API Key" style="width:260px" />
        </el-form-item>
        <el-form-item label="资源ID">
          <el-input v-model="config.resourceId" placeholder="volc.speech.dialog" style="width:200px" />
        </el-form-item>
        <el-form-item label="音色">
          <el-select v-model="config.voiceType" style="width:240px">
            <el-option v-for="v in voiceList" :key="v.name" :value="v.name" :label="v.label" />
          </el-select>
        </el-form-item>
      </el-form>
    </el-card>

    <div class="call-area">
      <div class="call-status" :class="connectionStatus">
        <div class="status-indicator">
          <span class="status-dot" :class="connectionStatus"></span>
          <span class="status-text">{{ statusText }}</span>
        </div>
        <div class="call-timer" v-if="callDuration > 0">{{ formatTime(callDuration) }}</div>
      </div>

      <div class="call-visual" :class="{ active: connectionStatus === 'connected' }">
        <div class="visual-rings">
          <div class="ring r1"></div>
          <div class="ring r2"></div>
          <div class="ring r3"></div>
        </div>
        <div class="visual-center">
          <el-button
            class="call-btn"
            :class="{ recording: connectionStatus === 'connected' }"
            :type="connectionStatus === 'connected' ? 'danger' : 'primary'"
            :icon="connectionStatus === 'connected' ? undefined : Phone"
            circle
            size="large"
            :loading="connecting"
            @click="toggleCall"
          />
        </div>
      </div>

      <div class="chat-log" v-if="messages.length > 0">
        <div v-for="(msg, i) in messages" :key="i" class="chat-msg" :class="msg.role">
          <span class="msg-role">{{ msg.role === 'user' ? '你' : 'AI' }}</span>
          <span class="msg-text">{{ msg.text }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onUnmounted } from "vue"
import { ElMessage } from "element-plus"
import { Phone } from "@element-plus/icons-vue"
import { resolveWebSocketUrl } from "../../runtime/runtime-adapter"

const voiceList = [
  { name: "zh_female_vv_jupiter_bigtts", label: "vv - 活泼灵动女声" },
  { name: "zh_female_xiaohe_jupiter_bigtts", label: "xiaohe - 甜美活泼女声(台湾口音)" },
  { name: "zh_male_yunzhou_jupiter_bigtts", label: "yunzhou - 清爽沉稳男声" },
  { name: "zh_male_xiaotian_jupiter_bigtts", label: "xiaotian - 清爽磁性男声" },
  { name: "en_male_tim_uranus_bigtts", label: "Tim - 美式英语男声(O2.0)" },
  { name: "en_female_dacey_uranus_bigtts", label: "Dacey - 美式英语女声(O2.0)" },
  { name: "en_female_stokie_uranus_bigtts", label: "Stokie - 美式英语女声(O2.0)" },
]

const config = reactive({
  apiKey: "",
  resourceId: "volc.speech.dialog",
  voiceType: "zh_female_vv_jupiter_bigtts",
})

const connecting = ref(false)
const connectionStatus = ref<"idle" | "connecting" | "connected" | "error">("idle")
const callDuration = ref(0)
const messages = ref<{ role: string; text: string }[]>([])
let ws: WebSocket | null = null
let audioCtx: AudioContext | null = null
let mediaStream: MediaStream | null = null
let scriptNode: ScriptProcessorNode | null = null
let durationTimer: ReturnType<typeof setInterval> | null = null

const statusText = ref("就绪")

function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60).toString().padStart(2, "0")
  const s = (seconds % 60).toString().padStart(2, "0")
  return m + ":" + s
}

async function toggleCall() {
  if (connectionStatus.value === "connected") {
    await stopCall()
  } else {
    await startCall()
  }
}

async function startCall() {
  if (!config.apiKey.trim()) {
    ElMessage.warning("请先填入API Key")
    return
  }
  connecting.value = true
  connectionStatus.value = "connecting"
  statusText.value = "连接中..."

  try {
    mediaStream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        sampleRate: 16000,
        echoCancellation: true,
        noiseSuppression: true,
      },
    })
  } catch (err: any) {
    ElMessage.error("麦克风访问失败: " + (err?.message || "未知错误"))
    connectionStatus.value = "error"
    statusText.value = "麦克风错误"
    connecting.value = false
    return
  }

  audioCtx = new AudioContext({ sampleRate: 16000 })
  const source = audioCtx.createMediaStreamSource(mediaStream)
  scriptNode = audioCtx.createScriptProcessor(4096, 1, 1)

  const wsBaseUrl = await resolveWebSocketUrl("/api/realtime/session")
  const wsUrl = wsBaseUrl +
    "?apiKey=" + encodeURIComponent(config.apiKey) +
    "&voiceType=" + encodeURIComponent(config.voiceType) +
    "&resourceId=" + encodeURIComponent(config.resourceId)

  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    connectionStatus.value = "connected"
    statusText.value = "通话中"
    connecting.value = false
    callDuration.value = 0
    messages.value = []
    durationTimer = setInterval(() => { callDuration.value++ }, 1000)

    scriptNode!.onaudioprocess = (e) => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        const input = e.inputBuffer.getChannelData(0)
        const pcm = float32ToPCM(input)
        const base64 = arrayBufferToBase64(pcm)
        ws.send(JSON.stringify({ event: "audio", data: base64 }))
      }
    }
    source.connect(scriptNode!)
    scriptNode!.connect(audioCtx!.destination)
  }

  ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      switch (msg.event) {
        case "ChatTextResponse":
          if (msg.data?.text) {
            messages.value.push({ role: "assistant", text: msg.data.text })
          }
          break
        case "audio":
          playAudio(msg.data)
          break
        case "SessionFinished":
          messages.value.push({ role: "assistant", text: "通话结束" })
          break
        case "error":
          ElMessage.error(msg.data || "连接错误")
          stopCall()
          break
      }
    } catch {}
  }

  ws.onclose = () => {
    cleanup()
    if (connectionStatus.value === "connected") {
      connectionStatus.value = "idle"
      statusText.value = "通话已结束"
    }
  }

  ws.onerror = () => {
    ElMessage.error("WebSocket连接失败")
    connectionStatus.value = "error"
    statusText.value = "连接失败"
    connecting.value = false
    cleanup()
  }
}

async function stopCall() {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ event: "stop" }))
    setTimeout(() => {
      if (ws) ws.close()
    }, 500)
  }
  cleanup()
  connectionStatus.value = "idle"
  statusText.value = "就绪"
  callDuration.value = 0
  if (durationTimer) { clearInterval(durationTimer); durationTimer = null }
}

function cleanup() {
  if (scriptNode) {
    scriptNode.disconnect()
    scriptNode = null
  }
  if (mediaStream) {
    mediaStream.getTracks().forEach((t) => t.stop())
    mediaStream = null
  }
  if (audioCtx && audioCtx.state !== "closed") {
    audioCtx.close()
    audioCtx = null
  }
}

function float32ToPCM(float32: Float32Array): ArrayBuffer {
  const buffer = new ArrayBuffer(float32.length * 2)
  const view = new DataView(buffer)
  for (let i = 0; i < float32.length; i++) {
    let s = Math.max(-1, Math.min(1, float32[i]))
    s = s < 0 ? s * 0x8000 : s * 0x7FFF
    view.setInt16(i * 2, s, true)
  }
  return buffer
}

function arrayBufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ""
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary)
}

function playAudio(base64Data: string) {
  try {
    const binaryStr = atob(base64Data)
    const bytes = new Uint8Array(binaryStr.length)
    for (let i = 0; i < binaryStr.length; i++) {
      bytes[i] = binaryStr.charCodeAt(i)
    }
    const pcmBuffer = bytes.buffer
    const ctx = new AudioContext({ sampleRate: 24000 })
    const audioBuffer = ctx.createBuffer(1, pcmBuffer.byteLength / 2, 24000)
    const channelData = audioBuffer.getChannelData(0)
    const view = new DataView(pcmBuffer)
    for (let i = 0; i < channelData.length; i++) {
      channelData[i] = view.getInt16(i * 2, true) / 32768
    }
    const source = ctx.createBufferSource()
    source.buffer = audioBuffer
    source.connect(ctx.destination)
    source.start()
    source.onended = () => ctx.close()
  } catch {}
}

onUnmounted(() => {
  stopCall()
})
</script>

<style scoped>
.page { max-width: 640px; margin: 0 auto; padding: 20px 16px; }
.page-title { font-size: 20px; font-weight: 600; margin: 0 0 16px; }

.call-area {
  margin-top: 20px;
  display: flex; flex-direction: column; align-items: center; gap: 16px;
}

.call-status {
  display: flex; align-items: center; justify-content: space-between;
  width: 100%; padding: 10px 16px;
  background: var(--el-fill-color-light); border-radius: 10px;
}
.status-indicator { display: flex; align-items: center; gap: 8px; }
.status-dot {
  width: 10px; height: 10px; border-radius: 50%;
  background: var(--el-color-info);
}
.status-dot.connecting { background: var(--el-color-warning); animation: pulse 1s infinite; }
.status-dot.connected { background: var(--el-color-success); }
.status-dot.error { background: var(--el-color-danger); }
.status-text { font-size: 14px; color: var(--el-text-color-regular); }
.call-timer { font-size: 18px; font-family: monospace; color: var(--el-text-color-secondary); }

.call-visual { position: relative; width: 160px; height: 160px; }
.visual-center { position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); z-index: 2; }
.call-btn { width: 64px; height: 64px; font-size: 24px; transition: all 0.3s; }
.call-btn.recording { animation: btnPulse 2s infinite; }
.visual-rings { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; }
.ring { position: absolute; border-radius: 50%; border: 2px solid var(--el-border-color-light); opacity: 0; }
.r1 { width: 80px; height: 80px; }
.r2 { width: 120px; height: 120px; }
.r3 { width: 160px; height: 160px; }
.call-visual.active .ring {
  border-color: var(--el-color-primary);
  animation: ringExpand 2s ease-out infinite;
}
.call-visual.active .r2 { animation-delay: 0.6s; }
.call-visual.active .r3 { animation-delay: 1.2s; }

@keyframes ringExpand {
  0% { opacity: 0.6; transform: scale(0.7); }
  100% { opacity: 0; transform: scale(1.3); }
}
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}
@keyframes btnPulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(245, 108, 108, 0.4); }
  50% { box-shadow: 0 0 0 12px rgba(245, 108, 108, 0); }
}

.chat-log {
  width: 100%; max-height: 260px; overflow-y: auto;
  background: var(--el-fill-color-light); border-radius: 10px; padding: 12px;
}
.chat-msg { padding: 6px 10px; border-radius: 6px; margin-bottom: 6px; font-size: 13px; }
.chat-msg.assistant { background: var(--el-color-primary-light-9); }
.chat-msg.user { background: var(--el-color-success-light-9); text-align: right; }
.msg-role { font-weight: 600; margin-right: 6px; font-size: 12px; color: var(--el-text-color-secondary); }
.msg-text { color: var(--el-text-color-regular); line-height: 1.5; }
</style>

