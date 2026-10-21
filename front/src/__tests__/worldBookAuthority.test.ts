import { beforeEach, expect, it, vi } from "vitest";
import { ref } from "vue";
import { useWorldBook } from "../composables/useWorldBook";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
  refresh: vi.fn(),
  data: vi.fn(),
}));

vi.mock("../ui-index", () => ({ apiClient: { post: mocks.post } }));
vi.mock("../composables/useDeviceOwnedConversation", () => ({
  useDeviceOwnedConversation: () => ({
    enabled: ref(true),
    coreId: ref("b"),
    roleOwnerId: ref("b"),
    notice: ref(""),
    roles: ref([{ id: "one", name: "角色一", revision: 1 }]),
    refresh: mocks.refresh,
    selectInitialRole: () => "one",
    data: mocks.data,
  }),
}));

function scope() {
  return {
    spaceId: "space",
    initiatorDeviceId: "device-a",
    targetDeviceId: "b",
    coreId: "b",
    providerEpoch: 1,
    coordinated: true,
    modeRevision: 1,
    permissionRevision: 1,
    roleOwnerId: "device-a",
    resourceOwnerId: "b",
    roleId: "one",
    roleRevision: 1,
  };
}

function snapshot() {
  return {
    executionScope: scope(),
    snapshot: {
      ownerId: "b",
      resources: [
        { kind: "worldbook", id: "global", roleId: "@global-worldbook", ownerId: "b", revision: 2, deleted: false, body: { scope: "global", matchType: "keyword", matchPattern: "晨星", matchScope: "full_context", injectContent: "全局规则", priority: 1, enabled: true } },
        { kind: "worldbook", id: "role", roleId: "one", ownerId: "b", revision: 1, deleted: false, body: { scope: "role", matchType: "exact", matchPattern: "茶", matchScope: "user_message", injectContent: "角色规则", priority: 2, enabled: true } },
      ],
    },
  };
}

beforeEach(() => {
  Object.values(mocks).forEach((mock) => mock.mockReset());
  mocks.refresh.mockResolvedValue(true);
  mocks.data.mockImplementation(async () => snapshot());
  mocks.post.mockImplementation(async (_url, payload) => ({
    code: 200,
    data: {
      saved: true,
      executionScope: { ...scope(), requestId: payload.requestId, worldbookRoleId: payload.rule.scope === "global" ? "@global-worldbook" : "one" },
      acknowledgement: {
        ownerId: "b",
        requestId: payload.requestId,
        versions: { [`checkpoint/worldbook/${payload.requestId}`]: 1 },
      },
      rule: payload.action === "delete" ? undefined : { ...payload.rule, id: payload.rule.id || "created", ownerId: "b", roleId: "one", revision: 1 },
    },
  }));
});

it("加载全局和角色世界书并保留所有者与修订", async () => {
  const book = useWorldBook();
  await book.fetchRules();
  expect(book.rules.value.map((rule) => [rule.id, rule.scope, rule.revision])).toEqual([
    ["global", "global", 2],
    ["role", "role", 1],
  ]);
  expect(book.loadedContext.value).not.toBe("");
});

it("创建携带原执行范围并核验所有者确认", async () => {
  const book = useWorldBook();
  await book.fetchRules();
  await book.createRule({ scope: "role", matchType: "keyword", matchPattern: "雨", injectContent: "下雨" });
  expect(mocks.post).toHaveBeenCalledWith(
    "/api/device-mesh/v1/business/worldbook/manage",
    expect.objectContaining({
      characterId: "one",
      targetDeviceId: "b",
      expectedExecutionScope: scope(),
      action: "create",
      rule: expect.objectContaining({ scope: "role", characterId: "one" }),
    }),
  );
});

it("所有者确认不匹配时拒绝显示保存成功", async () => {
  mocks.post.mockImplementation(async (_url, payload) => ({
    data: {
      saved: true,
      executionScope: { ...scope(), requestId: payload.requestId },
      acknowledgement: { ownerId: "other", requestId: payload.requestId, versions: { [`checkpoint/worldbook/${payload.requestId}`]: 1 } },
    },
  }));
  const book = useWorldBook();
  await book.fetchRules();
  await expect(book.createRule({ scope: "role", matchType: "keyword", matchPattern: "雨", injectContent: "下雨" })).rejects.toThrow("所有者保存确认");
});

it("Core 变化后旧页面不能继续更新世界书", async () => {
  const book = useWorldBook();
  await book.fetchRules();
  const expected = book.loadedContext.value;
  mocks.data.mockRejectedValue(new Error("Core 已变化"));
  mocks.refresh.mockResolvedValue(false);
  await expect(book.updateRule("role", { injectContent: "新内容" }, expected)).rejects.toThrow();
  expect(mocks.post).not.toHaveBeenCalled();
});

it("批量导入中保存失败后不再提交后续规则", async () => {
  const book = useWorldBook();
  await book.fetchRules();
  mocks.post.mockRejectedValueOnce(new Error("请求冲突"));
  await expect(book.createRules([
    { scope: "role", matchType: "keyword", matchPattern: "一", injectContent: "一" },
    { scope: "role", matchType: "keyword", matchPattern: "二", injectContent: "二" },
  ])).rejects.toThrow("请求冲突");
  expect(mocks.post).toHaveBeenCalledTimes(1);
});
