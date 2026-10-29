import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDeviceOwnedConversation } from "../composables/useDeviceOwnedConversation";

const mocks = vi.hoisted(() => ({ post: vi.fn(), patch: vi.fn(), del: vi.fn() }));

vi.mock("../composables/useApi", () => ({
  apiClient: { post: mocks.post, patch: mocks.patch, delete: mocks.del },
}));

vi.mock("../runtime/runtime-adapter", () => ({
  getDeploymentConfig: vi.fn(),
  getNativeProviderTransition: vi.fn(),
}));

vi.mock("../runtime/request-auth", () => ({ createAuthenticatedFetchInit: vi.fn() }));
vi.mock("../runtime/owned-chat-poll", () => ({ OwnedChatStateChangedError: class extends Error {} }));

describe("群聊变更返回会话引用", () => {
  const owned = useDeviceOwnedConversation();
  const conversationId = "meshconv1:source:group";
  const group = {
    conversationId: "group",
    title: "群聊",
    membershipRevision: 2,
    settingsRevision: 2,
    groupSettings: { allowCharacterMentions: true, maxAutoReplies: 6 },
    members: [],
  };

  beforeEach(() => {
    mocks.post.mockReset();
    mocks.patch.mockReset();
    mocks.del.mockReset();
    owned.enabled.value = true;
    owned.coreId.value = "core";
    owned.roleOwnerId.value = "source";
  });

  it("邀请、设置和移除成员后都保留数据所有者引用", async () => {
    mocks.post.mockResolvedValue({ data: { group, saved: true, acknowledgement: { ownerId: "source" } } });
    mocks.patch.mockResolvedValue({ data: { group, saved: true, acknowledgement: { ownerId: "source" } } });
    mocks.del.mockResolvedValue({ data: { group, saved: true, acknowledgement: { ownerId: "source" } } });
    await expect(owned.inviteGroupMembers(conversationId, "role-a", [{ characterId: "role-b", shareHistory: false }], 1)).resolves.toMatchObject({ conversationId });
    await expect(owned.updateGroupSettings(conversationId, "role-a", 1, { allowCharacterMentions: true })).resolves.toMatchObject({ conversationId });
    await expect(owned.removeGroupMember(conversationId, "role-a", "member-b", 2)).resolves.toMatchObject({ conversationId });
  });
});
