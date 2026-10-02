import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createMemoryHistory, createRouter } from "vue-router";
import ElementPlus from "element-plus";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import SideNav from "../components/SideNav.vue";
import { useChatStore } from "../stores/chat";

const mocks = vi.hoisted(() => ({ get: vi.fn(), startDraft: vi.fn() }));

vi.mock("../composables/useApi", () => ({
  apiClient: { get: mocks.get },
  useApi: () => ({ get: vi.fn(), post: vi.fn() }),
}));
vi.mock("../composables/useConversationWorkspace", () => ({
  useConversationWorkspace: () => ({ startDraftConversation: mocks.startDraft }),
}));
vi.mock("../composables/useBrandLogo", () => ({
  useBrandLogo: () => ({ logoUrl: "/test-logo.svg" }),
}));
vi.mock("../stores/extensionUI", () => ({ useExtensionUIStore: () => ({}) }));
vi.mock("../ui-runtime/componentRegistry", () => ({
  useUIComponentVariant: () => ({ style: {} }),
}));
vi.mock("../ui-runtime/navigationRegistry", async () => {
  const { computed } = await import("vue");
  return {
    useUINavigationRegistry: () => ({ groups: computed(() => []), items: computed(() => []) }),
    isUINavigationItemActive: () => false,
  };
});
vi.mock("../components/SearchModal.vue", () => ({ default: { template: "<div />" } }));

const conversation = {
  id: "sent-conversation",
  projectId: "",
  title: "第一条消息生成的标题",
  channel: "web",
  source: "web",
  messageCount: 1,
  createdAt: "2026-10-03T00:00:00Z",
  updatedAt: "2026-10-03T00:00:00Z",
};

async function renderSidebar(path: string) {
  const pinia = createPinia();
  setActivePinia(pinia);
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: "/chat", component: { template: "<div />" } }],
  });
  await router.push(path);
  await router.isReady();
  const wrapper = mount(SideNav, { global: { plugins: [pinia, router, ElementPlus] } });
  await flushPromises();
  return { wrapper, router, chat: useChatStore(pinia) };
}

describe("sidebar conversation selection", () => {
  beforeEach(() => {
    localStorage.clear();
    mocks.get.mockResolvedValue({ data: { pinned: [], recent: [], projects: [] } });
    mocks.startDraft.mockResolvedValue(null);
  });
  afterEach(() => vi.clearAllMocks());

  it("moves selection from the draft to the title after a conversation is created", async () => {
    const { wrapper, router, chat } = await renderSidebar("/chat");
    try {
      expect(wrapper.find(".el-menu-item.is-active").text()).toBe("新对话");
      mocks.get.mockResolvedValue({ data: { pinned: [], recent: [conversation], projects: [] } });
      await router.replace({ path: "/chat", query: { conversationId: conversation.id } });
      await chat.fetchSidebar();
      await flushPromises();
      expect(wrapper.find(".el-menu-item.is-active").exists()).toBe(false);
      expect(wrapper.get('[aria-current="page"]').text()).toBe(conversation.title);
      await wrapper.get(".el-menu-item").trigger("click");
      await flushPromises();
      expect(mocks.startDraft).toHaveBeenCalledOnce();
      expect(router.currentRoute.value.query.conversationId).toBeUndefined();
      expect(wrapper.find('[aria-current="page"]').exists()).toBe(false);
      expect(wrapper.get(".el-menu-item.is-active").text()).toBe("新对话");
    } finally {
      wrapper.unmount();
    }
  });

  it("selects the conversation title when opening an existing conversation directly", async () => {
    mocks.get.mockResolvedValue({ data: { pinned: [], recent: [conversation], projects: [] } });
    const { wrapper } = await renderSidebar(`/chat?conversationId=${conversation.id}`);
    try {
      expect(wrapper.find(".el-menu-item.is-active").exists()).toBe(false);
      expect(wrapper.get('[aria-current="page"]').text()).toBe(conversation.title);
    } finally {
      wrapper.unmount();
    }
  });
});
