import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createMemoryHistory, createRouter } from "vue-router";
import ElementPlus, { ElMessage, ElMessageBox } from "element-plus";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import WorkspaceSidebar from "../components/WorkspaceSidebar.vue";
import { useChatStore } from "../stores/chat";
import SidebarProjectBlock from "../components/SidebarProjectBlock.vue";

const mocks = vi.hoisted(() => ({ get: vi.fn(), startDraft: vi.fn(), pickProjectWorkspace: vi.fn(), registerProjectWorkspaces: vi.fn(), createProjectFromWorkspaces: vi.fn() }));

vi.mock("../composables/useApi", () => ({
  apiClient: { get: mocks.get },
  useApi: () => ({ get: vi.fn(), post: vi.fn() }),
}));
vi.mock("../composables/useConversationWorkspace", () => ({
  useConversationWorkspace: () => ({ startDraftConversation: mocks.startDraft, pickProjectWorkspace: mocks.pickProjectWorkspace, registerProjectWorkspaces: mocks.registerProjectWorkspaces, createProjectFromWorkspaces: mocks.createProjectFromWorkspaces, addWorkspaceToProject: vi.fn(), removeWorkspaceFromProject: vi.fn(), setPrimaryWorkspace: vi.fn() }),
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
  const wrapper = mount(WorkspaceSidebar, {
    attachTo: document.body,
    global: { plugins: [pinia, router, ElementPlus], config: { errorHandler } },
  });
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
      expect(wrapper.get('[aria-label="搜索"]').text()).toBe("");
      expect(wrapper.get('[data-amitia-ui="workspace.new-chat"]').text()).toBe("新对话");
      chat.sidebar.recent = [conversation];
      await router.replace({ path: "/chat", query: { conversationId: conversation.id } });
      await flushPromises();
      expect(wrapper.get('[aria-current="page"]').text()).toBe(conversation.title);
      await wrapper.get('[data-amitia-ui="workspace.new-chat"]').trigger("click");
      await flushPromises();
      expect(mocks.startDraft).toHaveBeenCalledOnce();
      expect(router.currentRoute.value.query.conversationId).toBeUndefined();
      expect(wrapper.find('[aria-current="page"]').exists()).toBe(false);
    } finally {
      wrapper.unmount();
    }
  });

  it("selects the conversation title when opening an existing conversation directly", async () => {
    const { wrapper, chat } = await renderSidebar(`/chat?conversationId=${conversation.id}`);
    try {
      chat.sidebar.recent = [conversation];
      await flushPromises();
      expect(wrapper.get('[aria-current="page"]').text()).toBe(conversation.title);
    } finally {
      wrapper.unmount();
    }
  });

  it("keeps the page open when creating a conversation fails", async () => {
    const errors = vi.fn();
    const message = vi.spyOn(ElMessage, "error");
    const { wrapper, router } = await renderSidebar(`/chat?conversationId=${conversation.id}`, errors);
    mocks.startDraft.mockRejectedValueOnce(new Error("服务不可用"));
    try {
      await wrapper.get('.section-caption .section-add[aria-label="新建对话"]').trigger("click");
      await flushPromises();
      expect(message).toHaveBeenCalledWith("服务不可用");
      expect(errors).not.toHaveBeenCalled();
      expect(router.currentRoute.value.query.conversationId).toBe(conversation.id);
    } finally {
      message.mockRestore();
      wrapper.unmount();
    }
  });

  it("renders recent conversations before projects", async () => {
    const { wrapper, chat } = await renderSidebar("/chat");
    const project = { id: "project-order", name: "排序项目", workspaceId: "workspace-order", deviceId: "source", rootUri: "amitia://workspace/@workspace-order/", workspaces: [{ id: "workspace-order", deviceId: "source", name: "排序项目", kind: "local", rootUri: "amitia://workspace/@workspace-order/" }], primaryWorkspaceId: "workspace-order", available: true, status: "available", conversationCount: 0, conversations: [] };
    try {
      chat.sidebar.recent = [conversation];
      chat.sidebar.projects = [project];
      await flushPromises();
      expect(wrapper.findAll(".section-caption > span").map((item) => item.text())).toEqual(["最近", "项目"]);
    } finally {
      wrapper.unmount();
    }
  });

  it("shows the project empty state only when there are no projects", async () => {
    const { wrapper, chat } = await renderSidebar("/chat");
    const project = { id: "project-empty-state", name: "已有项目", workspaceId: "workspace-empty-state", deviceId: "source", rootUri: "amitia://workspace/@workspace-empty-state/", workspaces: [{ id: "workspace-empty-state", deviceId: "source", name: "已有项目", kind: "local", rootUri: "amitia://workspace/@workspace-empty-state/" }], primaryWorkspaceId: "workspace-empty-state", available: true, status: "available", conversationCount: 0, conversations: [] };
    try {
      expect(wrapper.text()).toContain("暂无项目");
      chat.sidebar.projects = [project];
      await flushPromises();
      expect(wrapper.text()).not.toContain("暂无项目");
    } finally {
      wrapper.unmount();
    }
  });

});

