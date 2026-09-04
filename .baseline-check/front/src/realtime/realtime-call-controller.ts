// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

import {
  resolveApiUrl,
  resolveWebSocketUrl,
} from "../runtime/runtime-adapter";
import { createAuthenticatedFetchInit } from "../runtime/request-auth";

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
  visualLatestWins?: boolean;
  separateVisualWs?: boolean;
  dynamicSourceSwap?: boolean;
  maxVisualFps?: number;
  maxVisualBytes?: number;
}

export interface RealtimeCallConnectedInfo {
  callId: string;
  sessionId: string;
  visualEndpoint: string;
  visualTicket: string;
  capabilities: RealtimeCallCapabilities;
}

export interface RealtimeCallControllerOptions {
  conversationId: string;
  dialogId?: string;
  voiceType?: string;
  resourceId?: string;
  onState?: (state: RealtimeCallState, error?: string) => void;
  onConnected?: (info: RealtimeCallConnectedInfo) => void;
  onAssistantText?: (text: string) => void;
  onASRFinal?: (data: Record<string, unknown>) => void;
  onVision?: (data: Record<string, unknown>) => void;
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
  private controlSocket: WebSocket | null = null;
  private visualSocket: WebSocket | null = null;
  private audioContext: AudioContext | null = null;
  private playbackContext: AudioContext | null = null;
  private audioStream: MediaStream | null = null;
  private audioNode: ScriptProcessorNode | null = null;
  private silenceGain: GainNode | null = null;
  private cameraRuntime: VisualCaptureRuntime | null = null;
  private screenRuntime: VisualCaptureRuntime | null = null;
  private state: RealtimeCallState = "idle";
  private mediaState: RealtimeMediaState = {
    audio: false,
    camera: false,
    screen: false,
    muted: false,
  };
  private connectedInfo: RealtimeCallConnectedInfo | null = null;
  private nextPlayTime = 0;
  private aiSpeaking = false;
  private lastSpeechVisualBoostAt = 0;

  constructor(options: RealtimeCallControllerOptions) {
    this.options = options;
  }

  get callState(): RealtimeCallState {
    return this.state;
  }

  get media(): Readonly<RealtimeMediaState> {
    return this.mediaState;
  }

