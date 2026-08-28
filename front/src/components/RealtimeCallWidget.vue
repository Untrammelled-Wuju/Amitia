<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="realtime-widget" v-if="visible">
    <div class="rw-bar" :class="{ active: callState === 'connected' }">
      <div class="rw-left">
        <div class="rw-icon" :class="callState">
          <span class="rw-dot"></span>
        </div>
        <span class="rw-status">{{ statusLabel }}</span>
        <span class="rw-duration" v-if="callState === 'connected'">{{
          formatDuration(callDuration)
        }}</span>
      </div>
      <div class="rw-right">
        <span class="rw-error" v-if="callState === 'error'" :title="errorMsg">{{
          errorMsg
        }}</span>
        <el-button
          v-if="callState === 'idle' || callState === 'error'"
          type="primary"
          size="small"
          :icon="Phone"
          @click="start"
        >
          语音通话
        </el-button>
        <el-button
          v-else-if="callState === 'connected'"
          type="danger"
          size="small"
          @click="stop"
        >
          挂断
        </el-button>
        <el-button
          v-else-if="callState === 'connecting'"
          type="warning"
          size="small"
          loading
        >
          连接中
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from "vue";
import { Phone } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { resolveWebSocketUrl } from "../runtime/runtime-adapter";
import { notifyDesktopPetChatState } from "../runtime/desktop-pet-chat-state";

const props = defineProps<{
  visible: boolean;
  apiKey: string;
  voiceType: string;
  resourceId: string;
  conversationId: string;
  dialogId?: string;
}>();

const emit = defineEmits<{
  message: [data: { role: string; text: string }];
  stateChange: [state: string];
}>();

type CallState = "idle" | "connecting" | "connected" | "error";

const callState = ref<CallState>("idle");
const callDuration = ref(0);
const errorMsg = ref("");
let ws: WebSocket | null = null;
let audioCtx: AudioContext | null = null;
let playCtx: AudioContext | null = null;
let mediaStream: MediaStream | null = null;
let scriptNode: ScriptProcessorNode | null = null;
let durationTimer: ReturnType<typeof setInterval> | null = null;
let nextPlayTime = 0;
let isAiSpeaking = false;
let petSpeakingDrainTimer: ReturnType<typeof setTimeout> | null = null;
const statusLabel = computed(() => {
  const map: Record<string, string> = {
    idle: "未连接",
    connecting: "连接中...",
    connected: "通话中",
    error: "连接失败",
  };
  return map[callState.value] || "";
});

watch(
  () => props.visible,
  (visible) => {
    if (visible && callState.value === "idle") {
      void start();
      return;
    }
    if (!visible && callState.value !== "idle") {
      stop();
    }
  },
  { immediate: true },
);

function formatDuration(s: number): string {
  const m = Math.floor(s / 60)
    .toString()
    .padStart(2, "0");
  const sec = (s % 60).toString().padStart(2, "0");
  return m + ":" + sec;
}

