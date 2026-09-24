<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <teleport to="body">
    <div class="call-overlay">
      <div class="call-page" :style="callPageStyle">
        <div class="call-backdrop" aria-hidden="true">
          <div class="call-backdrop-image" />
          <div class="call-backdrop-gradient" />
        </div>
        <video v-show="media.screen" ref="screenPreview" class="call-media call-media-screen" autoplay muted playsinline />
        <video v-show="media.camera && !media.screen" ref="cameraPreview" class="call-media call-media-camera" autoplay muted playsinline />
        <video v-show="media.camera && media.screen" ref="cameraPip" class="call-pip" autoplay muted playsinline />
        <div class="call-center">
          <div
            class="call-avatar"
            :class="{ speaking: aiSpeaking, error: callState === 'error' }"
            :style="avatarBackground ? { background: avatarBackground } : undefined"
          >
            <el-icon v-if="callState === 'error'" class="call-avatar-error"><WarningFilled /></el-icon>
            <span v-else-if="!avatarBackground">{{ avatarInitial }}</span>
          </div>
          <div class="call-name">{{ displayName }}</div>
          <div class="call-status" :class="{ error: callState === 'error' }">{{ statusText }}</div>
          <div class="call-detail" :class="{ error: callState === 'error' }">{{ detailText }}</div>
        </div>
        <div class="call-controls">
          <template v-if="callState === 'error'">
            <button class="call-control" type="button" @click="restart">
              <span class="call-control-icon"><el-icon><RefreshRight /></el-icon></span>
              <span class="call-control-label">重试</span>
            </button>
            <button class="call-control destructive" type="button" @click="hangUp">
              <span class="call-control-icon"><el-icon class="call-end-icon"><Phone /></el-icon></span>
              <span class="call-control-label">挂断</span>
            </button>
            <button class="call-control" type="button" @click="emit('close')">
              <span class="call-control-icon"><el-icon><Close /></el-icon></span>
              <span class="call-control-label">关闭</span>
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
              v-if="mode === 'screen'"
              class="call-control"
              :class="{ selected: media.screen }"
              type="button"
              :disabled="callState !== 'connected'"
              @click="toggleScreen"
            >
              <span class="call-control-icon"><el-icon><Monitor /></el-icon></span>
              <span class="call-control-label">{{ media.screen ? "停止共享" : "共享屏幕" }}</span>
            </button>
            <button
              v-else
              class="call-control"
              :class="{ selected: media.camera }"
              type="button"
              :disabled="callState !== 'connected'"
              @click="toggleCamera"
            >
              <span class="call-control-icon"><el-icon><VideoCamera /></el-icon></span>
              <span class="call-control-label">{{ media.camera ? "关闭视频" : "视频" }}</span>
            </button>
            <button class="call-control destructive" type="button" @click="hangUp">
              <span class="call-control-icon"><el-icon class="call-end-icon"><Phone /></el-icon></span>
              <span class="call-control-label">挂断</span>
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
const callPageStyle = computed<Record<string, string>>(() => ({
  "--call-avatar-background": avatarBackground.value || "none",
}));

const modeLabel = computed(() => {
  if (media.value.screen) return "屏幕通话";
  if (media.value.camera) return "视频通话";
  return { voice: "语音通话", video: "视频通话", screen: "屏幕通话" }[props.mode];
});

const statusText = computed(() => {
  const stateLabel =
    callState.value === "connecting"
      ? "正在建立通话"
      : callState.value === "connected"
        ? "通话中"
        : callState.value === "error"
          ? "通话连接失败"
          : "通话已结束";
  return `${stateLabel} · ${formatDuration(callDuration.value)} · AI 实时语音`;
});
const detailText = computed(() =>
  callState.value === "error"
    ? errorMsg.value || "连接失败"
    : visionStatus.value || (media.value.muted ? "麦克风已静音" : modeLabel.value),
);

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
  background: #121212;
}
.call-page {
  position: relative;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  background: #121212;
  color: #fff;
  animation: call-overlay-in .22s ease;
}
@keyframes call-overlay-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
.call-backdrop {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
}
.call-backdrop-image {
  position: absolute;
  inset: -18%;
  background: var(--call-avatar-background, #2b2b2d) center / cover no-repeat;
  filter: blur(36px);
  opacity: .72;
  transform: scale(1.24);
}
.call-backdrop-gradient {
  position: absolute;
  inset: 0;
  background: linear-gradient(
    to bottom,
    rgba(0, 0, 0, .3),
    rgba(0, 0, 0, .45) 55%,
    rgba(0, 0, 0, .65)
  );
}
.call-media {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  z-index: 1;
  background: #000;
}
.call-media-screen {
  object-fit: contain;
}
.call-media-camera {
  object-fit: cover;
}
.call-pip {
  position: absolute;
  top: clamp(80px, 12vh, 132px);
  right: clamp(18px, 3vw, 40px);
  z-index: 3;
  width: 132px;
  height: 176px;
  object-fit: cover;
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.22);
  box-shadow: 0 12px 36px rgba(0, 0, 0, 0.42);
  background: #000;
}
.call-center {
  position: absolute;
  top: clamp(84px, 16vh, 180px);
  left: 50%;
  z-index: 4;
  display: flex;
  flex-direction: column;
  align-items: center;
  width: min(460px, calc(100vw - 48px));
  transform: translateX(-50%);
  pointer-events: none;
}
.call-avatar {
  display: grid;
  place-items: center;
  width: 96px;
  height: 96px;
  border-radius: 24px;
  background: #2b2b2d center / cover no-repeat;
  border: 1px solid rgba(255, 255, 255, 0.24);
  color: #fff;
  font-size: 34px;
  font-weight: 700;
  overflow: hidden;
  transition: border-color .18s ease, border-width .18s ease;
}
.call-avatar.speaking {
  border-width: 3px;
  border-color: #fff;
}
.call-avatar.error {
  background: #2b2b2d;
  color: #ff453a;
  border-color: rgba(255, 69, 58, 0.55);
}
.call-avatar-error { font-size: 36px; }
.call-name {
  margin-top: 16px;
  color: #fff;
  font-size: 21px;
  font-weight: 650;
}
.call-status {
  margin-top: 7px;
  color: #bcbcc0;
  font-size: 11px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}
.call-status.error { color: #ff453a; }
.call-detail {
  max-width: 290px;
  min-height: 26px;
  margin-top: 14px;
  color: #8e8e93;
  font-size: 10px;
  line-height: 1.35;
  text-align: center;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
.call-detail.error { color: #ff453a; }
.call-controls {
  position: absolute;
  right: 0;
  bottom: clamp(42px, 6vh, 72px);
  left: 0;
  z-index: 5;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  gap: 28px;
}
.call-control {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
  width: 66px;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
}
.call-control:disabled { cursor: not-allowed; opacity: .45; }
.call-control-icon {
  display: grid;
  place-items: center;
  width: 58px;
  height: 58px;
  border-radius: 50%;
  background: #2b2b2d;
  color: #fff;
  font-size: 25px;
  transition: background .18s ease, color .18s ease, transform .18s ease;
}
.call-control:not(:disabled):hover .call-control-icon {
  background: #3a3a3d;
  transform: translateY(-1px);
}
.call-control.selected .call-control-icon {
  background: #fff;
  color: #111;
}
.call-control.destructive .call-control-icon {
  background: #ff453a;
  color: #fff;
}
.call-end-icon { transform: rotate(135deg); }
.call-control-label {
  color: #c3c3c6;
  font-size: 9px;
  white-space: nowrap;
}
</style>
