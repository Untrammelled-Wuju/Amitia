import { expect, it, vi } from "vitest";
import { nextTick, ref } from "vue";
import { useWorldBook } from "../composables/useWorldBook";

const fixture = vi.hoisted(() => ({ data: vi.fn() }));
const coreId = ref("core");
vi.mock("../ui-index", () => ({ apiClient: {} }));
vi.mock("../composables/useDeviceOwnedConversation", () => ({
  useDeviceOwnedConversation: () => ({
    enabled: ref(true), coreId, roleOwnerId: ref("owner"),
    refresh: async () => true, selectInitialRole: () => "role", data: fixture.data,
  }),
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail; });
  return { promise, resolve, reject };
}

function snapshot() {
  return {
    executionScope: { coreId: "core", roleId: "role", resourceOwnerId: "owner" },
    snapshot: { ownerId: "owner", resources: ["keyword", "exact"].map(matchType => ({
      kind: "worldbook", id: matchType, roleId: "role", ownerId: "owner", revision: 1,
      body: { matchType, matchPattern: matchType, injectContent: matchType },
    })) },
  };
}

it("快速切换筛选后，较晚返回的旧请求不能覆盖最新筛选", async () => {
  const first = deferred<ReturnType<typeof snapshot>>();
  const second = deferred<ReturnType<typeof snapshot>>();
  fixture.data.mockReset().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
  const book = useWorldBook();
  const older = book.fetchRules({ matchType: "keyword" });
  await vi.waitFor(() => expect(fixture.data).toHaveBeenCalledTimes(1));
  const newer = book.fetchRules({ matchType: "exact" });
  await vi.waitFor(() => expect(fixture.data).toHaveBeenCalledTimes(2));
  second.resolve(snapshot());
  await newer;
  expect(book.rules.value.map(rule => rule.id)).toEqual(["exact"]);
  first.resolve(snapshot());
  await older;
  expect(book.rules.value.map(rule => rule.id)).toEqual(["exact"]);
});

it("旧请求完成时，仍在处理的最新请求必须保持加载状态", async () => {
  const first = deferred<ReturnType<typeof snapshot>>();
  const second = deferred<ReturnType<typeof snapshot>>();
  fixture.data.mockReset().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
  const book = useWorldBook();
  const older = book.fetchRules({ matchType: "keyword" });
  await vi.waitFor(() => expect(fixture.data).toHaveBeenCalledTimes(1));
  const newer = book.fetchRules({ matchType: "exact" });
  await vi.waitFor(() => expect(fixture.data).toHaveBeenCalledTimes(2));
  first.resolve(snapshot());
  await older;
  const loadingWhileNewestPending = book.loading.value;
  second.resolve(snapshot());
  await newer;
  expect(loadingWhileNewestPending).toBe(true);
  expect(book.loading.value).toBe(false);
});

it("过期请求失败仍返回原始异常，但不能清空最新成功数据", async () => {
  const first = deferred<ReturnType<typeof snapshot>>();
  fixture.data.mockReset().mockReturnValueOnce(first.promise).mockResolvedValueOnce(snapshot());
  const book = useWorldBook();
  const older = book.fetchRules({ matchType: "keyword" });
  await vi.waitFor(() => expect(fixture.data).toHaveBeenCalledTimes(1));
  await book.fetchRules({ matchType: "exact" });
  const failure = new Error("QA-original-network-error", { cause: new Error("QA-original-cause") });
  const rejection = expect(older).rejects.toBe(failure);
  first.reject(failure);
  await rejection;
  expect(book.rules.value.map(rule => rule.id)).toEqual(["exact"]);
  expect(book.total.value).toBe(1);
  expect(book.loadedContext.value).not.toBe("");
});

it("来源变化使尚未完成的响应失效", async () => {
  const pending = deferred<ReturnType<typeof snapshot>>();
  fixture.data.mockReset().mockReturnValueOnce(pending.promise);
  const book = useWorldBook();
  const loading = book.fetchRules();
  await vi.waitFor(() => expect(fixture.data).toHaveBeenCalledTimes(1));
  coreId.value = "other-core";
  await nextTick();
  pending.resolve(snapshot());
  await loading;
  expect(book.rules.value).toEqual([]);
  expect(book.loadedContext.value).toBe("");
  expect(book.loading.value).toBe(false);
  coreId.value = "core";
  await nextTick();
});
