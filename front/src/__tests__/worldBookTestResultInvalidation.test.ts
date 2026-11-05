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
  state.deleteRule.mockClear();
  state.deleteRule.mockImplementation(async () => {
    state.rules.value = [];
  });
  state.testMatch.mockClear();
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
