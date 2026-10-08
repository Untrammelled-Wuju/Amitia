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
  const project = (ownerId: string, roleId = "role", readOnly = false) => ({ id: "same-project", title: `${ownerId}的分组`, ownerId, roleId, revision: 1, readOnly, createdAt: "2026-10-07T00:00:00Z", updatedAt: "2026-10-07T00:00:00Z" });
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
      if (path.endsWith("/historical-roles")) return { data: { roles: historicalRoles, executionScope: structuredClone(scope) } };
      if (path.endsWith("/projects")) return { data: { projects: currentProjects, historicalProjects: config.params.historicalRoleId ? oldProjects : [], executionScope: structuredClone(scope) } };
      if (path.endsWith("/resources")) return { data: { resource: { kind: config.params.kind, id: config.params.id, roleId: scope.roleId, ownerId: scope.resourceOwnerId, revision: 1, deleted: false, body: {} }, executionScope: structuredClone(scope) } };
      if (path.endsWith("/conversations")) return { data: { executionScope: structuredClone(scope), snapshot: { ownerId: scope.resourceOwnerId, resources: [{ kind: "conversation", id: "chat", roleId: "role", ownerId: scope.resourceOwnerId, revision: 1, deleted: false, body: { id: "chat", title: "当前对话", projectId: "same-project", updatedAt: "2026-10-07T00:00:00Z" } }] }, ...(scope.coordinated ? { historicalSnapshot: { ownerId: "source", resources: [{ kind: "conversation", id: "chat", roleId: "old-role", ownerId: "source", revision: 1, deleted: false, body: { id: "chat", title: "原设备对话", projectId: "same-project", updatedAt: "2026-10-06T00:00:00Z" } }] } } : {}) } };
      throw new Error(`测试意外访问 ${path}`);
    });
    mocks.post.mockImplementation(async (path, payload) => {
      if (path.endsWith("/projects")) {
        const hash = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(`${scope.coreId}\0${scope.initiatorDeviceId}\0${payload.requestId}`))), (value) => value.toString(16).padStart(2, "0")).join("");
        const id = `project-${hash}`;
        return { data: { project: { id, title: payload.title }, saved: true, executionScope: { ...scope, requestId: payload.requestId }, acknowledgement: { ownerId: scope.resourceOwnerId, requestId: payload.requestId, versions: { [`project/${id}`]: 1, [`checkpoint/project/${payload.requestId}`]: 1 } } } };
      }
      return { data: { ownerId: scope.resourceOwnerId, versions: { [`${payload.kind}/${payload.id}`]: payload.expectedRevision + 1 } } };
    });
  });

  it("OFF 项目仅由 Source 保存，逻辑分组不调用工作区或旧项目接口", async () => {
    const store = useChatStore();
    await store.fetchSidebar();
    expect(store.sidebar.projects[0]).toMatchObject({ id: ownedProjectReference("source", "same-project"), logical: true, ownerId: "source", rootUri: "", workspaceId: "", conversationCount: 1 });
    expect(store.sidebar.recent).toHaveLength(0);
    await store.createProject({ name: "新分组", expectedExecutionScope: store.projectIntent! });
    expect(mocks.post.mock.calls[0][1]).toMatchObject({ title: "新分组", characterId: "role", expectedExecutionScope: scope });
    expect(mocks.get.mock.calls.every(([path]) => !path.includes("web-chat") && !path.includes("workspaces"))).toBe(true);
    await expect(store.createProject({ name: "非法文件目录", workspaceId: "private-workspace" })).rejects.toThrow("文件访问需单独授权");
  });

  it("ON 当前 Core 与旧 Source 同 ID 项目及聊天分别归组，旧项目只读", async () => {
    scope.coordinated = true;
    scope.roleOwnerId = "core";
    scope.resourceOwnerId = "core";
    owned.policy.value = { ...owned.policy.value!, coordinated: true };
    currentProjects = [project("core")];
    historicalRoles = [{ id: "old-role", name: "旧设备角色" }];
    oldProjects = [project("source", "old-role", true)];
    const store = useChatStore();
    await store.fetchSidebar();
    expect(store.sidebar.projects.map((row) => row.id)).toEqual([ownedProjectReference("core", "same-project"), ownedProjectReference("source", "same-project")]);
    expect(store.sidebar.projects[0].conversations[0].title).toBe("当前对话");
    expect(store.sidebar.projects[1].conversations[0].title).toBe("原设备对话");
    await expect(store.updateProject(store.sidebar.projects[1].id, { name: "改旧设备" })).rejects.toThrow("旧项目为只读");
    await expect(store.deleteProject(store.sidebar.projects[1].id)).rejects.toThrow("旧项目为只读");
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("旧设备多个角色时不猜测 Core 同 ID 角色，显式选择后才加载旧分组", async () => {
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
    expect(store.historicalProjectRoles).toHaveLength(2);
    store.historicalProjectRole = "old-role";
    await store.fetchSidebar();
    expect(mocks.get.mock.calls.filter(([path]) => path.endsWith("/projects")).at(-1)?.[1].params.historicalRoleId).toBe("old-role");
    expect(store.sidebar.projects).toHaveLength(2);
  });

  it("重命名、置顶、删除使用原 Owner 和项目 CAS 版本，对话移动发送裸资源ID", async () => {
    const store = useChatStore();
    await store.fetchSidebar();
    const target = store.sidebar.projects[0];
    await store.updateProject(target.id, { name: "新名称", pinned: true }, target);
    expect(mocks.post.mock.calls[0][1]).toMatchObject({ kind: "project", id: "same-project", expectedRevision: 1, expectedExecutionScope: scope, changes: { title: "新名称", pinned: true } });
    await store.moveConversation(store.sidebar.projects[0].conversations[0].id, target.id);
    expect(mocks.post.mock.calls[1][1]).toMatchObject({ kind: "conversation", id: "chat", changes: { projectId: "same-project" } });
    await store.deleteProject(target.id, target);
    expect(mocks.post.mock.calls[2][1]).toMatchObject({ kind: "project", id: "same-project", deleted: true, expectedRevision: 1 });
    expect(mocks.put).not.toHaveBeenCalled();
    expect(mocks.patch).not.toHaveBeenCalled();
    expect(mocks.del).not.toHaveBeenCalled();
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

  it("逻辑及历史项目菜单不暴露目录访问，历史项目不暴露写操作", () => {
    const logical = { ...project("source", "old-role", true), name: "旧设备分组", logical: true, available: true, status: "logical", workspaceId: "", deviceId: "", rootUri: "", conversationCount: 0, conversations: [] };
    const wrapper = mount(SidebarProjectBlock, { props: { project: logical, active: false, activeConversationId: "" }, global: { stubs: { ElIcon: { template: "<span><slot /></span>" }, ElDropdown: { template: "<div><slot /><slot name='dropdown' /></div>" }, ElDropdownMenu: { template: "<div><slot /></div>" }, ElDropdownItem: { template: "<span><slot /></span>" } } } });
    expect(wrapper.text()).toContain("只读");
    for (const text of ["更换根目录", "资源管理器", "重命名项目", "移除项目", "置顶"]) expect(wrapper.text()).not.toContain(text);
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
    mocks.get.mockImplementation(async (_path, config) => ({ data: { projects: currentProjects, executionScope: { ...scope, roleRevision: config.params.cursor ? 2 : 1 }, nextCursor: config.params.cursor ? "" : "next" } }));
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

  it("逻辑项目草稿只携带执行归属，目录入口拦截且 Core 切换清空旧草稿", async () => {
    const store = useChatStore();
    const workspace = useConversationWorkspace();
    await workspace.startDraftConversation(ownedProjectReference("source", "same-project"));
    expect(workspace.currentWorkspace.value).toMatchObject({ workspaceKind: "logical", rootUri: "", executionScope: scope });
    expect(workspace.getWorkspaceRequestFields()).toEqual({});
    expect(await workspace.chooseWorkspaceDirectory()).toBeNull();
    expect(mocks.post).not.toHaveBeenCalled();
    expect(mocks.get.mock.calls.some(([path]) => path.includes("workspaces"))).toBe(false);
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
    await expect(owned.createProject("新分组", scope)).rejects.toThrow("尚未获得数据所有者保存确认");
  });
});