describe("侧边栏项目弹窗", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
    mocks.get.mockResolvedValue({ data: { pinned: [], recent: [], projects: [] } });
  });

  afterEach(() => vi.restoreAllMocks());

  it("新增项目必须选择文件夹且名称默认取首次选择的文件夹", async () => {
    const errors = vi.fn();
    const { wrapper, chat } = await renderSidebar("/chat", errors);
    const project = { id: "project-test", name: "测试项目", workspaceId: "workspace-test", deviceId: "source", rootUri: "amitia://workspace/@workspace-test/", workspaces: [{ id: "workspace-test", deviceId: "source", name: "测试项目", kind: "local", rootUri: "amitia://workspace/@workspace-test/" }], primaryWorkspaceId: "workspace-test", available: true, status: "available", conversationCount: 0, conversations: [] };
    const firstWorkspace = { id: "workspace-first", deviceId: "source", name: "第一个文件夹", kind: "local", rootUri: "amitia://workspace/@workspace-first/" };
    const secondWorkspace = { id: "workspace-second", deviceId: "source", name: "第二个文件夹", kind: "local", rootUri: "amitia://workspace/@workspace-second/" };
    chat.ownedProjects = true;
    chat.projectIntent = { coreId: "test-core" } as NonNullable<typeof chat.projectIntent>;
    mocks.pickProjectWorkspace.mockResolvedValue(firstWorkspace);
    mocks.registerProjectWorkspaces.mockResolvedValue([secondWorkspace]);
    mocks.createProjectFromWorkspaces.mockResolvedValue(project);
    const warning = vi.spyOn(ElMessage, "warning");
    const originalDesktop = window.amitiaDesktop;
    const droppedFile = new File(["folder"], "第二个文件夹");
    Object.defineProperty(window, "amitiaDesktop", {
      configurable: true,
      value: {
        ...originalDesktop,
        getPathForFile: (file: File) => file === droppedFile ? "/tmp/第二个文件夹" : "",
      },
    });
    try {
      await flushPromises();
      await wrapper.get('[aria-label="新增项目"]').trigger("click");
      await flushPromises();
      expect(document.body.textContent).toContain("新建项目");
      [...document.querySelectorAll<HTMLButtonElement>("button")].find((button) => button.textContent?.trim() === "创建项目")?.click();
      await flushPromises();
      expect(warning).toHaveBeenCalledWith("请选择一个文件夹");
      expect(mocks.createProjectFromWorkspaces).not.toHaveBeenCalled();
      document.querySelector<HTMLButtonElement>('[aria-label="选择项目文件夹"]')?.click();
      await flushPromises();
      const input = document.querySelector<HTMLInputElement>(".project-create-name input");
      expect(input?.value).toBe("第一个文件夹");
      const dropEvent = new Event("drop", { bubbles: true, cancelable: true });
      Object.defineProperty(dropEvent, "dataTransfer", {
        value: { files: [droppedFile], getData: () => "" },
      });
      document.querySelector<HTMLElement>(".project-folder-dropzone")?.dispatchEvent(dropEvent);
      await flushPromises();
      expect(input?.value).toBe("第一个文件夹");
      expect(document.body.textContent).toContain("主文件夹");
      expect(document.body.textContent).toContain("子文件夹");
      [...document.querySelectorAll<HTMLButtonElement>("button")].find((button) => button.textContent?.trim() === "创建项目")?.click();
      await flushPromises();
      expect(mocks.registerProjectWorkspaces).toHaveBeenCalledWith(["/tmp/第二个文件夹"]);
      expect(mocks.createProjectFromWorkspaces).toHaveBeenCalledOnce();
      expect(mocks.createProjectFromWorkspaces).toHaveBeenCalledWith("第一个文件夹", [firstWorkspace, secondWorkspace]);
      expect(errors).not.toHaveBeenCalled();
    } finally {
      Object.defineProperty(window, "amitiaDesktop", { configurable: true, value: originalDesktop });
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
