import { beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import { flushPromises } from "@vue/test-utils";
const mocks = vi.hoisted(() => ({ refresh: vi.fn(), data: vi.fn(), send: vi.fn(), warning: vi.fn() }));
vi.mock("@/composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => ({ enabled: { value: true }, coreId: { value: "core" }, notice: { value: "" }, refresh: mocks.refresh, data: mocks.data, send: mocks.send }) }));
vi.mock("@/composables/useConversationWorkspace", () => ({ useConversationWorkspace: () => ({ currentWorkspace: { value: null } }) }));
vi.mock("@/composables/useChatAppearancePreference", () => ({ useChatAppearancePreference: () => ({ messageStyle: { value: "flow" } }) }));
vi.mock("@/runtime/desktop-pet-chat-state", () => ({ notifyDesktopPetChatState: vi.fn() }));
vi.mock("@/runtime/owned-chat-management", () => ({ ownedChatRuntimeRequest: vi.fn() }));
vi.mock("element-plus", () => ({ ElMessage: { warning: mocks.warning, error: vi.fn() }, ElMessageBox: {} }));
import { useWebChatSend } from "@/composables/useWebChatSend";

describe("saved reply completion", () => {
  beforeEach(() => vi.clearAllMocks());
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
