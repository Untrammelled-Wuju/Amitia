import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createMemoryHistory, createRouter } from "vue-router";
import ElementPlus, { ElMessage, ElMessageBox } from "element-plus";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import WorkspaceSidebar from "../components/WorkspaceSidebar.vue";
import { useChatStore } from "../stores/chat";
import SidebarProjectBlock from "../components/SidebarProjectBlock.vue";
import SidebarConversationRow from "../components/SidebarConversationRow.vue";
import { contextMenuState, closeContextMenu } from "../components/context-menu/contextMenuStore";

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
vi.mock("@/runtime/runtime-capabilities", async (importOriginal) => ({ ...(await importOriginal<Record<string, unknown>>()), isDesktopShell: () => true }));

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

async function showProjects(wrapper: Awaited<ReturnType<typeof renderSidebar>>["wrapper"]) {
  await wrapper.get('[data-amitia-ui="workspace.view.projects"]').trigger("click");
  await flushPromises();
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
      expect(wrapper.get('[aria-label="搜索对话"]').attributes("placeholder")).toBe("搜索对话...");
      expect(wrapper.find('[data-amitia-ui="workspace.create-menu"]').exists()).toBe(true);
      expect((wrapper.get('[data-amitia-ui="workspace.create-menu"]').element as HTMLElement).closest(".workspace-chat-search")).toBeNull();
      chat.sidebar.recent = [conversation];
      await router.replace({ path: "/chat", query: { conversationId: conversation.id } });
      await flushPromises();
      expect(wrapper.get('[aria-current="page"] .thread-title').text()).toBe(conversation.title);
      await wrapper.get('[data-amitia-ui="workspace.create-menu"]').trigger("click");
      expect(contextMenuState.items.map((item) => item.label)).toEqual(["新对话", "新群聊", "新团队", "新项目"]);
      await contextMenuState.items[0].action?.();
      await flushPromises();
      expect(mocks.startDraft).toHaveBeenCalledOnce();
      expect(router.currentRoute.value.query.conversationId).toBeUndefined();
      expect(wrapper.find('[aria-current="page"]').exists()).toBe(false);
    } finally {
      wrapper.unmount();
    }
  });

  it("requests a fresh draft even when the current route is already the draft route", async () => {
    const { wrapper } = await renderSidebar("/chat");
    const listener = vi.fn();
    window.addEventListener("amitia:draft-created", listener);
    try {
      await wrapper.get('[data-amitia-ui="workspace.create-menu"]').trigger("click");
      await contextMenuState.items[0].action?.();
      await flushPromises();
      expect(listener).toHaveBeenCalledOnce();
    } finally {
      window.removeEventListener("amitia:draft-created", listener);
      wrapper.unmount();
    }
  });

  it("asks for a target project before creating a conversation in project view", async () => {
    const { wrapper, chat, router } = await renderSidebar("/chat");
    const project = { id: "project-target", name: "目标项目", workspaceId: "workspace-target", deviceId: "source", rootUri: "amitia://workspace/@workspace-target/", workspaces: [{ id: "workspace-target", deviceId: "source", name: "目标项目", kind: "local", rootUri: "amitia://workspace/@workspace-target/" }], primaryWorkspaceId: "workspace-target", available: true, status: "available", conversationCount: 0, conversations: [] };
    try {
      chat.sidebar.projects = [project];
      await flushPromises();
      await showProjects(wrapper);
      await wrapper.get('[data-amitia-ui="workspace.create-menu"]').trigger("click");
      await contextMenuState.items.find((item) => item.id === "new-chat")?.action?.();
      await flushPromises();
      expect(document.body.textContent).toContain("选择要为哪个项目创建对话");
      expect(mocks.startDraft).not.toHaveBeenCalled();
      document.querySelector<HTMLButtonElement>(".project-conversation-choice")?.click();
      await flushPromises();
      [...document.querySelectorAll<HTMLButtonElement>("button")].find((button) => button.textContent?.trim() === "创建对话")?.click();
      await flushPromises();
      expect(mocks.startDraft).toHaveBeenCalledWith(project.id);
      expect(router.currentRoute.value.query.projectId).toBe(project.id);
    } finally {
      closeContextMenu();
      wrapper.unmount();
    }
  });

  it("filters conversations in the sidebar search field without opening the global search", async () => {
    const { wrapper, chat } = await renderSidebar("/chat");
    try {
      chat.sidebar.recent = [
        conversation,
        { ...conversation, id: "another-conversation", title: "另一个话题" },
      ];
      await flushPromises();
      await wrapper.get('[aria-label="搜索对话"]').setValue("第一条");
      expect(wrapper.get('[data-amitia-ui="workspace.chat-search-results"]').text()).toContain(conversation.title);
      expect(wrapper.get('[data-amitia-ui="workspace.chat-search-results"]').text()).not.toContain("另一个话题");
      expect(wrapper.find(".search-overlay").exists()).toBe(false);
    } finally {
      wrapper.unmount();
    }
  });

  it("places active search results before archived results and restores archived conversations", async () => {
    const { wrapper, chat } = await renderSidebar("/chat");
    const active = { ...conversation, id: "active-search-result", title: "话题：进行中" };
    const archived = { ...conversation, id: "archived-search-result", title: "话题：已归档", archivedAt: "2026-11-01T00:00:00Z" };
    const restore = vi.spyOn(chat, "restoreConversation").mockResolvedValue(undefined);
    try {
      chat.sidebar.recent = [archived, active];
      await flushPromises();
      await wrapper.get('[aria-label="搜索对话"]').setValue("话题");
      const results = wrapper.findAll(".workspace-search-result");
      expect(results.map((item) => item.get(".workspace-search-result-copy strong").text())).toEqual([active.title, archived.title]);
      expect(results[0].find(".workspace-search-restore").exists()).toBe(false);
      expect(results[1].get(".workspace-search-restore").text()).toBe("撤销归档");
      await results[1].get(".workspace-search-restore").trigger("click");
      await flushPromises();
      expect(restore).toHaveBeenCalledWith(archived.id);
    } finally {
      restore.mockRestore();
      wrapper.unmount();
    }
  });

  it("selects the conversation title when opening an existing conversation directly", async () => {
    const { wrapper, chat } = await renderSidebar(`/chat?conversationId=${conversation.id}`);
    try {
      chat.sidebar.recent = [conversation];
      await flushPromises();
      expect(wrapper.get('[aria-current="page"] .thread-title').text()).toBe(conversation.title);
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

  it("switches between conversation and project views", async () => {
    const { wrapper, chat } = await renderSidebar("/chat");
    const project = { id: "project-order", name: "排序项目", workspaceId: "workspace-order", deviceId: "source", rootUri: "amitia://workspace/@workspace-order/", workspaces: [{ id: "workspace-order", deviceId: "source", name: "排序项目", kind: "local", rootUri: "amitia://workspace/@workspace-order/" }], primaryWorkspaceId: "workspace-order", available: true, status: "available", conversationCount: 0, conversations: [] };
    try {
      chat.sidebar.recent = [conversation];
      chat.sidebar.projects = [project];
      await flushPromises();
      expect(wrapper.findAll(".section-caption > span").map((item) => item.text())).toEqual(["聊天"]);
      expect(wrapper.get('[data-amitia-ui="workspace.view.chat"]').attributes("aria-selected")).toBe("true");
      expect(wrapper.find(".workspace-view-indicator").exists()).toBe(true);
      await showProjects(wrapper);
      expect(wrapper.findAll(".section-caption > span").map((item) => item.text())).toEqual(["项目"]);
      expect(wrapper.get('[data-amitia-ui="workspace.view.projects"]').attributes("aria-selected")).toBe("true");
    } finally {
      wrapper.unmount();
    }
  });

  it("persists the selected sidebar view", async () => {
    const first = await renderSidebar("/chat");
    await showProjects(first.wrapper);
    expect(localStorage.getItem("uai-workspace-sidebar-view")).toBe("project");
    first.wrapper.unmount();

    const second = await renderSidebar("/chat");
    try {
      expect(second.wrapper.get('[data-amitia-ui="workspace.view.projects"]').attributes("aria-selected")).toBe("true");
      expect(second.wrapper.findAll(".section-caption > span").map((item) => item.text())).toEqual(["项目"]);
    } finally {
      second.wrapper.unmount();
    }
  });

  it("分组标题右键只提供多选与对应新增操作", async () => {
    const { wrapper } = await renderSidebar("/chat");
    await wrapper.get('.project-caption').trigger('contextmenu');
    expect(contextMenuState.items.map((item) => item.label)).toEqual(['多选', '新建对话']);
    await contextMenuState.items[0].action?.();
    await flushPromises();
    await showProjects(wrapper);
    await wrapper.get('.project-caption').trigger('contextmenu');
    expect(contextMenuState.items.map((item) => item.label)).toEqual(['取消多选', '新增项目']);
    closeContextMenu();
    wrapper.unmount();
  });

  it("项目右键与更多菜单使用相同命令和禁用状态", async () => {
    const { wrapper, chat } = await renderSidebar("/chat");
    const project = { id: "project-context", name: "右键项目", workspaceId: "main", deviceId: "source", rootUri: "amitia://workspace/@main/", workspaces: [{ id: "main", deviceId: "source", name: "主目录", kind: "local", rootUri: "amitia://workspace/@main/" }, { id: "sub", deviceId: "source", name: "子目录", kind: "local", rootUri: "amitia://workspace/@sub/" }], primaryWorkspaceId: "main", available: true, status: "available", conversationCount: 0, conversations: [] };
    chat.sidebar.projects = [project];
    await flushPromises();
    await showProjects(wrapper);
    await wrapper.get('.project-row').trigger('contextmenu');
    expect(contextMenuState.title).toBe('右键项目');
    expect(contextMenuState.items.map((item) => item.label)).toEqual(['新建对话', '重命名项目', '添加子工作区', '主工作区：主目录', '子工作区：子目录（设为主工作区）', '移除工作区：主目录', '移除工作区：子目录', '置顶', '在资源管理器中打开主工作区', '从侧边栏移除']);
    expect(contextMenuState.items.find((item) => item.id === 'primary:main')?.disabled).toBe(true);
    closeContextMenu();
    wrapper.unmount();
  });

  it("keeps multi-selection while switching between conversations and projects", async () => {
    const { wrapper, chat } = await renderSidebar("/chat");
    const projectConversation = { ...conversation, id: "project-conversation", projectId: "project" };
    chat.sidebar.recent = [conversation];
    chat.sidebar.projects = [{ id: "project", name: "项目", workspaceId: "", deviceId: "", rootUri: "", workspaces: [], primaryWorkspaceId: "", available: true, status: "available", conversationCount: 1, conversations: [projectConversation] }];
    await flushPromises();
    await wrapper.findAll('[title="批量选择"]')[0].trigger("click");
    await wrapper.get('.thread-main').trigger('click');
    expect((wrapper.get('.thread-row input[type="checkbox"]').element as HTMLInputElement).checked).toBe(true);
    await showProjects(wrapper);
    await wrapper.get('.project-main').trigger('click');
    await flushPromises();
    expect(wrapper.findAll('.thread-row input[type="checkbox"]')).toHaveLength(1);
    await wrapper.findAll('.thread-main')[0].trigger('click');
    expect((wrapper.findAll('.thread-row input[type="checkbox"]')[0].element as HTMLInputElement).checked).toBe(true);
    await wrapper.get('[data-amitia-ui="workspace.view.chat"]').trigger("click");
    await flushPromises();
    expect((wrapper.get('.thread-row input[type="checkbox"]').element as HTMLInputElement).checked).toBe(true);
    wrapper.unmount();
  });

  it("starts multi-selection from a project conversation and selects that conversation", async () => {
    const { wrapper, chat } = await renderSidebar("/chat");
    const projectConversation = { ...conversation, id: "project-conversation", projectId: "project" };
    chat.sidebar.recent = [conversation];
    chat.sidebar.projects = [{ id: "project", name: "项目", workspaceId: "", deviceId: "", rootUri: "", workspaces: [], primaryWorkspaceId: "", available: true, status: "available", conversationCount: 1, conversations: [projectConversation] }];
    await flushPromises();
    await showProjects(wrapper);
    await wrapper.get('.project-main').trigger('click');
    await flushPromises();
    const rows = wrapper.findAllComponents(SidebarConversationRow);
    rows[0].vm.$emit("startBatch", projectConversation);
    await flushPromises();
    const checkboxes = wrapper.findAll('.thread-row input[type="checkbox"]');
    expect(checkboxes).toHaveLength(1);
    expect((checkboxes[0].element as HTMLInputElement).checked).toBe(true);
    wrapper.unmount();
  });

  it("toggles select all and deselect all across chat and project conversations", async () => {
    const { wrapper, chat } = await renderSidebar("/chat");
    const projectConversation = { ...conversation, id: "project-conversation", projectId: "project" };
    chat.sidebar.recent = [conversation];
    chat.sidebar.projects = [{ id: "project", name: "项目", workspaceId: "", deviceId: "", rootUri: "", workspaces: [], primaryWorkspaceId: "", available: true, status: "available", conversationCount: 1, conversations: [projectConversation] }];
    await flushPromises();
    await showProjects(wrapper);
    await wrapper.get('.project-main').trigger('click');
    await wrapper.findAll('[title="批量选择"]')[0].trigger('click');
    await wrapper.findAll('.thread-main')[0].trigger('contextmenu');
    expect(contextMenuState.items[0].label).toBe('全选');
    await contextMenuState.items[0].action?.();
    await flushPromises();
    expect(wrapper.findAll('.thread-row input[type="checkbox"]').every((item) => (item.element as HTMLInputElement).checked)).toBe(true);
    await wrapper.findAll('.thread-main')[0].trigger('contextmenu');
    expect(contextMenuState.items[0].label).toBe('取消全选');
    await contextMenuState.items[0].action?.();
    await flushPromises();
    expect(wrapper.findAll('.thread-row input[type="checkbox"]').every((item) => !(item.element as HTMLInputElement).checked)).toBe(true);
    closeContextMenu();
    wrapper.unmount();
  });

  it("shows the project empty state only when there are no projects", async () => {
    const { wrapper, chat } = await renderSidebar("/chat");
    const project = { id: "project-empty-state", name: "已有项目", workspaceId: "workspace-empty-state", deviceId: "source", rootUri: "amitia://workspace/@workspace-empty-state/", workspaces: [{ id: "workspace-empty-state", deviceId: "source", name: "已有项目", kind: "local", rootUri: "amitia://workspace/@workspace-empty-state/" }], primaryWorkspaceId: "workspace-empty-state", available: true, status: "available", conversationCount: 0, conversations: [] };
    try {
      await showProjects(wrapper);
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
      await wrapper.get('[data-amitia-ui="workspace.create-menu"]').trigger("click");
      await contextMenuState.items.find((item) => item.id === "new-project")?.action?.();
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
      expect(document.body.textContent).toContain("主工作区");
      expect(document.body.textContent).toContain("子工作区");
      [...document.querySelectorAll<HTMLButtonElement>("button")].find((button) => button.textContent?.trim() === "创建项目")?.click();
      await flushPromises();
      expect(mocks.registerProjectWorkspaces).toHaveBeenCalledWith(["/tmp/第二个文件夹"]);
      expect(mocks.createProjectFromWorkspaces).toHaveBeenCalledOnce();
      expect(mocks.createProjectFromWorkspaces).toHaveBeenCalledWith("第一个文件夹", [firstWorkspace, secondWorkspace]);
      expect(wrapper.get('[data-amitia-ui="workspace.view.projects"]').attributes("aria-selected")).toBe("true");
      expect(localStorage.getItem("uai-workspace-sidebar-view")).toBe("project");
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
    await showProjects(wrapper);
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
