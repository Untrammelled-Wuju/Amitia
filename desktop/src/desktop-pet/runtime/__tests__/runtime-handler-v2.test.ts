import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { DesktopRuntimeHandlerV2 } from "../runtime-handler-v2";
import { buildEnvelope } from "../protocol-v2";
import type { RuntimeEnvelope } from "../protocol-v2";
import type { RuntimeCommandExecutionResult } from "../../../main/pet/runtime-v2-command-adapter";

class FakeWebSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static instances: FakeWebSocket[] = [];

  readonly url: string;
  readyState = FakeWebSocket.CONNECTING;
  sent: string[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: ((event: { code: number; reason: string }) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  send(data: string): void {
    this.sent.push(data);
  }

  close(code = 1000, reason = ""): void {
    if (this.readyState === FakeWebSocket.CLOSED) return;
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.({ code, reason });
  }

  open(): void {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.();
  }

  message(envelope: RuntimeEnvelope): void {
    this.onmessage?.({ data: JSON.stringify(envelope) });
  }
}

async function flushAsync(): Promise<void> {
  for (let i = 0; i < 6; i += 1) {
    await Promise.resolve();
  }
}

function helloAckEnvelope(): RuntimeEnvelope {
  return buildEnvelope(
    "hello_ack",
    "hello_ack",
    "user-1",
    "device-1",
    "runtime-1",
    "session-1",
    1,
    1,
    {
      accepted: true,
      sessionId: "session-1",
      serverTime: new Date().toISOString(),
      currentDesiredRevision: 7,
    },
  );
}

async function connectHandler(
  onCommand: () => Promise<RuntimeCommandExecutionResult>,
): Promise<{ handler: DesktopRuntimeHandlerV2; ws: FakeWebSocket }> {
  const handler = new DesktopRuntimeHandlerV2(
    {
      url: "ws://127.0.0.1/runtime?ticket=ticket-1&deviceId=device-1&runtimeId=runtime-1",
      userId: "user-1",
      deviceId: "device-1",
      runtimeId: "runtime-1",
      autoReconnect: false,
      heartbeatIntervalMs: 60_000,
      connectTimeoutMs: 5_000,
    },
    {
      onState: () => undefined,
      onHelloAck: () => undefined,
      onEvent: () => undefined,
      onError: () => undefined,
      onDesiredSync: () => undefined,
      onCommand: async () => onCommand(),
    },
  );

  const connecting = handler.connect("initial");
  const ws = FakeWebSocket.instances.at(-1);
  if (!ws) throw new Error("fake websocket not created");
  ws.open();
  await flushAsync();

  const hello = JSON.parse(ws.sent[0] ?? "{}") as RuntimeEnvelope;
  expect(hello.messageType).toBe("hello");
  expect(hello.userId).toBe("user-1");
  expect(hello.deviceId).toBe("device-1");
  expect(hello.runtimeId).toBe("runtime-1");

  ws.message(helloAckEnvelope());
  await connecting;
  await flushAsync();
  return { handler, ws };
}

describe("DesktopRuntimeHandlerV2", () => {
  beforeEach(() => {
    FakeWebSocket.instances = [];
    vi.stubGlobal("WebSocket", FakeWebSocket);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("resolves connect when hello_ack is accepted during handshaking", async () => {
    const { handler } = await connectHandler(async () => ({
      commandId: "unused",
      status: "applied",
      errorCode: "",
      errorMessage: "",
      appliedRevision: 0,
    }));

    expect(handler.getState()).toBe("connected");
    expect(handler.getSessionId()).toBe("session-1");
    handler.disconnect();
  });

  it("sends full command ACK progression and terminal completed", async () => {
    const { handler, ws } = await connectHandler(async () => ({
      commandId: "cmd-1",
      status: "applied",
      errorCode: "",
      errorMessage: "",
      appliedRevision: 9,
    }));

    ws.sent = [];
    ws.message(
      buildEnvelope(
        "command",
        "runtime.command.play_action",
        "user-1",
        "device-1",
        "runtime-1",
        "session-1",
        1,
        2,
        {
          commandId: "cmd-1",
          commandType: "runtime.command.play_action",
          commandSequence: 42,
          desiredRevision: 9,
          settingsRevision: 3,
          installationId: "inst-1",
          petId: "pet-1",
          releaseId: "release-1",
          payload: { actionKey: "wave" },
        },
      ),
    );
    await flushAsync();

    const statuses = ws.sent
      .map((item) => JSON.parse(item) as RuntimeEnvelope)
      .filter((env) => env.messageType === "command_ack")
      .map((env) => (env.payload as { commandId: string; status: string }).status);

    expect(statuses).toEqual([
      "runtime_received",
      "runtime_accepted",
      "renderer_accepted",
      "completed",
    ]);
    for (const raw of ws.sent) {
      const env = JSON.parse(raw) as RuntimeEnvelope;
      if (env.messageType !== "command_ack") continue;
      expect((env.payload as { commandId: string }).commandId).toBe("cmd-1");
    }
    handler.disconnect();
  });

  it("sends failed_terminal when renderer command execution fails", async () => {
    const { handler, ws } = await connectHandler(async () => ({
      commandId: "cmd-fail",
      status: "failed",
      errorCode: "RENDERER_FAILED",
      errorMessage: "renderer refused command",
      appliedRevision: 0,
    }));

    ws.sent = [];
    ws.message(
      buildEnvelope(
        "command",
        "runtime.command.recenter_once",
        "user-1",
        "device-1",
        "runtime-1",
        "session-1",
        1,
        3,
        {
          commandId: "cmd-fail",
          commandType: "runtime.command.recenter_once",
          commandSequence: 43,
          desiredRevision: 9,
          settingsRevision: 3,
          installationId: "inst-1",
          petId: "pet-1",
          releaseId: "release-1",
          payload: {},
        },
      ),
    );
    await flushAsync();

    const acks = ws.sent
      .map((item) => JSON.parse(item) as RuntimeEnvelope)
      .filter((env) => env.messageType === "command_ack")
      .map((env) => env.payload as {
        commandId: string;
        status: string;
        rejectErrorCode?: string;
        rejectReason?: string;
      });

    expect(acks.map((ack) => ack.status)).toEqual([
      "runtime_received",
      "runtime_accepted",
      "renderer_accepted",
      "failed_terminal",
    ]);
    expect(acks.at(-1)).toMatchObject({
      commandId: "cmd-fail",
      status: "failed_terminal",
      rejectErrorCode: "RENDERER_FAILED",
      rejectReason: "renderer refused command",
    });
    handler.disconnect();
  });

  it("does not reuse the same ticket by auto reconnecting after close", async () => {
    const { handler, ws } = await connectHandler(async () => ({
      commandId: "unused",
      status: "applied",
      errorCode: "",
      errorMessage: "",
      appliedRevision: 0,
    }));

    const countBeforeClose = FakeWebSocket.instances.length;
    ws.close(1006, "transport_lost");
    await flushAsync();
    expect(handler.getState()).toBe("disconnected");
    expect(FakeWebSocket.instances).toHaveLength(countBeforeClose);
  });
});