async function start() {
  if (!props.apiKey) {
    ElMessage.warning("请先在模型配置中设置语音API Key");
    return;
  }

  callState.value = "connecting";
  errorMsg.value = "";
  emit("stateChange", "connecting");

  try {
    mediaStream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        sampleRate: 16000,
        echoCancellation: true,
        noiseSuppression: true,
      },
    });
  } catch (err: any) {
    errorMsg.value = "麦克风访问失败";
    callState.value = "error";
    emit("stateChange", "error");
    notifyDesktopPetChatState("assistant_error", props.conversationId || undefined);
    return;
  }

  audioCtx = new AudioContext({ sampleRate: 16000 });
  playCtx = new AudioContext({ sampleRate: 24000 });
  const inputCtx = audioCtx;
  if (!inputCtx || !mediaStream) return;
  const source = inputCtx.createMediaStreamSource(mediaStream);
  scriptNode = inputCtx.createScriptProcessor(4096, 1, 1);

  const baseUrl = await resolveWebSocketUrl("/api/realtime/session");
  const params = new URLSearchParams({
    apiKey: props.apiKey,
    voiceType: props.voiceType || "zh_female_vv_jupiter_bigtts",
    resourceId: props.resourceId || "volc.speech.dialog",
    conversationId: props.conversationId,
    dialogId: props.dialogId || "",
  });
  ws = new WebSocket(baseUrl + "?" + params.toString());

  ws.onopen = () => {
    callDuration.value = 0;
    durationTimer = setInterval(() => {
      callDuration.value++;
    }, 1000);

    if (scriptNode) {
      scriptNode.onaudioprocess = (e) => {
        if (!isAiSpeaking && ws && ws.readyState === WebSocket.OPEN) {
          const input = e.inputBuffer.getChannelData(0);
          const pcm = float32ToPCM(input);
          const base64 = arrayBufferToBase64(pcm);
          ws.send(JSON.stringify({ event: "audio", data: base64 }));
        }
      };
      const silenceGain = inputCtx.createGain();
      silenceGain.gain.value = 0;
      source.connect(scriptNode);
      scriptNode.connect(silenceGain);
      silenceGain.connect(inputCtx.destination);
    }
  };

  ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data);
      switch (msg.event) {
        case "ChatTextResponse":
          if (msg.data?.text) {
            emit("message", { role: "assistant", text: msg.data.text });
          }
          break;
        case "audio":
          playAudio(msg.data);
          break;
        case "SessionFinished":
          emit("message", { role: "assistant", text: "[通话结束]" });
          break;
        case "connected":
          callState.value = "connected";
          emit("stateChange", "connected");
          notifyDesktopPetChatState(
            "assistant_listening",
            props.conversationId || undefined,
          );
          break;
        case "error":
          errorMsg.value = msg.data || "连接错误";
          callState.value = "error";
          emit("stateChange", "error");
          notifyDesktopPetChatState(
            "assistant_error",
            props.conversationId || undefined,
          );
          ElMessage.error(msg.data || "实时通话连接失败");
          cleanupCall();
          break;
      }
    } catch {}
  };

  ws.onclose = () => {
    const failed = callState.value === "error";
    cleanupCall();
    if (!failed && callState.value !== "idle") {
      callState.value = "idle";
      emit("stateChange", "idle");
      notifyDesktopPetChatState(
        "assistant_finished",
        props.conversationId || undefined,
      );
    }
  };

  ws.onerror = () => {
    errorMsg.value = "WebSocket连接失败";
    callState.value = "error";
    emit("stateChange", "error");
    notifyDesktopPetChatState(
      "assistant_error",
      props.conversationId || undefined,
    );
    cleanupCall();
  };
}

function stop() {
  const activeSocket = ws;
  if (activeSocket && activeSocket.readyState === WebSocket.OPEN) {
    activeSocket.send(JSON.stringify({ event: "stop" }));
    setTimeout(() => {
      if (activeSocket.readyState === WebSocket.OPEN || activeSocket.readyState === WebSocket.CLOSING) {
        activeSocket.close();
      }
    }, 500);
  }
  cleanupCall();
  callState.value = "idle";
  emit("stateChange", "idle");
  notifyDesktopPetChatState(
    "assistant_finished",
    props.conversationId || undefined,
  );
  callDuration.value = 0;
}

function cleanupCall() {
  if (durationTimer) {
    clearInterval(durationTimer);
    durationTimer = null;
  }
  ws = null;
  if (scriptNode) {
    scriptNode.disconnect();
    scriptNode = null;
  }
  if (mediaStream) {
    mediaStream.getTracks().forEach((t) => t.stop());
    mediaStream = null;
  }
  nextPlayTime = 0;
  isAiSpeaking = false;
  if (petSpeakingDrainTimer) {
    clearTimeout(petSpeakingDrainTimer);
    petSpeakingDrainTimer = null;
  }
  if (playCtx && playCtx.state !== "closed") {
    setTimeout(() => {
      try {
        playCtx?.close();
      } catch {}
      playCtx = null;
    }, 3000);
  }
  if (audioCtx && audioCtx.state !== "closed") {
    void audioCtx.close();
    audioCtx = null;
  }
}

