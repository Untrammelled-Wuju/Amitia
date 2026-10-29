import { beforeEach, describe, expect, it, vi } from "vitest";
import { OwnedChatStateChangedError, pollOwnedChat } from "../runtime/owned-chat-poll";

const mocks = vi.hoisted(() => ({ get: vi.fn(), transition: vi.fn(), stream: vi.fn(), mode: { value: "cloud" as "local" | "cloud" } }));
vi.mock("../composables/useApi", () => ({ apiClient: { get: mocks.get } }));
vi.mock("../runtime/runtime-adapter", () => ({ getDeploymentConfig: async () => ({ mode: mocks.mode.value, serverURL: mocks.mode.value === "cloud" ? "https://core" : undefined }), getNativeProviderTransition: mocks.transition }));
vi.mock("../runtime/device-owned-chat", () => ({ streamOwnedChat: mocks.stream }));

const unavailable = "云端服务暂不可用，当前回复已中断；不会自动切换到本机模型。";
const providerNotice = "云端服务提供者已从「B」切换为「C」，当前回复已中断。";
const stateKey = "amitia.mesh.provider-state";

describe("云端可用性恢复与中断通知", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
    localStorage.clear();
    mocks.mode.value = "cloud";
    mocks.transition.mockResolvedValue(null);
    mocks.get.mockImplementation(async (path: string) => ({ data: path.endsWith("/coordination/me")
      ? { coreId: "C", coordinationAvailable: true, policy: { coordinated: true, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } }
      : { roles: [{ id: "role", name: "角色", revision: 1 }], roleOwnerId: "C" } }));
  });

  it("多个入口同时刷新共用状态请求，完成后允许下一次刷新", async () => {
    const { useDeviceOwnedConversation } = await import("../composables/useDeviceOwnedConversation");
    const first = useDeviceOwnedConversation();
    const second = useDeviceOwnedConversation();
    expect(await Promise.all([first.refresh(), second.refresh(), first.refresh()])).toEqual([true, true, true]);
    expect(mocks.get.mock.calls.filter(([path]) => path.endsWith("/coordination/me"))).toHaveLength(1);
    expect(mocks.get.mock.calls.filter(([path]) => path.endsWith("/business/roles"))).toHaveLength(1);
    await second.refresh();
    expect(mocks.get.mock.calls.filter(([path]) => path.endsWith("/coordination/me"))).toHaveLength(2);
  });

  it("共享刷新失败后释放请求，下一次可以恢复服务", async () => {
    const { useDeviceOwnedConversation } = await import("../composables/useDeviceOwnedConversation");
    const owned = useDeviceOwnedConversation();
    mocks.get.mockRejectedValueOnce(new Error("offline"));
    const results = await Promise.allSettled([owned.refresh(), owned.refresh()]);
    expect(results.map((result) => result.status)).toEqual(["rejected", "rejected"]);
    expect(mocks.get).toHaveBeenCalledTimes(1);
    expect(await owned.refresh()).toBe(true);
    expect(mocks.get.mock.calls.filter(([path]) => path.endsWith("/coordination/me"))).toHaveLength(2);
  });

  it("持久化角色失效时回退到首个可用角色", async () => {
    mocks.get.mockImplementation(async (path: string) => ({ data: path.endsWith("/coordination/me")
      ? { coreId: "C", coordinationAvailable: true, policy: { coordinated: true, providerEpoch: 1, modeRevision: 1, permissionRevision: 1, selectedRole: "missing" } }
      : { roles: [{ id: "role-a", name: "角色甲", revision: 1 }, { id: "role-b", name: "角色乙", revision: 1 }], roleOwnerId: "C" } }));
    const { useDeviceOwnedConversation } = await import("../composables/useDeviceOwnedConversation");
    const owned = useDeviceOwnedConversation();
    await owned.refresh();
    expect(owned.selectInitialRole("also-missing")).toBe("role-a");
  });

  it("旧版持久化的核心切换提示不会在升级后恢复", async () => {
    localStorage.setItem(stateKey, JSON.stringify({ coreId: "B", notice: providerNotice }));
    const { useDeviceOwnedConversation } = await import("../composables/useDeviceOwnedConversation");
    const owned = useDeviceOwnedConversation();
    expect(owned.notice.value).toBe("");
    expect(await owned.refresh()).toBe(true);
    expect(owned.notice.value).toBe("");
    expect(JSON.parse(localStorage.getItem(stateKey)!)).toMatchObject({ version: 2, mode: "cloud", binding: "cloud:https://core" });
    expect(JSON.parse(localStorage.getItem(stateKey)!).notice).toBeUndefined();
    expect(mocks.stream).not.toHaveBeenCalled();
  });

  it("本地模式忽略历史推进状态并且不读取云端切换入口", async () => {
    mocks.mode.value = "local";
    localStorage.setItem(stateKey, JSON.stringify({ coreId: "B", notice: providerNotice }));
    const { useDeviceOwnedConversation } = await import("../composables/useDeviceOwnedConversation");
    const owned = useDeviceOwnedConversation();
    expect(await owned.refresh()).toBe(true);
    expect(owned.notice.value).toBe("");
    expect(mocks.transition).not.toHaveBeenCalled();
    expect(JSON.parse(localStorage.getItem(stateKey)!)).toMatchObject({ version: 2, mode: "local", binding: "local" });
  });

  it("临时断线恢复后清除不可用提示", async () => {
    const { useDeviceOwnedConversation } = await import("../composables/useDeviceOwnedConversation");
    const owned = useDeviceOwnedConversation();
    await owned.refresh();
    owned.markUnavailable();
    expect(owned.notice.value).toBe(unavailable);
    await owned.refresh();
    expect(owned.notice.value).toBe("");
    expect(mocks.stream).not.toHaveBeenCalled();
  });

  it("本地模式没有进行中的回复时核心变化不会声称回复中断", async () => {
    mocks.mode.value = "local";
    let currentCore = "C";
    mocks.get.mockImplementation(async (path: string) => ({ data: path.endsWith("/coordination/me")
      ? { coreId: currentCore, coordinationAvailable: true, policy: { coordinated: true, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } }
      : { roles: [{ id: "role", name: "角色", revision: 1 }], roleOwnerId: currentCore } }));
    const { useDeviceOwnedConversation } = await import("../composables/useDeviceOwnedConversation");
    const owned = useDeviceOwnedConversation();
    await owned.refresh();
    currentCore = "D";
    await owned.refresh();
    expect(owned.notice.value).toContain("本机 Core 已从「C」切换为「D」");
    expect(owned.notice.value).not.toContain("当前回复已中断");
    expect(owned.coreId.value).toBe("D");
  });

  it("本地模式核心变化时中止进行中的回复并使用本机提示", async () => {
    mocks.mode.value = "local";
    let currentCore = "C";
    mocks.get.mockImplementation(async (path: string) => ({ data: path.endsWith("/coordination/me")
      ? { coreId: currentCore, coordinationAvailable: true, policy: { coordinated: true, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } }
      : { roles: [{ id: "role", name: "角色", revision: 1 }], roleOwnerId: currentCore } }));
    const { useDeviceOwnedConversation } = await import("../composables/useDeviceOwnedConversation");
    const owned = useDeviceOwnedConversation();
    await owned.refresh();
    let signal!: AbortSignal;
    let finish!: (value: any) => void;
    mocks.stream.mockImplementation((_request, capturedSignal) => {
      signal = capturedSignal;
      return new Promise((resolve) => { finish = resolve; });
    });
    const pending = owned.send({ requestId: "request", characterId: "role", message: "继续" }, vi.fn());
    currentCore = "D";
    await owned.refresh();
    expect(owned.notice.value).toContain("本机 Core 已从「C」切换为「D」");
    expect(signal.aborted).toBe(true);
    finish({ conversationId: "chat", executionScope: { resourceOwnerId: "C" } });
    await expect(pending).rejects.toThrow("迟到回复已拦截");
  });

  it("角色接口仍失败时不能提前清除不可用提示，重复轮询失败不重复发中断事件", async () => {
    const { useDeviceOwnedConversation } = await import("../composables/useDeviceOwnedConversation");
    const owned = useDeviceOwnedConversation();
    await owned.refresh();
    const event = vi.fn();
    window.addEventListener("amitia:execution-scope-changed", event);
    try {
      owned.markUnavailable();
      owned.markUnavailable();
      expect(event).toHaveBeenCalledTimes(1);
      mocks.get.mockImplementation(async (path: string) => {
        if (path.endsWith("/business/roles")) throw new Error("offline");
        return { data: { coreId: "C", coordinationAvailable: true, policy: { coordinated: true, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } } };
      });
      await expect(owned.refresh()).rejects.toThrow("offline");
      expect(owned.notice.value).toBe(unavailable);
    } finally {
      window.removeEventListener("amitia:execution-scope-changed", event);
    }
  });

  it("断线取消进行中的生成，恢复后既不自动重发也不接收旧生成输出", async () => {
    const { useDeviceOwnedConversation } = await import("../composables/useDeviceOwnedConversation");
    const owned = useDeviceOwnedConversation();
    await owned.refresh();
    let signal!: AbortSignal;
    let oldEvent!: (event: any) => void;
    let finish!: (value: any) => void;
    mocks.stream.mockImplementation((_request, capturedSignal, onEvent) => {
      signal = capturedSignal;
      oldEvent = onEvent;
      return new Promise((resolve) => { finish = resolve; });
    });
    const onEvent = vi.fn();
    const pending = owned.send({ requestId: "request", characterId: "role", message: "继续" }, onEvent);
    owned.markUnavailable();
    expect(signal.aborted).toBe(true);
    await owned.refresh();
    oldEvent({ type: "delta", text: "旧回复" });
    expect(onEvent).not.toHaveBeenCalled();
    finish({ conversationId: "chat", executionScope: { resourceOwnerId: "C" } });
    await expect(pending).rejects.toThrow("迟到回复已拦截");
    expect(mocks.stream).toHaveBeenCalledTimes(1);
    expect(owned.notice.value).toBe("");
  });

  it("迟到角色状态不能清除新的权限变更通知", async () => {
    const { useDeviceOwnedConversation } = await import("../composables/useDeviceOwnedConversation");
    const owned = useDeviceOwnedConversation();
    await owned.refresh();
    owned.markUnavailable();
    let finish!: (value: unknown) => void;
    mocks.get.mockImplementation(async (path: string) => path.endsWith("/coordination/me")
      ? { data: { coreId: "C", coordinationAvailable: true, policy: { coordinated: true, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } } }
      : new Promise((resolve) => { finish = resolve; }));
    const pending = owned.refresh();
    await vi.waitFor(() => expect(finish).toBeTypeOf("function"));
    owned.stopLocal("权限已变化，请重新确认");
    finish({ data: { roles: [], roleOwnerId: "C" } });
    await expect(pending).rejects.toThrow("旧角色已丢弃");
    expect(owned.notice.value).toBe("权限已变化，请重新确认");
  });
});

