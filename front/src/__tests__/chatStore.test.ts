import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}));

vi.mock("@/composables/useApi", () => ({
  apiClient: {
    get: mocks.get,
    post: mocks.post,
    put: mocks.put,
  },
}));

import { useChatStore } from "@/stores/chat";

describe("chat store", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    mocks.get.mockReset();
    mocks.post.mockReset();
    mocks.put.mockReset();
  });

  it("refreshes the sidebar after restoring an archived conversation", async () => {
    const conversation = {
      id: "conversation-1",
      projectId: "",
      title: "Restored",
      channel: "web",
      source: "web",
      messageCount: 0,
      createdAt: "2026-09-19T00:00:00Z",
      updatedAt: "2026-09-19T00:00:00Z",
    };
    mocks.put.mockResolvedValueOnce({ data: { id: conversation.id } });
    mocks.get.mockResolvedValueOnce({
      data: {
        pinned: [],
        recent: [conversation],
        projects: [],
      },
    });

    const store = useChatStore();
    await store.restoreConversation(conversation.id);

    expect(mocks.put).toHaveBeenCalledWith(
      `/api/web-chat/conversations/${conversation.id}`,
      { archived: false },
    );
    expect(mocks.get).toHaveBeenCalledWith("/api/web-chat/sidebar");
    expect(store.sidebar.recent).toEqual([conversation]);
    expect(store.archivedRevision).toBe(1);
  });

  it("signals the archive page after archiving a conversation", async () => {
    mocks.put.mockResolvedValueOnce({ data: { id: "conversation-2" } });
    mocks.get.mockResolvedValueOnce({
      data: {
        pinned: [],
        recent: [],
        projects: [],
      },
    });

    const store = useChatStore();
    await store.archiveConversation("conversation-2");

    expect(mocks.put).toHaveBeenCalledWith(
      "/api/web-chat/conversations/conversation-2",
      { archived: true },
    );
    expect(store.archivedRevision).toBe(1);
  });

  it("marks an unread conversation as read without changing its order", async () => {
    const conversation = {
      id: "conversation-3",
      projectId: "",
      title: "Unread",
      channel: "web",
      source: "web",
      messageCount: 2,
      isGenerating: false,
      hasUnread: true,
      lastReadTurnSequence: 0,
      lastTerminalTurnSequence: 2,
      createdAt: "2026-09-19T00:00:00Z",
      updatedAt: "2026-09-19T00:00:00Z",
    };
    mocks.post.mockResolvedValueOnce({ data: { conversationId: conversation.id } });

    const store = useChatStore();
    store.sidebar.recent = [conversation];
    await store.markConversationRead(conversation.id, 2);

    expect(mocks.post).toHaveBeenCalledWith(
      `/api/web-chat/conversations/${conversation.id}/read`,
    );
    expect(conversation.hasUnread).toBe(false);
    expect(conversation.lastReadTurnSequence).toBe(2);
    expect(store.sidebar.recent).toEqual([conversation]);
  });
});
