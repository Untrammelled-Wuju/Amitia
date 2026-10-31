import { RealtimeVoiceActivityDetector } from "./realtime-vad";
import { OwnedRealtimeSession } from "./owned-realtime-session";
import { ownedImageAttachment } from "../runtime/device-owned-attachments";
import type { OwnedChatResponse, OwnedExecutionScope } from "../runtime/device-owned-chat";
import { parseConversationReference } from "../runtime/device-owned-conversation-reference";

export type RealtimeCallState = "idle" | "connecting" | "connected" | "error";
export type RealtimeVisualSource = "camera" | "screen";

export interface RealtimeMediaState {
  audio: boolean;
  camera: boolean;
  screen: boolean;
  muted: boolean;
}

export interface RealtimeCallCapabilities {
  audioInput?: boolean;
  audioOutput?: boolean;
  cameraInput?: boolean;
  screenInput?: boolean;
  visualContext?: boolean;
  streamingASR?: boolean;
  streamingText?: boolean;
  turnBasedTTS?: boolean;
  wakeWord?: boolean;
  playbackProgress?: boolean;
}

export interface RealtimeCallConnectedInfo {
  callId: string;
  sessionId: string;
  capabilities: RealtimeCallCapabilities;
}

export interface RealtimeCallControllerOptions {
  conversationId: string;
  characterId: string;
  expectedExecutionScope: OwnedExecutionScope;
  acceptedTicket?: Record<string, any>;
  conversationOrigin?: { ownerId: string; id: string };
  historicalRoleId?: string;
  voiceType?: string;
  resourceId?: string;
  dialogId?: string;
  onOwnedCompleted?: (response: OwnedChatResponse) => void;
  onState?: (state: RealtimeCallState, error?: string) => void;
  onConnected?: (info: RealtimeCallConnectedInfo) => void;
  onAssistantText?: (text: string, delta?: boolean) => void;
  onAssistantSpeaking?: (speaking: boolean) => void;
  onASRFinal?: (data: Record<string, unknown>) => void;
  onVision?: (data: Record<string, unknown>) => void;
  onWakeStatus?: (data: Record<string, unknown>) => void;
  onPlaybackStatus?: (data: Record<string, unknown>) => void;
  onMediaState?: (state: RealtimeMediaState) => void;
  onCameraPreview?: (stream: MediaStream | null) => void;
  onScreenPreview?: (stream: MediaStream | null) => void;
}

interface VisualCaptureRuntime {
  source: RealtimeVisualSource;
  stream: MediaStream;
  video: HTMLVideoElement;
  canvas: HTMLCanvasElement;
  timer: ReturnType<typeof setInterval> | null;
  sequence: number;
  encoding: boolean;
  lastSignature: number;
  immediateRequested: boolean;
}

const AUDIO_SAMPLE_RATE = 16000;
const PLAYBACK_SAMPLE_RATE = 24000;
const VISUAL_INTERVAL_MS: Record<RealtimeVisualSource, number> = {
  camera: 650,
  screen: 750,
};

export class RealtimeCallController {
  private readonly options: RealtimeCallControllerOptions;
  private ownedSession: OwnedRealtimeSession | null = null;
  private audioContext: AudioContext | null = null;
  private playbackContext: AudioContext | null = null;
  private audioStream: MediaStream | null = null;
  private audioNode: ScriptProcessorNode | null = null;
  private silenceGain: GainNode | null = null;
  private cameraRuntime: VisualCaptureRuntime | null = null;
  private screenRuntime: VisualCaptureRuntime | null = null;
  private state: RealtimeCallState = "idle";
  private mediaState: RealtimeMediaState = { audio: false, camera: false, screen: false, muted: false };
  private activeSources = new Set<AudioBufferSourceNode>();
  private sourcePlayback = new Map<AudioBufferSourceNode, { generation: number; durationMs: number }>();
  private playbackGeneration = 0;
  private receivedAudioMs = 0;
  private playedAudioMs = 0;
  private ttsEndedGeneration = 0;
  private progressTimer: ReturnType<typeof setInterval> | null = null;
  private readonly vad = new RealtimeVoiceActivityDetector();
  private failureCleanup: Promise<void> | null = null;

  constructor(options: RealtimeCallControllerOptions) {
    this.options = options;
  }

  get callState(): RealtimeCallState {
    return this.state;
  }

  get media(): Readonly<RealtimeMediaState> {
    return this.mediaState;
  }

  private isActive(): boolean {
    return this.state === "connecting" || this.state === "connected";
  }

