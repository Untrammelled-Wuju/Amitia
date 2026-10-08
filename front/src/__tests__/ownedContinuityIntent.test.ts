import { beforeEach, describe, expect, it, vi } from "vitest";
import { createContinuityThread, getContinuityThread, listContinuityThreads, updateContinuityThread } from "../views/continuity/api";
const cryptoModule = "node:crypto";
const { webcrypto } = await import(cryptoModule);
const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), mesh: { refresh: vi.fn(), historicalRoles: vi.fn(), data: vi.fn(), selectInitialRole: () => "role", coreId: { value: "core" }, policy: { value: {} as any } } }));
vi.mock("../composables/useApi", () => ({ apiClient: { get: mocks.get, post: mocks.post } }));
vi.mock("../composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => mocks.mesh }));
vi.mock("../runtime/runtime-adapter", () => ({ getDeploymentConfig: vi.fn().mockResolvedValue({ mode: "cloud" }) }));

describe("持续事项的原管理意图", () => {
  let scope: any;
  const response = (data: unknown) => ({ data: { code: 200, data } });
  const document = () => ({ ownerId: "device", coreId: "core", modeRevision: 1, executionScope: { ...scope, modeRevision: 1 }, managementExecutionScope: { ...scope }, thread: { id: "original", characterId: "role", revision: 1, status: "paused", title: "事项" }, waits: [], events: [], pausedReason: "服务权限已变化，请确认后恢复" });
  beforeEach(() => {
    vi.stubGlobal("crypto", webcrypto);
    mocks.get.mockReset(); mocks.post.mockReset();
    mocks.mesh.data.mockReset(); mocks.mesh.historicalRoles.mockReset();
    mocks.mesh.refresh.mockResolvedValue(true);
    scope = { authorizationRealm: "realm", spaceId: "core", initiatorDeviceId: "device", targetDeviceId: "device", coreId: "core", providerEpoch: 1, coordinated: false, modeRevision: 3, permissionRevision: 1, targetPermissionRevision: 1, targetProviderEpoch: 1, roleId: "role", roleRevision: 1, roleOwnerId: "device", resourceOwnerId: "device", requestId: "read" };
    mocks.mesh.policy.value = { ...scope };
    mocks.get.mockResolvedValue(response({ documents: [document()], executionScope: { ...scope }, nextCursor: "" }));
    mocks.post.mockImplementation(async (_path, payload) => {
      const result = document();
      result.thread.revision = 2;
      result.executionScope = { ...payload.expectedExecutionScope, requestId: payload.requestId };
      return response({ document: result, acknowledgement: { ownerId: "device", requestId: payload.requestId, versions: { "continuity/original": 2, [`checkpoint/continuity-operation/${payload.requestId}`]: 1 } } });
    });
  });
  it("重新打开暂停事项使用当前管理范围并保留原执行范围", async () => {
    const rows = await listContinuityThreads();
    expect(rows[0].ownedDocument?.executionScope.modeRevision).toBe(3);
    expect(rows[0].ownedDocument?.persistedExecutionScope?.modeRevision).toBe(1);
    await updateContinuityThread("original", { status: "active" }, rows[0].ownedDocument);
    expect(mocks.post.mock.calls[0][1].expectedExecutionScope.modeRevision).toBe(3);
  });
  it("旧详情无法换成同ID的新管理范围", async () => {
    const rows = await listContinuityThreads();
    scope.permissionRevision = 2;
    mocks.get.mockResolvedValue(response(document()));
    await expect(getContinuityThread("original", rows[0].ownedDocument)).rejects.toThrow("已变化");
  });
  it("撤权后重新授予不能继续写入旧表单", async () => {
    const rows = await listContinuityThreads();
    mocks.mesh.policy.value.permissionRevision = 3;
    await expect(updateContinuityThread("original", { title: "迟到修改" }, rows[0].ownedDocument)).rejects.toThrow("已变化");
    expect(mocks.post).not.toHaveBeenCalled();
  });
  it("缺少原请求checkpoint回执不报告保存成功", async () => {
    const rows = await listContinuityThreads();
    mocks.post.mockImplementation(async (_path, payload) => response({ document: { ...document(), executionScope: { ...scope, requestId: payload.requestId }, thread: { ...document().thread, revision: 2 } }, acknowledgement: { ownerId: "device", requestId: payload.requestId, versions: { "continuity/original": 2 } } }));
    await expect(updateContinuityThread("original", { title: "修改" }, rows[0].ownedDocument)).rejects.toThrow("尚未确认保存");
  });
  it("提交时不重新读取同 ID 新记录，始终携带原详情 scope 和 CAS", async () => {
    const rows = await listContinuityThreads();
    mocks.get.mockClear();
    await updateContinuityThread("original", { status: "active" }, rows[0].ownedDocument);
    expect(mocks.get).not.toHaveBeenCalled();
    expect(mocks.post.mock.calls[0][1]).toMatchObject({ expectedRevision: 1, expectedExecutionScope: scope, expectedCoreId: "core", expectedOwnerId: "device", expectedModeRevision: 3 });
  });
  it("原设备历史事项读取 Owner 与同 ID 当前事项隔离，拒绝任何写入", async () => {
    mocks.mesh.historicalRoles.mockResolvedValue([{ id: "old-role", name: "旧角色" }]);
    mocks.mesh.data.mockResolvedValue({ executionScope: { ...scope, coordinated: true, resourceOwnerId: "core" }, historicalSnapshot: { ownerId: "device", resources: [{ kind: "continuity", id: "original", ownerId: "device", roleId: "old-role", revision: 1, body: { ...document(), thread: { ...document().thread, characterId: "old-role" } } }] } });
    const rows = await listContinuityThreads({ historicalRoleId: "old-role" });
    expect(rows[0].ownedDocument).toMatchObject({ ownerId: "device", readOnly: true });
    mocks.get.mockClear();
    const detail = await getContinuityThread("original", rows[0].ownedDocument);
    expect(detail.readOnly).toBe(true);
    expect(mocks.get).not.toHaveBeenCalled();
    await expect(updateContinuityThread("original", { status: "active" }, rows[0].ownedDocument)).rejects.toThrow("旧持续事项只读");
    expect(mocks.post).not.toHaveBeenCalled();
  });
  it("历史角色只能来自已验证目录，不能将 Core 同 ID 角色自动用作旧 Source", async () => {
    mocks.mesh.historicalRoles.mockResolvedValue([{ id: "old-role", name: "旧角色" }]);
    await expect(listContinuityThreads({ historicalRoleId: "role" })).rejects.toThrow("已变化");
    expect(mocks.mesh.data).not.toHaveBeenCalled();
  });
  it("创建表单切换 Core 后拒绝旧 create scope，缺少 scope 也不能提交", async () => {
    await expect(createContinuityThread({ title: "新事项", characterId: "role" })).rejects.toThrow("缺少原始数据归属");
    mocks.mesh.coreId.value = "core-c";
    await expect(createContinuityThread({ title: "迟到事项", characterId: "role" }, scope)).rejects.toThrow("已变化");
    mocks.mesh.coreId.value = "core";
    expect(mocks.post).not.toHaveBeenCalled();
  });
});
