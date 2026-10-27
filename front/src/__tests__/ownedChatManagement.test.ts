import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ post: vi.fn(), refresh: vi.fn(), data: vi.fn(), owned: null as any }));
vi.mock("@/composables/useApi", () => ({ apiClient: { post: mocks.post } }));
vi.mock("@/composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => mocks.owned }));

import { ownedChatRuntimeRequest, ownedManagementRequest } from "@/runtime/owned-chat-management";

const scope = { coreId: "core", roleId: "role", resourceOwnerId: "device", roleOwnerId: "device", targetDeviceId: "device", providerEpoch: 1, permissionRevision: 1 };

describe("Owner聊天管理与控制", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.owned = { refresh: mocks.refresh, data: mocks.data, selectInitialRole: () => "role", coreId: { value: "core" }, roleOwnerId: { value: "device" }, notice: { value: "" } };
    mocks.refresh.mockResolvedValue(true);
    mocks.data.mockResolvedValue({ executionScope: scope });
    mocks.post.mockResolvedValue({ data: { executionScope: scope, items: [] } });
  });

  it("统计与执行控制均使用POST并绑定原会话Owner", async () => {
    await ownedManagementRequest("stats");
    await ownedChatRuntimeRequest("retry", { conversationId: "meshconv1:device:chat", turnId: "turn" });
    expect(mocks.post.mock.calls[0][0]).toBe("/api/device-mesh/v1/business/chat-management/stats");
    expect(mocks.post.mock.calls[1][0]).toBe("/api/device-mesh/v1/business/chat-runtime/retry");
    expect(mocks.post.mock.calls[1][1]).toMatchObject({ conversationId: "chat", conversationOwnerId: "device", turnId: "turn", characterId: "role", expectedExecutionScope: scope });
    expect(mocks.post.mock.calls[0][1].requestId).not.toBe(mocks.post.mock.calls[1][1].requestId);
  });

  it("会话快照返回额外 conversationRoleId 时不误判归属变化", async () => {
    const conversationScope = { ...scope, conversationRoleId: "role" };
    mocks.post.mockResolvedValue({ data: { executionScope: conversationScope } });

    await ownedChatRuntimeRequest("snapshot", { conversationId: "chat" });

    expect(mocks.data).toHaveBeenCalledWith("working", "role");
    expect(mocks.post.mock.calls[0][1].expectedExecutionScope).toEqual(scope);
  });

  it("明确绑定 conversationRoleId 后仍拒绝不同角色", async () => {
    mocks.data.mockResolvedValueOnce({ executionScope: { ...scope, conversationRoleId: "expected-role" } });
    mocks.post.mockResolvedValueOnce({ data: { executionScope: { ...scope, conversationRoleId: "other-role" } } });

    await expect(ownedChatRuntimeRequest("snapshot", { conversationId: "chat" })).rejects.toThrow("结果已丢弃");
  });

  it("清理预览归属变化时拒绝确认且不发出请求", async () => {
    await expect(ownedManagementRequest("cleanup/confirm", { expectedExecutionScope: { ...scope, providerEpoch: 2 } })).rejects.toThrow("重新预览");
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("Owner响应或Core迟到切换时不展示旧结果", async () => {
    mocks.post.mockResolvedValueOnce({ data: { executionScope: { ...scope, resourceOwnerId: "other" } } });
    await expect(ownedChatRuntimeRequest("snapshot", { conversationId: "chat" })).rejects.toThrow("结果已丢弃");
    mocks.post.mockImplementationOnce(async () => {
      mocks.owned.coreId.value = "new-core";
      return { data: { executionScope: scope } };
    });
    await expect(ownedManagementRequest("stats")).rejects.toThrow("结果已丢弃");
  });
});
