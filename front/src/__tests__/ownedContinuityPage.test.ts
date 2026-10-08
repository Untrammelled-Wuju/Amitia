import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import ContinuityThreadsView from "../views/continuity/ContinuityThreadsView.vue";
const mocks = vi.hoisted(() => ({ list: vi.fn(), get: vi.fn(), update: vi.fn(), create: vi.fn(), cancel: vi.fn(), confirm: vi.fn(), refresh: vi.fn(), data: vi.fn(), historicalRoles: vi.fn(), owned: null as any }));
vi.mock("../composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => mocks.owned }));
vi.mock("../views/continuity/api", () => ({ listContinuityThreads: mocks.list, getContinuityThread: mocks.get, updateContinuityThread: mocks.update, createContinuityThread: mocks.create, cancelContinuityWait: mocks.cancel, resolveContinuityWait: vi.fn(), createContinuityWait: vi.fn(), confirmContinuityExecution: vi.fn() }));
vi.mock("element-plus", () => ({ ElMessage: { error: vi.fn(), warning: vi.fn() }, ElMessageBox: { confirm: mocks.confirm, prompt: vi.fn() } }));
const global = { stubs: {
  ElButton: { props: ["disabled"], emits: ["click"], template: "<button :disabled='disabled' @click='$emit(\"click\")'><slot /></button>" },
  ElSelect: { props: ["modelValue"], emits: ["update:modelValue", "change"], template: "<select :value='modelValue' @change='$emit(\"update:modelValue\", $event.target.value); $emit(\"change\")'><slot /></select>" },
  ElOption: { props: ["value", "label"], template: "<option :value='value'>{{ label }}</option>" },
  ElInput: { props: ["modelValue"], emits: ["update:modelValue"], template: "<input :value='modelValue' @input='$emit(\"update:modelValue\", $event.target.value)' />" },
  ElDrawer: { props: ["modelValue"], template: "<aside v-if='modelValue'><slot /></aside>" }, ElDialog: { props: ["modelValue"], template: "<section v-if='modelValue' class='dialog'><slot /><slot name='footer' /></section>" },
  ElAlert: { props: ["title"], template: "<p>{{ title }}</p>" }, ElTable: { template: "<div><slot /></div>" }, ElTableColumn: true,
  ElDescriptions: { template: "<div><slot /></div>" }, ElDescriptionsItem: { template: "<div><slot /></div>" }, ElForm: { template: "<div><slot /></div>" }, ElFormItem: { template: "<div><slot /></div>" }, ElTag: true, ElEmpty: true, ElTimeline: true, ElTimelineItem: true, ElDatePicker: true,
} };
describe("持续事项页面的原始详情意图", () => {
  const wrappers: ReturnType<typeof mount>[] = [];
  let scope: any;
  let document: any;
  const open = () => { const wrapper = mount(ContinuityThreadsView, { global }); wrappers.push(wrapper); return wrapper; };
  const button = (wrapper: ReturnType<typeof mount>, text: string) => wrapper.findAll("button").find((item) => item.text() === text)!;
  beforeEach(() => {
    Object.values(mocks).filter((value) => typeof value === "function").forEach((mock: any) => mock.mockReset());
    scope = { coreId: "core", roleId: "role", resourceOwnerId: "source", modeRevision: 1, permissionRevision: 1, providerEpoch: 1 };
    document = { thread: { id: "same-id", characterId: "role", title: "当前事项", status: "paused", revision: 1 }, ownerId: "source", coreId: "core", modeRevision: 1, executionScope: scope, waits: [], events: [] };
    mocks.owned = { enabled: ref(true), coreId: ref("core"), coordinated: ref(true), roles: ref([{ id: "role", name: "当前角色" }]), policy: ref({ providerEpoch: 1, modeRevision: 1, permissionRevision: 1 }), notice: ref(""), selectInitialRole: () => "role", refresh: mocks.refresh, data: mocks.data, historicalRoles: mocks.historicalRoles };
    mocks.refresh.mockResolvedValue(true);
    mocks.historicalRoles.mockResolvedValue([{ id: "old-1", name: "旧角色一" }, { id: "old-2", name: "旧角色二" }]);
    mocks.data.mockResolvedValue({ executionScope: scope });
    mocks.list.mockImplementation(async (params) => [{ ...document.thread, ownedDocument: { ...document, readOnly: !!params.historicalRoleId } }]);
    mocks.get.mockImplementation(async (_id, intent) => ({ ...intent, bindings: [] }));
    mocks.update.mockResolvedValue(document.thread);
  });
  afterEach(() => wrappers.splice(0).forEach((wrapper) => wrapper.unmount()));
  it("详情修改使用被打开记录的原 scope，而非提交时刷新另一份详情", async () => {
    const wrapper = open(); await flushPromises();
    await wrapper.get(".thread-card").trigger("click"); await flushPromises();
    await button(wrapper, "恢复").trigger("click"); await flushPromises();
    expect(mocks.update.mock.calls[0]).toEqual(["same-id", { status: "active" }, { ...document, readOnly: false, bindings: [] }]);
  });
  it("多个旧角色需明确选择，历史详情隐藏恢复及新建入口", async () => {
    const wrapper = open(); await flushPromises();
    expect(mocks.list.mock.calls[0][0].historicalRoleId).toBeUndefined();
    const source = wrapper.findAll("select").find((item) => item.text().includes("旧角色二"))!;
    await source.setValue("history:old-2"); await flushPromises();
    expect(mocks.list.mock.calls.at(-1)?.[0].historicalRoleId).toBe("old-2");
    await wrapper.get(".thread-card").trigger("click"); await flushPromises();
    expect(wrapper.text()).toContain("仅供读取");
    expect(wrapper.findAll("button").some((item) => item.text() === "恢复")).toBe(false);
    expect(button(wrapper, "新建事项").attributes("disabled")).toBeDefined();
  });
  it("Core 变化立即关闭已打开详情及创建表单，迟到详情不能重新打开", async () => {
    const wrapper = open(); await flushPromises();
    await button(wrapper, "新建事项").trigger("click"); await flushPromises();
    expect(wrapper.find(".dialog").exists()).toBe(true);
    let resolveDetail!: (value: any) => void;
    mocks.get.mockImplementation(() => new Promise((resolve) => { resolveDetail = resolve; }));
    await wrapper.get(".thread-card").trigger("click");
    mocks.owned.coreId.value = "core-c";
    await flushPromises();
    resolveDetail({ ...document, bindings: [] });
    await flushPromises();
    expect(wrapper.find("aside").exists()).toBe(false);
    expect(wrapper.find(".dialog").exists()).toBe(false);
    expect(wrapper.findAll(".thread-card")).toHaveLength(0);
    expect(mocks.create).not.toHaveBeenCalled();
  });
});
