import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
}));

vi.mock("@/composables/useApi", () => ({
  apiClient: {
    get: mocks.get,
    put: mocks.put,
  },
  useApi: () => ({
    get: async (url: string, params?: unknown) => {
      const response = await mocks.get(url, { params });
      return response.data;
    },
  }),
}));

import ChatLogsView from "@/views/chat-logs/ChatLogsView.vue";
import { useChatStore } from "@/stores/chat";

describe("chat logs archive", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.put.mockReset();
  });

  it("shows a newly archived conversation without a manual refresh", async () => {
    const archivedItems: Array<Record<string, unknown>> = [];
    mocks.get.mockImplementation((url: string) => {
      if (url === "/api/web-chat/sidebar") {
        return Promise.resolve({ data: { pinned: [], recent: [], projects: [] } });
      }
      if (url === "/api/characters") {
        return Promise.resolve({ data: [] });
      }
      if (url === "/api/web-chat/conversations") {
        return Promise.resolve({ data: { items: archivedItems } });
      }
      throw new Error(`unexpected GET ${url}`);
    });
    mocks.put.mockResolvedValue({ data: {} });

    const pinia = createPinia();
    setActivePinia(pinia);
    const store = useChatStore();
    const wrapper = mount(ChatLogsView, {
      global: {
        plugins: [pinia],
        components: {
          ElButton: { template: "<button><slot /></button>" },
          ElEmpty: { template: "<div><slot /></div>" },
          ElIcon: { template: "<span><slot /></span>" },
          ElInput: { template: "<div />" },
          ElOption: { template: "<div />" },
          ElSelect: { template: "<div><slot /></div>" },
        },
        directives: {
          loading: {},
        },
        stubs: {
          ArchiveConversationIcon: true,
          ChatBubble: true,
        },
      },
    });

    await flushPromises();
    expect(wrapper.text()).not.toContain("新归档会话");

    archivedItems.push({
      id: "archived-1",
      title: "新归档会话",
      messageCount: 1,
      archivedAt: "2026-09-19T00:00:00Z",
    });
    await store.archiveConversation("archived-1");
    await flushPromises();

    expect(wrapper.text()).toContain("新归档会话");
  });

  it("orders archived conversations by archived time", async () => {
    const archivedItems = [
      {
        id: "older",
        title: "较早归档",
        messageCount: 1,
        archivedAt: "2026-09-19 10:00:00",
      },
      {
        id: "newer",
        title: "较晚归档",
        messageCount: 99,
        archivedAt: "2026-09-19 11:00:00",
      },
    ];
    mocks.get.mockImplementation((url: string) => {
      if (url === "/api/web-chat/sidebar") {
        return Promise.resolve({ data: { pinned: [], recent: [], projects: [] } });
      }
      if (url === "/api/characters") {
        return Promise.resolve({ data: [] });
      }
      if (url === "/api/web-chat/conversations") {
        return Promise.resolve({ data: { items: archivedItems } });
      }
      throw new Error(`unexpected GET ${url}`);
    });

    const pinia = createPinia();
    setActivePinia(pinia);
    const wrapper = mount(ChatLogsView, {
      global: {
        plugins: [pinia],
        components: {
          ElButton: { template: "<button><slot /></button>" },
          ElEmpty: { template: "<div><slot /></div>" },
          ElIcon: { template: "<span><slot /></span>" },
          ElInput: { template: "<div />" },
          ElOption: { template: "<div />" },
          ElSelect: { template: "<div><slot /></div>" },
        },
        directives: {
          loading: {},
        },
        stubs: {
          ArchiveConversationIcon: true,
          ChatBubble: true,
        },
      },
    });

    await flushPromises();
    const rows = wrapper.findAll(".archive-item");

    expect(rows).toHaveLength(2);
    expect(rows[0].text()).toContain("较晚归档");
    expect(rows[1].text()).toContain("较早归档");
  });
});
