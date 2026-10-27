import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("../runtime/runtime-adapter", () => ({ resolveApiUrl: vi.fn() }));
vi.mock("../runtime/request-auth", () => ({ createAuthenticatedFetchInit: vi.fn(async () => ({})) }));

import { resolveApiUrl } from "../runtime/runtime-adapter";
import { streamOwnedGroupChat, type OwnedExecutionScope, type OwnedGroupChatRequest } from "../runtime/device-owned-chat";

const base: OwnedExecutionScope = {
  spaceId: "core", initiatorDeviceId: "device", targetDeviceId: "device", coreId: "core",
  providerEpoch: 1, coordinated: false, modeRevision: 1, permissionRevision: 1,
  targetPermissionRevision: 1, targetProviderEpoch: 1, roleId: "role-a", roleRevision: 1,
  roleOwnerId: "device", resourceOwnerId: "device", requestId: "request", turnId: "turn",
  executionId: "execution", conversationRoleId: "role-a",
};

function response(body: string) {
  const encoded = new TextEncoder().encode(body);
  let offset = 0;
  return new Response(new ReadableStream<Uint8Array>({
    pull(controller) {
      if (offset >= encoded.length) {
        controller.close();
        return;
      }
      controller.enqueue(encoded.slice(offset, offset + 7));
      offset += 7;
    },
  }), { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

afterEach(() => vi.unstubAllGlobals());

describe("多角色群聊流", () => {
  it("按投递身份接收多个角色回复并校验最终保存确认", async () => {
    const a = { ...base, requestId: "request/reply/a", turnId: "turn-a", executionId: "execution-a" };
    const b = { ...base, roleId: "role-b", roleRevision: 2, requestId: "request/reply/b", turnId: "turn-b", executionId: "execution-b" };
    const deliveries = [
      { requestId: a.requestId, conversationId: "chat", executionScope: a, reply: "A 回复", saved: true, memoryStatus: "saved", groupRoundId: "round", deliveryId: "delivery-a", characterName: "小夏" },
      { requestId: b.requestId, conversationId: "chat", executionScope: b, reply: "B 回复", saved: true, memoryStatus: "saved", groupRoundId: "round", deliveryId: "delivery-b", characterName: "小林" },
    ];
    const frames = [
      { type: "started", conversationId: "chat", groupRoundId: "round", deliveryId: "delivery-a", characterId: "role-a", executionScope: a },
      { type: "delta", conversationId: "chat", groupRoundId: "round", deliveryId: "delivery-a", text: "A 回复", executionScope: a },
      { type: "started", conversationId: "chat", groupRoundId: "round", deliveryId: "delivery-b", characterId: "role-b", executionScope: b },
      { type: "delta", conversationId: "chat", groupRoundId: "round", deliveryId: "delivery-b", text: "B 回复", executionScope: b },
      { type: "group.completed", data: { conversationId: "chat", requestId: "request", groupRoundId: "round", status: "completed", userMessageId: "request/user", saved: true, deliveries } },
    ];
    vi.mocked(resolveApiUrl).mockResolvedValue("/messages");
    vi.stubGlobal("fetch", vi.fn(async () => response(frames.map((frame) => `data:${JSON.stringify(frame)}\n\n`).join(""))));
    const seen: string[] = [];
    const request = { requestId: "request", conversationId: "chat", characterId: "role-a", message: "@小夏 @小林", group: { expectedMembershipRevision: 1, expectedSettingsRevision: 1 }, mentions: [] } as OwnedGroupChatRequest;
    const result = await streamOwnedGroupChat(request, new AbortController().signal, (event) => seen.push(event.type));
    expect(seen).toEqual(["started", "delta", "started", "delta", "group.completed"]);
    expect(result.deliveries.map((delivery) => delivery.characterName)).toEqual(["小夏", "小林"]);
  });

  it("拒绝跨投递复用执行范围", async () => {
    const foreign = { ...base, roleId: "role-b", requestId: "request/reply/b" };
    vi.mocked(resolveApiUrl).mockResolvedValue("/messages");
    vi.stubGlobal("fetch", vi.fn(async () => response([
      { type: "started", conversationId: "chat", groupRoundId: "round", deliveryId: "delivery-a", executionScope: { ...base, requestId: "request/reply/a" } },
      { type: "delta", conversationId: "chat", groupRoundId: "round", deliveryId: "delivery-a", executionScope: foreign, text: "越权" },
    ].map((frame) => `data:${JSON.stringify(frame)}\n\n`).join(""))));
    const request = { requestId: "request", conversationId: "chat", characterId: "role-a", message: "你好", group: { expectedMembershipRevision: 1, expectedSettingsRevision: 1 }, mentions: [] } as OwnedGroupChatRequest;
    await expect(streamOwnedGroupChat(request, new AbortController().signal, () => undefined)).rejects.toThrow("投递身份");
  });

  it("拒绝同一投递在流中更换执行范围", async () => {
    vi.mocked(resolveApiUrl).mockResolvedValue("/messages");
    vi.stubGlobal("fetch", vi.fn(async () => response([
      { type: "started", conversationId: "chat", groupRoundId: "round", deliveryId: "delivery-a", executionScope: { ...base, requestId: "request/reply/a" } },
      { type: "started", conversationId: "chat", groupRoundId: "round", deliveryId: "delivery-a", executionScope: { ...base, roleId: "role-b", requestId: "request/reply/a" } },
    ].map((frame) => `data:${JSON.stringify(frame)}\n\n`).join(""))));
    const request = { requestId: "request", conversationId: "chat", characterId: "role-a", message: "你好", group: { expectedMembershipRevision: 1, expectedSettingsRevision: 1 }, mentions: [] } as OwnedGroupChatRequest;
    await expect(streamOwnedGroupChat(request, new AbortController().signal, () => undefined)).rejects.toThrow("投递身份");
  });

  it("拒绝最终回复冒充其他群聊轮次", async () => {
    const scope = { ...base, requestId: "request/reply/a" };
    vi.mocked(resolveApiUrl).mockResolvedValue("/messages");
    vi.stubGlobal("fetch", vi.fn(async () => response([
      { type: "started", conversationId: "chat", groupRoundId: "round", deliveryId: "delivery-a", executionScope: scope },
      { type: "group.completed", data: { conversationId: "chat", requestId: "request", groupRoundId: "other-round", status: "completed", userMessageId: "request/user", saved: true, deliveries: [{ requestId: scope.requestId, conversationId: "chat", turnId: scope.turnId, executionId: scope.executionId, executionScope: scope, reply: "A 回复", saved: true, memoryStatus: "saved", groupRoundId: "other-round", deliveryId: "delivery-a" }] } },
    ].map((frame) => `data:${JSON.stringify(frame)}\n\n`).join(""))));
    const request = { requestId: "request", conversationId: "chat", characterId: "role-a", message: "你好", group: { expectedMembershipRevision: 1, expectedSettingsRevision: 1 }, mentions: [] } as OwnedGroupChatRequest;
    await expect(streamOwnedGroupChat(request, new AbortController().signal, () => undefined)).rejects.toThrow("轮次归属");
  });

  it("传递群成员失败事件并确认最终轮次", async () => {
    const scope = { ...base, requestId: "request/reply/a" };
    vi.mocked(resolveApiUrl).mockResolvedValue("/messages");
    vi.stubGlobal("fetch", vi.fn(async () => response([
      { type: "started", conversationId: "chat", groupRoundId: "round", deliveryId: "delivery-a", characterId: "role-a", executionScope: scope },
      { type: "group.delivery.failed", conversationId: "chat", groupRoundId: "round", deliveryId: "delivery-a", text: "生成失败", executionScope: scope },
      { type: "group.completed", data: { conversationId: "chat", requestId: "request", groupRoundId: "round", status: "completed_with_errors", userMessageId: "request/user", saved: true, deliveries: [] } },
    ].map((frame) => `data:${JSON.stringify(frame)}\n\n`).join(""))));
    const request = { requestId: "request", conversationId: "chat", characterId: "role-a", message: "你好", group: { expectedMembershipRevision: 1, expectedSettingsRevision: 1 }, mentions: [] } as OwnedGroupChatRequest;
    const seen: string[] = [];
    const result = await streamOwnedGroupChat(request, new AbortController().signal, (event) => seen.push(event.type));
    expect(seen).toEqual(["started", "group.delivery.failed", "group.completed"]);
    expect(result.status).toBe("completed_with_errors");
    expect(result.deliveries).toEqual([]);
  });
});
