import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus, { ElMessageBox } from "element-plus";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ref } from "vue";
import WorldBookView from "@/views/world-book/WorldBookView.vue";

const state = {
  access: {
    canConfigure: ref(true),
    requireAccess: vi.fn(async () => undefined),
  },
  loadedContext: ref("context"),
  rules: ref([{
    id: "rule-1",
    matchType: "exact",
    matchPattern: "QA-XSS-UI",
    matchScope: "full_context",
    injectContent: "<img src=x>",
    priority: 1,
  }]),
  loading: ref(false),
  total: ref(1),
  page: ref(1),
  totalPages: ref(1),
  fetchRules: vi.fn(async () => undefined),
  createRule: vi.fn(async () => undefined),
  createRules: vi.fn(async () => undefined),
  updateRule: vi.fn(async () => undefined),
  deleteRule: vi.fn(async () => {
    state.rules.value = [];
  }),
  testMatch: vi.fn(async () => ({
    matches: [{
      entry: state.rules.value[0],
      hitText: "QA-XSS-UI",
    }],
  })),
  matchTypeLabel: (value: string) => value,
  scopeLabel: (value: string) => value,
};

vi.mock("@/composables/useWorldBook", () => ({
  useWorldBook: () => state,
}));

beforeEach(() => {
  state.fetchRules.mockClear();
  state.createRule.mockReset();
  state.createRule.mockResolvedValue(undefined);
  state.createRules.mockClear();
  state.updateRule.mockClear();
  state.updateRule.mockResolvedValue(undefined);
  state.deleteRule.mockClear();
  state.deleteRule.mockImplementation(async () => {
    state.rules.value = [];
  });
  state.testMatch.mockClear();
  state.testMatch.mockImplementation(async () => ({ matches: [{ entry: state.rules.value[0], hitText: "QA-XSS-UI" }] }));
});

afterEach(() => {
  vi.restoreAllMocks();
  state.rules.value = [{
    id: "rule-1",
    matchType: "exact",
    matchPattern: "QA-XSS-UI",
    matchScope: "full_context",
    injectContent: "<img src=x>",
    priority: 1,
  }];
});

it("clears stale online results after the matching rule is deleted", async () => {
  vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
  const wrapper = mount(WorldBookView, {
    global: {
      plugins: [ElementPlus],
      stubs: {
        ExtensionPageHeader: {
          template: "<section><slot name=\"actions\" /><slot /></section>",
        },
      },
    },
  });
  await flushPromises();

  const testButton = wrapper.findAll("button").find((button) => button.text() === "在线测试");
  expect(testButton).toBeDefined();
  await testButton!.trigger("click");
  await wrapper.find("textarea").setValue("QA-XSS-UI");
  await wrapper.findAll("button")
    .find((button) => button.text() === "测试匹配")!
    .trigger("click");
  await flushPromises();
  expect(wrapper.text()).toContain("命中规则 (1)");

  await wrapper.findAll("button")
    .find((button) => button.text() === "删除")!
    .trigger("click");
  await flushPromises();

  expect(state.deleteRule).toHaveBeenCalledWith("rule-1", "context");
  expect(wrapper.text()).toContain("暂无世界书规则");
  expect(wrapper.text()).not.toContain("命中规则 (1)");
});

it("ignores a repeated create click while the first mutation is pending", async () => {
  let resolveCreate!: () => void;
  const pending = new Promise<void>((resolve) => {
    resolveCreate = resolve;
  });
  state.createRule.mockImplementationOnce(async () => {
    await pending;
    return undefined;
  });
  const wrapper = mount(WorldBookView, {
    global: {
      plugins: [ElementPlus],
      stubs: {
        ExtensionPageHeader: {
          template: "<section><slot name=\"actions\" /><slot /></section>",
        },
      },
    },
  });
  await flushPromises();

  const create = (wrapper.vm as unknown as {
    handleCreate: () => Promise<void>;
  }).handleCreate;
  void create();
  void create();

  expect(state.createRule).toHaveBeenCalledTimes(1);
  resolveCreate();
  await flushPromises();
});

