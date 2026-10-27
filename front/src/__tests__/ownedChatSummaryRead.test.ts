import { beforeEach, describe, expect, it, vi } from "vitest";
import { effectScope, nextTick, ref } from "vue";
import { useWebChatConversation } from "../composables/useWebChatConversation";
const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  summary: vi.fn(),
  enabled: { value: true },
  query: vi.fn(),
  messages: vi.fn(),
  hasMore: vi.fn(),
  markConversationRead: vi.fn(),
  applySnapshotWorkspace: vi.fn(),
  sidebar: { pinned: [], recent: [], projects: [] } as any,
}));
vi.mock("../composables/useApi", () => ({ useApi: () => ({ get: mocks.get, del: vi.fn() }) }));
vi.mock("../composables/useCachedApi", () => ({ useCachedApi: () => ({ saveCache: vi.fn() }) }));
vi.mock("../composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => ({ enabled: mocks.enabled, coreId: { value: "core" }, roleOwnerId: { value: "owner" }, notice: { value: "" }, conversationSummary: mocks.summary, query: mocks.query, messages: mocks.messages, hasMore: mocks.hasMore }) }));
vi.mock("../stores/chat", () => ({ useChatStore: () => ({ markConversationRead: mocks.markConversationRead, sidebar: mocks.sidebar }) }));
vi.mock("../composables/useConversationWorkspace", () => ({ useConversationWorkspace: () => ({ applySnapshotWorkspace: mocks.applySnapshotWorkspace }) }));
describe("网页摘要读取原设备历史", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.summary.mockReset();
    mocks.query.mockReset();
    mocks.messages.mockReset();
    mocks.hasMore.mockReset();
    mocks.markConversationRead.mockReset();
    mocks.applySnapshotWorkspace.mockReset();
    mocks.sidebar.pinned = [];
    mocks.sidebar.recent = [];
    mocks.sidebar.projects = [];
    mocks.enabled.value = true;
    mocks.hasMore.mockReturnValue(false);
    mocks.markConversationRead.mockResolvedValue(undefined);
    mocks.messages.mockImplementation((result) => result.rows);
  });
  const conversation = () => useWebChatConversation(ref([]), ref("meshconv1:source:chat"), ref("role"), ref(""), ref(""), ref(""), ref(""), ref(false), vi.fn(), vi.fn());
  it("Core 无当前摘要时可显示 Source 历史摘要，保留 Owner 引用且不读 legacy API", async () => {
    mocks.summary.mockResolvedValue({ summaryText: "旧设备聊天摘要", sourceOwnerId: "source", editable: false });
    expect(await conversation().fetchConvSummary()).toBe("旧设备聊天摘要");
    expect(mocks.summary).toHaveBeenCalledWith("meshconv1:source:chat", "role");
    expect(mocks.get).not.toHaveBeenCalled();
  });
  it("历史旧格式摘要保持可读，查询失败不会回落到另一 Owner API", async () => {
    mocks.summary.mockResolvedValue({ summary_text: "历史旧格式" });
    expect(await conversation().fetchConvSummary()).toBe("历史旧格式");
    mocks.summary.mockRejectedValue(new Error("Core 已切换"));
    expect(await conversation().fetchConvSummary()).toBe("");
    expect(mocks.get).not.toHaveBeenCalled();
  });
});

describe("当前聊天角色头像", () => {
  it("角色列表刷新后同步当前头像和名称，移除头像后清空且不切换会话", async () => {
    const scope = effectScope();
    const messages = ref<any[]>([{ id: "existing", role: "assistant" }]);
    const conversationId = ref("conversation");
    const characterId = ref("role");
    const name = ref("");
    const avatar = ref("");
    const disconnect = vi.fn();
    const api = scope.run(() => useWebChatConversation(messages, conversationId, characterId, ref(""), name, ref(""), avatar, ref(false), disconnect, vi.fn()))!;
    try {
      api.selectCharacter({ id: "role", name: "角色", avatar: "data:image/png;base64,first" });
      expect(avatar.value).toBe("data:image/png;base64,first");
      api.characters.value = [{ id: "role", name: "新名称", avatar: "data:image/png;base64,updated" }, { id: "other", name: "其他角色", avatar: "/other.png" }];
      await nextTick();
      expect(avatar.value).toBe("data:image/png;base64,updated");
      expect(name.value).toBe("新名称");
      api.characters.value = [{ id: "role", name: "新名称", avatar: "" }];
      await nextTick();
      expect(avatar.value).toBe("");
      expect(characterId.value).toBe("role");
      expect(conversationId.value).toBe("conversation");
      expect(messages.value).toEqual([{ id: "existing", role: "assistant" }]);
      expect(disconnect).not.toHaveBeenCalled();
    } finally {
      scope.stop();
    }
  });
});

