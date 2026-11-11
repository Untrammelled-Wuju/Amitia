import { describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus, { ElMessage } from "element-plus";
import TaskRunListView from "../views/extensions/TaskRunListView.vue";

const mocks = vi.hoisted(() => ({ fetch: vi.fn(), cancel: vi.fn(), retry: vi.fn() }));
vi.mock("../views/extensions/api", () => ({ fetchTasks: mocks.fetch, cancelTask: mocks.cancel, retryTask: mocks.retry }));
vi.mock("../views/extensions/TaskRunDetailView.vue", () => ({ default: { template: "<div />" } }));

describe("任务不可用操作提示", () => {
  it("只读或状态不允许的操作解释原因且不调用服务", async () => {
    mocks.fetch.mockResolvedValue({ items: [{ taskRunId: "t1", extensionId: "p", status: "succeeded", readOnly: true, priority: 0, attempt: 1, maxAttempts: 1, createdAt: "" }], total: 1 });
    const warning = vi.spyOn(ElMessage, "warning").mockImplementation(() => ({}) as never);
    const wrapper = mount(TaskRunListView, { global: { plugins: [ElementPlus], stubs: { ExtensionPageHeader: { template: "<header><slot name='actions' /></header>" } } } });
    try {
      await flushPromises();
      const cancel = wrapper.findAll("button").find((button) => button.text() === "取消");
      const retry = wrapper.findAll("button").find((button) => button.text() === "重试");
      expect(cancel).toBeDefined();
      expect(retry).toBeDefined();
      await cancel!.trigger("click");
      await retry!.trigger("click");
      expect(warning).toHaveBeenCalledTimes(2);
      expect(mocks.cancel).not.toHaveBeenCalled();
      expect(mocks.retry).not.toHaveBeenCalled();
    } finally { wrapper.unmount(); warning.mockRestore(); }
  });
});
