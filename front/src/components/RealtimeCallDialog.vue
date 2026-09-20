<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <teleport to="body">
    <div class="call-overlay">
      <div class="call-page">
        <div class="call-mode-label">{{ modeLabel }}</div>
        <div class="call-stage">
          <video v-show="media.screen" ref="screenPreview" class="call-stage-video" autoplay muted playsinline />
          <video v-show="media.camera && !media.screen" ref="cameraPreview" class="call-stage-video" autoplay muted playsinline />
          <video v-show="media.camera && media.screen" ref="cameraPip" class="call-pip" autoplay muted playsinline />
          <div
            v-if="!media.camera && !media.screen"
            class="call-avatar"
            :class="{ speaking: aiSpeaking, error: callState === 'error' }"
          >
            <el-icon v-if="callState === 'error'" class="call-avatar-error"><WarningFilled /></el-icon>
            <span v-else>{{ avatarInitial }}</span>
          </div>
        </div>
        <div class="call-name">{{ displayName }}</div>
        <div class="call-status" :class="{ error: callState === 'error' }">{{ statusText }}</div>
        <div class="call-controls">
          <template v-if="callState === 'error'">
            <button class="call-control" type="button" @click="emit('close')">
              <span class="call-control-icon"><el-icon><Close /></el-icon></span>
              <span class="call-control-label">关闭</span>
            </button>
            <button class="call-control" type="button" @click="restart">
              <span class="call-control-icon"><el-icon><RefreshRight /></el-icon></span>
              <span class="call-control-label">重新连接</span>
            </button>
          </template>
          <template v-else>
            <button
              class="call-control"
              :class="{ selected: media.muted }"
              type="button"
              :disabled="callState !== 'connected'"
              @click="toggleMute"
            >
              <span class="call-control-icon"><el-icon><Microphone /></el-icon></span>
              <span class="call-control-label">{{ media.muted ? "取消静音" : "静音" }}</span>
            </button>
            <button
              class="call-control"
              :class="{ selected: media.camera }"
              type="button"
              :disabled="callState !== 'connected'"
              @click="toggleCamera"
            >
              <span class="call-control-icon"><el-icon><VideoCamera /></el-icon></span>
              <span class="call-control-label">{{ media.camera ? "关闭视频" : "视频" }}</span>
            </button>
            <button
              class="call-control"
              :class="{ selected: media.screen }"
              type="button"
              :disabled="callState !== 'connected'"
              @click="toggleScreen"
            >
              <span class="call-control-icon"><el-icon><Monitor /></el-icon></span>
              <span class="call-control-label">{{ media.screen ? "停止共享" : "共享屏幕" }}</span>
            </button>
            <button class="call-control destructive" type="button" @click="hangUp">
              <span class="call-control-icon"><el-icon class="call-end-icon"><Phone /></el-icon></span>
              <span class="call-control-label">结束</span>
            </button>
          </template>
        </div>
      </div>
    </div>
  </teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, type Ref } from "vue";
import {
  Close,
  Microphone,
  Monitor,
  Phone,
  RefreshRight,
  VideoCamera,
  WarningFilled,
} from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { publishLocalVoiceASRFinal } from "../runtime/runtime-adapter";
import { notifyDesktopPetChatState } from "../runtime/desktop-pet-chat-state";
import {
  RealtimeCallController,
  type RealtimeCallState,
  type RealtimeMediaState,
} from "../realtime/realtime-call-controller";

export type RealtimeCallMode = "voice" | "video" | "screen";

const props = defineProps<{
  mode: RealtimeCallMode;
  voiceType: string;
  resourceId: string;
  conversationId: string;
  charName?: string;
  charAvatar?: string;
}>();

const emit = defineEmits<{
  close: [];
  stateChange: [state: string];
}>();

const callState = ref<RealtimeCallState>("idle");
const callDuration = ref(0);
const errorMsg = ref("");
const visionStatus = ref("");
const aiSpeaking = ref(false);
const media = ref<RealtimeMediaState>({ audio: false, camera: false, screen: false, muted: false });
const cameraPreview = ref<HTMLVideoElement | null>(null);
const cameraPip = ref<HTMLVideoElement | null>(null);
const screenPreview = ref<HTMLVideoElement | null>(null);
let durationTimer: ReturnType<typeof setInterval> | null = null;
let controller: RealtimeCallController | null = null;
let disposed = false;

