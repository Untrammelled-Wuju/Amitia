import { beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  del: vi.fn(),
}));

vi.mock("@/composables/useApi", () => ({
  useApi: () => ({ get: mocks.get, post: mocks.post, del: mocks.del }),
}));

vi.mock("@/composables/useCachedApi", () => ({
  useCachedApi: () => ({
    cachedGet: async () => ({ data: ref([]), refresh: async () => {} }),
    saveCache: vi.fn(),
    invalidateCache: vi.fn(),
  }),
}));

vi.mock("element-plus", () => ({
  ElMessage: { success: vi.fn(), warning: vi.fn(), error: vi.fn() },
  ElMessageBox: { confirm: vi.fn() },
}));

import { useWebChatConversation } from "@/composables/useWebChatConversation";

function setupConversationComposable() {
  const messages = ref<any[]>([]);
  const convId = ref("");
  const characterId = ref("");
  const convTitle = ref("");
  const charName = ref("");
  const charIdentity = ref("");
  const charAvatar = ref("");
  const hasMoreHistory = ref(false);
  const msgPage = ref(1);

  const api = useWebChatConversation(
    messages,
    convId,
    characterId,
    convTitle,
    charName,
    charIdentity,
    charAvatar,
    hasMoreHistory,
    msgPage,
    () => {},
    () => {},
    () => {},
    () => {},
    () => {},
  );

  return { api, convId, characterId };
}

describe("web chat conversation recovery", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    mocks.get.mockResolvedValue({ items: [], total: 0, totalPages: 1 });
    mocks.post.mockResolvedValue({});
  });

  it("drops a cached conversation that no longer exists and recreates it", async () => {
    localStorage.setItem("webchat-conv-id", "removed-conversation");
    mocks.get.mockImplementation(async (url: string) => {
      if (url === "/api/web-chat/conversations") {
        return { items: [{ id: "existing-conversation" }], total: 1, totalPages: 1 };
      }
      return { items: [], total: 0, totalPages: 1 };
    });
    mocks.post.mockResolvedValue({ id: "recreated-conversation" });

    const { api, convId } = setupConversationComposable();
    api.selectCharacter({ id: "character-1", name: "角色" });
    await api.loadCharacterConversation();

    expect(localStorage.getItem("webchat-conv-id")).toBeNull();
    expect(mocks.post).toHaveBeenCalledWith("/api/web-chat/conversations", {
      characterId: "character-1",
      title: "",
    });
    expect(convId.value).toBe("recreated-conversation");
  });

  it("keeps a cached conversation that still exists", async () => {
    localStorage.setItem("webchat-conv-id", "existing-conversation");
    mocks.get.mockImplementation(async (url: string) => {
      if (url === "/api/web-chat/conversations") {
        return { items: [{ id: "existing-conversation" }], total: 1, totalPages: 1 };
      }
      return { items: [], total: 0, totalPages: 1 };
    });

    const { api, convId } = setupConversationComposable();
    api.selectCharacter({ id: "character-1", name: "角色" });
    await api.loadCharacterConversation();

    expect(localStorage.getItem("webchat-conv-id")).toBe("existing-conversation");
    expect(mocks.post).not.toHaveBeenCalled();
    expect(convId.value).toBe("existing-conversation");
  });
});
