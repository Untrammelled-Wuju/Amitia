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

describe("项目中首次对话的保存确认", () => {
  let scope: any;
  beforeEach(() => {
    vi.clearAllMocks();
    scope = { coreId: "core", roleId: "role", roleRevision: 1, resourceOwnerId: "source", targetDeviceId: "source", modeRevision: 1 };
    mocks.owned = { enabled: ref(true), coreId: ref("core"), refresh: vi.fn().mockResolvedValue(true), data: vi.fn().mockResolvedValue({ executionScope: scope }), send: mocks.send, query: mocks.query, edit: mocks.edit, previousSummary: vi.fn() };
    mocks.workspace = ref({ projectId: ownedProjectReference("source", "project"), workspaceId: "workspace-a", primaryWorkspaceId: "workspace-a", workspaces: [{ id: "workspace-a", deviceId: "source", name: "A", kind: "local", rootUri: "amitia://workspace/@workspace-a/" }], executionScope: scope });
    mocks.send.mockResolvedValue({ conversationId: "conversation", requestId: "request", executionScope: scope, saved: true, reply: "回答" });
    mocks.query.mockResolvedValue({});
    mocks.edit.mockResolvedValue({});
  });

  function sender() {
    const messages = ref<any[]>([]);
    const convId = ref("");
    const modelError = ref("");
    const onCreated = vi.fn();
    const send = useWebChatSend(messages, convId, ref("role"), ref(false), modelError, ref(false), ref(null), ref(null), ref(null), ref(null), ref(null), vi.fn(), vi.fn(), ref(null), undefined, undefined, onCreated);
    return { send, messages, convId, modelError, onCreated };
  }

  it("发送时携带发起项目的 projectId，且后续界面切换不改写本次请求", async () => {
    let finish!: (response: any) => void;
    mocks.send.mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    const state = sender();
    const pending = state.send.doActualSend("开始对话");
    await vi.waitFor(() => expect(finish).toBeTypeOf("function"));
    expect(mocks.send.mock.calls[0][0]).toMatchObject({ projectId: "project", characterId: "role" });
    mocks.workspace.value = { projectId: ownedProjectReference("source", "other"), workspaceId: "workspace-b", primaryWorkspaceId: "workspace-b", workspaces: [], executionScope: { ...scope, modeRevision: 2 } };
    finish({ conversationId: "conversation", requestId: "request", executionScope: scope, saved: true, reply: "回答" });
    await pending;
    expect(mocks.query).not.toHaveBeenCalled();
    expect(mocks.edit).not.toHaveBeenCalled();
    expect(state.onCreated).toHaveBeenCalledWith("conversation");
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("项目数据归属与当前执行范围不一致时拒绝发送", async () => {
    mocks.workspace.value = { projectId: ownedProjectReference("other", "project"), workspaceId: "workspace-a", primaryWorkspaceId: "workspace-a", workspaces: [], executionScope: scope };
    const state = sender();
    await state.send.doActualSend("开始对话");
    expect(mocks.send).not.toHaveBeenCalled();
    expect(state.modelError.value).toBe("项目数据归属已变化，请重新选择项目");
    expect(state.messages.value.at(-1)?.status).toBe("interrupted");
  });

  it("项目内首次回答失败时不会伪造成功状态", async () => {
    mocks.send.mockRejectedValue(new Error("首次回答失败"));
    const state = sender();
    await state.send.doActualSend("开始对话");
    expect(state.convId.value).toBe("");
    expect(state.messages.value.find((message) => message.role === "user")?.status).toBe("failed");
    expect(state.messages.value.find((message) => message.role === "assistant")?.status).toBe("interrupted");
    expect(mocks.edit).not.toHaveBeenCalled();
  });
});
