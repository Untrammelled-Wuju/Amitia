import { beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import {
  hasRenderableMessageContent,
  shouldShowTurnErrorNotice,
} from "../conversation/rendering/errorNoticeProjection";
import { useWebChatSend } from "../composables/useWebChatSend";
import { chatSendDisabledReason } from "../views/web-chat/chatSendGuard";

const mocks = vi.hoisted(() => ({
  send: vi.fn(),
  query: vi.fn(),
  edit: vi.fn(),
  post: vi.fn(),
  warning: vi.fn(),
  owned: null as any,
  workspace: null as any,
}));

vi.mock("../composables/useDeviceOwnedConversation", () => ({
  useDeviceOwnedConversation: () => mocks.owned,
}));
vi.mock("../composables/useConversationWorkspace", () => ({
  useConversationWorkspace: () => ({
    currentWorkspace: mocks.workspace,
    getWorkspaceRequestFields: () => ({}),
  }),
}));
vi.mock("../composables/useApi", () => ({
  useApi: () => ({ post: mocks.post, del: vi.fn() }),
}));
vi.mock("element-plus", () => ({
  ElMessage: { warning: mocks.warning, error: vi.fn(), success: vi.fn() },
  ElMessageBox: { confirm: vi.fn() },
}));
vi.mock("../runtime/desktop-pet-chat-state", () => ({
  notifyDesktopPetChatState: vi.fn(),
}));

describe("发送失败的错误行判定", () => {
  it("助手占位没有任何内容且模型错误非空时显示错误行", () => {
    const message = {
      id: "req-1/assistant",
      role: "assistant",
      content: "",
      status: "interrupted",
    };
    expect(hasRenderableMessageContent(message)).toBe(false);
    expect(shouldShowTurnErrorNotice(message, "设备对话服务尚未就绪")).toBe(true);
  });

  it("助手已产生内容后中断时不覆盖原有内容", () => {
    const message = {
      id: "req-2/assistant",
      role: "assistant",
      content: "已经生成的部分内容",
      status: "interrupted",
    };
    expect(hasRenderableMessageContent(message)).toBe(true);
    expect(shouldShowTurnErrorNotice(message, "连接中断")).toBe(false);
  });

  it("助手中断但缺少模型错误文本时不显示错误行", () => {
    const message = { id: "req-3/assistant", role: "assistant", content: "", status: "interrupted" };
    expect(shouldShowTurnErrorNotice(message, "   ")).toBe(false);
  });

  it("助手消息状态为失败时始终显示错误行", () => {
    expect(shouldShowTurnErrorNotice({ role: "assistant", status: "failed", content: "" }, "")).toBe(true);
    expect(shouldShowTurnErrorNotice({ role: "assistant", status: "error", content: "残留内容" }, "")).toBe(true);
  });

  it("用户消息发送失败时显示错误行", () => {
    expect(shouldShowTurnErrorNotice({ role: "user", status: "failed", content: "你好" }, "")).toBe(true);
    expect(shouldShowTurnErrorNotice({ role: "user", status: "completed", content: "你好" }, "")).toBe(false);
  });

  it("带渲染块的中断消息不显示错误行", () => {
    const message = {
      role: "assistant",
      status: "interrupted",
      content: "",
      blocks: [{ kind: "tool", id: "tool-1", name: "search" }],
    };
    expect(hasRenderableMessageContent(message)).toBe(true);
    expect(shouldShowTurnErrorNotice(message, "工具执行中断")).toBe(false);
  });

  it("正常完成的消息不显示错误行", () => {
    expect(shouldShowTurnErrorNotice({ role: "assistant", status: "completed", content: "回答" }, "")).toBe(false);
    expect(shouldShowTurnErrorNotice(undefined, "错误")).toBe(false);
  });
});

describe("零角色与未配置模型时的发送禁用", () => {
  it("没有角色时禁止发送并提示先创建角色", () => {
    expect(chatSendDisabledReason(false, 0)).toBe("请先创建角色，才能发送消息。");
  });

  it("未配置模型时禁止发送并提示先配置模型", () => {
    expect(chatSendDisabledReason(true, 0)).toBe("请先配置模型，才能发送消息。");
    expect(chatSendDisabledReason(true, 3)).toBe("请先配置模型，才能发送消息。");
  });

  it("模型与角色都就绪时允许发送", () => {
    expect(chatSendDisabledReason(false, 1)).toBe("");
  });
});

describe("发送请求失败后的本地消息状态", () => {
  let scope: any;

  beforeEach(() => {
    vi.clearAllMocks();
    scope = {
      coreId: "core",
      roleId: "role",
      roleRevision: 1,
      resourceOwnerId: "source",
      targetDeviceId: "source",
      modeRevision: 1,
    };
    mocks.workspace = ref(null);
    mocks.owned = {
      enabled: ref(true),
      coreId: ref("core"),
      refresh: vi.fn().mockResolvedValue(true),
      data: vi.fn().mockResolvedValue({ executionScope: scope }),
      chatScope: (roleId: string) => ({ ...scope, roleId }),
      send: mocks.send,
      query: mocks.query,
      edit: mocks.edit,
      previousSummary: vi.fn(),
    };
  });

  function sender() {
    const messages = ref<any[]>([]);
    const convId = ref("");
    const modelError = ref("");
    const send = useWebChatSend(
      messages,
      convId,
      ref("role"),
      ref(false),
      modelError,
      ref(false),
      ref(null),
      ref(null),
      ref(null),
      ref(null),
      ref(null),
      vi.fn(),
      vi.fn(),
      ref(null),
    );
    return { send, messages, convId, modelError };
  }

  it("请求失败后用户与助手消息都标记失败并保留原始错误", async () => {
    mocks.send.mockRejectedValue(new Error("设备对话写入失败: connection refused"));
    const state = sender();
    await state.send.doActualSend("你好");
    const user = state.messages.value.find((message) => message.role === "user");
    const assistant = state.messages.value.find((message) => message.role === "assistant");
    expect(user.status).toBe("failed");
    expect(assistant.status).toBe("failed");
    expect(state.modelError.value).toBe("设备对话写入失败: connection refused");
    expect(shouldShowTurnErrorNotice(user, state.modelError.value)).toBe(true);
    expect(shouldShowTurnErrorNotice(assistant, state.modelError.value)).toBe(true);
  });

  it("缺少调用角色时拦截发送且不请求后端", async () => {
    const messages = ref<any[]>([]);
    const convId = ref("");
    const modelError = ref("");
    const send = useWebChatSend(
      messages,
      convId,
      ref(""),
      ref(false),
      modelError,
      ref(false),
      ref(null),
      ref(null),
      ref(null),
      ref(null),
      ref(null),
      vi.fn(),
      vi.fn(),
      ref(null),
    );
    await send.doActualSend("你好");
    expect(mocks.send).not.toHaveBeenCalled();
    expect(modelError.value).toBe("请选择调用角色后发送消息");
    const user = messages.value.find((message) => message.role === "user");
    expect(user.status).toBe("failed");
  });

  it("请求失败后重试入口仍能取到原始用户输入", async () => {
    mocks.send.mockRejectedValue(new Error("设备对话服务尚未就绪"));
    const state = sender();
    await state.send.doActualSend("继续上一个话题");
    const user = state.messages.value.find((message) => message.role === "user");
    expect(user.content).toBe("继续上一个话题");
    expect(user.status).toBe("failed");
  });
});
