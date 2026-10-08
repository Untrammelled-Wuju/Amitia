import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises } from "@vue/test-utils";
import { OwnedRealtimeSession } from "../realtime/owned-realtime-session";
const cryptoModule = "node:crypto";
const { webcrypto } = await import(cryptoModule);
const mocks = vi.hoisted(() => ({ post: vi.fn(), ws: vi.fn() }));
vi.mock("../composables/useApi", () => ({ apiClient: { post: mocks.post } }));
vi.mock("../runtime/runtime-adapter", () => ({ resolveWebSocketUrl: mocks.ws }));

class Socket {
  static OPEN = 1;
  readyState = 1;
  static latest: Socket;
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;
  sent: unknown[] = [];
  constructor(readonly url: string) { Socket.latest = this; }
  send(value: unknown) { this.sent.push(value); }
  close() { this.readyState = 3; this.onclose?.(); }
  frame(value: unknown) { this.onmessage?.({ data: JSON.stringify(value) }); }
}

describe("Owned 单连接通话原权限及保存确认", () => {
  let scope: any;
  const sessions: OwnedRealtimeSession[] = [];
  const open = async () => {
    const options = { characterId: "role", conversationId: "chat", expectedExecutionScope: scope, onReady: vi.fn(), onCompleted: vi.fn(), onAudio: vi.fn().mockResolvedValue(undefined), onError: vi.fn(), onInterrupted: vi.fn() };
    const session = new OwnedRealtimeSession(options);
    sessions.push(session);
    await session.start();
    Socket.latest.frame({ type: "ready", executionScope: scope });
    await flushPromises();
    return { session, options, socket: Socket.latest };
  };
  const turn = (socket: Socket) => JSON.parse([...socket.sent].reverse().find((value) => typeof value === "string" && JSON.parse(value).type === "turn_start") as string).requestId;
  const completed = (requestId: string) => ({ requestId, executionScope: { ...scope, requestId }, turnId: "turn", executionId: "execution", conversationId: "chat", reply: "已存回复", saved: true });
  const audio = async (requestId: string) => {
    const digest = async (text: string) => Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(text))), (value) => value.toString(16).padStart(2, "0")).join("");
    const resource = `speech/${(await digest(`${scope.coreId}\0${scope.initiatorDeviceId}\0${requestId}`)).slice(0,32)}`;
    return { requestId, executionScope: { ...scope, requestId: `speech/${requestId}` }, saved: true, acknowledgement: { requestId: `speech/${requestId}`, ownerId: scope.resourceOwnerId, versions: { [`tool-result/${resource}`]: 1, [`checkpoint/${resource}`]: 2 } }, audio: { mime: "audio/mpeg", data: btoa("mp3"), sha256: await digest("mp3") } };
  };
  beforeEach(() => {
    vi.stubGlobal("crypto", webcrypto);
    vi.stubGlobal("WebSocket", Socket);
    scope = { authorizationRealm: "mesh", spaceId: "b", initiatorDeviceId: "a", targetDeviceId: "a", coreId: "b", providerEpoch: 1, coordinated: false, modeRevision: 1, permissionRevision: 1, targetPermissionRevision: 1, targetProviderEpoch: 1, roleId: "role", roleRevision: 1, roleOwnerId: "a", resourceOwnerId: "a", requestId: "query", turnId: "q", executionId: "q" };
    mocks.post.mockImplementation(async (_path, payload) => ({ data: { ticket: "once", wsPath: "/api/device-mesh/v1/business/realtime/session", executionScope: payload.expectedExecutionScope } }));
    mocks.ws.mockImplementation(async (path) => `ws://127.0.0.1:18899/internal/device-mesh/provider${path}`);
  });
  afterEach(() => { sessions.splice(0).forEach((session) => session.stop()); vi.unstubAllGlobals(); vi.clearAllMocks(); });
  it("原scope与实际PCM走provider，保存和精确ACK后播放，收到turn_ready才开始下一轮", async () => {
    const { session, socket, options } = await open();
    scope.permissionRevision = 9;
    expect(session.startTurn()).toBe(true);
    const id = turn(socket);
    expect(JSON.parse(socket.sent[0] as string).expectedExecutionScope.permissionRevision).toBe(1);
    scope.permissionRevision = 1;
    const pcm = new Uint8Array([1,2]).buffer;
    session.appendAudio(pcm); session.endTurn();
    expect(socket.sent).toContain(pcm);
    socket.frame({ type: "completed", requestId: id, data: completed(id) });
    socket.frame({ type: "audio", requestId: id, data: await audio(id) });
    socket.frame({ type: "turn_ready", requestId: id });
    await new Promise((resolve) => setTimeout(resolve, 20));
    await flushPromises();
    expect(options.onCompleted).toHaveBeenCalledOnce();
    expect(options.onAudio).toHaveBeenCalledOnce();
    expect(session.startTurn()).toBe(true);
    expect(socket.url).toContain("/internal/device-mesh/provider/api/device-mesh/v1/business/realtime/session?ticket=once");
    expect(mocks.post.mock.calls[0][0]).toBe("/api/device-mesh/v1/business/realtime/tickets");
  });
  it.each(["realm", "ack", "unsaved"])("拦截%s不一致的通话结果", async (kind) => {
    const { session, socket, options } = await open();
    session.startTurn(); session.endTurn();
    const id = turn(socket);
    const reply = completed(id);
    if (kind === "realm") reply.executionScope.authorizationRealm = "foreign";
    if (kind === "unsaved") reply.saved = false;
    socket.frame({ type: "completed", requestId: id, data: reply });
    const speech = await audio(id);
    if (kind === "ack") speech.acknowledgement.versions = { unrelated: 99 };
    socket.frame({ type: "audio", requestId: id, data: speech });
    await new Promise((resolve) => setTimeout(resolve, 20)); await flushPromises();
    expect(options.onAudio).not.toHaveBeenCalled(); expect(options.onError).toHaveBeenCalled();
  });
  it("Core变化立即关闭并丢弃迟到回复", async () => {
    const { session, socket, options } = await open();
    session.startTurn(); const id = turn(socket);
    window.dispatchEvent(new Event("amitia:execution-scope-changed"));
    socket.frame({ type: "completed", requestId: id, data: completed(id) });
    await flushPromises();
    expect(socket.readyState).toBe(3); expect(options.onCompleted).not.toHaveBeenCalled(); expect(options.onError).toHaveBeenCalledWith(expect.stringContaining("通话已中断"));
  });
});
