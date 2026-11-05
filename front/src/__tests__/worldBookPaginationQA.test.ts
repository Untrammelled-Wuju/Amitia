import { expect, it, vi } from "vitest";
import { ref } from "vue";
import { useWorldBook } from "../composables/useWorldBook";

const fixture = vi.hoisted(() => ({ data: vi.fn() }));
vi.mock("../ui-index", () => ({ apiClient: {} }));
vi.mock("../composables/useDeviceOwnedConversation", () => ({
  useDeviceOwnedConversation: () => ({
    enabled: ref(true), coreId: ref("core"), roleOwnerId: ref("owner"),
    refresh: async () => true, selectInitialRole: () => "role", data: fixture.data,
  }),
}));

it("筛选缩页与删除末页数据后页码和内容一致", async () => {
  const resources = Array.from({ length: 21 }, (_, index) => ({
    kind: "worldbook", id: `rule-${index}`, roleId: "role", ownerId: "owner", revision: 1,
    body: { matchPattern: index === 0 ? "目标" : "其他", injectContent: "内容" },
  }));
  fixture.data.mockImplementation(async () => ({
    executionScope: { coreId: "core", roleId: "role", resourceOwnerId: "owner" },
    snapshot: { ownerId: "owner", resources: [...resources] },
  }));
  const book = useWorldBook();
  await book.fetchRules({ page: 2, pageSize: 20 });
  expect(book.rules.value.map(rule => rule.id)).toEqual(["rule-20"]);
  await book.fetchRules({ page: 2, pageSize: 20, keyword: "目标" });
  expect(book.total.value).toBe(1);
  expect(book.page.value).toBe(1);
  expect(book.rules.value.map(rule => rule.id)).toEqual(["rule-0"]);
  resources.pop();
  await book.fetchRules({ page: 2, pageSize: 20 });
  expect(book.page.value).toBe(1);
  expect(book.rules.value).toHaveLength(20);
});