it("keeps failed creation inside the form, preserves raw errors and allows retry", async () => {
  const cause = new Error("cause-QA-1105");
  const failure = Object.assign(new Error("invalid-worldbook-QA", { cause }), {
    detail: "detail-QA-1105", internalMessage: "internal-QA-1105",
    reason: "reason-QA-1105", raw: { error: "raw-QA-1105", msg: "msg-QA-1105" },
  });
  state.createRule.mockRejectedValueOnce(failure);
  const escaped: unknown[] = [];
  const wrapper = mount(WorldBookView, {
    global: {
      plugins: [ElementPlus],
      config: { errorHandler: error => { escaped.push(error); } },
      stubs: { ExtensionPageHeader: { template: '<section><slot name="actions" /><slot /></section>' } },
    },
  });
  await flushPromises();
  await wrapper.findAll("button").find(button => button.text() === "新增规则")!.trigger("click");
  await flushPromises();
  await wrapper.find('input[placeholder="正则表达式/精确文本/关键词(逗号分隔)"]').setValue("QA-RETRY");
  await wrapper.findAll("button").find(button => button.text() === "创建")!.trigger("click");
  await flushPromises();
  expect(state.createRule).toHaveBeenCalledTimes(1);
  expect(escaped).toEqual([]);
  const detail = wrapper.find('[aria-label="世界书原始错误"]').text();
  for (const value of [failure.message, cause.message, failure.detail, failure.internalMessage, failure.reason, failure.raw.error, failure.raw.msg]) {
    expect(detail).toContain(value);
  }
  expect(detail).toContain("stack");
  expect((wrapper.find('input[placeholder="正则表达式/精确文本/关键词(逗号分隔)"]').element as HTMLInputElement).value).toBe("QA-RETRY");
  await wrapper.findAll("button").find(button => button.text() === "创建")!.trigger("click");
  await flushPromises();
  expect(state.createRule).toHaveBeenCalledTimes(2);
  expect(wrapper.find('[aria-label="世界书原始错误"]').exists()).toBe(false);
  wrapper.unmount();
});

it.each(["update", "delete", "match", "load"])("keeps %s failures visible without escaping to the application boundary", async action => {
  const failure = new Error(`QA-${action}-failure`, { cause: new Error(`QA-${action}-cause`) });
  const escaped: unknown[] = [];
  if (action === "update") state.updateRule.mockRejectedValueOnce(failure);
  if (action === "delete") {
    vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    state.deleteRule.mockRejectedValueOnce(failure);
  }
  if (action === "match") state.testMatch.mockRejectedValueOnce(failure);
  if (action === "load") state.fetchRules.mockRejectedValueOnce(failure);
  const wrapper = mount(WorldBookView, {
    global: {
      plugins: [ElementPlus],
      config: { errorHandler: error => { escaped.push(error); } },
      stubs: { ExtensionPageHeader: { template: '<section><slot name="actions" /><slot /></section>' } },
    },
  });
  await flushPromises();
  if (action === "update") {
    await wrapper.findAll("button").find(button => button.text() === "编辑")!.trigger("click");
    await flushPromises();
    await wrapper.findAll("button").find(button => button.text() === "保存")!.trigger("click");
  }
  if (action === "delete") await wrapper.findAll("button").find(button => button.text() === "删除")!.trigger("click");
  if (action === "match") {
    await wrapper.findAll("button").find(button => button.text() === "在线测试")!.trigger("click");
    await wrapper.find("textarea").setValue("QA-XSS-UI");
    await wrapper.findAll("button").find(button => button.text() === "测试匹配")!.trigger("click");
  }
  await flushPromises();
  expect(escaped).toEqual([]);
  const detail = wrapper.find('[aria-label="世界书原始错误"]').text();
  expect(detail).toContain(failure.message);
  expect(detail).toContain(`QA-${action}-cause`);
  expect(detail).toContain("stack");
  expect(wrapper.text()).toContain("QA-XSS-UI");
  if (action === "match") expect(wrapper.text()).not.toContain("无规则命中");
  wrapper.unmount();
});

it("treats deletion cancellation as a user action without calling the API", async () => {
  vi.spyOn(ElMessageBox, "confirm").mockRejectedValue("cancel");
  const wrapper = mount(WorldBookView, {
    global: { plugins: [ElementPlus], stubs: { ExtensionPageHeader: true } },
  });
  await flushPromises();
  await wrapper.findAll("button").find(button => button.text() === "删除")!.trigger("click");
  await flushPromises();
  expect(state.deleteRule).not.toHaveBeenCalled();
  expect(wrapper.find('[aria-label="世界书原始错误"]').exists()).toBe(false);
  expect(wrapper.text()).toContain("QA-XSS-UI");
  wrapper.unmount();
});

it("ignores a match response after the user changes the test text", async () => {
  let resolveMatch!: (value: any) => void;
  state.testMatch.mockImplementationOnce(() => new Promise(resolve => {
    resolveMatch = resolve;
  }));
  const wrapper = mount(WorldBookView, {
    global: {
      plugins: [ElementPlus],
      stubs: {
        ExtensionPageHeader: {
          template: "<section><slot name=\"actions\" /><slot /></section>",
        },
      },
    },
  });
  await flushPromises();

  await wrapper.findAll("button").find(button => button.text() === "在线测试")!.trigger("click");
  const textarea = wrapper.find("textarea");
  await textarea.setValue("第一段文本");
  await wrapper.findAll("button").find(button => button.text() === "测试匹配")!.trigger("click");
  await flushPromises();
  await textarea.setValue("完全不同的第二段文本");
  resolveMatch({
    matches: [{ entry: state.rules.value[0], hitText: "第一段文本" }],
  });
  await flushPromises();

  expect(state.testMatch).toHaveBeenCalledTimes(1);
  expect(wrapper.text()).not.toContain("命中规则 (1)");
  expect(wrapper.text()).not.toContain("无规则命中");
  wrapper.unmount();
});
