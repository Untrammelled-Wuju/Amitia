import { beforeEach, describe, expect, it, vi } from "vitest";
const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/composables/useApi", () => ({ apiClient: api }));
import { enqueueOwnedTask, prepareOwnedTask, listOwnedDeviceTaskCatalog } from "../views/kernel/tasks/api";

const scope = { coreId: "core-b", targetDeviceId: "phone", roleOwnerId: "phone", resourceOwnerId: "phone", modeRevision: 4, roleId: "role", roleRevision: 2, spaceId: "core-b", initiatorDeviceId: "phone", providerEpoch: 1, coordinated: false, permissionRevision: 1, targetPermissionRevision: 1, targetProviderEpoch: 1, requestId: "", turnId: "", executionId: "" };
const options = { roles: [{ id: "role", name: "角色", revision: 2 }], selectedRole: "role", roleOwnerId: "phone", executionScope: scope };
const submission = { taskDefinitionId: "task", targetDeviceId: "phone", characterId: "role", input: {}, expectedCoreId: "core-b", expectedModeRevision: 4, expectedRoleRevision: 2, requestId: "same-request", expectedExecutionScope: scope };
const catalog = () => ({ executionScope: { ...scope }, revision: "a".repeat(64), entries: [{ reference: { catalogId: "mesh-task-" + "b".repeat(64), coreId: "core-b", deviceId: "phone", sourceTaskId: "task", portableFingerprint: "c".repeat(64) }, definition: { taskId: "task" }, target: { deviceId: "phone", taskId: "task", portableFingerprint: "c".repeat(64), definitionFingerprint: "d".repeat(64), installedGeneration: 2 } }] });
beforeEach(() => vi.resetAllMocks());

describe("普通设备任务提交", () => {
  it("任务目录使用已选择角色的完整授权且不导入来源任务", async () => {
    const page = catalog();
    api.post.mockResolvedValueOnce({ data: page });
    expect(await listOwnedDeviceTaskCatalog(options, "role")).toEqual(page);
    expect(api.post).toHaveBeenCalledTimes(1);
    expect(api.post.mock.calls[0][0]).toBe("/api/device-mesh/v1/business/tasks/catalog");
    expect(api.post.mock.calls[0][1]).toMatchObject({ targetDeviceId: "phone", characterId: "role", expectedExecutionScope: scope, limit: 8 });
  });
  it.each(["core", "device", "task", "fingerprint", "installation", "duplicate", "scope"])("任务目录拒绝 %s 混用", async (kind) => {
    const page = catalog();
    if (kind === "core") page.entries[0].reference.coreId = "core-c";
    if (kind === "device") page.entries[0].target.deviceId = "other";
    if (kind === "task") page.entries[0].reference.sourceTaskId = "other";
    if (kind === "fingerprint") page.entries[0].target.portableFingerprint = "e".repeat(64);
    if (kind === "installation") page.entries[0].target.installedGeneration = 0;
    if (kind === "duplicate") page.entries.push(structuredClone(page.entries[0]));
    if (kind === "scope") page.executionScope.modeRevision++;
    api.post.mockResolvedValueOnce({ data: page });
    await expect(listOwnedDeviceTaskCatalog(options, "role")).rejects.toThrow();
  });
  it("缺少当前角色时不请求任务目录", async () => {
    await expect(listOwnedDeviceTaskCatalog(options, "deleted-role")).rejects.toThrow("请先选择");
    expect(api.post).not.toHaveBeenCalled();
  });
  it("只读取任务授权角色并在准备前后确认同一 Core", async () => {
    api.get.mockResolvedValueOnce({ data: { coreId: "core-b", coordinationAvailable: true } }).mockResolvedValueOnce({ data: options }).mockResolvedValueOnce({ data: { coreId: "core-b", coordinationAvailable: true } });
    expect(await prepareOwnedTask("phone")).toEqual(options);
    expect(api.get.mock.calls[1]).toEqual(["/api/device-mesh/v1/business/tasks/roles", { params: { targetDeviceId: "phone" } }]);
  });
  it.each(["provider", "owner", "target", "revision"])("拦截 %s 变化", async (kind) => {
    const changed = structuredClone(options);
    if (kind === "owner") changed.executionScope.resourceOwnerId = "other";
    if (kind === "target") changed.executionScope.targetDeviceId = "other";
    if (kind === "revision") changed.roles[0].revision = 0;
    api.get.mockResolvedValueOnce({ data: { coreId: "core-b", coordinationAvailable: true } }).mockResolvedValueOnce({ data: changed }).mockResolvedValueOnce({ data: { coreId: kind === "provider" ? "core-c" : "core-b", coordinationAvailable: true } });
    await expect(prepareOwnedTask("phone")).rejects.toThrow("已变化");
    expect(api.post).not.toHaveBeenCalled();
  });
  it("未就绪时停止准备", async () => {
    api.get.mockResolvedValueOnce({ data: { coreId: "core-b", coordinationAvailable: false } });
    await expect(prepareOwnedTask("phone")).rejects.toThrow("尚未就绪");
    expect(api.get).toHaveBeenCalledTimes(1);
  });
  it("重试使用相同请求并呈现服务器确认的完成状态", async () => {
    const task = { taskRunId: "owned-run", status: "succeeded", queued: false };
    api.post.mockRejectedValueOnce(new Error("网络中断")).mockResolvedValueOnce({ data: { task, executionScope: scope } });
    await expect(enqueueOwnedTask(submission)).rejects.toThrow("网络中断");
    expect(await enqueueOwnedTask(submission)).toEqual(task);
    expect(api.post.mock.calls[0]).toEqual(api.post.mock.calls[1]);
    expect(api.post.mock.calls[0][0]).toEqual("/api/device-mesh/v1/business/tasks");
  });
  it.each(["coreId", "targetDeviceId", "roleId", "roleRevision", "modeRevision"])("不接受不匹配的 %s 确认", async (field) => {
    api.post.mockResolvedValueOnce({ data: { task: { taskRunId: "run" }, executionScope: { ...scope, [field]: "changed" } } });
    await expect(enqueueOwnedTask(submission)).rejects.toThrow("有效确认");
  });
});