  async start(): Promise<void> {
    if (this.state === "connecting" || this.state === "connected") return;
    this.setState("connecting");
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

      const access = await this.createAccessTicket();
      const endpoint = await resolveWebSocketUrl(access.wsPath);
      const params = new URLSearchParams({ ticket: access.ticket });
      this.controlSocket = new WebSocket(`${endpoint}?${params.toString()}`);
      this.bindControlSocket(this.controlSocket);
    } catch (error) {
      await this.cleanup(false);
      this.setState("error", error instanceof Error ? error.message : String(error));
      throw error;
    }
  }

  async stop(): Promise<void> {
    const control = this.controlSocket;
    if (control && control.readyState === WebSocket.OPEN) {
      try {
        control.send(JSON.stringify({ event: "stop" }));
      } catch {}
    }
    const visual = this.visualSocket;
    if (visual && visual.readyState === WebSocket.OPEN) {
      try {
        visual.send(JSON.stringify({ event: "stop" }));
      } catch {}
    }
    await this.cleanup(true);
    this.setState("idle");
  }

  async setMuted(muted: boolean): Promise<void> {
    this.mediaState.muted = muted;
    this.emitMediaState();
  }

  async toggleCamera(): Promise<void> {
    if (this.cameraRuntime) {
      this.stopVisualSource("camera");
      return;
    }
    const stream = await navigator.mediaDevices.getUserMedia({
      video: {
        width: { ideal: 1280 },
        height: { ideal: 720 },
        frameRate: { ideal: 24, max: 30 },
      },
      audio: false,
    });
    this.cameraRuntime = await this.startVisualSource("camera", stream);
    this.mediaState.camera = true;
    this.options.onCameraPreview?.(stream);
    this.emitMediaState();
    this.publishSources();
  }

  async toggleScreen(): Promise<void> {
    if (this.screenRuntime) {
      this.stopVisualSource("screen");
      return;
    }
    const stream = await navigator.mediaDevices.getDisplayMedia({
      video: {
        frameRate: { ideal: 10, max: 15 },
      },
      audio: false,
    });
    const track = stream.getVideoTracks()[0];
    track?.addEventListener("ended", () => this.stopVisualSource("screen"), {
      once: true,
    });
    this.screenRuntime = await this.startVisualSource("screen", stream);
    this.mediaState.screen = true;
    this.options.onScreenPreview?.(stream);
    this.emitMediaState();
    this.publishSources();
  }

  requestImmediateVisualFrame(): void {
    if (this.cameraRuntime) this.cameraRuntime.immediateRequested = true;
    if (this.screenRuntime) this.screenRuntime.immediateRequested = true;
    void this.captureVisualFrame(this.cameraRuntime);
    void this.captureVisualFrame(this.screenRuntime);
  }

  private async createAccessTicket(): Promise<{ ticket: string; wsPath: string }> {
    const path = "/api/realtime/v2/tickets";
    const url = await resolveApiUrl(path);
    const init = await createAuthenticatedFetchInit(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        conversationId: this.options.conversationId || "",
        dialogId: this.options.dialogId || "",
        voiceType: this.options.voiceType || "",
        resourceId: this.options.resourceId || "",
      }),
    });
    const response = await fetch(url, init);
    let body: any = null;
    try {
      body = await response.json();
    } catch {}
    if (!response.ok) {
      throw new Error(
        body?.message || body?.msg || `创建实时通话授权失败 (${response.status})`,
      );
    }
    const data = body?.data ?? body;
    const ticket = String(data?.ticket || "").trim();
    const wsPath = String(data?.wsPath || "").trim();
    if (!ticket || !wsPath.startsWith("/api/realtime/")) {
      throw new Error("实时通话授权返回无效");
    }
    return { ticket, wsPath };
  }

  private bindControlSocket(socket: WebSocket): void {
    socket.onopen = () => {
      this.attachAudioProcessor();
    };
    socket.onmessage = (event) => this.handleControlMessage(event.data);
    socket.onerror = () => {
      this.setState("error", "实时通话连接失败");
    };
    socket.onclose = () => {
      if (this.state !== "idle" && this.state !== "error") {
        void this.cleanup(false).then(() => this.setState("idle"));
      }
    };
  }

  private async handleControlMessage(raw: unknown): Promise<void> {
    if (typeof raw !== "string") return;
    let message: any;
    try {
      message = JSON.parse(raw);
    } catch {
      return;
    }
    switch (message.event) {
      case "connected": {
        const call = message.call || {};
        if (!call.callId || !call.visualEndpoint || !call.visualTicket) {
          this.setState("error", "实时通话视觉会话初始化失败");
          return;
        }
        this.connectedInfo = {
          callId: String(call.callId),
          sessionId: String(call.sessionId || ""),
          visualEndpoint: String(call.visualEndpoint),
          visualTicket: String(call.visualTicket),
          capabilities: call.capabilities || {},
        };
        await this.connectVisualSocket(this.connectedInfo);
        this.setState("connected");
        this.options.onConnected?.(this.connectedInfo);
        this.publishSources();
        break;
      }
      case "audio":
        if (typeof message.data === "string") this.playAudio(message.data);
        break;
      case "tts_ended":
        this.aiSpeaking = false;
        break;
      case "ChatTextResponse":
        if (message.data?.text) this.options.onAssistantText?.(String(message.data.text));
        break;
      case "asr_final":
        if (message.data && typeof message.data === "object") {
          this.options.onASRFinal?.(message.data as Record<string, unknown>);
        }
        break;
      case "vision.updated":
      case "vision.status":
        if (message.data && typeof message.data === "object") {
          this.options.onVision?.(message.data as Record<string, unknown>);
        }
        break;
      case "error":
        this.setState("error", String(message.data || "实时通话连接失败"));
        await this.cleanup(false);
        break;
      case "disconnected":
      case "SessionFinished":
        await this.cleanup(false);
        this.setState("idle");
        break;
    }
  }

  private async connectVisualSocket(info: RealtimeCallConnectedInfo): Promise<void> {
    const endpoint = await resolveWebSocketUrl(info.visualEndpoint);
    const params = new URLSearchParams({
      callId: info.callId,
      ticket: info.visualTicket,
    });
    const socket = new WebSocket(`${endpoint}?${params.toString()}`);
    this.visualSocket = socket;
    await new Promise<void>((resolve, reject) => {
      const timeout = setTimeout(() => reject(new Error("视觉通道连接超时")), 8000);
      socket.onopen = () => {
        clearTimeout(timeout);
        resolve();
      };
      socket.onerror = () => {
        clearTimeout(timeout);
        reject(new Error("视觉通道连接失败"));
      };
      socket.onmessage = (event) => {
        if (typeof event.data !== "string") return;
        try {
          const message = JSON.parse(event.data);
          if (message.event === "visual.rejected") {
            console.warn("[RealtimeCall] visual frame rejected", message.data);
          }
        } catch {}
      };
    });
  }

  private prepareAudioCapture(): void {
    if (!this.audioStream) return;
    this.audioContext = new AudioContext({ sampleRate: AUDIO_SAMPLE_RATE });
    this.playbackContext = new AudioContext({ sampleRate: PLAYBACK_SAMPLE_RATE });
    const source = this.audioContext.createMediaStreamSource(this.audioStream);
    this.audioNode = this.audioContext.createScriptProcessor(4096, 1, 1);
    this.silenceGain = this.audioContext.createGain();
    this.silenceGain.gain.value = 0;
    source.connect(this.audioNode);
    this.audioNode.connect(this.silenceGain);
    this.silenceGain.connect(this.audioContext.destination);
  }

  private attachAudioProcessor(): void {
    if (!this.audioNode) return;
    this.audioNode.onaudioprocess = (event) => {
      const socket = this.controlSocket;
      if (
        !socket ||
        socket.readyState !== WebSocket.OPEN ||
        this.aiSpeaking ||
        this.mediaState.muted
      ) {
        return;
      }
      const input = event.inputBuffer.getChannelData(0);
      let energy = 0;
      for (let i = 0; i < input.length; i += 16) energy += Math.abs(input[i]);
      if (energy > 0.08 && performance.now() - this.lastSpeechVisualBoostAt > 900) {
        this.lastSpeechVisualBoostAt = performance.now();
        this.requestImmediateVisualFrame();
      }
      const pcm = float32ToPCM(input);
      socket.send(JSON.stringify({ event: "audio", data: arrayBufferToBase64(pcm) }));
    };
  }

  private async startVisualSource(
    source: RealtimeVisualSource,
    stream: MediaStream,
  ): Promise<VisualCaptureRuntime> {
    const video = document.createElement("video");
    video.autoplay = true;
    video.muted = true;
    video.playsInline = true;
    video.srcObject = stream;
    await video.play();
    const canvas = document.createElement("canvas");
    const runtime: VisualCaptureRuntime = {
      source,
      stream,
      video,
      canvas,
      timer: null,
      sequence: 0,
      encoding: false,
      lastSignature: 0,
      immediateRequested: true,
    };
    runtime.timer = setInterval(() => {
      void this.captureVisualFrame(runtime);
    }, VISUAL_INTERVAL_MS[source]);
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
    this.publishSources();
  }

  private async captureVisualFrame(runtime: VisualCaptureRuntime | null): Promise<void> {
    if (!runtime || runtime.encoding || runtime.video.readyState < HTMLMediaElement.HAVE_CURRENT_DATA) return;
    const socket = this.visualSocket;
    if (!socket || socket.readyState !== WebSocket.OPEN) return;
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
      runtime.sequence++;
      const bytes = await blob.arrayBuffer();
      socket.send(
        JSON.stringify({
          event: "visual.frame",
          data: {
            source: runtime.source,
            sequence: runtime.sequence,
            captureTimestamp: new Date().toISOString(),
            mime: "image/jpeg",
            width,
            height,
            immediate,
            data: arrayBufferToBase64(bytes),
          },
        }),
      );
    } finally {
      runtime.encoding = false;
    }
  }

  private publishSources(): void {
    const socket = this.controlSocket;
    if (!socket || socket.readyState !== WebSocket.OPEN) return;
    socket.send(
      JSON.stringify({
        event: "media.sources",
        data: {
          audio: this.mediaState.audio && !this.mediaState.muted,
          camera: this.mediaState.camera,
          screen: this.mediaState.screen,
        },
      }),
    );
  }

  private playAudio(base64Data: string): void {
    try {
      if (!this.playbackContext || this.playbackContext.state === "closed") {
        this.playbackContext = new AudioContext({ sampleRate: PLAYBACK_SAMPLE_RATE });
      }
      if (this.playbackContext.state === "suspended") void this.playbackContext.resume();
      const bytes = base64ToUint8Array(base64Data);
      const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
      const samples = Math.floor(bytes.byteLength / 2);
      const buffer = this.playbackContext.createBuffer(1, samples, PLAYBACK_SAMPLE_RATE);
      const channel = buffer.getChannelData(0);
      for (let index = 0; index < samples; index++) {
        channel[index] = view.getInt16(index * 2, true) / 32768;
      }
      const source = this.playbackContext.createBufferSource();
      source.buffer = buffer;
      source.connect(this.playbackContext.destination);
      const now = this.playbackContext.currentTime;
      this.nextPlayTime = Math.max(now, this.nextPlayTime);
      source.start(this.nextPlayTime);
      this.nextPlayTime += buffer.duration;
      this.aiSpeaking = true;
      source.addEventListener("ended", () => {
        if (!this.playbackContext) return;
        if (this.playbackContext.currentTime + 0.04 >= this.nextPlayTime) this.aiSpeaking = false;
      });
    } catch (error) {
      console.warn("[RealtimeCall] audio playback failed", error);
    }
  }

  private async cleanup(closeSockets: boolean): Promise<void> {
    this.stopVisualSource("camera");
    this.stopVisualSource("screen");
    const visual = this.visualSocket;
    this.visualSocket = null;
    const control = this.controlSocket;
    this.controlSocket = null;
    // Always detach handlers and close the peer channel as well. If one
    // realtime socket dies, leaving the other one open leaks camera/screen
    // sessions until the remote timeout fires.
    if (visual) {
      visual.onopen = null;
      visual.onmessage = null;
      visual.onerror = null;
      visual.onclose = null;
      try { visual.close(); } catch {}
    }
    if (control) {
      control.onopen = null;
      control.onmessage = null;
      control.onerror = null;
      control.onclose = null;
      if (closeSockets || control.readyState !== WebSocket.CLOSED) {
        try { control.close(); } catch {}
      }
    }
    if (this.audioNode) {
      this.audioNode.onaudioprocess = null;
      try { this.audioNode.disconnect(); } catch {}
      this.audioNode = null;
    }
    if (this.silenceGain) {
      try { this.silenceGain.disconnect(); } catch {}
      this.silenceGain = null;
    }
    this.audioStream?.getTracks().forEach((track) => track.stop());
    this.audioStream = null;
    if (this.audioContext && this.audioContext.state !== "closed") {
      try { await this.audioContext.close(); } catch {}
    }
    this.audioContext = null;
    if (this.playbackContext && this.playbackContext.state !== "closed") {
      try { await this.playbackContext.close(); } catch {}
    }
    this.playbackContext = null;
    this.connectedInfo = null;
    this.nextPlayTime = 0;
    this.aiSpeaking = false;
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
  for (let i = 0; i < float32.length; i++) {
    let sample = Math.max(-1, Math.min(1, float32[i]));
    sample = sample < 0 ? sample * 0x8000 : sample * 0x7fff;
    view.setInt16(i * 2, sample, true);
  }
  return buffer;
}

function arrayBufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  const chunk = 0x8000;
  let binary = "";
  for (let offset = 0; offset < bytes.length; offset += chunk) {
    binary += String.fromCharCode(...bytes.subarray(offset, Math.min(offset + chunk, bytes.length)));
  }
  return btoa(binary);
}

function base64ToUint8Array(value: string): Uint8Array {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return bytes;
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string, quality: number): Promise<Blob | null> {
  return new Promise((resolve) => canvas.toBlob(resolve, type, quality));
}

function computeFrameSignature(context: CanvasRenderingContext2D, width: number, height: number): number {
  const sampleWidth = Math.min(32, width);
  const sampleHeight = Math.min(20, height);
  const scratch = document.createElement("canvas");
  scratch.width = sampleWidth;
  scratch.height = sampleHeight;
  const scratchContext = scratch.getContext("2d", { willReadFrequently: true });
  if (!scratchContext) return Date.now();
  scratchContext.drawImage(context.canvas, 0, 0, sampleWidth, sampleHeight);
  const data = scratchContext.getImageData(0, 0, sampleWidth, sampleHeight).data;
  let hash = 2166136261;
  for (let i = 0; i < data.length; i += 4) {
    const luminance = ((data[i] * 3 + data[i + 1] * 6 + data[i + 2]) / 10) | 0;
    hash ^= luminance;
    hash = Math.imul(hash, 16777619);
  }
  return hash >>> 0;
}
