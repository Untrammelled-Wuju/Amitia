import { describe, expect, it, vi } from "vitest";

vi.mock("../runtime/runtime-adapter", () => ({ resolveApiUrl: vi.fn() }));
vi.mock("../runtime/request-auth", () => ({ createAuthenticatedFetchInit: vi.fn() }));

import { consumeOwnedChatStream, type OwnedExecutionScope } from "../runtime/device-owned-chat";

const scope: OwnedExecutionScope = {
  spaceId: "core", initiatorDeviceId: "a", targetDeviceId: "a", coreId: "core", providerEpoch: 1,
  coordinated: false, modeRevision: 1, permissionRevision: 1, targetPermissionRevision: 1,
  targetProviderEpoch: 1, roleId: "role", roleRevision: 1, roleOwnerId: "a", resourceOwnerId: "a",
  requestId: "request", turnId: "turn", executionId: "execution",
};

function stream(events: unknown[], split = false) {
  const encoded = new TextEncoder().encode(events.map((event) => `event:message\ndata:${JSON.stringify(event)}\n\n`).join(""));
  return new ReadableStream<Uint8Array>({ start(controller) {
    if (split) for (let i = 0; i < encoded.length; i++) controller.enqueue(encoded.slice(i, i + 1));
    else controller.enqueue(encoded);
    controller.close();
  } });
}

