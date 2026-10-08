import { apiClient } from "../composables/useApi";
import { resolveWebSocketUrl } from "../runtime/runtime-adapter";
import type { OwnedChatResponse, OwnedExecutionScope } from "../runtime/device-owned-chat";
import type { OwnedConversationOrigin } from "../runtime/device-owned-conversation-reference";
import type { OwnedAttachment } from "../runtime/device-owned-attachments";
import { ownedAuthorityFields, sameOwnedAuthority, validateOwnedSpeechResult } from "../runtime/owned-speech-result";

export interface OwnedRealtimeOptions {
  characterId: string;
  conversationId: string;
  conversationOrigin?: OwnedConversationOrigin;
  historicalRoleId?: string;
  expectedExecutionScope: OwnedExecutionScope;
  acceptedTicket?: Record<string, any>;
  onReady: () => void;
  onCompleted: (response: OwnedChatResponse) => void;
  onAudio: (bytes: Uint8Array) => Promise<void>;
  onError: (message: string) => void;
  onInterrupted: () => void;
}

export class OwnedRealtimeSession {
  private socket: WebSocket | null = null;
  private controller = new AbortController();
  private generation = 0;
  private ready = false;
  private turnId = "";
  private audioBytes = 0;
  private recording = false;
  private completed = false;
  private turnReady = true;
  private cancelled = false;
  private audioReceived = false;
  private inbox = Promise.resolve();
  private latestVisual: OwnedAttachment | null = null;
  private readonly scope: OwnedExecutionScope;
  private readonly invalidated = () => this.fail("云端服务或数据归属已变化，通话已中断");

  constructor(private readonly options: OwnedRealtimeOptions) {
    this.scope = JSON.parse(JSON.stringify(options.expectedExecutionScope));
  }

  async start(): Promise<void> {
    if (!this.scope.coreId || this.scope.roleId !== this.options.characterId || ownedAuthorityFields.some((key) => this.scope[key] === undefined)) throw new Error("通话角色权限或数据归属无法确认");
    const captured = this.generation;
    window.addEventListener("amitia:runtime-connection-changed", this.invalidated);
    window.addEventListener("amitia:execution-scope-changed", this.invalidated);
    const response = this.options.acceptedTicket ? undefined : await apiClient.post("/api/device-mesh/v1/business/realtime/tickets", { requestId: crypto.randomUUID(), characterId: this.options.characterId, conversationId: this.options.conversationId, conversationOrigin: this.options.conversationOrigin, historicalRoleId: this.options.historicalRoleId, expectedExecutionScope: this.scope }, { signal: this.controller.signal });
    const ticket = this.options.acceptedTicket ?? response?.data?.data ?? response?.data;
    if (captured !== this.generation) throw new Error("通话服务已变化");
    if (!ticket?.ticket || ticket.wsPath !== "/api/device-mesh/v1/business/realtime/session" || !sameOwnedAuthority(this.scope, ticket.executionScope)) throw new Error("通话票据归属不一致");
    const url = await resolveWebSocketUrl(`${ticket.wsPath}?ticket=${encodeURIComponent(ticket.ticket)}`);
    if (captured !== this.generation) throw new Error("通话服务已变化");
    this.socket = new WebSocket(url);
    this.socket.onmessage = (event) => { this.inbox = this.inbox.then(() => this.receive(event.data, captured)).catch((error) => this.fail(error instanceof Error ? error.message : "通话响应无效")); };
    this.socket.onerror = () => this.fail("云端通话连接失败");
    this.socket.onclose = () => { if (captured === this.generation) this.fail("云端通话连接已关闭"); };
  }

  startTurn(): boolean {
    if (!this.ready || this.recording || !this.turnReady) return false;
    this.turnId = crypto.randomUUID();
    this.audioBytes = 0;
    this.recording = true;
    this.completed = false;
    this.turnReady = false;
    this.cancelled = false;
    this.audioReceived = false;
    this.options.onInterrupted();
    this.send({ type: "turn_start", requestId: this.turnId, expectedExecutionScope: this.scope });
    if (this.latestVisual) this.send({ type: "visual", attachment: this.latestVisual });
    return true;
  }

