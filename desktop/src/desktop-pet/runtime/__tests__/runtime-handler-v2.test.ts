import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { DesktopRuntimeHandlerV2 } from "../runtime-handler-v2";
import type { RuntimeCommandReplayEntry } from "../runtime-handler-v2";
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
  readonly protocols: string[];
  readyState = FakeWebSocket.CONNECTING;
  sent: string[] = [];
  failCommandAckStatusOnce: string | null = null;
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: ((event: { code: number; reason: string }) => void) | null = null;

  constructor(url: string, protocols?: string | string[]) {
    this.url = url;
    this.protocols = Array.isArray(protocols)
      ? [...protocols]
      : protocols
        ? [protocols]
        : [];
    FakeWebSocket.instances.push(this);
  }

  send(data: string): void {
    if (this.failCommandAckStatusOnce) {
      const envelope = JSON.parse(data) as RuntimeEnvelope;
      const status = envelope.messageType === "command_ack"
        ? (envelope.payload as { status?: string } | undefined)?.status
        : undefined;
      if (status === this.failCommandAckStatusOnce) {
        this.failCommandAckStatusOnce = null;
        throw new Error(`synthetic send failure for ${status}`);
      }
    }
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

function helloAckEnvelope(overrides: Record<string, unknown> = {}): RuntimeEnvelope {
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
      ...overrides,
    },
  );
}

