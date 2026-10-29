import { beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import { flushPromises } from "@vue/test-utils";
const mocks = vi.hoisted(() => ({ refresh: vi.fn(), data: vi.fn(), send: vi.fn(), groupState: vi.fn(), query: vi.fn(), sendGroup: vi.fn(), warning: vi.fn() }));
vi.mock("@/composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => ({ enabled: { value: true }, coreId: { value: "core" }, notice: { value: "" }, refresh: mocks.refresh, data: mocks.data, send: mocks.send, groupState: mocks.groupState, query: mocks.query, sendGroup: mocks.sendGroup }) }));
vi.mock("@/composables/useConversationWorkspace", () => ({ useConversationWorkspace: () => ({ currentWorkspace: { value: null } }) }));
vi.mock("@/composables/useChatAppearancePreference", () => ({ useChatAppearancePreference: () => ({ messageStyle: { value: "flow" } }) }));
vi.mock("@/runtime/desktop-pet-chat-state", () => ({ notifyDesktopPetChatState: vi.fn() }));
vi.mock("@/runtime/owned-chat-management", () => ({ ownedChatRuntimeRequest: vi.fn() }));
vi.mock("element-plus", () => ({ ElMessage: { warning: mocks.warning, error: vi.fn() }, ElMessageBox: {} }));
import { useWebChatSend } from "@/composables/useWebChatSend";

describe("saved reply completion", () => {
  beforeEach(() => {
    for (const mock of Object.values(mocks)) {
      if (vi.isMockFunction(mock)) mock.mockReset();
    }
    mocks.groupState.mockResolvedValue(null);
  });
  it("群聊保存后不等待会话列表刷新即可恢复发送", async () => {
    const scope = { coreId: "core", resourceOwnerId: "owner", roleId: "role" };
    mocks.refresh.mockResolvedValue(true);
    mocks.data.mockResolvedValue({ executionScope: scope });
    mocks.groupState.mockResolvedValue({ members: [], membershipRevision: 1, settingsRevision: 1 });
    mocks.query.mockResolvedValue({ executionScope: scope });
    mocks.sendGroup.mockImplementation(async (request) => ({ requestId: request.requestId, conversationId: "conversation", groupRoundId: "round", userMessageId: `${request.requestId}/user`, saved: true, status: "completed", deliveries: [] }));
    const sending = ref(false);
    const groupSending = ref(false);
    let completeRefresh = () => {};
    const onCreated = vi.fn(() => new Promise<void>((resolve) => { completeRefresh = resolve; }));
    const send = useWebChatSend(ref([]), ref("conversation"), ref("role"), sending, ref(""), ref(false), ref(null), ref(null), ref(null), ref(null), ref(null), () => {}, () => {}, ref(null), undefined, undefined, onCreated, undefined, undefined, undefined, undefined, undefined, undefined, undefined, groupSending);
    await send.handleSend("你好");
    expect(onCreated).toHaveBeenCalledOnce();
    expect(sending.value).toBe(false);
    expect(groupSending.value).toBe(false);
    expect(send.isSubmitting.value).toBe(false);
    completeRefresh();
  });
  it("多角色讨论模式不生成用户点名，并携带讨论调度声明", async () => {
    const scope = { coreId: "core", resourceOwnerId: "owner", roleId: "role" };
    let captured: any;
    const pending = vi.fn();
    mocks.refresh.mockResolvedValue(true);
    mocks.data.mockResolvedValue({ executionScope: scope });
    const group = { members: [{ characterId: "role", characterName: "默认角色", characterAvatar: "avatar-default" }, { characterId: "role-two", characterName: "第二角色", characterAvatar: "avatar-two" }], membershipRevision: 1, settingsRevision: 2, groupSettings: { allowCharacterMentions: true } };
    mocks.groupState.mockResolvedValue(group);
    mocks.query.mockResolvedValue({ executionScope: scope });
    mocks.sendGroup.mockImplementation(async (request, onEvent) => {
      captured = request;
      onEvent({ type: "group.delivery.started", conversationId: "conversation", groupRoundId: "round", deliveryId: "delivery-two", childRequestId: `${request.requestId}/reply/two`, deliveryIndex: 1, characterId: "role-two", characterName: "第二角色" });
      return { requestId: request.requestId, conversationId: "conversation", groupRoundId: "round", userMessageId: `${request.requestId}/user`, saved: true, status: "completed", deliveries: [{ requestId: `${request.requestId}/reply/two`, conversationId: "conversation", groupRoundId: "round", deliveryId: "delivery-two", characterName: "第二角色", reply: "第二角色回复", saved: true, executionScope: { ...scope, roleId: "role-two" } }] };
    });
    const groupDiscussion = ref(true);
    const messages = ref<any[]>([]);
    const send = useWebChatSend(messages, ref("conversation"), ref("role"), ref(false), ref(""), ref(false), ref(null), ref(null), ref(null), ref(null), ref(null), () => {}, () => {}, ref(null), undefined, undefined, undefined, undefined, undefined, undefined, undefined, undefined, pending, undefined, ref(false), groupDiscussion, ref(group as any));
    await send.handleSend("讨论小商家服务方案");
    expect(captured.group.mode).toBe("discussion");
    expect(captured.mentions).toEqual([]);
    expect(pending).not.toHaveBeenCalled();
    expect(messages.value.filter(row => row.role === "assistant").every(row => row.characterAvatar === "avatar-two")).toBe(true);
  });
  it("creates a pending assistant before waiting for service initialization", async () => {
    let finish: (value: boolean) => void = () => {};
    mocks.refresh.mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
    const pending = vi.fn();
    const failed = vi.fn();
    const send = useWebChatSend(ref([]), ref("conversation"), ref("role"), ref(false), ref(""), ref(false), ref(null), ref(null), ref(null), ref(null), ref(null), () => {}, () => {}, ref(null), undefined, undefined, undefined, undefined, undefined, undefined, undefined, undefined, pending, failed);
    const sending = send.handleSend("你好");
    expect(pending).toHaveBeenCalledTimes(1);
    expect(mocks.send).not.toHaveBeenCalled();
    finish(false);
    await sending;
    expect(failed).toHaveBeenCalled();
  });
  it("creates the pending assistant immediately in an existing single-role owned conversation", async () => {
    let finish: (value: boolean) => void = () => {};
    mocks.refresh.mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
    const pending = vi.fn();
    const failed = vi.fn();
    const messages = ref<any[]>([]);
    const send = useWebChatSend(messages, ref("meshconv1:space:conversation"), ref("role"), ref(false), ref(""), ref(false), ref(null), ref(null), ref(null), ref(null), ref(null), () => {}, () => {}, ref(null), undefined, undefined, undefined, undefined, undefined, undefined, undefined, undefined, pending, failed, ref(false), ref(false), ref(null));
    const sending = send.handleSend("你好");
    expect(pending).toHaveBeenCalledTimes(1);
    expect(messages.value.map(message => message.role)).toEqual(["user", "assistant"]);
    expect(messages.value[1]).toMatchObject({ status: "queued", characterId: "role", uiKey: expect.stringMatching(/^request:/) });
    finish(false);
    await sending;
    expect(failed).toHaveBeenCalled();
    expect(messages.value[1].status).toBe("interrupted");
  });
  it("keeps a saved reply completed when sidebar refresh fails", async () => {
    const scope = { coreId: "core", resourceOwnerId: "owner", roleId: "role" };
    mocks.refresh.mockResolvedValue(true);
    mocks.data.mockResolvedValue({ executionScope: scope });
    mocks.send.mockImplementation(async (request, onEvent) => {
      onEvent({ type: "started", conversationId: "conversation", executionScope: scope });
      return { requestId: request.requestId, conversationId: "conversation", executionScope: scope, reply: "成功回复", saved: true };
    });
    const messages = ref<any[]>([]);
    const sending = ref(false);
    const error = ref("");
    const failPending = vi.fn();
    const send = useWebChatSend(messages, ref("conversation"), ref("role"), sending, error, ref(false), ref(null), ref(null), ref(null), ref(null), ref(null), () => {}, () => {}, ref(null), undefined, undefined, async () => { throw new Error("sidebar unavailable"); }, undefined, undefined, undefined, undefined, undefined, undefined, failPending);
    await send.handleSend("你好");
    expect(messages.value.find(row => row.role === "assistant")).toMatchObject({ content: "成功回复", status: "completed", saved: true });
    expect(messages.value.find(row => row.role === "user").status).toBe("completed");
    expect(failPending).not.toHaveBeenCalled();
    expect(error.value).toBe("");
    expect(sending.value).toBe(false);
    expect(mocks.warning).toHaveBeenCalledOnce();
  });
  it("reuses the immediate assistant placeholder when the stream starts", async () => {
    const scope = { coreId: "core", resourceOwnerId: "owner", roleId: "role" };
    const messages = ref<any[]>([]);
    mocks.refresh.mockResolvedValue(true);
    mocks.data.mockResolvedValue({ executionScope: scope });
    mocks.send.mockImplementation(async (request, onEvent) => {
      const placeholder = messages.value.find(row => row.role === "assistant");
      expect(placeholder.status).toBe("queued");
      onEvent({ type: "started", conversationId: "conversation", executionScope: scope });
      const assistants = messages.value.filter(row => row.role === "assistant");
      expect(assistants).toHaveLength(1);
      expect(assistants[0]).toMatchObject({ id: placeholder.id, uiKey: placeholder.uiKey, createdAt: placeholder.createdAt, status: "streaming" });
      onEvent({ type: "delta", conversationId: "conversation", executionScope: scope, text: "你好" });
      return { requestId: request.requestId, conversationId: "conversation", executionScope: scope, reply: "你好", saved: true };
    });
    const send = useWebChatSend(messages, ref("conversation"), ref("role"), ref(false), ref(""), ref(false), ref(null), ref(null), ref(null), ref(null), ref(null), () => {}, () => {}, ref(null));
    await send.handleSend("你好");
    expect(messages.value.filter(row => row.role === "assistant")).toHaveLength(1);
    expect(messages.value.find(row => row.role === "assistant")).toMatchObject({ content: "你好", status: "completed" });
  });
  it("does not interrupt a reply while tools start and complete", async () => {
    const scope = { coreId: "core", resourceOwnerId: "owner", roleId: "role" };
    mocks.refresh.mockResolvedValue(true);
    mocks.data.mockResolvedValue({ executionScope: scope });
    mocks.send.mockImplementation(async (request, onEvent) => {
      onEvent({ type: "started", conversationId: "conversation", executionScope: scope });
      onEvent({ type: "tool-started", conversationId: "conversation", executionScope: scope, progress: { toolCallId: "call-1", toolName: "calculate", arguments: "{\"expression\":\"1+1\"}", status: "", result: "", durationMs: 0 } });
      onEvent({ type: "tool-completed", conversationId: "conversation", executionScope: scope, progress: { toolCallId: "call-1", toolName: "calculate", arguments: "{\"expression\":\"1+1\"}", status: "success", result: "{\"result\":2}", durationMs: 1 } });
      onEvent({ type: "delta", conversationId: "conversation", executionScope: scope, text: "计算完成" });
      return { requestId: request.requestId, conversationId: "conversation", executionScope: scope, reply: "计算完成", saved: true };
    });
    const messages = ref<any[]>([]);
    const sending = ref(false);
    const error = ref("");
    const failPending = vi.fn();
    const refresh = vi.fn();
    window.addEventListener("amitia:owned-runtime-refresh", refresh);
    const send = useWebChatSend(messages, ref(""), ref("role"), sending, error, ref(false), ref(null), ref(null), ref(null), ref(null), ref(null), () => {}, () => {}, ref(null), undefined, undefined, undefined, undefined, undefined, undefined, undefined, undefined, undefined, failPending);
    await send.handleSend("调用工具");
    expect(mocks.send).toHaveBeenCalledOnce();
    expect(messages.value.find(row => row.role === "assistant")).toMatchObject({ content: "计算完成", status: "completed", saved: true });
    expect(failPending).not.toHaveBeenCalled();
    expect(error.value).toBe("");
    expect(sending.value).toBe(false);
    expect(refresh).toHaveBeenCalledOnce();
    expect((refresh.mock.calls[0][0] as CustomEvent).detail).toEqual({ conversationId: "conversation" });
    window.removeEventListener("amitia:owned-runtime-refresh", refresh);
  });
});
