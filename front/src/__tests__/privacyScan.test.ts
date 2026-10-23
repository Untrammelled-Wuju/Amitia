import { beforeEach, expect, it, vi } from "vitest";
const mocks = vi.hoisted(() => ({ post: vi.fn() }));
vi.mock("@/composables/useApi", () => ({ apiClient: mocks }));
import { postMask, postScan } from "../views/privacy-scan/api";

beforeEach(() => mocks.post.mockReset());
it("扫描只接受明确的完整结果，保留真实零风险", async () => {
  mocks.post.mockResolvedValue({ data: { data: { status: "completed", complete: true, totalFound: 0 } } });
  expect((await postScan(["messages", "import_items"])).totalFound).toBe(0);
  expect(mocks.post).toHaveBeenCalledWith("/api/privacy/scan", { scope: ["messages", "import_items"] });
  mocks.post.mockResolvedValue({ data: { data: { status: "completed", totalFound: 0 } } });
  await expect(postScan(["messages"])).rejects.toThrow("未完整完成");
});
it("脱敏必须获得全部数据持有方确认，不能使用请求数量冒充成功数量", async () => {
  mocks.post.mockResolvedValue({ data: { data: { masked: true, maskedCount: 0 } } });
  await expect(postMask([{ id: "receipt", sourceTable: "messages" }], "确认脱敏")).rejects.toThrow("未确认全部脱敏");
  mocks.post.mockResolvedValue({ data: { data: { masked: true, maskedCount: 1 } } });
  expect((await postMask([{ id: "receipt", sourceTable: "messages" }], "确认脱敏")).maskedCount).toBe(1);
});

it("删除预览绑定来源和版本，处理中不能冒充已完成", async () => {
  const { previewDeletion, requestDeletion } = await import("../views/privacy-scan/api");
  const target = { id: "message", kind: "message", ownerId: "phone", roleId: "role", revision: 3, executionScope: { modeRevision: 4 } };
  mocks.post.mockResolvedValueOnce({ data: { data: { targets: [target] } } });
  const intents = await previewDeletion({ targetId: "message", targetType: "message", scope: "all" });
  expect(intents[0]?.target).toEqual(target);
  expect(intents[0]?.requestId).toBeTruthy();
  mocks.post.mockResolvedValueOnce({ data: { data: { id: "receipt", status: "failed", retrievalBlocked: false } } });
  const result = await requestDeletion(intents[0]!);
  expect(result.retrievalBlocked).toBe(false);
  expect(result.status).toBe("failed");
  expect(mocks.post).toHaveBeenLastCalledWith("/api/privacy/deletion/request", intents[0]);
});

it("没有真实目标和没有删除回执时拒绝成功", async () => {
  const { previewDeletion, requestDeletion } = await import("../views/privacy-scan/api");
  mocks.post.mockResolvedValue({ data: { data: { targets: [] } } });
  await expect(previewDeletion({ targetId: "missing", targetType: "message", scope: "all" })).rejects.toThrow("没有找到");
  mocks.post.mockResolvedValue({ data: { data: {} } });
  await expect(requestDeletion({ targetId: "message", targetType: "message", scope: "all" })).rejects.toThrow("有效删除回执");
});