  async start(): Promise<void> {
    if (this.isActive()) return;
    if (this.failureCleanup) await this.failureCleanup;
    if (this.isActive()) return;
    if (!this.options.characterId || this.options.expectedExecutionScope.roleId !== this.options.characterId) {
      throw new Error("通话缺少当前角色与完整数据归属");
    }
    this.setState("connecting");
    this.vad.reset();
    try {
      this.audioStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          channelCount: 1,
          sampleRate: AUDIO_SAMPLE_RATE,
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
      });
      this.mediaState.audio = true;
      this.emitMediaState();
      this.prepareAudioCapture();
      const origin = parseConversationReference(this.options.conversationId);
      this.ownedSession = new OwnedRealtimeSession({
        characterId: this.options.characterId,
        conversationId: origin?.id || this.options.conversationId,
        conversationOrigin: this.options.conversationOrigin ?? origin,
        historicalRoleId: this.options.historicalRoleId,
        voiceType: this.options.voiceType,
        resourceId: this.options.resourceId,
        dialogId: this.options.dialogId,
        expectedExecutionScope: this.options.expectedExecutionScope,
        acceptedTicket: this.options.acceptedTicket,
        onReady: () => {
          this.setState("connected");
          this.attachAudioProcessor();
          this.options.onConnected?.({ callId: this.options.conversationId, sessionId: this.options.expectedExecutionScope.executionId || "", capabilities: { audioInput: true, audioOutput: true, cameraInput: true, screenInput: true, visualContext: true, streamingASR: true, streamingText: true, turnBasedTTS: true, wakeWord: true, playbackProgress: true } });
        },
        onCompleted: (result) => this.options.onOwnedCompleted?.(result),
        onAudio: (bytes) => this.playOwnedAudio(bytes),
        onError: (message) => { void this.fail(message); },
        onInterrupted: () => this.flushPlayback(),
        onASRFinal: (data) => this.options.onASRFinal?.(data),
        onAssistantText: (text, delta) => this.options.onAssistantText?.(text, delta),
        onVision: (data) => this.options.onVision?.(data),
        onWakeStatus: (data) => this.options.onWakeStatus?.(data),
        onPlaybackStatus: (data) => {
          this.options.onPlaybackStatus?.(data);
          if (data.type === "tts_ended") {
            this.ttsEndedGeneration = Number(data.generationId) || this.playbackGeneration;
            this.reportPlaybackEndedIfDrained();
          }
        },
      });
      await this.ownedSession.start();
    } catch (error) {
      await this.cleanup();
      this.setState("error", error instanceof Error ? error.message : String(error));
      throw error;
    }
  }

  async stop(): Promise<void> {
    await this.cleanup();
    this.setState("idle");
  }

  async setMuted(muted: boolean): Promise<void> {
    if (muted && this.ownedSession) this.ownedSession.endTurn();
    if (muted) this.vad.reset();
    this.mediaState.muted = muted;
    this.emitMediaState();
  }

  armWake(configId = ""): void {
    this.ownedSession?.setWakeArmed(true, configId);
  }

  disarmWake(): void {
    this.ownedSession?.setWakeArmed(false);
  }

  async toggleCamera(): Promise<void> {
    if (this.cameraRuntime) {
      this.stopVisualSource("camera");
      return;
    }
    const stream = await navigator.mediaDevices.getUserMedia({
      video: { width: { ideal: 1280 }, height: { ideal: 720 }, frameRate: { ideal: 24, max: 30 } },
      audio: false,
    });
    this.cameraRuntime = await this.startVisualSource("camera", stream);
    this.mediaState.camera = true;
    this.options.onCameraPreview?.(stream);
    this.emitMediaState();
  }

  async toggleScreen(): Promise<void> {
    if (this.screenRuntime) {
      this.stopVisualSource("screen");
      return;
    }
    const stream = await navigator.mediaDevices.getDisplayMedia({ video: { frameRate: { ideal: 10, max: 15 } }, audio: false });
    const track = stream.getVideoTracks()[0];
    track?.addEventListener("ended", () => this.stopVisualSource("screen"), { once: true });
    this.screenRuntime = await this.startVisualSource("screen", stream);
    this.mediaState.screen = true;
    this.options.onScreenPreview?.(stream);
    this.emitMediaState();
  }

  requestImmediateVisualFrame(): void {
    if (this.cameraRuntime) this.cameraRuntime.immediateRequested = true;
    if (this.screenRuntime) this.screenRuntime.immediateRequested = true;
    void this.captureVisualFrame(this.cameraRuntime);
    void this.captureVisualFrame(this.screenRuntime);
  }

  private prepareAudioCapture(): void {
    if (!this.audioStream) return;
    this.audioContext = new AudioContext({ sampleRate: AUDIO_SAMPLE_RATE });
    this.playbackContext = new AudioContext({ sampleRate: PLAYBACK_SAMPLE_RATE });
    const source = this.audioContext.createMediaStreamSource(this.audioStream);
    this.audioNode = this.audioContext.createScriptProcessor(2048, 1, 1);
    this.silenceGain = this.audioContext.createGain();
    this.silenceGain.gain.value = 0;
    source.connect(this.audioNode);
    this.audioNode.connect(this.silenceGain);
    this.silenceGain.connect(this.audioContext.destination);
  }

  private attachAudioProcessor(): void {
    if (!this.audioNode || !this.ownedSession) return;
    this.audioNode.onaudioprocess = (event) => {
      const session = this.ownedSession;
      if (!session || this.mediaState.muted) return;
      const input = event.inputBuffer.getChannelData(0);
      let sumSquares = 0;
      for (let index = 0; index < input.length; index++) sumSquares += input[index] * input[index];
      const rms = Math.sqrt(sumSquares / input.length);
      const vadEvent = this.vad.process(rms);
      if (vadEvent === "speech_start") session.startTurn();
      session.appendAudio(float32ToPCM(input));
      if (vadEvent === "speech_end") session.endTurn();
      if (rms >= 0.02) this.requestImmediateVisualFrame();
    };
  }

  private async playOwnedAudio(bytes: Uint8Array): Promise<void> {
    const context = this.playbackContext;
    if (!context) return;
    const decoded = await context.decodeAudioData(bytes.slice().buffer as ArrayBuffer);
    if (context !== this.playbackContext || !this.ownedSession) return;
    const generation = ++this.playbackGeneration;
    this.receivedAudioMs = decoded.duration * 1000;
    this.playedAudioMs = 0;
    this.sourcePlayback.clear();
    const source = context.createBufferSource();
    source.buffer = decoded;
    source.connect(context.destination);
    this.activeSources.add(source);
    this.sourcePlayback.set(source, { generation, durationMs: decoded.duration * 1000 });
    this.startProgressTimer();
    source.onended = () => {
      this.activeSources.delete(source);
      this.playedAudioMs = decoded.duration * 1000;
      this.sendPlaybackProgress(generation);
      this.sourcePlayback.delete(source);
      if (!this.activeSources.size) {
        this.options.onAssistantSpeaking?.(false);
        this.reportPlaybackEndedIfDrained();
      }
    };
    this.options.onAssistantSpeaking?.(true);
    source.start();
  }

  private startProgressTimer(): void {
    if (this.progressTimer) return;
    this.progressTimer = setInterval(() => {
      if (this.playbackGeneration) this.sendPlaybackProgress(this.playbackGeneration);
    }, 400);
  }

  private sendPlaybackProgress(generation: number): void {
    this.ownedSession?.reportPlayback(generation, Math.round(this.playedAudioMs), Math.round(this.receivedAudioMs));
  }

  private reportPlaybackEndedIfDrained(): void {
    if (!this.ttsEndedGeneration || this.activeSources.size) return;
    const generation = this.ttsEndedGeneration;
    this.ttsEndedGeneration = 0;
    this.ownedSession?.reportPlaybackEnded(generation);
    if (this.progressTimer) clearInterval(this.progressTimer);
    this.progressTimer = null;
  }

  private flushPlayback(): void {
    if (this.playbackGeneration) this.sendPlaybackProgress(this.playbackGeneration);
    for (const source of this.activeSources) {
      try { source.stop(); } catch (error) {
  console.error("[original-error]", error);
}
      try { source.disconnect(); } catch (error) {
  console.error("[original-error]", error);
}
    }
    this.activeSources.clear();
    this.sourcePlayback.clear();
    this.options.onAssistantSpeaking?.(false);
    this.ttsEndedGeneration = 0;
    if (this.progressTimer) clearInterval(this.progressTimer);
    this.progressTimer = null;
  }

  private async startVisualSource(source: RealtimeVisualSource, stream: MediaStream): Promise<VisualCaptureRuntime> {
    const video = document.createElement("video");
    video.autoplay = true;
    video.muted = true;
    video.playsInline = true;
    video.srcObject = stream;
    await video.play();
    const runtime: VisualCaptureRuntime = { source, stream, video, canvas: document.createElement("canvas"), timer: null, sequence: 0, encoding: false, lastSignature: 0, immediateRequested: true };
    runtime.timer = setInterval(() => { void this.captureVisualFrame(runtime); }, VISUAL_INTERVAL_MS[source]);
    void this.captureVisualFrame(runtime);
    return runtime;
  }

  private stopVisualSource(source: RealtimeVisualSource): void {
    const runtime = source === "camera" ? this.cameraRuntime : this.screenRuntime;
    if (!runtime) return;
    if (runtime.timer) clearInterval(runtime.timer);
    runtime.stream.getTracks().forEach((track) => track.stop());
    runtime.video.srcObject = null;
    if (source === "camera") {
      this.cameraRuntime = null;
      this.mediaState.camera = false;
      this.options.onCameraPreview?.(null);
    } else {
      this.screenRuntime = null;
      this.mediaState.screen = false;
      this.options.onScreenPreview?.(null);
    }
    this.emitMediaState();
  }

  private async captureVisualFrame(runtime: VisualCaptureRuntime | null): Promise<void> {
    if (!runtime || runtime.encoding || runtime.video.readyState < HTMLMediaElement.HAVE_CURRENT_DATA || !this.ownedSession) return;
    runtime.encoding = true;
    try {
      const originalWidth = runtime.video.videoWidth;
      const originalHeight = runtime.video.videoHeight;
      if (!originalWidth || !originalHeight) return;
      const maxWidth = runtime.source === "screen" ? 1280 : 1024;
      const maxHeight = runtime.source === "screen" ? 800 : 768;
      const scale = Math.min(1, maxWidth / originalWidth, maxHeight / originalHeight);
      const width = Math.max(1, Math.round(originalWidth * scale));
      const height = Math.max(1, Math.round(originalHeight * scale));
      runtime.canvas.width = width;
      runtime.canvas.height = height;
      const context = runtime.canvas.getContext("2d", { alpha: false });
      if (!context) return;
      context.drawImage(runtime.video, 0, 0, width, height);
      const signature = computeFrameSignature(context, width, height);
      const immediate = runtime.immediateRequested;
      runtime.immediateRequested = false;
      if (!immediate && signature === runtime.lastSignature) return;
      runtime.lastSignature = signature;
      const blob = await canvasToBlob(runtime.canvas, "image/jpeg", runtime.source === "screen" ? 0.78 : 0.72);
      if (!blob || blob.size === 0 || blob.size > 2 * 1024 * 1024) return;
      const attachment = await ownedImageAttachment(await blobToDataURL(blob));
      runtime.sequence++;
      this.ownedSession?.setVisual(attachment);
      this.options.onVision?.({ available: true, source: runtime.source, sequence: runtime.sequence });
    } finally {
      runtime.encoding = false;
    }
  }

  private async fail(message: string): Promise<void> {
    this.setState("error", message);
    if (!this.failureCleanup) {
      this.failureCleanup = this.cleanup().finally(() => { this.failureCleanup = null; });
    }
    await this.failureCleanup;
  }

  private async cleanup(): Promise<void> {
    this.ownedSession?.stop();
    this.ownedSession = null;
    this.flushPlayback();
    this.stopVisualSource("camera");
    this.stopVisualSource("screen");
    if (this.audioNode) {
      this.audioNode.onaudioprocess = null;
      try { this.audioNode.disconnect(); } catch (error) {
  console.error("[original-error]", error);
}
      this.audioNode = null;
    }
    if (this.silenceGain) {
      try { this.silenceGain.disconnect(); } catch (error) {
  console.error("[original-error]", error);
}
      this.silenceGain = null;
    }
    this.audioStream?.getTracks().forEach((track) => track.stop());
    this.audioStream = null;
    if (this.audioContext && this.audioContext.state !== "closed") {
      try { await this.audioContext.close(); } catch (error) {
  console.error("[original-error]", error);
}
    }
    this.audioContext = null;
    if (this.playbackContext && this.playbackContext.state !== "closed") {
      try { await this.playbackContext.close(); } catch (error) {
  console.error("[original-error]", error);
}
    }
    this.playbackContext = null;
    this.mediaState = { audio: false, camera: false, screen: false, muted: false };
    this.emitMediaState();
  }

  private setState(state: RealtimeCallState, error?: string): void {
    this.state = state;
    this.options.onState?.(state, error);
  }

  private emitMediaState(): void {
    this.options.onMediaState?.({ ...this.mediaState });
  }
}

function float32ToPCM(float32: Float32Array): ArrayBuffer {
  const buffer = new ArrayBuffer(float32.length * 2);
  const view = new DataView(buffer);
  for (let index = 0; index < float32.length; index++) {
    const sample = Math.max(-1, Math.min(1, float32[index]));
    view.setInt16(index * 2, sample < 0 ? sample * 0x8000 : sample * 0x7fff, true);
  }
  return buffer;
}

function computeFrameSignature(context: CanvasRenderingContext2D, width: number, height: number): number {
  const data = context.getImageData(0, 0, width, height).data;
  let signature = 2166136261;
  const step = Math.max(4, Math.floor(data.length / 2048 / 4) * 4);
  for (let index = 0; index < data.length; index += step) {
    signature ^= data[index];
    signature = Math.imul(signature, 16777619);
  }
  return signature >>> 0;
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string, quality: number): Promise<Blob | null> {
  return new Promise((resolve) => canvas.toBlob(resolve, type, quality));
}

function blobToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ""));
    reader.onerror = () => reject(reader.error || new Error("视觉帧编码失败"));
    reader.readAsDataURL(blob);
  });
}
