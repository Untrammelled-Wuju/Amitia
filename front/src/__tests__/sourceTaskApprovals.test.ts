import { beforeEach, expect, it, vi } from "vitest";
const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/composables/useApi", () => ({ apiClient: api }));
import { listSourceTaskApprovals, decideSourceTaskApproval, revokeSourceTaskApproval } from "../views/kernel/tasks/api";
import type { SourceTaskApproval } from "../views/kernel/tasks/types";

const approval = (): SourceTaskApproval => ({ id: "task-approval-test", revision: 1, status: "pending", expiresAt: "2026-10-06T20:00:00Z", binding: { executionScope: { coreId: "core", initiatorDeviceId: "caller", targetDeviceId: "source" } as any, taskRunId: "run", taskGeneration: 1, executionTarget: { spaceId: "core", deviceId: "source", runtimeId: "runtime", runtimeSessionId: "session", connectionGeneration: 1 }, inputHash: "a".repeat(64), target: { taskId: "task", extensionId: "plugin", moduleId: "module", installedGeneration: 2, definitionFingerprint: "b".repeat(64) } }, permissions: [{ permissionId: "filesystem.read", optional: false }] });
beforeEach(() => vi.resetAllMocks());
it("审批读取和决定均使用本机路径，并携带审批版本", async () => {
  const item = approval();
  api.get.mockResolvedValueOnce({ data: [item] });
  expect(await listSourceTaskApprovals()).toEqual([item]);
  api.post.mockResolvedValueOnce({ data: { ...item, status: "approved", revision: 2 } });
  await decideSourceTaskApproval(item, true);
  expect(api.get).toHaveBeenCalledWith("/internal/device-mesh/task-approvals");
  expect(api.post).toHaveBeenCalledWith("/internal/device-mesh/task-approvals/task-approval-test/decision", { expectedRevision: 1, approved: true });
});
it.each(["hash", "permissions", "revision", "installation", "generation", "session", "connection", "source", "core"])("拒绝缺失或错误的 %s", async (field) => {
  const item = approval();
  if (field === "hash") item.binding.inputHash = "invalid";
  if (field === "permissions") item.permissions = null as any;
  if (field === "revision") item.revision = 0;
  if (field === "installation") item.binding.target.installedGeneration = 0;
  if (field === "generation") item.binding.taskGeneration = 0;
  if (field === "session") item.binding.executionTarget.runtimeSessionId = "";
  if (field === "connection") item.binding.executionTarget.connectionGeneration = 0;
  if (field === "source") item.binding.executionTarget.deviceId = "other";
  if (field === "core") item.binding.executionTarget.spaceId = "other";
  api.get.mockResolvedValueOnce({ data: [item] });
  await expect(listSourceTaskApprovals()).rejects.toThrow("无效");
});
it("拒绝其他请求的决定确认", async () => {
  const item = approval();
  api.post.mockResolvedValueOnce({ data: { ...item, revision: 2, status: "approved", binding: { ...item.binding, inputHash: "c".repeat(64) } } });
  await expect(decideSourceTaskApproval(item, true)).rejects.toThrow("未确认");
});
it("已处理审批不再提交决定", async () => {
  const item = approval(); item.status = "claimed";
  await expect(decideSourceTaskApproval(item, false)).rejects.toThrow("已处理");
  expect(api.post).not.toHaveBeenCalled();
});
it("已消费的单次授权在本机按当前版本撤销", async () => {
  const item = approval(); item.status = "claimed"; item.revision = 3;
  api.post.mockResolvedValueOnce({ data: { ...item, status: "revoked", revision: 4 } });
  await revokeSourceTaskApproval(item);
  expect(api.post).toHaveBeenCalledWith("/internal/device-mesh/task-approvals/task-approval-test/revoke", { expectedRevision: 3 });
});
