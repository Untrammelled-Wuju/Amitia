import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createMemoryHistory, createRouter } from "vue-router";
import ElementPlus, { ElMessageBox } from "element-plus";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import SideNav from "../components/SideNav.vue";
import { useChatStore } from "../stores/chat";
import SidebarProjectBlock from "../components/SidebarProjectBlock.vue";

const mocks = vi.hoisted(() => ({ get: vi.fn(), startDraft: vi.fn(), chooseWorkspace: vi.fn() }));

vi.mock("../composables/useApi", () => ({
  apiClient: { get: mocks.get },
  useApi: () => ({ get: vi.fn(), post: vi.fn() }),
}));
vi.mock("../composables/useConversationWorkspace", () => ({
  useConversationWorkspace: () => ({ startDraftConversation: mocks.startDraft, chooseWorkspaceDirectory: mocks.chooseWorkspace, addWorkspaceToProject: vi.fn(), removeWorkspaceFromProject: vi.fn(), setPrimaryWorkspace: vi.fn() }),
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

async function renderSidebar(path: string, errorHandler = vi.fn()) {
  const pinia = createPinia();
  setActivePinia(pinia);
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: "/chat", component: { template: "<div />" } }],
  });
  await router.push(path);
  await router.isReady();
  const wrapper = mount(SideNav, { global: { plugins: [pinia, router, ElementPlus], config: { errorHandler } } });
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

describe("侧边栏项目弹窗", () => {
  beforeEach(() => {
    localStorage.clear();
    mocks.get.mockResolvedValue({ data: { pinned: [], recent: [], projects: [] } });
  });

  afterEach(() => vi.restoreAllMocks());

  it("新增项目直接选择文件夹，不弹出名称输入框", async () => {
    const errors = vi.fn();
    const { wrapper, chat } = await renderSidebar("/chat", errors);
    const project = { id: "project-test", name: "测试项目", workspaceId: "workspace-test", deviceId: "source", rootUri: "amitia://workspace/@workspace-test/", workspaces: [{ id: "workspace-test", deviceId: "source", name: "测试项目", kind: "local", rootUri: "amitia://workspace/@workspace-test/" }], primaryWorkspaceId: "workspace-test", available: true, status: "available", conversationCount: 0, conversations: [] };
    chat.ownedProjects = true;
    chat.projectIntent = { coreId: "test-core" } as NonNullable<typeof chat.projectIntent>;
    chat.sidebar.projects = [project];
    mocks.chooseWorkspace.mockResolvedValue(project);
    try {
      await flushPromises();
      await wrapper.get('[aria-label="新增项目"]').trigger("click");
      await flushPromises();
      expect(mocks.chooseWorkspace).toHaveBeenCalledOnce();
      expect(errors).not.toHaveBeenCalled();
    } finally {
      chat.ownedProjects = false;
      wrapper.unmount();
    }
  });

  it.each(["重命名", "移除"])("%s项目正确处理取消、关闭、确认和异常", async (operation) => {
    const errors = vi.fn();
    const { wrapper, chat } = await renderSidebar("/chat", errors);
    const project = { id: "project-test", name: "测试项目", workspaceId: "workspace-test", deviceId: "source", rootUri: "amitia://workspace/@workspace-test/", workspaces: [{ id: "workspace-test", deviceId: "source", name: "测试项目", kind: "local", rootUri: "amitia://workspace/@workspace-test/" }], primaryWorkspaceId: "workspace-test", available: true, status: "available", conversationCount: 0, conversations: [] };
    chat.ownedProjects = true;
    chat.projectIntent = { coreId: "test-core" } as NonNullable<typeof chat.projectIntent>;
    chat.sidebar.projects = [project];
    const action = operation === "重命名" ? vi.spyOn(chat, "updateProject").mockResolvedValue(undefined) : vi.spyOn(chat, "hideProject").mockResolvedValue(undefined);
    const dialog = operation === "移除" ? vi.spyOn(ElMessageBox, "confirm") : vi.spyOn(ElMessageBox, "prompt");
    async function trigger() {
      await flushPromises();
      wrapper.getComponent(SidebarProjectBlock).vm.$emit("command", project, operation === "重命名" ? "rename" : "remove");
      await flushPromises();
    }
    try {
      for (const dismissal of ["cancel", "close"]) {
        dialog.mockRejectedValueOnce(dismissal);
        await trigger();
        expect(action).not.toHaveBeenCalled();
        expect(errors).not.toHaveBeenCalled();
      }
      dialog.mockResolvedValueOnce((operation === "移除" ? "confirm" : { value: " 新名称 ", action: "confirm" }) as Awaited<ReturnType<typeof ElMessageBox.prompt>>);
      await trigger();
      expect(action).toHaveBeenCalledOnce();
      if (operation === "重命名") expect(action).toHaveBeenCalledWith(project.id, { name: "新名称" }, project);
      if (operation === "移除") expect(action).toHaveBeenCalledWith(project.id, project);
      const failure = new Error("弹窗异常");
      dialog.mockRejectedValueOnce(failure);
      await trigger();
      expect(action).toHaveBeenCalledOnce();
      expect(errors).toHaveBeenCalledWith(failure, expect.anything(), expect.any(String));
    } finally {
      chat.ownedProjects = false;
      wrapper.unmount();
    }
  });
});
