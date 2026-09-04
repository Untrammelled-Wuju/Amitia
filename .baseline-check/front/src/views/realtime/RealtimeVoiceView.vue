<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="page">
    <h2 class="page-title">实时多模态通话</h2>

    <el-alert type="info" :closable="false" show-icon style="margin-bottom: 14px">
      <template #title>
        支持语音、摄像头和屏幕共享。视觉画面采用变化检测与关键帧采样，不会把完整视频逐帧发送给模型。
      </template>
    </el-alert>

    <el-card>
      <template #header>会话配置</template>
      <el-form label-position="top" :inline="true">
        <el-form-item label="会话 ID">
          <el-input v-model="config.conversationId" placeholder="可选，用于绑定角色与对话" style="width: 300px" />
        </el-form-item>
        <el-form-item label="音色">
          <el-select v-model="config.voiceType" style="width: 240px">
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
          <span v-if="visionStatus" class="vision-status">{{ visionStatus }}</span>
        </div>
        <div class="call-timer" v-if="callDuration > 0">{{ formatTime(callDuration) }}</div>
      </div>

      <div v-if="media.camera || media.screen" class="media-stage">
        <video v-show="media.screen" ref="screenPreview" class="screen-preview" autoplay muted playsinline />
        <video v-show="media.camera" ref="cameraPreview" class="camera-preview" autoplay muted playsinline />
      </div>

      <div class="call-controls">
        <el-button v-if="connectionStatus !== 'connected'" type="primary" size="large" :loading="connecting" @click="startCall">
          开始通话
        </el-button>
        <template v-else>
          <el-button :type="media.muted ? 'warning' : 'default'" @click="toggleMute">{{ media.muted ? '取消静音' : '静音' }}</el-button>
          <el-button :type="media.camera ? 'primary' : 'default'" @click="toggleCamera">{{ media.camera ? '关闭摄像头' : '打开摄像头' }}</el-button>
          <el-button :type="media.screen ? 'primary' : 'default'" @click="toggleScreen">{{ media.screen ? '停止共享' : '共享屏幕' }}</el-button>
          <el-button type="danger" @click="stopCall">结束通话</el-button>
        </template>
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
import { nextTick, onUnmounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { publishLocalVoiceASRFinal } from "../../runtime/runtime-adapter";
import {
  RealtimeCallController,
  type RealtimeCallState,
  type RealtimeMediaState,
} from "../../realtime/realtime-call-controller";

const voiceList = [
  { name: "zh_female_vv_jupiter_bigtts", label: "vv - 活泼灵动女声" },
  { name: "zh_female_xiaohe_jupiter_bigtts", label: "xiaohe - 甜美活泼女声" },
  { name: "zh_male_yunzhou_jupiter_bigtts", label: "yunzhou - 清爽沉稳男声" },
  { name: "zh_male_xiaotian_jupiter_bigtts", label: "xiaotian - 清爽磁性男声" },
];

const config = reactive({
  conversationId: "",
  resourceId: "volc.speech.dialog",
  voiceType: "zh_female_vv_jupiter_bigtts",
});
const connecting = ref(false);
const connectionStatus = ref<RealtimeCallState>("idle");
const statusText = ref("就绪");
const visionStatus = ref("");
const callDuration = ref(0);
const messages = ref<{ role: string; text: string }[]>([]);
const media = ref<RealtimeMediaState>({ audio: false, camera: false, screen: false, muted: false });
const cameraPreview = ref<HTMLVideoElement | null>(null);
const screenPreview = ref<HTMLVideoElement | null>(null);
let controller: RealtimeCallController | null = null;
let durationTimer: ReturnType<typeof setInterval> | null = null;

function buildController(): RealtimeCallController {
  return new RealtimeCallController({
    conversationId: config.conversationId,
    voiceType: config.voiceType,
    resourceId: config.resourceId,
    onState: (state, error) => {
      connectionStatus.value = state;
      connecting.value = state === "connecting";
      statusText.value = state === "connecting" ? "连接中..." : state === "connected" ? "通话中" : state === "error" ? (error || "连接失败") : "就绪";
      if (state === "connected") startTimer();
      if (state === "idle" || state === "error") stopTimer();
    },
    onAssistantText: (text) => messages.value.push({ role: "assistant", text }),
    onASRFinal: (data) => {
      const transcript = typeof data.transcript === "string" ? data.transcript.trim() : "";
      if (transcript) messages.value.push({ role: "user", text: transcript });
      void forwardASRFinal(data);
    },
    onVision: (data) => {
      if (typeof data.context === "string" && data.context.trim()) visionStatus.value = "视觉上下文已更新";
      if (data.available === false) visionStatus.value = "视觉模型暂不可用";
    },
    onMediaState: (state) => { media.value = state; },
    onCameraPreview: (stream) => void assignPreview(cameraPreview, stream),
    onScreenPreview: (stream) => void assignPreview(screenPreview, stream),
  });
}

async function startCall() {
  if (controller) await controller.stop().catch(() => undefined);
  controller = buildController();
  messages.value = [];
  visionStatus.value = "";
  callDuration.value = 0;
  try {
    await controller.start();
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "实时通话启动失败");
  }
}

