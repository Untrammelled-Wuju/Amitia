import { beforeEach, expect, it, vi } from "vitest";
import { getRelationshipTimeSettings, getRelationshipTimeState, updateRelationshipTimeSettings } from "../api/temporal";

const api = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }));
vi.mock("../composables/useApi", () => ({ apiClient: api }));

function view(owner = "phone") {
  return { ownerId: owner, roleId: "role", executionScope: { resourceOwnerId: owner, roleId: "role", modeRevision: 1 }, settings: { characterId: "role", enabled: true, updatedAt: "v1" }, state: { interactionCount: 2 }, episodes: [] };
}

beforeEach(() => { api.get.mockReset(); api.put.mockReset(); });

it("关系时间读取和保存使用同一 Owner 权限", async () => {
  api.get.mockResolvedValue({ data: view() });
  const settings = await getRelationshipTimeSettings("role");
  expect((await getRelationshipTimeState("role")).interactionCount).toBe(2);
  api.put.mockResolvedValue({ data: view() });
  await updateRelationshipTimeSettings("role", settings);
  expect(api.put).toHaveBeenCalledWith("/api/device-mesh/v1/business/relationship-time", { settings, expectedExecutionScope: view().executionScope }, { params: { characterId: "role" } });
});

it("刷新状态不能悄悄更换设置的保存 Owner", async () => {
  api.get.mockResolvedValueOnce({ data: view() });
  const settings = await getRelationshipTimeSettings("role");
  api.get.mockResolvedValueOnce({ data: view("core") });
  await getRelationshipTimeState("role");
  api.put.mockResolvedValue({ data: view("core") });
  await expect(updateRelationshipTimeSettings("role", settings)).rejects.toThrow("归属");
  expect(api.put.mock.calls[0][1].expectedExecutionScope.resourceOwnerId).toBe("phone");
});

it("拒绝角色或 Owner 不匹配的响应", async () => {
  api.get.mockResolvedValue({ data: { ...view(), ownerId: "other" } });
  await expect(getRelationshipTimeSettings("role")).rejects.toThrow("归属");
});
