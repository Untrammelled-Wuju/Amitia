import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import DeviceOwnedMemoryManagement from "../components/DeviceOwnedMemoryManagement.vue";
const mocks = vi.hoisted(() => ({ write: vi.fn(), read: vi.fn(), conflicts: vi.fn(), conversations: vi.fn(), historicalRoles: vi.fn(), data: vi.fn() }));
vi.mock("../composables/useOwnedMemoryManagement", async () => ({ ...await vi.importActual<any>("../composables/useOwnedMemoryManagement"), useOwnedMemoryManagement: () => mocks }));
vi.mock("../composables/useApi", () => ({ apiClient: {} }));
vi.mock("../composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => mocks }));
vi.mock("element-plus", () => ({ ElMessage: { success: vi.fn() }, ElMessageBox: { confirm: vi.fn().mockResolvedValue(true) } }));
const global = { stubs: {
  ElButton: { props: ["disabled", "loading"], emits: ["click"], template: `<button :disabled="disabled || loading" @click="$emit('click')"><slot /></button>` },
  ElInput: { props: ["modelValue"], emits: ["update:modelValue"], template: `<input :value="modelValue" @input="$emit('update:modelValue', $event.target.value)" />` },
  ElSelect: { props: ["modelValue"], emits: ["update:modelValue", "change"], template: `<select :value="modelValue" @change="$emit('update:modelValue', $event.target.value); $emit('change', $event.target.value)"><slot /></select>` },
  ElOption: { props: ["value", "label"], template: `<option :value="value">{{label}}</option>` },
  ElDialog: { props: ["modelValue"], template: `<div v-if="modelValue" class="dialog"><slot /><slot name="footer" /></div>` },
  ElAlert: { props: ["title"], template: `<p>{{title}}</p>` }, ElForm: { template: `<div><slot /></div>` }, ElFormItem: { template: `<div><slot /></div>` }, ElInputNumber: true,
} };
describe("Owned 记忆管理表单原意图", () => {
  let scope: any; const wrappers: any[] = [];
  const open = () => { const wrapper = mount(DeviceOwnedMemoryManagement, { props: { executionScope: scope }, global }); wrappers.push(wrapper); return wrapper; };
  const click = async (wrapper: any, text: string) => { await wrapper.findAll("button").find((button: any) => button.text() === text).trigger("click"); await flushPromises(); };
  beforeEach(() => { Object.values(mocks).forEach((fn) => fn.mockReset()); scope = { authorizationRealm: "realm", coreId: "core", resourceOwnerId: "device", roleOwnerId: "device", targetDeviceId: "device", roleId: "role", roleRevision: 1, permissionRevision: 1, coordinated: false }; mocks.conflicts.mockReturnValue([]); mocks.read.mockResolvedValue({ resources: [], nextCursor: "" }); mocks.historicalRoles.mockResolvedValue([{ id: "old-role", name: "旧角色" }]); });
  afterEach(() => wrappers.splice(0).forEach((wrapper) => wrapper.unmount()));
  it("超时重试沿用同一 UUID 和原 scope，切换 Core 后关闭旧表单", async () => { const wrapper = open(); await click(wrapper, "新增记忆"); const inputs = wrapper.findAll(".dialog input"); await inputs[0].setValue("茶"); await inputs[1].setValue("喜欢茶"); mocks.write.mockRejectedValue(new Error("暂未确认")); await click(wrapper, "保存"); await click(wrapper, "保存"); expect(mocks.write.mock.calls[1][1]).toBe(mocks.write.mock.calls[0][1]); expect(mocks.write.mock.calls[0][0]).toEqual(scope); await wrapper.setProps({ executionScope: { ...scope, coreId: "next", providerEpoch: 2 } }); expect(wrapper.find(".dialog").exists()).toBe(false); });
  it("旧设备多角色目录显式选择，提取携带真实 Owner 和历史角色", async () => { scope = { ...scope, coordinated: true, resourceOwnerId: "core", roleOwnerId: "core" }; const wrapper = open(); mocks.conversations.mockResolvedValue([]); await click(wrapper, "从对话生成候选"); mocks.data.mockResolvedValue({ executionScope: scope, historicalSnapshot: { ownerId: "device", role: { id: "old-role" }, resources: [{ kind: "conversation", id: "same", ownerId: "device", roleId: "old-role", body: { title: "旧聊天" } }] } }); await wrapper.find(".dialog select").setValue("old-role"); await flushPromises(); await wrapper.findAll(".dialog select")[1].setValue("same"); mocks.write.mockResolvedValue([]); await click(wrapper, "生成候选"); expect(mocks.write.mock.calls[0][2]).toMatchObject({ conversationId: "same", conversationOrigin: { ownerId: "device", id: "same" }, historicalRoleId: "old-role" }); expect(mocks.write.mock.calls[0][0]).toEqual(scope); });
  it("筛选变更丢弃旧检索结果及原游标", async () => { const wrapper = open(); let finish!: (value: any) => void; mocks.read.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; })); await wrapper.findAll("button").find((button) => button.text() === "检索")!.trigger("click"); await wrapper.find("input").setValue("新条件"); finish({ resources: [{ ownerId: "device", id: "old", body: { key: "迟到旧结果" } }], nextCursor: "old-cursor" }); await flushPromises(); expect(wrapper.text()).not.toContain("迟到旧结果"); expect(wrapper.text()).not.toContain("加载更多检索结果"); });
});