async function stopCall() {
  const active = controller;
  controller = null;
  if (active) await active.stop().catch(() => undefined);
  connectionStatus.value = "idle";
  statusText.value = "就绪";
  callDuration.value = 0;
  stopTimer();
}

async function toggleMute() {
  if (controller) await controller.setMuted(!media.value.muted);
}

async function toggleCamera() {
  if (!controller) return;
  try { await controller.toggleCamera(); }
  catch (error) { ElMessage.error("摄像头启动失败：" + (error instanceof Error ? error.message : String(error))); }
}

async function toggleScreen() {
  if (!controller) return;
  try { await controller.toggleScreen(); }
  catch (error) { ElMessage.error("屏幕共享启动失败：" + (error instanceof Error ? error.message : String(error))); }
}

async function assignPreview(target: typeof cameraPreview, stream: MediaStream | null) {
  await nextTick();
  if (!target.value) return;
  target.value.srcObject = stream;
  if (stream) await target.value.play().catch(() => undefined);
}

async function forwardASRFinal(data: Record<string, unknown>) {
  const transcript = typeof data.transcript === "string" ? data.transcript.trim() : "";
  const eventId = typeof data.eventId === "string" ? data.eventId.trim() : "";
  if (!transcript || !eventId) return;
  try {
    await publishLocalVoiceASRFinal({
      eventId,
      transcript,
      sessionId: typeof data.sessionId === "string" ? data.sessionId : undefined,
      conversationId: typeof data.conversationId === "string" ? data.conversationId : config.conversationId || undefined,
      visualContext: typeof data.visualContext === "string" ? data.visualContext : undefined,
      visualSource: data.visualSource === "camera" || data.visualSource === "screen" ? data.visualSource : undefined,
      occurredAt: new Date().toISOString(),
    });
  } catch (error) {
    console.warn("[RealtimeCallView] 本地工作流事件投递失败", error);
  }
}

function startTimer() {
  stopTimer();
  durationTimer = setInterval(() => callDuration.value++, 1000);
}
function stopTimer() {
  if (durationTimer) clearInterval(durationTimer);
  durationTimer = null;
}
function formatTime(seconds: number): string {
  const minutes = Math.floor(seconds / 60).toString().padStart(2, "0");
  const rest = (seconds % 60).toString().padStart(2, "0");
  return `${minutes}:${rest}`;
}

onUnmounted(() => { void stopCall(); });
</script>

<style scoped>
.page { padding: 20px; }
.page-title { margin: 0 0 16px; }
.call-area { margin-top: 18px; display: flex; flex-direction: column; gap: 14px; }
.call-status { display: flex; align-items: center; justify-content: space-between; }
.status-indicator { display: flex; align-items: center; gap: 8px; }
.status-dot { width: 9px; height: 9px; border-radius: 50%; background: var(--el-color-info); }
.status-dot.connected { background: var(--el-color-success); }
.status-dot.connecting { background: var(--el-color-warning); }
.status-dot.error { background: var(--el-color-danger); }
.vision-status { color: var(--el-text-color-secondary); font-size: 12px; }
.call-timer { font-variant-numeric: tabular-nums; color: var(--el-text-color-secondary); }
.media-stage { min-height: 240px; max-height: 520px; position: relative; display: flex; align-items: center; justify-content: center; overflow: hidden; border-radius: 14px; background: #111; }
.screen-preview { width: 100%; height: 100%; max-height: 520px; object-fit: contain; }
.camera-preview { width: 100%; height: 100%; max-height: 520px; object-fit: cover; }
.media-stage .screen-preview:not([style*="display: none"]) + .camera-preview { position: absolute; right: 16px; bottom: 16px; width: 180px; height: 120px; border-radius: 10px; border: 1px solid rgba(255,255,255,.3); box-shadow: 0 8px 24px rgba(0,0,0,.35); }
.call-controls { display: flex; justify-content: center; gap: 10px; flex-wrap: wrap; }
.chat-log { display: flex; flex-direction: column; gap: 8px; max-height: 280px; overflow: auto; }
.chat-msg { display: flex; gap: 8px; padding: 8px 10px; border-radius: 8px; background: var(--el-fill-color-light); }
.msg-role { font-weight: 600; flex: 0 0 auto; }
</style>
