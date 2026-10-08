import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
const runtime = vi.hoisted(() => ({ bound: true, base: "https://core" }));
vi.mock("@/composables/useApi", () => ({ apiClient: api }));
vi.mock("@/runtime/runtime-adapter", () => ({
  getRuntimeConnection: async () => ({ apiBaseURL: runtime.base }),
  getDeploymentConfig: async () => ({ mode: runtime.bound ? "cloud" : "local" }),
  isCurrentDevicePaired: async () => runtime.bound,
}));
import { cancelTask, downloadTaskArtifact, getTask, getTaskResult, listTasks, retryTask } from "../views/kernel/tasks/api";
let revision = 1;
const scope = () => ({ coreId: "core", resourceOwnerId: "phone", roleOwnerId: "source", roleId: "role", roleRevision: 2, providerEpoch: revision, modeRevision: 1, permissionRevision: 1, coordinated: false, targetPermissionRevision: 3, authorizationRealm: "core" });
const row = () => ({ taskRunId: "run", status: "running", ownerId: "phone", readOnly: false, executionScope: scope(), managementExecutionScope: scope() });
describe("任务原范围", () => {
  afterEach(() => vi.unstubAllGlobals());
  beforeEach(() => {
    revision = 1; runtime.bound = true; runtime.base = "https://core";
    api.get.mockReset(); api.post.mockReset();
    api.get.mockImplementation(async (path: string) => ({ data: path.endsWith("/me") ? { coreId: "core", coordinationAvailable: true, policy: scope() } : path.endsWith("/tasks") ? { items: [row()], total: 1 } : row() }));
    api.post.mockResolvedValue({ data: row() });
  });
  it("冻结原范围并在取消请求携带全部 scope", async () => {
    const task = (await listTasks()).items[0];
    expect(Object.isFrozen(task.executionScope)).toBe(true);
    await cancelTask("run", undefined, task);
    expect(api.post.mock.calls[0][1].expectedExecutionScope).toEqual(task.executionScope);
  });
  it("拒绝同 Core 权限 ABA 旧行", async () => {
    const task = (await listTasks()).items[0]; revision = 3;
    await expect(cancelTask("run", undefined, task)).rejects.toThrow("范围已变化");
    expect(api.post).not.toHaveBeenCalled();
  });
  it("绑定裸 ID 控制不临时获取新的任务范围", async () => {
    await expect(cancelTask("run")).rejects.toThrow("原始范围");
    expect(api.post).not.toHaveBeenCalled();
  });
  it("历史只读拒绝控制", async () => {
    const task = (await listTasks()).items[0];
    const readOnly = { ...task, readOnly: true };
    await expect(cancelTask("run", undefined, readOnly)).rejects.toThrow();
    expect(api.post).not.toHaveBeenCalled();
  });
  it("详情结果拒绝混用 owner", async () => {
    const task = (await listTasks()).items[0];
    api.get.mockImplementation(async (path: string) => ({ data: path.endsWith("/me") ? { coreId: "core", coordinationAvailable: true, policy: scope() } : { ...row(), executionScope: { ...scope(), resourceOwnerId: "other" } } }));
    await expect(getTaskResult("run", task)).rejects.toThrow("归属");
  });
  it("操作完成后切换策略不得提示成功", async () => {
    const task = (await listTasks()).items[0];
    api.post.mockImplementation(async () => { revision++; return { data: row() }; });
    await expect(cancelTask("run", undefined, task)).rejects.toThrow("范围已变化");
  });
  it("连接 ABA 事件使旧行失效", async () => {
    const task = (await listTasks()).items[0];
    window.dispatchEvent(new Event("amitia:runtime-connection-changed"));
    await expect(getTask("run", task)).rejects.toThrow("范围已变化");
  });
  it("原范围不能用于另一任务 ID", async () => {
    const task = (await listTasks()).items[0];
    await expect(cancelTask("other", undefined, task)).rejects.toThrow("原任务列表");
    expect(api.post).not.toHaveBeenCalled();
  });
  it("重试接收同 authority 的新运行 ID", async () => {
    const task = (await listTasks()).items[0];
    api.post.mockResolvedValue({ data: { ...row(), taskRunId: "retry-run" } });
    expect((await retryTask("run", task)).taskRunId).toBe("retry-run");
  });
  it("本机旧控制接口保留兼容", async () => {
    runtime.bound = false;
    await cancelTask("run");
    expect(api.post).toHaveBeenCalledWith("/api/extensions/tasks/run/cancel", { reason: "user_requested" });
  });
  it("产物下载携原 scope query 并验证原 SHA256", async () => {
    const { webcrypto } = await vi.importActual<{webcrypto: Crypto}>("node:crypto");
    vi.stubGlobal("crypto", webcrypto);
    const task = (await listTasks()).items[0];
    const blob = { arrayBuffer: async () => new Uint8Array([97, 98, 99]).buffer };
    api.get.mockImplementation(async (path: string) => ({ data: path.endsWith("/me") ? { coreId: "core", coordinationAvailable: true, policy: scope() } : blob }));
    expect(await downloadTaskArtifact(task, "artifact", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")).toBe(blob);
    const call = api.get.mock.calls.find(call => String(call[0]).endsWith("/result/artifact"));
    expect(JSON.parse(call?.[1].params.expectedExecutionScope)).toEqual(task.executionScope);
  });
  it("错误产物 hash 拒绝下载", async () => {
    const { webcrypto } = await vi.importActual<{webcrypto: Crypto}>("node:crypto");
    vi.stubGlobal("crypto", webcrypto);
    const task = (await listTasks()).items[0];
    api.get.mockImplementation(async (path: string) => ({ data: path.endsWith("/me") ? { coreId: "core", coordinationAvailable: true, policy: scope() } : { arrayBuffer: async () => new Uint8Array([97]).buffer } }));
    await expect(downloadTaskArtifact(task, "artifact", "a".repeat(64))).rejects.toThrow("哈希");
  });
});
