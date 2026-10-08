import { beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import { useWebChatSend } from "../composables/useWebChatSend";
import { ownedProjectReference } from "../runtime/owned-project-reference";

const mocks = vi.hoisted(() => ({ send: vi.fn(), query: vi.fn(), edit: vi.fn(), post: vi.fn(), warning: vi.fn(), owned: null as any, workspace: null as any }));
vi.mock("../composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => mocks.owned }));
vi.mock("../composables/useConversationWorkspace", () => ({ useConversationWorkspace: () => ({ currentWorkspace: mocks.workspace, getWorkspaceRequestFields: () => ({}) }) }));
vi.mock("../composables/useApi", () => ({ useApi: () => ({ post: mocks.post, del: vi.fn() }) }));
vi.mock("element-plus", () => ({ ElMessage: { warning: mocks.warning, error: vi.fn(), success: vi.fn() }, ElMessageBox: { confirm: vi.fn() } }));
vi.mock("../runtime/desktop-pet-chat-state", () => ({ notifyDesktopPetChatState: vi.fn() }));

describe("逻辑项目中首次对话的保存确认", () => {
  let scope: any;
  beforeEach(() => {
    vi.clearAllMocks();
    scope = { coreId: "core", roleId: "role", roleRevision: 1, resourceOwnerId: "source", modeRevision: 1 };
    mocks.owned = { enabled: ref(true), coreId: ref("core"), refresh: vi.fn().mockResolvedValue(true), data: vi.fn().mockResolvedValue({ executionScope: scope }), send: mocks.send, query: mocks.query, edit: mocks.edit, previousSummary: vi.fn() };
    mocks.workspace = ref({ projectId: ownedProjectReference("source", "group"), workspaceKind: "logical", executionScope: scope });
    mocks.send.mockResolvedValue({ conversationId: "conversation", requestId: "request", executionScope: scope, saved: true, reply: "回答" });
    mocks.query.mockResolvedValue({});
    mocks.edit.mockResolvedValue({});
  });
  function sender() {
    const messages = ref<any[]>([]);
    const convId = ref("");
    const onCreated = vi.fn();
    const send = useWebChatSend(messages, convId, ref("role"), ref(false), ref(""), ref(false), ref(null), ref(null), ref(null), ref(null), ref(null), vi.fn(), vi.fn(), ref(null), undefined, undefined, onCreated);
    return { send, messages, convId, onCreated };
  }

  it("发送期间选择其他项目仍按发起时的原项目及执行归属提交", async () => {
    let finish!: (response: any) => void;
    mocks.send.mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    const state = sender();
    const pending = state.send.doActualSend("开始对话");
    await vi.waitFor(() => expect(finish).toBeTypeOf("function"));
    mocks.workspace.value = { projectId: ownedProjectReference("source", "another"), workspaceKind: "logical", executionScope: { ...scope, modeRevision: 2 } };
    finish({ conversationId: "conversation", requestId: "request", executionScope: scope, saved: true, reply: "回答" });
    await pending;
    expect(mocks.query).toHaveBeenCalledWith("conversation", "role");
    expect(mocks.edit).toHaveBeenCalledWith("conversation", "conversation", { projectId: "group" }, { characterId: "role", expectedExecutionScope: scope, expectedOwnerId: "source" });
    expect(state.onCreated).toHaveBeenCalledWith("conversation");
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("分组 ACK 失败保留已保存对话并提示，不能把对话标记为发送失败", async () => {
    mocks.edit.mockRejectedValue(new Error("项目版本已变化"));
    const state = sender();
    await state.send.doActualSend("开始对话");
    expect(state.messages.value.at(-1)).toMatchObject({ status: "completed", saved: true, content: "回答" });
    expect(state.convId.value).toBe("conversation");
    expect(mocks.warning).toHaveBeenCalledWith("项目版本已变化");
    expect(state.onCreated).toHaveBeenCalledWith("conversation");
  });

  it("首次回答所属设备已变化时拒绝将同 ID 分组写入新 Owner", async () => {
    mocks.send.mockResolvedValue({ conversationId: "conversation", requestId: "request", executionScope: { ...scope, coreId: "core-c", resourceOwnerId: "core-c" }, saved: true, reply: "回答" });
    const state = sender();
    await state.send.doActualSend("开始对话");
    expect(mocks.edit).not.toHaveBeenCalled();
    expect(mocks.query).not.toHaveBeenCalled();
    expect(mocks.warning).toHaveBeenCalledWith("项目数据所有者已变化，请重新选择项目");
    expect(state.messages.value.at(-1).saved).toBe(true);
  });
});