describe("device owned chat stream", () => {
  it("validates phase events without treating output completion as a save acknowledgement", async () => {
    const result = { requestId: "request", conversationId: "conversation", executionScope: scope, saved: true, reply: "正文", memoryStatus: "pending" };
    const events = [{ type: "started", executionScope: scope }, { type: "delta", executionScope: scope, text: "思考", reasoning: true }, { type: "reasoning-completed", executionScope: scope }, { type: "delta", executionScope: scope, text: "正文" }, { type: "output-completed", executionScope: scope }];
    await expect(consumeOwnedChatStream(stream(events), "request", () => {})).rejects.toThrow("结果尚未确认");
    const seen: string[] = [];
    await consumeOwnedChatStream(stream([...events, { type: "completed", data: result }]), "request", event => seen.push(event.type));
    expect(seen).toEqual(["started", "delta", "reasoning-completed", "delta", "output-completed", "completed"]);
    await expect(consumeOwnedChatStream(stream([events[0], { type: "output-completed", executionScope: { ...scope, roleId: "foreign" } }]), "request", () => {})).rejects.toThrow("归属");
  });
  it("accepts null, missing and empty citation snapshots from saved completion frames", async () => {
    for (const citation of [{ citationSnapshot: null }, {}, { citationSnapshot: [] }]) {
      const seen: any[] = [];
      const result = { requestId: "request", conversationId: "conversation", executionScope: scope, saved: true, reply: "状态诊断完成", memoryStatus: "pending", userRevision: 1, ...citation };
      const response = await consumeOwnedChatStream(stream([{ type: "started", executionScope: scope }, { type: "delta", executionScope: scope, text: "状态诊断完成" }, { type: "completed", data: result }], true), "request", event => seen.push(event), scope);
      expect(response.saved).toBe(true);
      expect(response.citationSnapshot).toEqual([]);
      expect(seen.at(-1)).toMatchObject({ type: "completed", data: { saved: true, citationSnapshot: [] } });
    }
  });

	it("accepts scoped task progress without ending or changing reply text", async () => {
	  const seen: any[] = [];
	  const result = { requestId: "request", conversationId: "conversation", executionScope: scope, saved: true, reply: "你好", memoryStatus: "saved" };
	  const response = await consumeOwnedChatStream(stream([{ type: "started", executionScope: scope, conversationId: "conversation" }, { type: "task", executionScope: scope, conversationId: "conversation", task: { coordinationId: "coordination-a", status: "running" } }, { type: "delta", executionScope: scope, text: "你好" }, { type: "completed", data: result }]), "request", event => seen.push(event));
	  expect(response.reply).toBe("你好");
	  expect(seen.map(event => event.type)).toEqual(["started", "task", "delta", "completed"]);
	});
  it("accepts tool lifecycle events without interrupting the reply", async () => {
    const seen: any[] = [];
    const result = { requestId: "request", conversationId: "conversation", executionScope: scope, saved: true, reply: "计算完成", memoryStatus: "saved" };
    const response = await consumeOwnedChatStream(stream([
      { type: "started", executionScope: scope, conversationId: "conversation" },
      { type: "tool-started", executionScope: scope, conversationId: "conversation", progress: { toolCallId: "call-1", toolName: "calculate", arguments: "{\"expression\":\"1+1\"}", result: "", status: "", durationMs: 0 } },
      { type: "tool-completed", executionScope: scope, conversationId: "conversation", progress: { toolCallId: "call-1", toolName: "calculate", arguments: "{\"expression\":\"1+1\"}", result: "{\"result\":2}", status: "success", durationMs: 2 } },
      { type: "delta", executionScope: scope, text: "计算完成" },
      { type: "completed", data: result },
    ]), "request", event => seen.push(event));
    expect(response.reply).toBe("计算完成");
    expect(seen.map(event => event.type)).toEqual(["started", "tool-started", "tool-completed", "delta", "completed"]);
  });
  it("keeps the server-provided task failure message and error code", async () => {
    const seen: any[] = [];
    await expect(consumeOwnedChatStream(stream([
      { type: "started", executionScope: scope, conversationId: "conversation" },
      { type: "failed", message: "浏览器运行时未启用或不可用，无法执行网页自动化任务。", errorCode: "browser_runtime_unavailable", retryable: true },
    ]), "request", event => seen.push(event))).rejects.toThrow("浏览器运行时未启用或不");
    expect(seen.at(-1)).toMatchObject({ type: "failed", errorCode: "browser_runtime_unavailable", retryable: true });
  });
  it("rejects malformed, foreign or dishonest tool lifecycle events", async () => {
    const base = { type: "tool-started", executionScope: scope, conversationId: "conversation", progress: { toolCallId: "call-1", toolName: "calculate", arguments: "{\"expression\":\"1+1\"}", result: "", status: "", durationMs: 0 } };
    const changes = [
      { conversationId: "other" },
      { executionScope: { ...scope, coreId: "other" } },
      { progress: undefined },
      { progress: { ...base.progress, toolCallId: "" } },
      { progress: { ...base.progress, toolName: "bad name" } },
      { progress: { ...base.progress, arguments: "{" } },
      { progress: { ...base.progress, result: "x".repeat(1024 * 1024 + 1) } },
      { progress: { ...base.progress, durationMs: -1 } },
      { type: "tool-completed", progress: { ...base.progress, status: "" } },
      { type: "tool-completed", progress: { ...base.progress, status: "failed", errorCode: "", error: "" } },
    ];
    for (const change of changes) {
      const event = { ...base, ...change };
      await expect(consumeOwnedChatStream(stream([
        { type: "started", executionScope: scope, conversationId: "conversation" },
        event,
      ]), "request", () => undefined)).rejects.toThrow();
    }
  });
  it("normalizes the committed citation snapshot from the final response", async () => {
    const result = {
      requestId: "request",
      conversationId: "conversation",
      executionScope: scope,
      saved: true,
      reply: "项目代号是晨星。[K1]",
      memoryStatus: "saved",
      citationSnapshot: [
        { marker: "[K1]", kind: "knowledge", index: 1, sourceId: "source-a", title: "项目资料", excerpt: "项目代号是晨星。" },
      ],
    };
    const response = await consumeOwnedChatStream(stream([{ type: "started", executionScope: scope }, { type: "completed", data: result }]), "request", () => undefined);
    expect(response.citationSnapshot).toEqual([
      { marker: "[K1]", kind: "knowledge", index: 1, sourceId: "source-a", title: "项目资料", excerpt: "项目代号是晨星。" },
    ]);
  });
  it("rejects malformed committed citation snapshots", async () => {
    const result = { requestId: "request", conversationId: "conversation", executionScope: scope, saved: true, reply: "回答", memoryStatus: "saved", citationSnapshot: "invalid" };
    await expect(consumeOwnedChatStream(stream([{ type: "started", executionScope: scope }, { type: "completed", data: result }]), "request", () => undefined)).rejects.toThrow("引用快照格式无效");
  });
	it("rejects task events from other scope, conversation and invalid task states", async () => {
	  for (const change of [{ executionScope: { ...scope, coreId: "other" } }, { executionScope: { ...scope, roleId: "other" } }, { executionScope: { ...scope, resourceOwnerId: "other" } }, { conversationId: "other" }, { task: { coordinationId: "coordination-a", status: "invented" } }, { task: { status: "running" } }]) {
	    const event = { type: "task", executionScope: scope, conversationId: "conversation", task: { coordinationId: "coordination-a", status: "running" }, ...change };
	    const seen: any[] = [];
	    await expect(consumeOwnedChatStream(stream([{ type: "started", executionScope: scope, conversationId: "conversation" }, event]), "request", value => seen.push(value))).rejects.toThrow();
	    expect(seen).toHaveLength(1);
	  }
	});
  it("拒绝首帧及后续帧跨realm或原编辑角色同ID变更", async () => {
    const original = { ...scope, authorizationRealm: "mesh" };
    await expect(consumeOwnedChatStream(stream([{ type: "started", executionScope: { ...original, roleRevision: 2 } }]), "request", () => {}, original)).rejects.toThrow("迟到回复");
    await expect(consumeOwnedChatStream(stream([{ type: "started", executionScope: original }, { type: "delta", text: "foreign", executionScope: { ...original, authorizationRealm: "other" } }]), "request", () => {})).rejects.toThrow("迟到回复");
  });
	it("accepts acknowledged transcription and rejects late or duplicate transcription", async () => {
		const result = { requestId: "request", conversationId: "conversation", executionScope: scope, saved: true, reply: "你好", transcription: "喝茶", userRevision: 2, memoryStatus: "saved" };
		const seen: any[] = [];
		const response = await consumeOwnedChatStream(stream([{ type: "started", executionScope: scope }, { type: "transcribed", executionScope: scope, text: "喝茶" }, { type: "completed", data: result }]), "request", (event) => seen.push(event));
		expect(response.transcription).toBe("喝茶");
		expect(seen[1].type).toBe("transcribed");
		await expect(consumeOwnedChatStream(stream([{ type: "started", executionScope: scope }, { type: "transcribed", executionScope: { ...scope, providerEpoch: 2 }, text: "late" }]), "request", () => undefined)).rejects.toThrow("迟到回复");
		await expect(consumeOwnedChatStream(stream([{ type: "started", executionScope: scope }, { type: "transcribed", executionScope: scope, text: "first" }, { type: "transcribed", executionScope: scope, text: "second" }]), "request", () => undefined)).rejects.toThrow("转写事件无效");
	});
  it("preserves split Chinese text and requires owner save acknowledgement", async () => {
    const events: any[] = [];
    const result = { requestId: "request", conversationId: "conversation", executionScope: scope, saved: true, reply: "你好", memoryStatus: "saved" };
    const response = await consumeOwnedChatStream(stream([{ type: "started", executionScope: scope }, { type: "delta", executionScope: scope, text: "你好" }, { type: "completed", data: result }], true), "request", (event) => events.push(event));
    expect(response.reply).toBe("你好");
    expect(events[1].text).toBe("你好");
  });
  it("requires the exact acknowledged transcript and accepts completed cached audio", async () => {
    const result = { requestId: "request", conversationId: "conversation", executionScope: scope, saved: true, reply: "你好", transcription: "喝茶", userRevision: 2, memoryStatus: "saved" };
    const prefix = [{ type: "started", executionScope: scope }, { type: "transcribed", executionScope: scope, text: "喝茶" }];
    for (const change of [{ transcription: "changed" }, { userRevision: 1 }, { transcription: undefined }, { transcription: " " }]) {
      await expect(consumeOwnedChatStream(stream([...prefix, { type: "completed", data: { ...result, ...change } }]), "request", () => undefined)).rejects.toThrow("保存确认不一致");
    }
    expect((await consumeOwnedChatStream(stream([{ type: "completed", data: result }]), "request", () => undefined)).transcription).toBe("喝茶");
  });
  it("blocks events from a changed provider epoch", async () => {
    const seen: any[] = [];
    await expect(consumeOwnedChatStream(stream([{ type: "started", executionScope: scope }, { type: "delta", executionScope: { ...scope, providerEpoch: 2 }, text: "late" }]), "request", (event) => seen.push(event))).rejects.toThrow("迟到回复已拦截");
    expect(seen).toHaveLength(1);
  });
  it("does not mark disconnected or unacknowledged replies as saved", async () => {
    await expect(consumeOwnedChatStream(stream([{ type: "started", executionScope: scope }, { type: "delta", executionScope: scope, text: "partial" }]), "request", () => undefined)).rejects.toThrow("不会自动重新发送");
    await expect(consumeOwnedChatStream(stream([{ type: "completed", data: { requestId: "request", executionScope: scope, saved: false } }]), "request", () => undefined)).rejects.toThrow("尚未确认保存");
  });
});
