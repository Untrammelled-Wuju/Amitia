import { beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import type { AgentUIEvent } from "@/conversation/runtime/agentEventReducer";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  snapshot: { value: null as any },
}));

vi.mock("@/composables/useApi", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
  useApi: () => ({ get: mocks.get }),
}));

vi.mock("@/runtime/runtime-adapter", () => ({
  resolveApiUrl: vi.fn(async (value: string) => value),
  getDeploymentConfig: vi.fn(async () => ({ mode: "local" })),
  getNativeProviderTransition: vi.fn(async () => null),
}));

vi.mock("@/runtime/request-auth", () => ({
  createAuthenticatedFetchInit: vi.fn(async () => ({})),
}));

vi.mock("@/runtime/desktop-pet-chat-state", () => ({
  notifyDesktopPetChatState: vi.fn(),
}));

import { useConversationRuntime } from "@/composables/useConversationRuntime";

const executionScope = {
  spaceId: "space-1",
  coreId: "core-1",
  authorizationRealm: "space-1",
  initiatorDeviceId: "device-1",
  targetDeviceId: "device-1",
  resourceOwnerId: "device-1",
  roleId: "role-1",
  roleRevision: 1,
  roleOwnerId: "device-1",
  providerEpoch: 1,
  targetProviderEpoch: 1,
  coordinated: false,
  modeRevision: 1,
  permissionRevision: 1,
  targetPermissionRevision: 1,
  requestId: "request-1",
  turnId: "",
  executionId: "",
  memoryRoleId: "role-1",
};

function event(
  eventSequence: number,
  type: string,
  overrides: Partial<AgentUIEvent> = {},
): AgentUIEvent {
  return {
    version: 1,
    eventId: `event-${eventSequence}`,
    eventSequence,
    conversationId: "conversation-1",
    requestId: "request-1",
    executionId: "execution-1",
    turnId: "turn-1",
    turnSequence: 1,
    type,
    createdAt: `2026-09-21T00:00:0${eventSequence}Z`,
    ...overrides,
  };
}

