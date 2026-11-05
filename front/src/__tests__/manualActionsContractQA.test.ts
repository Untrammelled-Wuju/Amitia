import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ElementPlus from "element-plus";
import ManualActionsPanel from "@/views/long-running/components/ManualActionsPanel.vue";

const actions = vi.hoisted(() => ({
  requireAccess: vi.fn(async () => "local"),
  cleanupTempApi: vi.fn(),
  rotateLogsApi: vi.fn(),
  checkDbIntegrityApi: vi.fn(),
}));

vi.mock("@/composables/useCoreConfigurationAccess", () => ({
  useCoreConfigurationAccess: () => ({ requireAccess: actions.requireAccess }),
}));

vi.mock("@/views/long-running/api", () => ({
  cleanupTempApi: actions.cleanupTempApi,
  rotateLogsApi: actions.rotateLogsApi,
  checkDbIntegrityApi: actions.checkDbIntegrityApi,
}));

let wrapper: VueWrapper;

async function mountPanel() {
  wrapper = mount(ManualActionsPanel, {
    global: { plugins: [ElementPlus] },
  });
  await flushPromises();
}

beforeEach(() => {
  actions.requireAccess.mockClear();
  actions.cleanupTempApi.mockReset();
  actions.rotateLogsApi.mockReset();
  actions.checkDbIntegrityApi.mockReset();
});

afterEach(() => {
  wrapper?.unmount();
  document.body.innerHTML = "";
});

describe("运行维护操作契约", () => {
  it("显示健康数据库检查结果而不是固定失败", async () => {
    actions.checkDbIntegrityApi.mockResolvedValue({
      ok: true,
      errors: [],
      status: "ok",
      issues: [],
    });
    await mountPanel();
    await wrapper.findAll("button").find(button => button.text() === "立即检查")!.trigger("click");
    await flushPromises();

    expect(wrapper.text()).toContain("数据库完整性检查通过");
    expect(wrapper.text()).not.toContain("数据库检查失败");
  });

  it("使用后端真实清理字段展示文件数与释放空间", async () => {
    actions.cleanupTempApi.mockResolvedValue({
      cleaned: true,
      processedFiles: 2,
      removedLines: 0,
      bytesFreed: 2048,
    });
    await mountPanel();
    await wrapper.findAll("button").find(button => button.text() === "立即清理")!.trigger("click");
    await flushPromises();

    expect(wrapper.text()).toContain("处理 2 个文件");
    expect(wrapper.text()).toContain("2.0 KB");
    expect(wrapper.text()).not.toContain("undefined");
  });

  it("数据库检查失败时保留完整原始错误", async () => {
    const failure = Object.assign(new Error("database-QA-failure", {
      cause: new Error("database-QA-cause"),
    }), {
      detail: "database-QA-detail",
      raw: { error: "database-QA-raw" },
    });
    actions.checkDbIntegrityApi.mockRejectedValue(failure);
    await mountPanel();
    await wrapper.findAll("button").find(button => button.text() === "立即检查")!.trigger("click");
    await flushPromises();

    const text = wrapper.text();
    expect(text).toContain("database-QA-failure");
    expect(text).toContain("database-QA-cause");
    expect(text).toContain("database-QA-detail");
    expect(text).toContain("database-QA-raw");
    expect(text).toContain("stack");
  });
});
