import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { mount } from "@vue/test-utils";
import SidebarProjectBlock from "../components/SidebarProjectBlock.vue";
import { useChatStore } from "../stores/chat";
import { useDeviceOwnedConversation } from "../composables/useDeviceOwnedConversation";
import { ownedProjectReference } from "../runtime/owned-project-reference";
import { useConversationWorkspace } from "../composables/useConversationWorkspace";
const cryptoModule = "node:crypto";
const { webcrypto } = await import(cryptoModule);

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), del: vi.fn() }));
vi.mock("../composables/useApi", () => ({ apiClient: { get: mocks.get, post: mocks.post, put: mocks.put, patch: mocks.patch, delete: mocks.del }, useApi: () => ({ post: mocks.post }) }));

describe("按角色和数据所有者保存的逻辑项目", () => {
  let scope: any;
  let historicalRoles: any[];
  let currentProjects: any[];
  let oldProjects: any[];
  const owned = useDeviceOwnedConversation();
  const project = (ownerId: string, roleId = "role", readOnly = false) => ({ id: "same-project", title: `${ownerId}的项目`, workspaces: [{ id: `workspace-${ownerId}`, deviceId: ownerId, name: `${ownerId}工作区`, kind: "local", rootUri: `amitia://workspace/@workspace-${ownerId}/` }], primaryWorkspaceId: `workspace-${ownerId}`, ownerId, roleId, revision: 1, readOnly, createdAt: "2026-10-07T00:00:00Z", updatedAt: "2026-10-07T00:00:00Z" });
  beforeEach(() => {
    vi.stubGlobal("crypto", webcrypto);
    Object.values(mocks).forEach((mock) => mock.mockReset());
    setActivePinia(createPinia());
    owned.stopLocal("");
    scope = { authorizationRealm: "mesh", spaceId: "core", initiatorDeviceId: "source", targetDeviceId: "source", coreId: "core", providerEpoch: 1, coordinated: false, modeRevision: 1, permissionRevision: 1, targetPermissionRevision: 1, targetProviderEpoch: 1, roleId: "role", roleRevision: 1, roleOwnerId: "source", resourceOwnerId: "source", requestId: "list", turnId: "list", executionId: "list" };
    owned.enabled.value = true;
    owned.coreId.value = "core";
    owned.roles.value = [{ id: "role", name: "当前角色", revision: 1 }];
    owned.policy.value = { coordinated: false, providerEpoch: 1, modeRevision: 1, permissionRevision: 1, selectedRole: "role" };
    historicalRoles = [];
    currentProjects = [project("source")];
    oldProjects = [];
    mocks.get.mockImplementation(async (path: string, config: any) => {
      if (path.endsWith("/historical-roles")) {
        if (!scope.coordinated) throw new Error("禁止将设备专属数据保存到其他设备");
        return { data: { roles: historicalRoles, executionScope: structuredClone(scope) } };
      }
      if (path.endsWith("/projects")) return { data: { projects: currentProjects, historicalProjects: config.params.historicalRoleId ? oldProjects : [], executionScope: structuredClone(scope) } };
      if (path.endsWith("/resources")) return { data: { resource: { kind: config.params.kind, id: config.params.id, roleId: scope.roleId, ownerId: scope.resourceOwnerId, revision: 1, deleted: false, body: {} }, executionScope: structuredClone(scope) } };
      if (path.endsWith("/conversations")) return { data: { executionScope: structuredClone(scope), snapshot: { ownerId: scope.resourceOwnerId, resources: [{ kind: "conversation", id: "chat", roleId: "role", ownerId: scope.resourceOwnerId, revision: 1, deleted: false, body: { id: "chat", title: "当前对话", projectId: "same-project", updatedAt: "2026-10-07T00:00:00Z" } }] }, ...(scope.coordinated ? { historicalSnapshot: { ownerId: "source", resources: [{ kind: "conversation", id: "chat", roleId: "old-role", ownerId: "source", revision: 1, deleted: false, body: { id: "chat", title: "原设备对话", projectId: "same-project", updatedAt: "2026-10-06T00:00:00Z" } }] } } : {}) } };
      throw new Error(`测试意外访问 ${path}`);
    });
    mocks.post.mockImplementation(async (path, payload) => {
      if (path.endsWith("/projects")) {
        const hash = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(`${scope.coreId}\0${scope.initiatorDeviceId}\0${payload.requestId}`))), (value) => value.toString(16).padStart(2, "0")).join("");
        const id = `project-${hash}`;
        return { data: { project: { id, title: payload.title, workspaces: payload.workspaces, primaryWorkspaceId: payload.primaryWorkspaceId }, saved: true, executionScope: { ...scope, requestId: payload.requestId }, acknowledgement: { ownerId: scope.resourceOwnerId, requestId: payload.requestId, versions: { [`project/${id}`]: 1, [`checkpoint/project/${payload.requestId}`]: 1 } } } };
      }
      return { data: { ownerId: scope.resourceOwnerId, versions: { [`${payload.kind}/${payload.id}`]: payload.expectedRevision + 1 } } };
    });
  });

  it("普通新对话不依赖侧边栏接口", async () => {
    const store = useChatStore();
    const fetchSidebar = vi.spyOn(store, "fetchSidebar").mockRejectedValue(new Error("接口不可用"));
    await expect(store.fetchSidebar()).rejects.toThrow("接口不可用");
    await expect(useConversationWorkspace().startDraftConversation()).resolves.toBeNull();
    expect(fetchSidebar).toHaveBeenCalledOnce();
  });

  it("OFF 项目由 Source 保存，并携带多工作区集合", async () => {
    const store = useChatStore();
    await store.fetchSidebar();
    expect(mocks.get.mock.calls.some(([path]) => path.endsWith("/historical-roles"))).toBe(false);
    expect(store.sidebar.projects[0]).toMatchObject({ id: ownedProjectReference("source", "same-project"), ownerId: "source", rootUri: "amitia://workspace/@workspace-source/", workspaceId: "workspace-source", primaryWorkspaceId: "workspace-source", conversationCount: 1 });
    expect(store.sidebar.recent).toHaveLength(0);
    await store.createProject({ name: "新项目", expectedExecutionScope: store.projectIntent!, workspaces: [{ id: "workspace-new", deviceId: "source", name: "新项目", kind: "local", rootUri: "amitia://workspace/@workspace-new/" }], primaryWorkspaceId: "workspace-new" });
    expect(mocks.post.mock.calls[0][1]).toMatchObject({ title: "新项目", primaryWorkspaceId: "workspace-new", characterId: "role", expectedExecutionScope: scope });
    expect(mocks.get.mock.calls.every(([path]) => !path.includes("web-chat") && !path.includes("workspaces"))).toBe(true);
  });

  it("ON 只展示当前 Core 项目，旧逻辑项目不再展示", async () => {
    scope.coordinated = true;
    scope.roleOwnerId = "core";
    scope.resourceOwnerId = "core";
    owned.policy.value = { ...owned.policy.value!, coordinated: true };
    currentProjects = [project("core")];
    historicalRoles = [{ id: "old-role", name: "旧设备角色" }];
    oldProjects = [project("source", "old-role", true)];
    const store = useChatStore();
    await store.fetchSidebar();
    expect(store.sidebar.projects.map((row) => row.id)).toEqual([ownedProjectReference("core", "same-project")]);
    expect(store.sidebar.projects[0].conversations[0].title).toBe("当前对话");
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("旧设备历史项目不再接入侧边栏查询", async () => {
    scope.coordinated = true;
    scope.roleOwnerId = "core";
    scope.resourceOwnerId = "core";
    owned.policy.value = { ...owned.policy.value!, coordinated: true };
    currentProjects = [project("core")];
    historicalRoles = [{ id: "role", name: "同名旧角色" }, { id: "old-role", name: "另一旧角色" }];
    oldProjects = [project("source", "old-role", true)];
    const store = useChatStore();
    await store.fetchSidebar();
    expect(mocks.get.mock.calls.find(([path]) => path.endsWith("/projects"))?.[1].params).not.toHaveProperty("historicalRoleId");
    expect(store.sidebar.projects).toHaveLength(1);
  });

  it("重命名、置顶、移动对话和隐藏使用原 Owner 与 CAS 版本", async () => {
    const store = useChatStore();
    await store.fetchSidebar();
    const target = store.sidebar.projects[0];
    await store.updateProject(target.id, { name: "新名称", pinned: true }, target);
    expect(mocks.post.mock.calls[0][1]).toMatchObject({ kind: "project", id: "same-project", expectedRevision: 1, expectedExecutionScope: scope, changes: { title: "新名称", pinned: true } });
    await store.moveConversation(store.sidebar.projects[0].conversations[0].id, "");
    expect(mocks.post.mock.calls[1][1]).toMatchObject({ kind: "conversation", id: "chat", changes: { projectId: "" } });
    await store.hideProject(target.id, target);
    expect(mocks.post.mock.calls[2][1]).toMatchObject({ kind: "project", id: "same-project", changes: { hidden: true }, expectedRevision: 1 });
    expect(mocks.put).not.toHaveBeenCalled();
    expect(mocks.patch).not.toHaveBeenCalled();
    expect(mocks.del).not.toHaveBeenCalled();
  });

  it("允许创建没有文件夹的项目", async () => {
    const store = useChatStore();
    await store.fetchSidebar();
    const created = await store.createProject({ name: "空项目", expectedExecutionScope: store.projectIntent!, workspaces: [], primaryWorkspaceId: "" });
    expect(mocks.post.mock.calls[0][1]).toMatchObject({ title: "空项目", workspaces: [], primaryWorkspaceId: "" });
    expect(created.workspaces).toEqual([]);
    expect(created.available).toBe(true);
  });

  it("允许不同项目复用同一个文件夹工作区", async () => {
    const store = useChatStore();
    const workspace = useConversationWorkspace();
    await store.fetchSidebar();
    const sharedWorkspace = { id: "workspace-source", deviceId: "source", name: "Downloads", kind: "local", rootUri: "amitia://workspace/@workspace-source/" };
    const created = await workspace.createProjectFromWorkspaces("另一个项目", [sharedWorkspace]);
    expect(mocks.post.mock.calls[0][1]).toMatchObject({
      title: "另一个项目",
      workspaces: [sharedWorkspace],
      primaryWorkspaceId: "workspace-source",
    });
    expect(created.workspaces).toEqual([expect.objectContaining(sharedWorkspace)]);
  });

  it("添加工作区和切换主根使用同一项目版本提交", async () => {
    const store = useChatStore();
    await store.fetchSidebar();
    const target = store.sidebar.projects[0];
    const workspaces = [
      ...target.workspaces,
      { id: "workspace-b", deviceId: "source", name: "B", kind: "local", rootUri: "amitia://workspace/@workspace-b/" },
    ];
    await store.updateProject(target.id, { workspaces, primaryWorkspaceId: "workspace-b" }, target);
    expect(mocks.post.mock.calls[0][1]).toMatchObject({
      kind: "project",
      id: "same-project",
      expectedRevision: 1,
      changes: {
        workspaces,
        primaryWorkspaceId: "workspace-b",
      },
    });
  });

  it.each([false, true])("添加工作区仅在项目为空时设置主工作区，空项目=%s", async (empty) => {
    if (empty) currentProjects = [{ ...project("source"), workspaces: [], primaryWorkspaceId: "" }];
    else currentProjects[0].workspaces.unshift({ id: "workspace-other", deviceId: "source", name: "其他", kind: "local", rootUri: "amitia://workspace/@workspace-other/" });
    const store = useChatStore();
    await store.fetchSidebar();
    const target = store.sidebar.projects[0];
    const workspace = { id: "workspace-added", name: "新增", kind: "local", rootUri: "amitia://workspace/@workspace-added/" };
    mocks.post.mockResolvedValueOnce(workspace);
    await useConversationWorkspace().addWorkspaceToProject(target);
    expect(mocks.post.mock.calls[1][1]).toMatchObject({
      kind: "project",
      expectedRevision: 1,
      changes: {
        primaryWorkspaceId: empty ? workspace.id : "workspace-source",
        workspaces: [...target.workspaces, { ...workspace, deviceId: "source" }],
      },
    });
  });

  it("重复添加工作区不会提交或切换主工作区", async () => {
    const store = useChatStore();
    await store.fetchSidebar();
    const target = store.sidebar.projects[0];
    mocks.post.mockResolvedValueOnce(target.workspaces[0]);
    await expect(useConversationWorkspace().addWorkspaceToProject(target)).rejects.toThrow("该工作区已经属于当前项目");
    expect(mocks.post).toHaveBeenCalledTimes(1);
  });

  it("移除主工作区后提升剩余工作区", async () => {
    const store = useChatStore();
    currentProjects[0].workspaces.push({ id: "workspace-child", deviceId: "source", name: "子目录", kind: "local", rootUri: "amitia://workspace/@workspace-child/" });
    await store.fetchSidebar();
    const target = store.sidebar.projects[0];
    await useConversationWorkspace().removeWorkspaceFromProject(target, target.primaryWorkspaceId);
    expect(mocks.post.mock.calls[0][1].changes).toEqual({ workspaces: [target.workspaces[1]], primaryWorkspaceId: "workspace-child" });
  });

  it.each([false, true])("项目菜单区分主子工作区和添加入口，空项目=%s", async (empty) => {
    const store = useChatStore();
    if (empty) currentProjects = [{ ...project("source"), workspaces: [], primaryWorkspaceId: "" }];
    else currentProjects[0].workspaces.push({ id: "workspace-child", deviceId: "source", name: "子目录", kind: "local", rootUri: "amitia://workspace/@workspace-child/" });
    await store.fetchSidebar();
    const wrapper = mount(SidebarProjectBlock, { props: { project: store.sidebar.projects[0], active: false, activeConversationId: "" }, global: { stubs: { ElIcon: { template: "<span><slot /></span>" }, ElDropdown: { template: "<div><slot /><slot name='dropdown' /></div>" }, ElDropdownMenu: { template: "<div><slot /></div>" }, ElDropdownItem: { template: "<span><slot /></span>" } } } });
    expect(wrapper.text()).toContain(empty ? "添加主工作区" : "添加子工作区");
    expect(wrapper.text()).not.toContain(empty ? "添加子工作区" : "添加主工作区");
    if (!empty) {
      expect(wrapper.text()).toContain("主工作区：source工作区");
      expect(wrapper.text()).toContain("子工作区：子目录（设为主工作区）");
    }
    wrapper.unmount();
  });

  it("B→C 后旧同 ID 项目操作被拦截，原项目列表立即失效", async () => {
    const store = useChatStore();
    await store.fetchSidebar();
    const target = store.sidebar.projects[0];
    owned.stopLocal("提供者已切换");
    owned.coreId.value = "core-c";
    scope.coreId = "core-c";
    scope.resourceOwnerId = "core-c";
    expect(store.sidebar.projects).toHaveLength(0);
    await expect(store.updateProject(target.id, { name: "不能修改C" }, target)).rejects.toThrow("所属设备已变化");
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("只读项目保留原有菜单限制", () => {
    const readOnly = { ...project("source", "old-role", true), name: "只读项目", available: true, status: "available", workspaceId: "workspace-source", deviceId: "source", rootUri: "amitia://workspace/@workspace-source/", conversationCount: 0, conversations: [] };
    const wrapper = mount(SidebarProjectBlock, { props: { project: readOnly, active: false, activeConversationId: "" }, global: { stubs: { ElIcon: { template: "<span><slot /></span>" }, ElDropdown: { template: "<div><slot /><slot name='dropdown' /></div>" }, ElDropdownMenu: { template: "<div><slot /></div>" }, ElDropdownItem: { template: "<span><slot /></span>" } } } });
    expect(wrapper.text()).toContain("只读");
    for (const text of ["重命名项目", "添加主工作区", "添加子工作区", "置顶", "从侧边栏移除"]) expect(wrapper.text()).not.toContain(text);
    expect(wrapper.get(".project-create-action").attributes("disabled")).toBeDefined();
    wrapper.unmount();
  });

  it.each([true, false])("当前与历史分页独立结束，当前先结束=%s", async (currentFirst) => {
    scope.coordinated = true;
    scope.roleOwnerId = "core";
    scope.resourceOwnerId = "core";
    owned.policy.value = { ...owned.policy.value!, coordinated: true };
    historicalRoles = [{ id: "old-role", name: "旧角色" }];
    const inheritedGet = mocks.get.getMockImplementation()!;
    mocks.get.mockImplementation(async (path, config) => {
      if (!path.endsWith("/projects")) return inheritedGet(path, config);
      const currentPage = config.params.cursor === "current-next" ? 2 : 1;
      const historyPage = config.params.historicalCursor === "history-next" ? 2 : 1;
      return { data: {
        projects: [{ ...project("core"), id: `current-${currentPage}` }],
        historicalProjects: config.params.historicalRoleId ? [{ ...project("source", "old-role", true), id: `history-${historyPage}` }] : [],
        executionScope: structuredClone(scope),
        nextCursor: !currentFirst && currentPage === 1 ? "current-next" : "",
        nextHistoricalCursor: currentFirst && historyPage === 1 ? "history-next" : "",
      } };
    });
    const result = await owned.projects("role");
    expect(result.projects.map((row) => row.id)).toEqual(currentFirst ? ["current-1"] : ["current-1", "current-2"]);
    expect(result.historicalProjects.map((row) => row.id)).toEqual(currentFirst ? ["history-1", "history-2"] : ["history-1"]);
    expect(mocks.get.mock.calls.filter(([path]) => path.endsWith("/projects"))).toHaveLength(2);
    if (!currentFirst) expect(mocks.get.mock.calls.at(-1)?.[1].params).not.toHaveProperty("historicalRoleId");
  });

  it("项目分页期间角色版本改变时拒绝拼接混合快照", async () => {
    const inheritedGet = mocks.get.getMockImplementation()!;
    mocks.get.mockImplementation(async (path, config) => path.endsWith("/projects")
      ? { data: { projects: currentProjects, executionScope: { ...scope, roleRevision: config.params.cursor ? 2 : 1 }, nextCursor: config.params.cursor ? "" : "next" } }
      : inheritedGet(path, config));
    await expect(owned.projects("role")).rejects.toThrow("项目数据归属已变化");
  });

  it("Core 切换后迟到的项目响应不能恢复原侧栏", async () => {
    const store = useChatStore();
    let resolveProjects!: (response: any) => void;
    const inheritedGet = mocks.get.getMockImplementation()!;
    mocks.get.mockImplementation((path, config) => path.endsWith("/projects") ? new Promise((resolve) => { resolveProjects = resolve; }) : inheritedGet(path, config));
    const pending = store.fetchSidebar();
    await vi.waitFor(() => expect(resolveProjects).toBeTypeOf("function"));
    owned.stopLocal("提供者已切换");
    owned.coreId.value = "core-c";
    resolveProjects({ data: { projects: currentProjects, executionScope: structuredClone(scope) } });
    await expect(pending).rejects.toThrow("项目数据归属已变化");
    expect(store.sidebar.projects).toHaveLength(0);
    expect(store.projectIntent).toBeNull();
  });

  it("并发加载侧栏只发起一次项目请求", async () => {
    const store = useChatStore();
    let resolveProjects!: (response: any) => void;
    let projectCalls = 0;
    const inheritedGet = mocks.get.getMockImplementation()!;
    mocks.get.mockImplementation((path, config) => {
      if (path.endsWith("/projects")) {
        projectCalls += 1;
        return new Promise((resolve) => {
          resolveProjects = resolve;
        });
      }
      return inheritedGet(path, config);
    });
    const first = store.fetchSidebar();
    const second = store.fetchSidebar();
    await vi.waitFor(() => expect(resolveProjects).toBeTypeOf("function"));
    resolveProjects({ data: { projects: currentProjects, executionScope: structuredClone(scope) } });
    await Promise.all([first, second]);
    expect(projectCalls).toBe(1);
    expect(store.sidebar.projects).toHaveLength(1);
  });

  it("项目草稿携带完整工作区集合，Core 切换清空旧草稿", async () => {
    const store = useChatStore();
    const workspace = useConversationWorkspace();
    await workspace.startDraftConversation(ownedProjectReference("source", "same-project"));
    expect(workspace.currentWorkspace.value).toMatchObject({ projectId: ownedProjectReference("source", "same-project"), workspaceId: "workspace-source", primaryWorkspaceId: "workspace-source", executionScope: scope });
    expect(workspace.currentWorkspace.value?.workspaces).toHaveLength(1);
    expect(workspace.getWorkspaceRequestFields()).toEqual({ workspaceId: "workspace-source", workspaceDeviceId: "source" });
    owned.stopLocal("提供者已切换");
    owned.coreId.value = "core-c";
    expect(store.currentProjectId).toBe("");
    expect(workspace.currentWorkspace.value).toBeNull();
  });

  it.each(["checkpoint", "resource", "realm"])("创建项目时拒绝错误 %s 确认", async (invalid) => {
    const inheritedPost = mocks.post.getMockImplementation()!;
    mocks.post.mockImplementation(async (path, payload) => {
      const response = await inheritedPost(path, payload);
      if (invalid === "checkpoint") delete response.data.acknowledgement.versions[`checkpoint/project/${payload.requestId}`];
      if (invalid === "resource") response.data.project.id = "project-of-another-request";
      if (invalid === "realm") response.data.executionScope.authorizationRealm = "other-realm";
      return response;
    });
    await expect(owned.createProject("新项目", scope, [{ id: "workspace-new", deviceId: "source", name: "新项目", kind: "local", rootUri: "amitia://workspace/@workspace-new/" }], "workspace-new")).rejects.toThrow("尚未获得数据所有者保存确认");
  });
});