describe("会话切换加载状态", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.summary.mockReset();
    mocks.query.mockReset();
    mocks.messages.mockReset();
    mocks.hasMore.mockReset();
    mocks.markConversationRead.mockReset();
    mocks.applySnapshotWorkspace.mockReset();
    mocks.sidebar.pinned = [];
    mocks.sidebar.recent = [];
    mocks.sidebar.projects = [];
    mocks.enabled.value = true;
    mocks.hasMore.mockReturnValue(false);
    mocks.markConversationRead.mockResolvedValue(undefined);
    mocks.messages.mockImplementation((result) => result.rows);
  });

  it("只在最新会话完成后关闭加载状态并提交其消息", async () => {
    let finishFirst!: (value: unknown) => void;
    let finishSecond!: (value: unknown) => void;
    mocks.query.mockImplementation((id: string) => new Promise((resolve) => {
      if (id === "first") finishFirst = resolve;
      else finishSecond = resolve;
    }));
    const messages = ref<any[]>([]);
    const conversationId = ref("first");
    const role = ref("role");
    const loadSnapshot = vi.fn().mockResolvedValue(true);
    const connect = vi.fn();
    const api = useWebChatConversation(messages, conversationId, role, ref(""), ref(""), ref(""), ref(""), ref(false), vi.fn(), connect, loadSnapshot, 0);

    const firstLoad = api.loadCharacterConversation();
    expect(api.conversationLoading.value).toBe(true);
    conversationId.value = "second";
    const secondLoad = api.loadCharacterConversation();
    expect(api.conversationLoading.value).toBe(true);

    finishSecond({ rows: [{ id: "second-message", role: "user", content: "second" }], snapshot: { ownerId: "a", resources: [{ kind: "conversation", body: { id: "second", title: "第二个会话" } }] } });
    await secondLoad;
    expect(loadSnapshot).toHaveBeenCalledTimes(1);
    expect(connect).toHaveBeenCalledWith(false);
    expect(api.conversationLoading.value).toBe(false);
    expect(messages.value.map((message) => message.content)).toEqual(["second"]);

    finishFirst({ rows: [{ id: "first-message", role: "user", content: "first" }], snapshot: { ownerId: "a", resources: [{ kind: "conversation", body: { id: "first", title: "第一个会话" } }] } });
    await firstLoad;
    expect(api.conversationLoading.value).toBe(false);
    expect(messages.value.map((message) => message.content)).toEqual(["second"]);
  });

  it("会话读取失败时关闭加载状态", async () => {
    mocks.query.mockRejectedValue(new Error("读取失败"));
    const api = useWebChatConversation(ref([]), ref("chat"), ref("role"), ref(""), ref(""), ref(""), ref(""), ref(false), vi.fn(), vi.fn(), undefined, 0);
    await expect(api.loadCharacterConversation()).rejects.toThrow("读取失败");
    expect(api.conversationLoading.value).toBe(false);
  });

  it("切换时优先使用侧边栏已加载的真实标题", async () => {
    let finish!: (value: unknown) => void;
    mocks.query.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    mocks.sidebar.recent = [{ id: "chat", title: "真实会话标题" }];
    const title = ref("");
    const api = useWebChatConversation(ref([]), ref("chat"), ref("role"), title, ref(""), ref(""), ref(""), ref(false), vi.fn(), vi.fn(), vi.fn().mockResolvedValue(true), 0);
    const pending = api.loadCharacterConversation();
    expect(title.value).toBe("真实会话标题");
    finish({ rows: [], snapshot: { ownerId: "a", resources: [{ kind: "conversation", body: { id: "chat", title: "真实会话标题" } }] } });
    await pending;
    expect(title.value).toBe("真实会话标题");
  });

  it("最短加载时间内保持动画可见", async () => {
    vi.useFakeTimers();
    try {
      mocks.query.mockResolvedValue({ rows: [], snapshot: { ownerId: "a", resources: [{ kind: "conversation", body: { id: "chat", title: "会话" } }] } });
      const api = useWebChatConversation(ref([]), ref("chat"), ref("role"), ref(""), ref(""), ref(""), ref(""), ref(false), vi.fn(), vi.fn());
      const pending = api.loadCharacterConversation();
      await vi.advanceTimersByTimeAsync(599);
      expect(api.conversationLoading.value).toBe(true);
      await vi.advanceTimersByTimeAsync(1);
      await pending;
      expect(api.conversationLoading.value).toBe(false);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("保留式会话刷新", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.summary.mockReset();
    mocks.query.mockReset();
    mocks.messages.mockReset();
    mocks.hasMore.mockReset();
    mocks.markConversationRead.mockReset();
    mocks.applySnapshotWorkspace.mockReset();
    mocks.sidebar.pinned = [];
    mocks.sidebar.recent = [];
    mocks.sidebar.projects = [];
    mocks.enabled.value = true;
    mocks.hasMore.mockReturnValue(false);
    mocks.markConversationRead.mockResolvedValue(undefined);
    mocks.messages.mockImplementation((result) => result.rows);
  });

  it("刷新期间保留旧消息，并在成功后原子替换消息", async () => {
    let finish!: (value: unknown) => void;
    mocks.query.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    const messages = ref<any[]>([{ id: "old", role: "user", content: "旧消息" }]);
    const api = useWebChatConversation(messages, ref("chat"), ref("role"), ref(""), ref(""), ref(""), ref(""), ref(false), vi.fn(), vi.fn(), undefined, 0);

    const pending = api.loadCharacterConversation({ preserve: true });
    expect(messages.value.map((message) => message.id)).toEqual(["old"]);
    expect(api.conversationLoading.value).toBe(false);

    finish({ rows: [{ id: "new", role: "assistant", content: "新消息" }], snapshot: { ownerId: "a", resources: [{ kind: "conversation", body: { id: "chat", title: "会话" } }] } });
    await pending;
    expect(messages.value.map((message) => message.id)).toEqual(["new"]);
  });

  it("刷新失败时保留旧消息并结束加载状态", async () => {
    mocks.query.mockRejectedValue(new Error("刷新失败"));
    const messages = ref<any[]>([{ id: "old", role: "user", content: "旧消息" }]);
    const api = useWebChatConversation(messages, ref("chat"), ref("role"), ref(""), ref(""), ref(""), ref(""), ref(false), vi.fn(), vi.fn(), undefined, 0);

    await expect(api.loadCharacterConversation({ preserve: true })).rejects.toThrow("刷新失败");
    expect(messages.value).toEqual([{ id: "old", role: "user", content: "旧消息" }]);
    expect(api.conversationLoading.value).toBe(false);
  });

  it("保留刷新覆盖普通加载时仍会结束加载状态", async () => {
    let finishFirst!: (value: unknown) => void;
    let finishPreserved!: (value: unknown) => void;
    mocks.query
      .mockReturnValueOnce(new Promise((resolve) => { finishFirst = resolve; }))
      .mockReturnValueOnce(new Promise((resolve) => { finishPreserved = resolve; }));
    const messages = ref<any[]>([{ id: "old", role: "user", content: "旧消息" }]);
    const api = useWebChatConversation(messages, ref("chat"), ref("role"), ref(""), ref(""), ref(""), ref(""), ref(false), vi.fn(), vi.fn(), undefined, 0);

    const firstLoad = api.loadCharacterConversation();
    expect(api.conversationLoading.value).toBe(true);
    const preservedLoad = api.loadCharacterConversation({ preserve: true });
    finishPreserved({ rows: [{ id: "new", role: "assistant", content: "新消息" }], snapshot: { ownerId: "a", resources: [{ kind: "conversation", body: { id: "chat", title: "会话" } }] } });
    await preservedLoad;
    expect(api.conversationLoading.value).toBe(false);
    expect(messages.value.map((message) => message.id)).toEqual(["new"]);

    finishFirst({ rows: [{ id: "stale", role: "assistant", content: "旧请求" }], snapshot: { ownerId: "a", resources: [{ kind: "conversation", body: { id: "chat", title: "会话" } }] } });
    await firstLoad;
    expect(messages.value.map((message) => message.id)).toEqual(["new"]);
  });

  it("普通切换仍然先清空旧会话消息", async () => {
    let finish!: (value: unknown) => void;
    mocks.query.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    const messages = ref<any[]>([{ id: "old", role: "user", content: "旧消息" }]);
    const api = useWebChatConversation(messages, ref("chat"), ref("role"), ref(""), ref(""), ref(""), ref(""), ref(false), vi.fn(), vi.fn(), undefined, 0);

    const pending = api.loadCharacterConversation();
    expect(messages.value).toEqual([]);
    finish({ rows: [], snapshot: { ownerId: "a", resources: [{ kind: "conversation", body: { id: "chat", title: "会话" } }] } });
    await pending;
  });
});
