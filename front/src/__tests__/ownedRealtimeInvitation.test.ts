import { beforeEach, describe, expect, it, vi } from "vitest";
import { acceptOwnedRealtimeInvitation } from "../realtime/owned-realtime-invitation";

const cryptoModule = "node:crypto";
const { webcrypto } = await import(cryptoModule);
const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("../composables/useApi", () => ({ apiClient: mocks }));
const call = "aa464513-bdb4-4306-9bde-0c7c0397b7bf";
const scope = { spaceId: "space", coreId: "core", initiatorDeviceId: "device", targetDeviceId: "device", roleId: "role", roleRevision: 1, roleOwnerId: "device", resourceOwnerId: "device", authorizationRealm: "device", providerEpoch: 1, targetProviderEpoch: 1, modeRevision: 1, permissionRevision: 1, targetPermissionRevision: 1, coordinated: false, requestId: "invite", turnId: "", executionId: "" };
const invitation = () => ({ id: call, characterId: "role", recipientDeviceId: "device", conversationId: "conversation", executionScope: { ...scope }, status: "pending", revision: 1, expiresAt: new Date(Date.now() + 45000).toISOString(), nonce: "a".repeat(43) });

describe("原所有者来电接听", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("crypto", webcrypto);
    mocks.get.mockResolvedValue({ data: { data: { invitation: invitation() } } });
    mocks.post.mockImplementation(async (_path, body) => ({ data: { data: { saved: true, invitation: { ...invitation(), status: "accepted", revision: 2 }, acknowledgement: { ownerId: "device", requestId: body.requestId, versions: { [`checkpoint/realtime-invitation/${call}`]: 2 } }, ticket: { ticket: "b".repeat(43), wsPath: "/api/device-mesh/v1/business/realtime/session", executionScope: { ...scope } } } } }));
  });
  it("使用实际邀请范围并校验单次所有者确认", async () => {
    const result = await acceptOwnedRealtimeInvitation(call, "role", new AbortController().signal);
    expect(result.scope).toEqual(scope);
    expect(result.conversationId).toBe("conversation");
    expect(mocks.post.mock.calls[0][1].expectedExecutionScope).toEqual(scope);
    expect(mocks.post.mock.calls[0][1].nonce).toBe("a".repeat(43));
  });
  it.each(["foreign", "expired", "accepted"])("拒绝%s邀请且不提交接听", async (kind) => {
    const value = invitation();
    if (kind === "foreign") value.recipientDeviceId = "other";
    if (kind === "expired") value.expiresAt = new Date(Date.now() - 1000).toISOString();
    if (kind === "accepted") value.status = "accepted";
    mocks.get.mockResolvedValue({ data: { data: { invitation: value } } });
    await expect(acceptOwnedRealtimeInvitation(call, "role", new AbortController().signal)).rejects.toThrow();
    expect(mocks.post).not.toHaveBeenCalled();
  });
  it("连接切换丢弃迟到邀请", async () => {
    const controller = new AbortController();
    mocks.get.mockImplementation(async () => { controller.abort(); return { data: { data: { invitation: invitation() } } }; });
    await expect(acceptOwnedRealtimeInvitation(call, "role", controller.signal)).rejects.toThrow();
    expect(mocks.post).not.toHaveBeenCalled();
  });
  it("缺少原所有者确认不得连接", async () => {
    mocks.post.mockResolvedValue({ data: { data: { saved: true, ticket: { ticket: "b".repeat(43) } } } });
    await expect(acceptOwnedRealtimeInvitation(call, "role", new AbortController().signal)).rejects.toThrow();
  });
});