async function connectHandler(
  onCommand: () => Promise<RuntimeCommandExecutionResult>,
  resumeCursor?: {
    lastAppliedDesiredRevision?: number;
    lastProcessedCommandSequence?: number;
    lastEventSequence?: number;
    actualStateHash?: string;
  },
  helloAckOverrides: Record<string, unknown> = {},
  replayEntries: readonly RuntimeCommandReplayEntry[] = [],
): Promise<{ handler: DesktopRuntimeHandlerV2; ws: FakeWebSocket; hello: RuntimeEnvelope }> {
  const handler = new DesktopRuntimeHandlerV2(
    {
      url: "ws://127.0.0.1/runtime?deviceId=device-1&runtimeId=runtime-1",
      bootstrapTicket: "ticket-1",
      userId: "user-1",
      deviceId: "device-1",
      runtimeId: "runtime-1",
      autoReconnect: false,
      heartbeatIntervalMs: 60_000,
      connectTimeoutMs: 5_000,
      resumeCursor,
      replayEntries,
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

  ws.message(helloAckEnvelope(helloAckOverrides));
  await connecting;
  await flushAsync();
  return { handler, ws, hello };
}

describe("DesktopRuntimeHandlerV2", () => {
  beforeEach(() => {
    FakeWebSocket.instances = [];
    vi.stubGlobal("WebSocket", FakeWebSocket);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("keeps bootstrap credentials out of the websocket URL", async () => {
    const { handler, ws } = await connectHandler(async () => ({
      commandId: "unused",
      status: "applied",
      errorCode: "",
      errorMessage: "",
      appliedRevision: 0,
    }));

    expect(new URL(ws.url).searchParams.has("ticket")).toBe(false);
    expect(ws.protocols).toEqual([
      "amitia.runtime.v2",
      "amitia.runtime.bootstrap.ticket-1",
    ]);
    handler.disconnect();
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


  it("keeps resume cursors separate from the server desired target", async () => {
    const { handler, hello } = await connectHandler(
      async () => ({
        commandId: "unused",
        status: "applied",
        errorCode: "",
        errorMessage: "",
        appliedRevision: 0,
      }),
      {
        lastAppliedDesiredRevision: 5,
        lastProcessedCommandSequence: 40,
        lastEventSequence: 80,
        actualStateHash: "sha256:resume",
      },
    );

    expect(hello.sequence).toBe(81);
    expect(hello.payload).toMatchObject({
      lastAppliedDesiredRevision: 5,
      lastProcessedCommandSequence: 40,
      lastEventSequence: 80,
      actualStateHash: "sha256:resume",
    });
    // hello_ack advertises currentDesiredRevision=7, but that is a target, not
    // proof that revision 7 has been applied by this runtime.
    expect(handler.getLastAppliedDesiredRevision()).toBe(5);
    expect(handler.getLastProcessedCommandSequence()).toBe(40);
    handler.disconnect();
  });

  it("advances an accepted playback command only after a terminal playback event", async () => {
    const { handler, ws } = await connectHandler(async () => ({
      commandId: "cmd-play",
      status: "accepted",
      errorCode: "",
      errorMessage: "",
      appliedRevision: 0,
      acceptedAction: "wave",
      playbackRequestId: "playback-1",
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
        4,
        {
          commandId: "cmd-play",
          commandType: "runtime.command.play_action",
          commandSequence: 44,
          desiredRevision: 9,
          payload: { actionKey: "wave" },
        },
      ),
    );
    await flushAsync();

    expect(handler.getLastProcessedCommandSequence()).toBe(0);
    expect(handler.getLastAppliedDesiredRevision()).toBe(0);

    await handler.sendPlaybackEnded(
      "playback-1",
      "cmd-play",
      "wave",
      1000,
      "natural_end",
    );
    expect(handler.getLastProcessedCommandSequence()).toBe(44);

    await handler.sendRendererState({
      connectionGeneration: handler.getConnectionGeneration(),
      eventSequence: 0,
      actualStateHash: "sha256:state",
      instanceStatus: "ready",
      windowStatus: "visible",
      rendererStatus: "runtime_ready",
      playbackStatus: "idle",
      appliedDesiredRevision: 0,
      appliedSettingsRevision: 0,
      installationId: "inst-1",
      petId: "pet-1",
      releaseId: "release-1",
      stableActionKey: "idle",
      currentActionKey: "idle",
      lastProcessedCommandSequence: handler.getLastProcessedCommandSequence(),
      capturedAt: new Date().toISOString(),
    });

    const snapshots = ws.sent
      .map((item) => JSON.parse(item) as RuntimeEnvelope)
      .filter((env) => env.messageName === "runtime.state.snapshot");
    expect(snapshots.at(-1)?.payload).toMatchObject({
      lastProcessedCommandSequence: 44,
      actualStateHash: "sha256:state",
    });
    handler.disconnect();
  });

  it("emits the canonical health-changed payload and suppresses duplicate healthy events", async () => {
    const { handler, ws } = await connectHandler(async () => ({
      commandId: "unused",
      status: "applied",
      errorCode: "",
      errorMessage: "",
      appliedRevision: 0,
    }));

    ws.sent = [];
    await handler.sendRendererHealth(true);
    await handler.sendRendererHealth(true);
    const healthEvents = ws.sent
      .map((item) => JSON.parse(item) as RuntimeEnvelope)
      .filter((env) => env.messageName === "runtime.health.changed");

    expect(healthEvents).toHaveLength(1);
    expect(healthEvents[0]?.payload).toMatchObject({
      previousStatus: "unknown",
      currentStatus: "healthy",
    });
    expect((healthEvents[0]?.payload as { changedAt?: string }).changedAt).toBeTruthy();
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
  it("rejects a server envelope from a superseded generation before command execution", async () => {
    const onCommand = vi.fn(async () => ({
      commandId: "cmd-stale",
      status: "applied" as const,
      errorCode: "",
      errorMessage: "",
      appliedRevision: 0,
    }));
    const { handler, ws } = await connectHandler(onCommand);

    ws.message(
      buildEnvelope(
        "command",
        "runtime.command.play_action",
        "user-1",
        "device-1",
        "runtime-1",
        "session-1",
        2,
        9,
        {
          commandId: "cmd-stale",
          commandType: "runtime.command.play_action",
          commandSequence: 99,
          payload: { actionKey: "wave" },
        },
      ),
    );
    await flushAsync();

    expect(onCommand).not.toHaveBeenCalled();
    expect(ws.readyState).toBe(FakeWebSocket.CLOSED);
    expect(handler.getState()).toBe("disconnected");
  });

  it("rejects a server envelope whose payload hash no longer matches", async () => {
    const onCommand = vi.fn(async () => ({
      commandId: "cmd-tampered",
      status: "applied" as const,
      errorCode: "",
      errorMessage: "",
      appliedRevision: 0,
    }));
    const { handler, ws } = await connectHandler(onCommand);

    const tampered = buildEnvelope(
      "command",
      "runtime.command.play_action",
      "user-1",
      "device-1",
      "runtime-1",
      "session-1",
      1,
      10,
      {
        commandId: "cmd-tampered",
        commandType: "runtime.command.play_action",
        commandSequence: 100,
        payload: { actionKey: "wave" },
      },
    );
    tampered.payload = {
      commandId: "cmd-tampered",
      commandType: "runtime.command.play_action",
      commandSequence: 100,
      payload: { actionKey: "shutdown" },
    };
    ws.message(tampered);
    await flushAsync();

    expect(onCommand).not.toHaveBeenCalled();
    expect(ws.readyState).toBe(FakeWebSocket.CLOSED);
    expect(handler.getState()).toBe("disconnected");
  });

  it("honors the negotiated server message-size limit for outbound envelopes", async () => {
    const { handler } = await connectHandler(
      async () => ({
        commandId: "unused",
        status: "applied",
        errorCode: "",
        errorMessage: "",
        appliedRevision: 0,
      }),
      undefined,
      { maxMessageBytes: 512 },
    );

    await expect(
      handler.sendRuntimeEvent("runtime.test.large", { body: "x".repeat(2048) }),
    ).rejects.toThrow("maxMessageBytes");
    handler.disconnect();
  });

  it("replays the cached terminal ACK without executing a redelivered command twice", async () => {
    const onCommand = vi.fn(async () => ({
      commandId: "cmd-replay",
      status: "applied" as const,
      errorCode: "",
      errorMessage: "",
      appliedRevision: 0,
    }));
    const { handler, ws } = await connectHandler(onCommand);

    const payload = {
      commandId: "cmd-replay",
      commandType: "runtime.command.recenter_once",
      commandSequence: 45,
      desiredRevision: 0,
      payload: {},
    };
    ws.message(buildEnvelope(
      "command",
      "runtime.command.recenter_once",
      "user-1",
      "device-1",
      "runtime-1",
      "session-1",
      1,
      11,
      payload,
    ));
    await flushAsync();
    expect(onCommand).toHaveBeenCalledTimes(1);

    ws.sent = [];
    ws.message(buildEnvelope(
      "command",
      "runtime.command.recenter_once",
      "user-1",
      "device-1",
      "runtime-1",
      "session-1",
      1,
      12,
      payload,
    ));
    await flushAsync();

    expect(onCommand).toHaveBeenCalledTimes(1);
    const replayStatuses = ws.sent
      .map((item) => JSON.parse(item) as RuntimeEnvelope)
      .filter((env) => env.messageType === "command_ack")
      .map((env) => (env.payload as { status: string }).status);
    expect(replayStatuses).toEqual(["completed"]);
    handler.disconnect();
  });

  it("does not treat the command sequence high-water mark as proof that every lower command ran", async () => {
    const onCommand = vi.fn(async () => ({
      commandId: "cmd-old",
      status: "applied" as const,
      errorCode: "",
      errorMessage: "",
      appliedRevision: 0,
    }));
    const { handler, ws } = await connectHandler(onCommand, {
      lastAppliedDesiredRevision: 5,
      lastProcessedCommandSequence: 50,
    });

    ws.sent = [];
    ws.message(buildEnvelope(
      "command",
      "runtime.command.recenter_once",
      "user-1",
      "device-1",
      "runtime-1",
      "session-1",
      1,
      13,
      {
        commandId: "cmd-old",
        commandType: "runtime.command.recenter_once",
        commandSequence: 49,
        desiredRevision: 0,
        payload: {},
      },
    ));
    await flushAsync();

    expect(onCommand).toHaveBeenCalledTimes(1);
    const statuses = ws.sent
      .map((item) => JSON.parse(item) as RuntimeEnvelope)
      .filter((env) => env.messageType === "command_ack")
      .map((env) => (env.payload as { status: string }).status);
    expect(statuses.at(-1)).toBe("completed");
    handler.disconnect();
  });

  it("carries terminal commandId replay protection across handler replacement", async () => {
    const firstExecutor = vi.fn(async () => ({
      commandId: "cmd-persisted-replay",
      status: "applied" as const,
      errorCode: "",
      errorMessage: "",
      appliedRevision: 0,
    }));
    const first = await connectHandler(firstExecutor);
    first.ws.message(buildEnvelope(
      "command",
      "runtime.command.recenter_once",
      "user-1",
      "device-1",
      "runtime-1",
      "session-1",
      1,
      14,
      {
        commandId: "cmd-persisted-replay",
        commandType: "runtime.command.recenter_once",
        commandSequence: 61,
        desiredRevision: 0,
        payload: {},
      },
    ));
    await flushAsync();
    expect(firstExecutor).toHaveBeenCalledTimes(1);
    const replayEntries = first.handler.getReplayEntries();
    first.handler.disconnect();

    const secondExecutor = vi.fn(async () => ({
      commandId: "cmd-persisted-replay",
      status: "applied" as const,
      errorCode: "",
      errorMessage: "",
      appliedRevision: 0,
    }));
    const second = await connectHandler(
      secondExecutor,
      { lastProcessedCommandSequence: 61 },
      {},
      replayEntries,
    );
    second.ws.sent = [];
    second.ws.message(buildEnvelope(
      "command",
      "runtime.command.recenter_once",
      "user-1",
      "device-1",
      "runtime-1",
      "session-1",
      1,
      15,
      {
        commandId: "cmd-persisted-replay",
        commandType: "runtime.command.recenter_once",
        commandSequence: 61,
        desiredRevision: 0,
        payload: {},
      },
    ));
    await flushAsync();

    expect(secondExecutor).not.toHaveBeenCalled();
    const statuses = second.ws.sent
      .map((item) => JSON.parse(item) as RuntimeEnvelope)
      .filter((env) => env.messageType === "command_ack")
      .map((env) => (env.payload as { status: string }).status);
    expect(statuses).toEqual(["completed"]);
    second.handler.disconnect();
  });


  it("records terminal execution before a terminal ACK transport failure", async () => {
    const firstExecutor = vi.fn(async () => ({
      commandId: "cmd-ack-loss",
      status: "applied" as const,
      errorCode: "",
      errorMessage: "",
      appliedRevision: 72,
    }));
    const first = await connectHandler(firstExecutor);
    first.ws.failCommandAckStatusOnce = "completed";
    first.ws.message(buildEnvelope(
      "command",
      "runtime.command.sync_desired_state",
      "user-1",
      "device-1",
      "runtime-1",
      "session-1",
      1,
      16,
      {
        commandId: "cmd-ack-loss",
        commandType: "runtime.command.sync_desired_state",
        commandSequence: 72,
        desiredRevision: 72,
        payload: {},
      },
    ));
    await flushAsync();

    expect(firstExecutor).toHaveBeenCalledTimes(1);
    const replayEntries = first.handler.getReplayEntries();
    expect(replayEntries).toEqual(expect.arrayContaining([
      expect.objectContaining({
        commandId: "cmd-ack-loss",
        commandSequence: 72,
        desiredRevision: 72,
        ackStatus: "completed",
      }),
    ]));

    const secondExecutor = vi.fn(async () => ({
      commandId: "cmd-ack-loss",
      status: "applied" as const,
      errorCode: "",
      errorMessage: "",
      appliedRevision: 72,
    }));
    const second = await connectHandler(
      secondExecutor,
      { lastAppliedDesiredRevision: 72, lastProcessedCommandSequence: 72 },
      {},
      replayEntries,
    );
    second.ws.sent = [];
    second.ws.message(buildEnvelope(
      "command",
      "runtime.command.sync_desired_state",
      "user-1",
      "device-1",
      "runtime-1",
      "session-1",
      1,
      17,
      {
        commandId: "cmd-ack-loss",
        commandType: "runtime.command.sync_desired_state",
        commandSequence: 72,
        desiredRevision: 72,
        payload: {},
      },
    ));
    await flushAsync();

    expect(secondExecutor).not.toHaveBeenCalled();
    const statuses = second.ws.sent
      .map((item) => JSON.parse(item) as RuntimeEnvelope)
      .filter((env) => env.messageType === "command_ack")
      .map((env) => (env.payload as { status: string }).status);
    expect(statuses).toEqual(["completed"]);
    second.handler.disconnect();
  });


});
