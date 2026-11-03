import { beforeEach, expect, it, vi } from "vitest";

const api = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  interceptors: { request: { use: vi.fn() } },
}));

vi.mock("@/composables/useApi", () => ({ apiClient: api }));

import {
  getAgentPlatformOverview,
  listActivity,
  listAgents,
  listApprovals,
  listReconciliations,
  listRuntimes,
  listTeams,
} from "@/api/agent-platform";

beforeEach(() => vi.resetAllMocks());

it("归一化空 Agent 集合及概览集合字段", async () => {
  api.get.mockResolvedValueOnce({ data: null });
  expect(await listAgents()).toEqual([]);

  api.get.mockResolvedValueOnce({
    data: {
      agents: null,
      activity: null,
      pendingApprovals: null,
      reconciliations: null,
      summary: {
        running: 0,
        waiting: 0,
        failed: 0,
        completed: 0,
        costMicroUsd: 0,
        averageDurationMs: 0,
      },
    },
  });
  const overview = await getAgentPlatformOverview();
  expect(overview.agents).toEqual([]);
  expect(overview.activity).toEqual([]);
  expect(overview.pendingApprovals).toEqual([]);
  expect(overview.reconciliations).toEqual([]);
});

it.each([
  ["团队", listTeams],
  ["审批", listApprovals],
  ["对账", listReconciliations],
  ["运行时", listRuntimes],
])("归一化空%s集合", async (_name, load) => {
  api.get.mockResolvedValueOnce({ data: null });
  expect(await load()).toEqual([]);
});

it("归一化空活动列表", async () => {
  api.get.mockResolvedValueOnce({
    data: {
      items: null,
      summary: {
        running: 0,
        waiting: 0,
        failed: 0,
        completed: 0,
        costMicroUsd: 0,
        averageDurationMs: 0,
      },
    },
  });
  expect((await listActivity()).items).toEqual([]);
});

it("拒绝非数组集合并保留原始返回值", async () => {
  api.get.mockResolvedValue({ data: { invalid: true } });
  await expect(listAgents()).rejects.toThrow("字段 agents");
  await expect(listAgents()).rejects.toThrow('{"invalid":true}');
});