function float32ToPCM(float32: Float32Array): ArrayBuffer {
  const buffer = new ArrayBuffer(float32.length * 2);
  const view = new DataView(buffer);
  for (let i = 0; i < float32.length; i++) {
    let s = Math.max(-1, Math.min(1, float32[i]));
    s = s < 0 ? s * 0x8000 : s * 0x7fff;
    view.setInt16(i * 2, s, true);
  }
  return buffer;
}

function arrayBufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

function playAudio(base64Data: string) {
  try {
    if (!playCtx || playCtx.state === "closed") {
      playCtx = new AudioContext({ sampleRate: 24000 });
    }
    if (playCtx.state === "suspended") {
      playCtx.resume();
    }
    const binaryStr = atob(base64Data);
    const len = binaryStr.length;
    const pcmBuf = new ArrayBuffer(len);
    const pcmView = new DataView(pcmBuf);
    for (let i = 0; i < len; i++) {
      pcmView.setUint8(i, binaryStr.charCodeAt(i));
    }
    const samples = len / 2;
    const audioBuffer = playCtx.createBuffer(1, samples, 24000);
    const channelData = audioBuffer.getChannelData(0);
    for (let i = 0; i < samples; i++) {
      channelData[i] = pcmView.getInt16(i * 2, true) / 32768;
    }
    const src = playCtx.createBufferSource();
    src.buffer = audioBuffer;
    src.connect(playCtx.destination);
    const now = playCtx.currentTime;
    nextPlayTime = Math.max(now, nextPlayTime);
    src.start(nextPlayTime);
    if (!isAiSpeaking) {
      isAiSpeaking = true;
      notifyDesktopPetChatState(
        "assistant_speaking",
        props.conversationId || undefined,
      );
    }
    nextPlayTime += audioBuffer.duration;
    if (petSpeakingDrainTimer) {
      clearTimeout(petSpeakingDrainTimer);
    }
    const remainingMs = Math.max(0, (nextPlayTime - playCtx.currentTime) * 1000);
    petSpeakingDrainTimer = setTimeout(() => {
      petSpeakingDrainTimer = null;
      if (!playCtx || playCtx.state === "closed") return;
      if (playCtx.currentTime + 0.05 < nextPlayTime) return;
      isAiSpeaking = false;
      if (callState.value === "connected") {
        notifyDesktopPetChatState(
          "assistant_listening",
          props.conversationId || undefined,
        );
      }
    }, remainingMs + 120);
  } catch {}
}

onUnmounted(() => {
  stop();
});
</script>

<style scoped>
.realtime-widget {
  flex-shrink: 0;
}
.rw-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 14px;
  border-radius: var(--radius-sm);
  background: var(--plugin-muted-bg);
  border: 1px solid var(--surface-border);
  transition:
    border-color 0.3s,
    background 0.3s;
}
.rw-bar.active {
  border-color: var(--el-color-success);
  background: var(--el-color-success-light-9);
}
.rw-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.rw-icon {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--el-color-info);
}
.rw-icon.connected {
  background: var(--el-color-success);
  animation: pulse 1.5s infinite;
}
.rw-icon.connecting {
  background: var(--el-color-warning);
  animation: pulse 0.8s infinite;
}
.rw-icon.error {
  background: var(--el-color-danger);
}
@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.3;
  }
}
.rw-status {
  font-size: 13px;
  color: var(--el-text-color-regular);
}
.rw-duration {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  font-variant-numeric: tabular-nums;
}
.rw-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.rw-error {
  font-size: 12px;
  color: var(--el-color-danger);
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
