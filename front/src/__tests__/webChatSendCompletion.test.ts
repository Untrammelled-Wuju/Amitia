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
});
