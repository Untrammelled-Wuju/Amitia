import { beforeEach, expect, it, vi } from "vitest";

const api = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  interceptors: { request: { use: vi.fn() } },
}));

vi.mock("@/composables/useApi", () => ({ apiClient: api }));

import {
  listAgents,
  listTeams,
} from "@/api/agent-platform";

beforeEach(() => vi.resetAllMocks());

it("归一化空角色集合", async () => {
  api.get.mockResolvedValueOnce({ data: null });
  expect(await listAgents()).toEqual([]);
});

it.each([
  ["团队", listTeams],
])("归一化空%s集合", async (_name, load) => {
  api.get.mockResolvedValueOnce({ data: null });
  expect(await load()).toEqual([]);
});

it("拒绝非数组集合并保留原始返回值", async () => {
  api.get.mockResolvedValue({ data: { invalid: true } });
  await expect(listAgents()).rejects.toThrow("字段 agents");
  await expect(listAgents()).rejects.toThrow('{"invalid":true}');
});

it("角色领域请求只使用 /api/roles 命名空间", async () => {
  api.get.mockResolvedValue({ data: [] });
  await listAgents();
  await listTeams();
  expect(api.get).toHaveBeenNthCalledWith(1, "/api/roles", {
    params: { includeDisabled: true },
  });
  expect(api.get).toHaveBeenNthCalledWith(2, "/api/roles/teams");
});