describe("conversation runtime projection", () => {
  beforeEach(() => {
    mocks.snapshot.value = null;
    mocks.get.mockReset();
    mocks.post.mockReset();
    mocks.get.mockImplementation(async (path: string) => {
      if (path.endsWith("/coordination/me")) {
        return { data: { coreId: "core-1", coordinationAvailable: true, policy: { selectedRole: "role-1", coordinated: false, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } } };
      }
      if (path.endsWith("/business/roles")) {
        return { data: { roles: [{ id: "role-1", name: "角色", revision: 1 }], roleOwnerId: "device-1", executionScope } };
      }
      if (path.endsWith("/business/data")) {
        return { data: { conversationId: "conversation-1", executionScope, snapshot: { ownerId: "device-1", resources: [] } } };
      }
      return { data: {} };
    });
    mocks.post.mockImplementation(async (path: string, payload: any) => {
      if (path.endsWith("/chat-runtime/snapshot")) {
        const response = await (mocks.snapshot.value || { version: 1, revision: 1, lastEventSequence: 0, messages: [], turns: [] });
        return { data: { ...response, executionScope: response.executionScope ?? payload.expectedExecutionScope } };
      }
      return { data: { executionScope: payload.expectedExecutionScope } };
    });
  });

  it("keeps the assistant row key when a snapshot has the message but no turn", async () => {
    const persisted = ref<any[]>([]);
    const runtime = useConversationRuntime(ref("conversation-1"), persisted, ref(false), () => undefined);
    runtime.beginPendingAssistant("request-1", "role-1");
    expect(runtime.messages.value.find(row => row.role === "assistant")?.uiKey).toBe("request:request-1");
    mocks.snapshot.value = { version: 1, lastEventSequence: 0, turns: [], messages: [
      { id: "request-1/assistant", requestId: "request-1", role: "assistant", content: "完成", conversationId: "conversation-1" },
    ] };
    await runtime.loadSnapshot(false);
    expect(runtime.messages.value.filter(row => row.role === "assistant")).toHaveLength(1);
    expect(runtime.messages.value.find(row => row.role === "assistant")?.uiKey).toBe("request:request-1");
    runtime.cleanup();
  });

  it("publishes the user message and assistant thinking placeholder to the visible list synchronously", () => {
    const persisted = ref<any[]>([]);
    const runtime = useConversationRuntime(ref("conversation-1"), persisted, ref(true), () => undefined);
    persisted.value.push({ id: "client:request-1", uiKey: "client:request-1", requestId: "request-1", role: "user", content: "你好", status: "sending", createdAt: "2026-10-28T00:00:00Z" });
    persisted.value.push({ id: "request-1/assistant", uiKey: "request:request-1", requestId: "request-1", role: "assistant", characterId: "role-1", content: "", status: "queued", createdAt: "2026-10-28T00:00:00Z" });
    runtime.beginPendingAssistant("request-1", "role-1");
    expect(runtime.messages.value.map((row) => row.role)).toEqual(["user", "assistant"]);
    expect(runtime.messages.value[1]).toMatchObject({ uiKey: "request:request-1", characterId: "role-1", status: "queued", assistantTurn: { status: "queued" } });
    runtime.cleanup();
  });

  it("keeps a completed turn visible while its saved message arrives before the turn snapshot", async () => {
    const runtime = useConversationRuntime(ref("conversation-1"), ref<any[]>([]), ref(false), () => undefined);
    await runtime.applyEvent(event(1, "turn.started"));
    await runtime.applyEvent(event(2, "text.completed", { blockId: "text-1", blockSequence: 1, messageId: "request-1/assistant", payload: { content: "完成回复" } }));
    mocks.snapshot.value = { version: 1, lastEventSequence: 3, turns: [], messages: [
      { id: "request-1/assistant", requestId: "request-1", role: "assistant", content: "完成回复", conversationId: "conversation-1" },
    ] };
    await runtime.applyEvent(event(3, "turn.completed", { messageId: "request-1/assistant" }));
    const row = runtime.messages.value.find(message => message.role === "assistant");
    expect(row?.uiKey).toBe("request:request-1");
    expect(row?.assistantTurn?.items).toMatchObject([{ type: "text", content: "完成回复" }]);
    runtime.cleanup();
  });

  it("保留并发群聊流消息直到对应快照出现", async () => {
    const sending = ref(true);
    const groupSending = ref(true);
    const persisted = ref<any[]>([
      { id: "reply-a", role: "assistant", requestId: "request-a", groupRoundId: "round", deliveryId: "a", deliveryIndex: 0, content: "甲的流内容", status: "streaming" },
      { id: "reply-b", role: "assistant", requestId: "request-b", groupRoundId: "round", deliveryId: "b", deliveryIndex: 1, content: "乙的流内容", status: "streaming" },
    ]);
    const runtime = useConversationRuntime(ref("conversation-1"), persisted, sending, () => undefined, undefined, undefined, groupSending);
    mocks.snapshot.value = { version: 1, lastEventSequence: 0, messages: [], turns: [] };
    await runtime.loadSnapshot(false);
    expect(runtime.messages.value.map((row) => row.content)).toEqual(["甲的流内容", "乙的流内容"]);
    expect(sending.value).toBe(true);
    mocks.snapshot.value = { version: 1, lastEventSequence: 0, messages: [{ ...persisted.value[0], content: "旧内容" }], turns: [] };
    await runtime.loadSnapshot(false);
    expect(runtime.messages.value.map((row) => row.content)).toEqual(["甲的流内容", "乙的流内容"]);
    expect(runtime.messages.value.map((row) => row.uiKey)).toEqual(["group-delivery:a", "group-delivery:b"]);
    groupSending.value = false;
    mocks.snapshot.value = { version: 1, lastEventSequence: 0, messages: [], turns: [] };
    await runtime.loadSnapshot(false);
    expect(runtime.messages.value).toHaveLength(0);
    runtime.cleanup();
  });

  it("投递占位和已保存快照属于同一角色回复", async () => {
    const groupSending = ref(true);
    const persisted = ref<any[]>([{ id: "group-delivery:a", uiKey: "group-delivery:a", role: "assistant", groupRoundId: "round", deliveryId: "a", status: "streaming", content: "" }]);
    const runtime = useConversationRuntime(ref("conversation-1"), persisted, ref(true), () => undefined, undefined, undefined, groupSending);
    mocks.snapshot.value = { version: 1, lastEventSequence: 0, turns: [], messages: [{ id: "child-a/assistant", requestId: "child-a", role: "assistant", groupRoundId: "round", deliveryId: "a", status: "completed", content: "甲回复" }] };
    await runtime.loadSnapshot(false);
    expect(runtime.messages.value.filter((row) => row.deliveryId === "a")).toHaveLength(1);
    runtime.cleanup();
  });

  it("投递占位与运行时事件只显示一条回复", async () => {
    const persisted = ref<any[]>([{ id: "child-a/assistant", requestId: "child-a", role: "assistant", groupRoundId: "round", deliveryId: "a", status: "streaming", content: "" }]);
    const runtime = useConversationRuntime(ref("conversation-1"), persisted, ref(true), () => undefined, undefined, undefined, ref(true));
    await runtime.applyEvent(event(1, "turn.started", { requestId: "child-a", turnId: "turn-a" }));
    await new Promise((resolve) => setTimeout(resolve, 30));
    expect(runtime.messages.value.filter((row) => row.role === "assistant")).toHaveLength(1);
    expect(runtime.messages.value[0].uiKey).toBe("group-delivery:a");
    runtime.cleanup();
  });

  it("群聊结束后旧快照和迟到成员事件不会重新占用发送状态", async () => {
    const sending = ref(false);
    const groupSending = ref(true);
    const persisted = ref<any[]>([{ id: "child-a/assistant", requestId: "child-a", role: "assistant", groupRoundId: "round", deliveryId: "a", status: "completed", content: "完成" }]);
    const runtime = useConversationRuntime(ref("conversation-1"), persisted, sending, () => undefined, undefined, undefined, groupSending);
    mocks.snapshot.value = { version: 1, lastEventSequence: 0, messages: persisted.value, turns: [] };
    await runtime.loadSnapshot(false);
    expect(sending.value).toBe(true);
    groupSending.value = false;
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(sending.value).toBe(false);
    await runtime.applyEvent(event(1, "turn.started", { requestId: "child-a", turnId: "turn-a" }));
    expect(sending.value).toBe(false);
    runtime.cleanup();
  });

  it("群聊流式回复不会切换到另一套运行时渲染", async () => {
    const persisted = ref<any[]>([{ id: "child-a/assistant", requestId: "child-a", role: "assistant", groupRoundId: "round", deliveryId: "a", status: "streaming", content: "流式正文" }]);
    const runtime = useConversationRuntime(ref("conversation-1"), persisted, ref(true), () => undefined, undefined, undefined, ref(true));
    await runtime.applyEvent(event(1, "turn.started", { requestId: "child-a", turnId: "turn-a" }));
    await new Promise((resolve) => setTimeout(resolve, 30));
    expect(runtime.messages.value[0].content).toBe("流式正文");
    expect(runtime.messages.value[0].assistantTurn).toBeUndefined();
    runtime.cleanup();
  });

  it("clears stale message state after receiving completed history", async () => {
    const persisted = ref<any[]>([{ id: "message-1", requestId: "request-1", role: "assistant", content: "成功回复", status: "interrupted", state: "interrupted" }]);
    const runtime = useConversationRuntime(ref("conversation-1"), persisted, ref(false), () => undefined);
    mocks.snapshot.value = { version: 1, lastEventSequence: 2, messages: persisted.value, turns: [{ id: "turn-1", requestId: "request-1", executionId: "execution-1", conversationId: "conversation-1", status: "completed", items: [] }] };
    await runtime.loadSnapshot();
    expect(runtime.messages.value[0]).toMatchObject({ status: "completed", state: "completed" });
    runtime.cleanup();
  });
  it("projects completed citations before and after snapshot refresh", async () => {
    const persistedMessages = ref<any[]>([]);
    const runtime = useConversationRuntime(ref("conversation-1"), persistedMessages, ref(false), () => undefined);
    const snapshot = [
      { marker: "[K1]", kind: "knowledge", index: 1, title: "文档", sourceId: "doc-1" },
      { marker: "[1]", kind: "web", index: 1, title: "网页", url: "https://example.com/" },
    ];
    await runtime.applyEvent(event(1, "turn.started"));
    await runtime.applyEvent(event(2, "text.completed", {
      blockId: "text-1", blockSequence: 1, messageId: "message-1", revision: 2,
      payload: { blockType: "text", content: "文档 [K1] 网页 [1]", citationSnapshot: snapshot },
    }));
    await new Promise((resolve) => setTimeout(resolve, 40));
    const live = runtime.messages.value.find((row) => row.role === "assistant");
    expect(live?.citationSnapshot).toMatchObject(snapshot);
    expect(live?.id).toBe("message-1");
    mocks.snapshot.value = {
      version: 1, revision: 1, lastEventSequence: 2, messages: [
        { id: "message-1", role: "assistant", conversationId: "conversation-1", content: "文档 [K1] 网页 [1]", citationSnapshot: snapshot },
      ],
      turns: [{ id: "turn-1", conversationId: "conversation-1", requestId: "request-1", sequence: 1, status: "completed", items: [
        { id: "text-1", type: "text", status: "completed", sequence: 1, revision: 2, messageId: "message-1", content: "文档 [K1] 网页 [1]" },
      ] }],
    };
    await runtime.loadSnapshot(false);
    const historic = runtime.messages.value.find((row) => row.role === "assistant");
    expect(historic?.citationSnapshot).toMatchObject(snapshot);
    expect(historic?.uiKey).toBe(live?.uiKey);
    runtime.cleanup();
  });

  it("holds an unconfirmed side effect without a false success notification or active sending spinner", async () => {
    const sending = ref(false);
    const runtime = useConversationRuntime(ref("conversation-1"), ref<any[]>([]), sending, () => undefined);
    await runtime.applyEvent(event(1, "turn.started", { status: "running" }));
    expect(sending.value).toBe(true);
    await runtime.applyEvent(event(2, "turn.waiting", {
      status: "needs_reconciliation",
      payload: { errorCode: "tool_requires_reconciliation", recoveryCheckpoint: true },
    }));
    expect(sending.value).toBe(false);
    expect(runtime.activeTurnId.value).toBe("turn-1");
    runtime.cleanup();
  });
  it("updates automation indication from live tool events and clears on completion or navigation", async () => {
    const runtime = useConversationRuntime(ref("conversation-1"), ref<any[]>([]), ref(false), () => undefined);
    await runtime.applyEvent(event(1, "turn.started", { status: "running" }));
    await runtime.applyEvent(event(2, "tool.running", {
      blockId: "automation", blockSequence: 1, revision: 1, status: "running",
      payload: { toolName: "browser_interact_click" },
    }));
    expect(runtime.automationStatus.value?.label).toBe("正在点击");
    await runtime.applyEvent(event(3, "tool.completed", {
      blockId: "automation", blockSequence: 1, revision: 2, status: "completed",
    }));
    expect(runtime.automationStatus.value).toBeNull();
    await runtime.applyEvent(event(4, "tool.running", {
      blockId: "next", blockSequence: 2, revision: 1, status: "running",
      payload: { toolName: "android_interaction_screenshot" },
    }));
    expect(runtime.automationStatus.value?.phase).toBe("observing");
    runtime.disconnect();
    expect(runtime.automationStatus.value).toBeNull();
    runtime.clear();
    runtime.cleanup();
  });
  it("keeps the live message identity stable until persistence replaces it", async () => {
    const conversationId = ref("conversation-1");
    const persistedMessages = ref<any[]>([
      {
        id: "user-1",
        role: "user",
        content: "hello",
        conversationId: "conversation-1",
        createdAt: "2026-09-21T00:00:00Z",
      },
    ]);
    const sending = ref(false);
    const runtime = useConversationRuntime(
      conversationId,
      persistedMessages,
      sending,
      () => undefined,
    );
    const messages = runtime.messages;
    const firstUser = messages.value.find((message) => message.role === "user");
    runtime.beginPendingAssistant("request-1", "character-1");
    const pendingAssistant = messages.value.find((message) => message.uiKey === "request:request-1");
    expect(pendingAssistant?.assistantTurn?.status).toBe("queued");
    expect(pendingAssistant?.assistantTurn?.items).toHaveLength(0);
    expect(pendingAssistant?.characterId).toBe("character-1");

    await runtime.applyEvent(event(1, "turn.queued", { status: "queued" }));
    await runtime.applyEvent(event(2, "turn.started", { status: "running" }));
    await runtime.applyEvent(event(3, "text.started", {
      blockId: "block-1",
      blockSequence: 1,
      revision: 1,
      status: "running",
      payload: { blockType: "text" },
    }));
    await runtime.applyEvent(event(4, "text.delta", {
      blockId: "block-1",
      blockSequence: 1,
      revision: 2,
      status: "running",
      payload: { blockType: "text", delta: "first" },
    }));
    await new Promise((resolve) => setTimeout(resolve, 40));

    const firstLive = messages.value.find((message) => message.uiKey === "request:request-1");
    expect(firstLive).toBeTruthy();

    await runtime.applyEvent(event(5, "text.delta", {
      blockId: "block-1",
      blockSequence: 1,
      revision: 3,
      status: "running",
      payload: { blockType: "text", delta: " second" },
    }));
    await new Promise((resolve) => setTimeout(resolve, 40));

    const secondLive = messages.value.find((message) => message.uiKey === "request:request-1");
    expect(secondLive).toBeTruthy();
    expect(secondLive.content).toBe("first second");
    expect(secondLive).not.toBe(firstLive);
    expect(messages.value.find((message) => message.role === "user")).toBe(firstUser);

    let resolveSnapshot: (value: any) => void = () => undefined;
    mocks.snapshot.value = new Promise((resolve) => {
        resolveSnapshot = resolve;
      });

    await runtime.applyEvent(event(6, "text.completed", {
      blockId: "block-1",
      blockSequence: 1,
      messageId: "assistant-1",
      messageSequence: 2,
      revision: 4,
      status: "completed",
      payload: { blockType: "text", content: "first second" },
    }));
    const terminal = runtime.applyEvent(event(7, "turn.completed", {
      messageId: "assistant-1",
      messageSequence: 2,
      status: "completed",
      payload: { messageIds: ["assistant-1"] },
    }));
    await new Promise((resolve) => setTimeout(resolve, 40));

    expect(messages.value.filter((message) => message.role === "assistant")).toHaveLength(1);
    expect(messages.value.find((message) => message.role === "assistant")?.uiKey).toBe("request:request-1");

    resolveSnapshot({
      version: 1,
      revision: 1,
      lastEventSequence: 7,
      conversation: { id: "conversation-1" },
      messages: [
        {
          id: "user-1",
          role: "user",
          content: "hello",
          conversationId: "conversation-1",
          createdAt: "2026-09-21T00:00:00Z",
        },
        {
          id: "assistant-1",
          role: "assistant",
          content: "first second",
          conversationId: "conversation-1",
          createdAt: "2026-09-21T00:00:07Z",
        },
      ],
      turns: [
        {
          id: "turn-1",
          conversationId: "conversation-1",
          requestId: "request-1",
          executionId: "execution-1",
          sequence: 1,
          status: "completed",
          createdAt: "2026-09-21T00:00:00Z",
          updatedAt: "2026-09-21T00:00:07Z",
          items: [
            {
              id: "block-1",
              turnId: "turn-1",
              conversationId: "conversation-1",
              sequence: 1,
              type: "text",
              status: "completed",
              revision: 4,
              content: "first second",
              messageId: "assistant-1",
              isFinal: 1,
            },
          ],
        },
      ],
      activeTurn: null,
      approvals: [],
      messageHistory: { hasMore: false, nextBefore: 0 },
      turnHistory: { hasMore: false, nextBefore: 1 },
    });
    await terminal;
    await new Promise((resolve) => setTimeout(resolve, 40));

    const assistantMessages = messages.value.filter((message) => message.role === "assistant");
    expect(assistantMessages).toHaveLength(1);
    expect(assistantMessages[0].id).toBe("assistant-1");
    expect(assistantMessages[0].uiKey).toBe("request:request-1");
  });

  it("removes the pending assistant when sending fails", () => {
    const conversationId = ref("conversation-1");
    const persistedMessages = ref<any[]>([]);
    const sending = ref(false);
    const runtime = useConversationRuntime(
      conversationId,
      persistedMessages,
      sending,
      () => undefined,
    );

    runtime.beginPendingAssistant("request-failed");
    expect(runtime.messages.value.some((message) => message.uiKey === "request:request-failed")).toBe(true);

    runtime.failPendingAssistant("request-failed");
    expect(runtime.messages.value.some((message) => message.uiKey === "request:request-failed")).toBe(false);
  });
});