const displayName = computed(() => (props.charName || "").trim() || "Amitia");
const avatarInitial = computed(() => displayName.value.charAt(0).toUpperCase());
const avatarBackground = computed(() =>
  props.charAvatar ? `center / cover no-repeat url(${JSON.stringify(props.charAvatar)})` : "",
);

const modeLabel = computed(() => {
  if (media.value.screen) return "屏幕通话";
  if (media.value.camera) return "视频通话";
  return { voice: "语音通话", video: "视频通话", screen: "屏幕通话" }[props.mode];
});

const statusText = computed(() => {
  if (callState.value === "connecting") return "正在连接实时通话…";
  if (callState.value === "connected") {
    if (aiSpeaking.value) return "对方正在说话";
    if (media.value.muted) return "麦克风已静音";
    const parts = [`${modeLabel.value}中`, formatDuration(callDuration.value)];
    if (visionStatus.value) parts.push(visionStatus.value);
    return parts.join(" · ");
  }
  if (callState.value === "error") return errorMsg.value || "连接失败";
  return "通话已结束";
});

onMounted(() => { void start(); });
onUnmounted(() => {
  disposed = true;
  void stop();
});

function createController(): RealtimeCallController {
  return new RealtimeCallController({
    conversationId: props.conversationId,
    voiceType: props.voiceType,
    resourceId: props.resourceId,
    onState: (state, error) => {
      if (disposed) return;
      callState.value = state;
      errorMsg.value = error || "";
      emit("stateChange", state);
      if (state === "connected") {
        startDurationTimer();
        notifyDesktopPetChatState("assistant_listening", props.conversationId || undefined);
        void applyInitialMedia();
      } else if (state === "error") {
        stopDurationTimer();
        notifyDesktopPetChatState("assistant_error", props.conversationId || undefined);
      } else if (state === "idle") {
        stopDurationTimer();
        notifyDesktopPetChatState("assistant_finished", props.conversationId || undefined);
      }
    },
    onAssistantSpeaking: (speaking) => {
      aiSpeaking.value = speaking;
      if (speaking) {
        notifyDesktopPetChatState("assistant_speaking", props.conversationId || undefined);
      } else if (callState.value === "connected") {
        notifyDesktopPetChatState("assistant_listening", props.conversationId || undefined);
      }
    },
    onASRFinal: (data) => void forwardASRFinalToLocalWorkflow(data),
    onVision: (data) => {
      const context = typeof data.context === "string" ? data.context.trim() : "";
      if (context) visionStatus.value = "视觉已更新";
      else if (data.available === false) visionStatus.value = "视觉模型不可用";
    },
    onMediaState: (state) => { media.value = state; },
    onCameraPreview: (stream) => void assignPreview(stream, cameraPreview, cameraPip),
    onScreenPreview: (stream) => void assignPreview(stream, screenPreview),
  });
}

async function start() {
  if (controller) await controller.stop().catch(() => undefined);
  controller = createController();
  callDuration.value = 0;
  visionStatus.value = "";
  aiSpeaking.value = false;
  try {
    await controller.start();
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "实时通话启动失败");
  }
}

async function restart() {
  await start();
}

async function stop() {
  const active = controller;
  controller = null;
  if (active) await active.stop().catch(() => undefined);
  callState.value = "idle";
  callDuration.value = 0;
  aiSpeaking.value = false;
  stopDurationTimer();
}

