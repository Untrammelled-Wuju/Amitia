import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";

const mocks = vi.hoisted(() => ({
  conversations: vi.fn(),
  edit: vi.fn(),
  projects: vi.fn(),
  refresh: vi.fn(),
}));

vi.mock("@/composables/useDeviceOwnedConversation", async () => {
  const { ref } = await import("vue");
  return {
    useDeviceOwnedConversation: () => ({
      enabled: ref(true),
      roles: ref([{ id: "role-1", name: "角色", revision: 1 }]),
      coreId: ref("core-1"),
      roleOwnerId: ref("owner-1"),
      notice: ref(""),
      policy: ref({ selectedRole: "role-1", providerEpoch: 1, coordinated: false, modeRevision: 1, permissionRevision: 1 }),
      refresh: mocks.refresh,
      selectInitialRole: () => "role-1",
      conversations: mocks.conversations,
      projects: mocks.projects,
      edit: mocks.edit,
    }),
  };
});

import { useChatStore } from "@/stores/chat";

describe("chat store", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    mocks.conversations.mockReset();
    mocks.edit.mockReset();
    mocks.projects.mockReset();
    mocks.refresh.mockReset();
    mocks.refresh.mockResolvedValue(true);
    mocks.edit.mockResolvedValue({});
    mocks.projects.mockResolvedValue({ projects: [] });
  });

  it("refreshes the sidebar after restoring an archived conversation", async () => {
    const conversation = {
      id: "conversation-1",
      projectId: "",
      title: "Restored",
      channel: "web",
      source: "device-mesh",
      messageCount: 0,
      createdAt: "2026-09-19T00:00:00Z",
      updatedAt: "2026-09-19T00:00:00Z",
    };
    mocks.conversations.mockResolvedValue([conversation]);

    const store = useChatStore();
    await store.restoreConversation(conversation.id);

    expect(mocks.edit).toHaveBeenCalledWith("conversation", conversation.id, { archived: false });
    expect(store.sidebar.recent).toEqual([conversation]);
    expect(store.archivedRevision).toBe(1);
  });

  it("signals the archive page after archiving a conversation", async () => {
    mocks.conversations.mockResolvedValue([]);
    const store = useChatStore();
    await store.archiveConversation("conversation-2");

    expect(mocks.edit).toHaveBeenCalledWith("conversation", "conversation-2", { archived: true });
    expect(store.archivedRevision).toBe(1);
  });

  it("marks an unread conversation as read without changing its order", async () => {
    const conversation = {
      id: "conversation-3",
      projectId: "",
      title: "Unread",
      channel: "web",
      source: "device-mesh",
      messageCount: 2,
      isGenerating: false,
      hasUnread: true,
      unreadCompletedCount: 1,
      lastReadTurnSequence: 0,
      lastTerminalTurnSequence: 2,
      createdAt: "2026-09-19T00:00:00Z",
      updatedAt: "2026-09-19T00:00:00Z",
    };
    mocks.conversations.mockResolvedValue([conversation]);
    const store = useChatStore();
    await store.fetchSidebar();
    await store.markConversationRead(conversation.id, 2);

    expect(mocks.edit).toHaveBeenCalledWith("conversation", conversation.id, { lastReadTurnSequence: 2 }, { characterId: undefined });
    expect(store.sidebar.recent[0].hasUnread).toBe(false);
    expect(store.sidebar.recent[0].unreadCompletedCount).toBe(0);
    expect(store.sidebar.recent[0].lastReadTurnSequence).toBe(2);
  });

  it("keeps a newer unread turn when the sidebar refreshes while marking read", async () => {
    const conversation = { id: "conversation-4", projectId: "", title: "Unread", channel: "web", source: "device-mesh", messageCount: 1, hasUnread: true, unreadCompletedCount: 1, lastReadTurnSequence: 0, lastTerminalTurnSequence: 2, createdAt: "2026-09-19T00:00:00Z", updatedAt: "2026-09-19T00:00:00Z" };
    mocks.conversations.mockResolvedValue([conversation]);
    let finishEdit!: () => void;
    mocks.edit.mockImplementation(() => new Promise<void>((resolve) => { finishEdit = resolve; }));
    const store = useChatStore();
    await store.fetchSidebar();
    const marking = store.markConversationRead(conversation.id, 2);
    mocks.conversations.mockResolvedValue([{ ...conversation, lastTerminalTurnSequence: 3 }]);
    await store.fetchSidebar();
    finishEdit();
    await marking;
    expect(store.sidebar.recent[0]).toMatchObject({ hasUnread: true, unreadCompletedCount: 1, lastReadTurnSequence: 2, lastTerminalTurnSequence: 3 });
  });
});