  appendAudio(bytes: ArrayBuffer): void {
    if (!this.recording) return;
    this.audioBytes += bytes.byteLength;
    if (this.audioBytes > 1048576 - 44) { this.interrupt(); this.options.onError("本轮语音超过大小上限，请缩短后重试"); return; }
    this.socket?.send(bytes);
  }

  endTurn(): void {
    if (!this.recording) return;
    this.recording = false;
    this.send({ type: "turn_end" });
  }

  setVisual(attachment: OwnedAttachment): void {
    if (attachment.kind !== "image") throw new Error("通话画面必须为图片");
    this.latestVisual = attachment;
    if (this.recording) this.send({ type: "visual", attachment });
  }

  interrupt(): void {
    this.send({ type: "interrupt" });
    this.recording = false;
    this.completed = false;
    this.cancelled = true;
    this.options.onInterrupted();
  }

  stop(): void {
    this.send({ type: "stop" });
    this.generation++;
    this.controller.abort();
    this.ready = false;
    this.recording = false;
    this.turnId = "";
    this.latestVisual = null;
    const socket = this.socket;
    this.socket = null;
    socket?.close();
    window.removeEventListener("amitia:runtime-connection-changed", this.invalidated);
    window.removeEventListener("amitia:execution-scope-changed", this.invalidated);
    this.options.onInterrupted();
  }

  private send(value: unknown): void {
    if (this.socket?.readyState === WebSocket.OPEN) this.socket.send(JSON.stringify(value));
  }

  private fail(message: string): void {
    this.stop();
    this.options.onError(message);
  }

  private async receive(raw: unknown, captured: number): Promise<void> {
    if (captured !== this.generation) return;
    if (typeof raw !== "string" || raw.length > 2 * 1048576) throw new Error("云端通话响应格式或大小无效");
    const frame = JSON.parse(raw);
    if (frame.type === "authority_changed") throw new Error("云端服务、角色或权限已变化，通话已中断");
    if (frame.type === "ready") {
      if (this.ready || !sameOwnedAuthority(this.scope, frame.executionScope)) throw new Error("通话服务归属不一致");
      this.ready = true;
      this.options.onReady();
      return;
    }
    if (!frame.requestId || frame.requestId !== this.turnId) return;
    if (frame.type === "turn_ready") { this.turnReady = true; return; }
    if (this.cancelled) return;
    if (frame.type === "error") {
      this.recording = false;
      this.completed = true;
      this.options.onError(`${frame.saved ? "对话已保存，语音处理失败：" : "本轮通话未完成："}${String(frame.message || "未知错误")}`);
      return;
    }
    if (frame.type === "completed") {
      const result = frame.data as OwnedChatResponse;
      if (this.completed || !result?.saved || result.requestId !== this.turnId || result.executionScope?.requestId !== this.turnId || !sameOwnedAuthority(this.scope, result.executionScope) || !result.conversationId || !result.turnId || !result.executionId) throw new Error("通话回复缺少原数据所有者保存确认");
      this.completed = true;
      this.options.onCompleted(result);
      return;
    }
    if (frame.type === "audio") {
      if (!this.completed || this.audioReceived) throw new Error("对话尚未保存或语音响应重复，已拦截语音播放");
      this.audioReceived = true;
      const requestId = this.turnId;
      const bytes = await validateOwnedSpeechResult(frame.data, requestId, this.scope);
      if (captured !== this.generation || requestId !== this.turnId) return;
      await this.options.onAudio(bytes);
      return;
    }
    if (frame.type === "event" && frame.data?.executionScope && !sameOwnedAuthority(this.scope, frame.data.executionScope)) throw new Error("通话草稿归属变化");
  }
}