describe("聊天轮询错误边界", () => {
  it("聊天列表失败只报告数据错误，不触发云端中断", async () => {
    const failure = new Error("项目列表加载失败");
    const owned = { refresh: vi.fn().mockResolvedValue(true), markUnavailable: vi.fn(), notice: { value: "" } };
    const onError = vi.fn();
    await pollOwnedChat(owned, vi.fn().mockRejectedValue(failure), onError, () => false);
    expect(onError).toHaveBeenCalledWith(failure);
    expect(owned.markUnavailable).not.toHaveBeenCalled();
  });

  it("状态接口断线才标记不可用，不继续读取旧聊天数据", async () => {
    const owned = { refresh: vi.fn().mockRejectedValue(new Error("offline")), markUnavailable: vi.fn(), notice: { value: "" } };
    const ready = vi.fn();
    await pollOwnedChat(owned, ready, vi.fn(), () => false);
    expect(owned.markUnavailable).toHaveBeenCalledOnce();
    expect(ready).not.toHaveBeenCalled();
  });

  it.each(["provider", "approval", "disposed"])("%s 不被替换为网络不可用提示", async (reason) => {
    const owned = { refresh: vi.fn().mockRejectedValue(reason === "provider" ? new OwnedChatStateChangedError("旧状态已丢弃") : new Error("pending")), markUnavailable: vi.fn(), notice: { value: reason === "approval" ? "等待新服务批准" : "" } };
    await pollOwnedChat(owned, vi.fn(), vi.fn(), () => reason === "disposed");
    expect(owned.markUnavailable).not.toHaveBeenCalled();
  });
});
