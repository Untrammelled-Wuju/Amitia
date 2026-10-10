import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defineComponent } from "vue";
import { mount, flushPromises } from "@vue/test-utils";
import { useWorldBook } from "../composables/useWorldBook";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn(), deployment: vi.fn(), policy: vi.fn() }));
vi.mock("../ui-index", () => ({ apiClient: mocks }));
vi.mock("../composables/useApi", () => ({ useApi: () => ({ get: mocks.policy }) }));
vi.mock("../runtime/runtime-adapter", () => ({ getDeploymentConfig: mocks.deployment }));
let status: any;
let book: ReturnType<typeof useWorldBook>;
let wrapper: ReturnType<typeof mount>;
beforeEach(() => {
  Object.values(mocks).forEach((mock) => mock.mockReset());
  status = { coreId: "b", canAdminister: true, policy: { coordinated: true, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } };
  mocks.deployment.mockResolvedValue({ mode: "cloud", serverURL: "https://b.example" });
  mocks.policy.mockImplementation(async () => structuredClone(status));
  mocks.get.mockResolvedValue({ data: { items: [{ id: "same-id", injectContent: "B内容" }], total: 1 } });
  wrapper = mount(defineComponent({ setup() { book = useWorldBook(); return () => null; } }));
});
afterEach(() => wrapper.unmount());

it.each([false, undefined])("普通或未知权限不读取世界书 %s", async (value) => {
  status.canAdminister = value;
  await flushPromises();
  await book.fetchRules();
  expect(mocks.get).not.toHaveBeenCalled();
  expect(book.rules.value).toEqual([]);
  await expect(book.createRule({ injectContent: "不能保存" })).rejects.toThrow();
  expect(mocks.post).not.toHaveBeenCalled();
});

it("管理员请求绑定原Core权限版本，换Core后旧规则同ID不可更新", async () => {
  await book.fetchRules();
  const original = book.loadedContext.value;
  await book.updateRule("same-id", { injectContent: "原内容" }, original);
  expect(mocks.put).toHaveBeenCalledWith("/api/world-book/same-id", expect.any(Object), { headers: { "X-Amitia-Expected-Core-ID": "b", "X-Amitia-Expected-Configuration-Policy": "1:1:1" } });
  status.coreId = "c";
  await expect(book.updateRule("same-id", {}, original)).rejects.toThrow();
  expect(mocks.put).toHaveBeenCalledTimes(1);
  expect(book.rules.value).toEqual([]);
});

it("权限撤销发生在列表返回前时丢弃私有世界书内容", async () => {
  let complete!: (value: any) => void;
  mocks.get.mockImplementation(() => new Promise((resolve) => { complete = resolve; }));
  const loading = book.fetchRules();
  await flushPromises();
  status.canAdminister = false;
  complete({ data: { items: [{ id: "private" }], total: 1 } });
  await loading;
  expect(book.rules.value).toEqual([]);
  expect(book.loadedContext.value).toBe("");
});

it("批量导入中权限版本变化不再提交后续规则", async () => {
  await book.fetchRules();
  mocks.post.mockImplementation(async () => { status.policy.permissionRevision = 3; });
  await expect(book.createRules([{ injectContent: "1" }, { injectContent: "2" }])).rejects.toThrow();
  expect(mocks.post).toHaveBeenCalledTimes(1);
});
