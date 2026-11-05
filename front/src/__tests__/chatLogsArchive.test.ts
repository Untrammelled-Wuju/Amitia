import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

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
      projects: mocks.projects,
      conversations: mocks.conversations,
      edit: mocks.edit,
      allMessages: vi.fn(async () => []),
    }),
  };
});

import ChatLogsView from "@/views/chat-logs/ChatLogsView.vue";
import { useChatStore } from "@/stores/chat";

function mountArchive() {
  const pinia = createPinia();
  setActivePinia(pinia);
  return mount(ChatLogsView, {
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
      directives: { loading: {} },
      stubs: {
        ArchiveConversationIcon: true,
        ChatBubble: true,
      },
    },
  });
}

describe("chat logs archive", () => {
  beforeEach(() => {
    mocks.conversations.mockReset();
    mocks.edit.mockReset();
    mocks.projects.mockReset();
    mocks.refresh.mockReset();
    mocks.refresh.mockResolvedValue(true);
    mocks.edit.mockResolvedValue({});
    mocks.projects.mockResolvedValue({ projects: [] });
  });

  it("shows a newly archived conversation without a manual refresh", async () => {
    const archivedItems: Array<Record<string, unknown>> = [];
    mocks.conversations.mockImplementation(async () => archivedItems);
    const wrapper = mountArchive();
    await flushPromises();
    expect(wrapper.text()).not.toContain("新归档会话");

    archivedItems.push({
      id: "archived-1",
      title: "新归档会话",
      messageCount: 1,
      archivedAt: "2026-09-19T00:00:00Z",
    });
    await useChatStore().archiveConversation("archived-1");
    await flushPromises();

    expect(mocks.edit).toHaveBeenCalledWith("conversation", "archived-1", { archived: true });
    expect(wrapper.text()).toContain("新归档会话");
  });

  it("orders archived conversations by archived time", async () => {
    mocks.conversations.mockResolvedValue([
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
    ]);
    const wrapper = mountArchive();
    await flushPromises();
    const rows = wrapper.findAll(".archive-item");

    expect(rows).toHaveLength(2);
    expect(rows[0].text()).toContain("较晚归档");
    expect(rows[1].text()).toContain("较早归档");
  });
});
