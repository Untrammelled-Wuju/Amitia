<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="realtime-widget" v-if="visible">
    <div v-if="media.camera || media.screen" class="rw-previews">
      <video v-show="media.screen" ref="screenPreview" class="rw-preview rw-preview-screen" autoplay muted playsinline />
      <video v-show="media.camera" ref="cameraPreview" class="rw-preview rw-preview-camera" autoplay muted playsinline />
    </div>

    <div class="rw-bar" :class="{ active: callState === 'connected' }">
      <div class="rw-left">
        <div class="rw-icon" :class="callState"><span class="rw-dot"></span></div>
        <span class="rw-status">{{ statusLabel }}</span>
        <span class="rw-duration" v-if="callState === 'connected'">{{ formatDuration(callDuration) }}</span>
        <span class="rw-vision" v-if="visionStatus">{{ visionStatus }}</span>
      </div>

      <div class="rw-right">
        <span class="rw-error" v-if="callState === 'error'" :title="errorMsg">{{ errorMsg }}</span>
        <template v-if="callState === 'connected'">
          <el-button size="small" :type="media.muted ? 'warning' : 'default'" @click="toggleMute">
            {{ media.muted ? "取消静音" : "静音" }}
          </el-button>
          <el-button size="small" :type="media.camera ? 'primary' : 'default'" @click="toggleCamera">
            {{ media.camera ? "关闭视频" : "视频" }}
          </el-button>
          <el-button size="small" :type="media.screen ? 'primary' : 'default'" @click="toggleScreen">
            {{ media.screen ? "停止共享" : "共享屏幕" }}
          </el-button>
          <el-button type="danger" size="small" @click="stop">挂断</el-button>
        </template>
        <el-button
          v-else-if="callState === 'idle' || callState === 'error'"
          type="primary"
          size="small"
          :icon="Phone"
          @click="start"
        >
          实时通话
        </el-button>
        <el-button v-else type="warning" size="small" loading>连接中</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import { Phone } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { publishLocalVoiceASRFinal } from "../runtime/runtime-adapter";
import { notifyDesktopPetChatState } from "../runtime/desktop-pet-chat-state";
import {
  RealtimeCallController,
  type RealtimeCallState,
  type RealtimeMediaState,
} from "../realtime/realtime-call-controller";

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

const callState = ref<RealtimeCallState>("idle");
const callDuration = ref(0);
const errorMsg = ref("");
const visionStatus = ref("");
const media = ref<RealtimeMediaState>({ audio: false, camera: false, screen: false, muted: false });
const cameraPreview = ref<HTMLVideoElement | null>(null);
const screenPreview = ref<HTMLVideoElement | null>(null);
let durationTimer: ReturnType<typeof setInterval> | null = null;
let controller: RealtimeCallController | null = null;

const statusLabel = computed(() => ({
  idle: "未连接",
  connecting: "连接中...",
  connected: media.value.screen ? "屏幕通话中" : media.value.camera ? "视频通话中" : "语音通话中",
  error: "连接失败",
}[callState.value]));

watch(
  () => props.visible,
  (visible) => {
    if (visible && callState.value === "idle") void start();
    if (!visible && callState.value !== "idle") void stop();
  },
  { immediate: true },
);

function createController(): RealtimeCallController {
  return new RealtimeCallController({
    conversationId: props.conversationId,
    dialogId: props.dialogId,
    voiceType: props.voiceType,
    resourceId: props.resourceId,
    onState: (state, error) => {
      callState.value = state;
      errorMsg.value = error || "";
      emit("stateChange", state);
      if (state === "connected") {
        startDurationTimer();
        notifyDesktopPetChatState("assistant_listening", props.conversationId || undefined);
      } else if (state === "error") {
        notifyDesktopPetChatState("assistant_error", props.conversationId || undefined);
      } else if (state === "idle") {
        stopDurationTimer();
        notifyDesktopPetChatState("assistant_finished", props.conversationId || undefined);
      }
    },
    onAssistantText: (text) => emit("message", { role: "assistant", text }),
    onASRFinal: (data) => void forwardASRFinalToLocalWorkflow(data),
    onVision: (data) => {
      const context = typeof data.context === "string" ? data.context.trim() : "";
      if (context) visionStatus.value = "视觉已更新";
      else if (data.available === false) visionStatus.value = "视觉模型不可用";
    },
    onMediaState: (state) => { media.value = state; },
    onCameraPreview: (stream) => void assignPreview(cameraPreview, stream),
    onScreenPreview: (stream) => void assignPreview(screenPreview, stream),
  });
}