async function applyInitialMedia() {
  if (!controller) return;
  try {
    if (props.mode === "video" && !media.value.camera) await controller.toggleCamera();
    if (props.mode === "screen" && !media.value.screen) await controller.toggleScreen();
  } catch (error) {
    visionStatus.value = "初始媒体采集失败：" + (error instanceof Error ? error.message : String(error));
  }
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

async function hangUp() {
  await stop();
  emit("close");
}

async function assignPreview(stream: MediaStream | null, ...targets: Array<Ref<HTMLVideoElement | null>>) {
  await nextTick();
  for (const target of targets) {
    if (!target.value) continue;
    target.value.srcObject = stream;
    if (stream) await target.value.play().catch(() => undefined);
  }
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
    console.warn("[RealtimeCallDialog] 本地实时工作流事件投递失败", error);
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
</script>

<style scoped>
.call-overlay {
  position: fixed;
  inset: 0;
  z-index: 3000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(8, 10, 16, 0.86);
  backdrop-filter: blur(10px);
  animation: call-overlay-in .22s ease;
}
@keyframes call-overlay-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
.call-page {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: min(520px, calc(100vw - 48px));
  max-height: calc(100vh - 64px);
  padding: 26px 30px 24px;
  border-radius: 24px;
  background: linear-gradient(160deg, #232a3a 0%, #161a26 58%, #10131c 100%);
  border: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: 0 28px 80px rgba(0, 0, 0, 0.55);
  animation: call-page-in .26s cubic-bezier(.2, .9, .3, 1.2);
}
@keyframes call-page-in {
  from { opacity: 0; transform: translateY(26px) scale(.96); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}
.call-mode-label {
  align-self: flex-start;
  color: rgba(255, 255, 255, 0.55);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: .04em;
}
.call-stage {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 300px;
  margin-top: 14px;
  border-radius: 18px;
  background: #0b0d13;
  overflow: hidden;
}
.call-stage-video {
  width: 100%;
  height: 100%;
  object-fit: cover;
  background: #0b0d13;
}
.call-pip {
  position: absolute;
  right: 12px;
  bottom: 12px;
  width: 116px;
  height: 156px;
  object-fit: cover;
  border-radius: 14px;
  border: 1px solid rgba(255, 255, 255, 0.22);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.4);
  background: #0b0d13;
}
.call-avatar {
  display: grid;
  place-items: center;
  width: 104px;
  height: 104px;
  border-radius: 32px;
  background: var(--ac-color-primary, #8a5728);
  border: 1px solid rgba(255, 255, 255, 0.14);
  color: #fff;
  font-size: 30px;
  font-weight: 700;
  transition: border-color .18s ease, box-shadow .18s ease;
}
.call-avatar.speaking {
  border-width: 4px;
  border-color: var(--ac-color-primary, #8a5728);
  box-shadow: 0 0 0 6px rgba(108, 123, 255, 0.18);
}
.call-avatar.error {
  background: rgba(245, 108, 108, 0.16);
  color: var(--el-color-danger);
  border-color: rgba(245, 108, 108, 0.4);
}
.call-avatar-error { font-size: 36px; }
.call-name {
  margin-top: 16px;
  color: #fff;
  font-size: 20px;
  font-weight: 700;
}
.call-status {
  margin-top: 7px;
  min-height: 18px;
  color: rgba(255, 255, 255, 0.6);
  font-size: 13px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}
.call-status.error { color: var(--el-color-danger); }
.call-controls {
  display: flex;
  align-items: flex-start;
  justify-content: center;
  gap: 18px;
  margin-top: 20px;
  flex-wrap: wrap;
}
.call-control {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
  width: 72px;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
}
.call-control:disabled { cursor: not-allowed; opacity: .55; }
.call-control-icon {
  display: grid;
  place-items: center;
  width: 54px;
  height: 54px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.14);
  color: rgba(255, 255, 255, 0.88);
  font-size: 22px;
  transition: background .18s ease, color .18s ease, border-color .18s ease;
}
.call-control:not(:disabled):hover .call-control-icon {
  background: rgba(255, 255, 255, 0.16);
}
.call-control.selected .call-control-icon {
  background: var(--ac-color-primary, #8a5728);
  border-color: transparent;
  color: #fff;
}
.call-control.destructive .call-control-icon {
  background: var(--el-color-danger);
  border-color: transparent;
  color: #fff;
}
.call-end-icon { transform: rotate(135deg); }
.call-control-label {
  color: rgba(255, 255, 255, 0.72);
  font-size: 12px;
  white-space: nowrap;
}
.call-control.selected .call-control-label { color: #fff; }
</style>