async function start() {
  if (controller) await controller.stop().catch(() => undefined);
  controller = createController();
  callDuration.value = 0;
  visionStatus.value = "";
  try {
    await controller.start();
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "实时通话启动失败");
  }
}

async function stop() {
  const active = controller;
  controller = null;
  if (active) await active.stop().catch(() => undefined);
  callState.value = "idle";
  callDuration.value = 0;
  stopDurationTimer();
}

async function toggleMute() {
  if (!controller) return;
  await controller.setMuted(!media.value.muted);
}

async function toggleCamera() {
  if (!controller) return;
  try {
    await controller.toggleCamera();
  } catch (error) {
    ElMessage.error("摄像头启动失败：" + (error instanceof Error ? error.message : String(error)));
  }
}

async function toggleScreen() {
  if (!controller) return;
  try {
    await controller.toggleScreen();
  } catch (error) {
    ElMessage.error("屏幕共享启动失败：" + (error instanceof Error ? error.message : String(error)));
  }
}

async function assignPreview(target: typeof cameraPreview, stream: MediaStream | null) {
  await nextTick();
  if (!target.value) return;
  target.value.srcObject = stream;
  if (stream) await target.value.play().catch(() => undefined);
}

async function forwardASRFinalToLocalWorkflow(data: Record<string, unknown>) {
  const transcript = typeof data.transcript === "string" ? data.transcript.trim() : "";
  const eventId = typeof data.eventId === "string" ? data.eventId.trim() : "";
  if (!transcript || !eventId) return;
  try {
    await publishLocalVoiceASRFinal({
      eventId,
      transcript,
      sessionId: typeof data.sessionId === "string" ? data.sessionId : undefined,
      conversationId: typeof data.conversationId === "string" ? data.conversationId : props.conversationId || undefined,
      visualContext: typeof data.visualContext === "string" ? data.visualContext : undefined,
      visualSource: data.visualSource === "camera" || data.visualSource === "screen" ? data.visualSource : undefined,
      occurredAt: new Date().toISOString(),
    });
  } catch (error) {
    console.warn("[RealtimeCallWidget] 本地实时工作流事件投递失败", error);
  }
}

function startDurationTimer() {
  stopDurationTimer();
  durationTimer = setInterval(() => callDuration.value++, 1000);
}

function stopDurationTimer() {
  if (durationTimer) clearInterval(durationTimer);
  durationTimer = null;
}

function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60).toString().padStart(2, "0");
  const rest = (seconds % 60).toString().padStart(2, "0");
  return `${minutes}:${rest}`;
}

onUnmounted(() => { void stop(); });
</script>

<style scoped>
.realtime-widget { flex-shrink: 0; }
.rw-previews { position: relative; display: flex; justify-content: flex-end; min-height: 88px; margin-bottom: 8px; overflow: hidden; border-radius: var(--radius-sm); background: #111; }
.rw-preview { object-fit: cover; background: #111; }
.rw-preview-screen { width: 100%; max-height: 220px; object-fit: contain; }
.rw-preview-camera { position: absolute; right: 10px; bottom: 10px; width: 112px; height: 76px; border-radius: 8px; border: 1px solid rgba(255,255,255,.28); box-shadow: 0 4px 18px rgba(0,0,0,.32); }
.rw-bar { display: flex; align-items: center; justify-content: space-between; padding: 8px 14px; border-radius: var(--radius-sm); background: var(--plugin-muted-bg); border: 1px solid var(--surface-border); transition: border-color .3s, background .3s; gap: 12px; }
.rw-bar.active { border-color: var(--el-color-success); background: var(--el-color-success-light-9); }
.rw-left, .rw-right { display: flex; align-items: center; gap: 8px; min-width: 0; }
.rw-right { flex-wrap: wrap; justify-content: flex-end; }
.rw-icon { width: 10px; height: 10px; border-radius: 50%; background: var(--el-color-info); flex: 0 0 auto; }
.rw-icon.connected { background: var(--el-color-success); animation: pulse 1.5s infinite; }
.rw-icon.connecting { background: var(--el-color-warning); animation: pulse .8s infinite; }
.rw-icon.error { background: var(--el-color-danger); }
@keyframes pulse { 0%,100%{opacity:1} 50%{opacity:.3} }
.rw-status, .rw-duration, .rw-vision { font-size: 13px; color: var(--el-text-color-regular); white-space: nowrap; }
.rw-duration, .rw-vision { color: var(--el-text-color-secondary); font-variant-numeric: tabular-nums; }
.rw-error { font-size: 12px; color: var(--el-color-danger); max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
