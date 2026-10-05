import { beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import { defineComponent } from "vue";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), stream: vi.fn() }));
vi.mock("../composables/useApi", () => ({ useApi: () => mocks }));
vi.mock("../runtime/device-owned-chat", () => ({ streamOwnedChat: mocks.stream }));
import DeviceCoreCall from "../components/DeviceCoreCall.vue";
import DeviceCapabilityGrants from "../components/DeviceCapabilityGrants.vue";

const stubs = {
  ElDialog: defineComponent({ props: ["modelValue"], template: '<div v-if="modelValue"><slot /></div>' }),
  ElAlert: defineComponent({ props: ["title"], template: '<p>{{title}}</p>' }),
  ElButton: defineComponent({ props: ["disabled"], emits: ["click"], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' }),
  ElInput: defineComponent({ props: ["modelValue"], emits: ["update:modelValue"], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' }),
  ElSelect: defineComponent({ props: ["modelValue"], emits: ["update:modelValue"], template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><slot /></select>' }),
  ElOption: defineComponent({ props: ["value", "label"], template: '<option :value="value">{{label}}</option>' }),
  ElForm: true, ElFormItem: true, ElEmpty: true,
};

describe("device Core calls", () => {
  beforeEach(() => {
    mocks.get.mockReset(); mocks.post.mockReset(); mocks.put.mockReset(); mocks.stream.mockReset();
    mocks.get.mockImplementation(async (path: string) => path.endsWith("/roles") ? { roles: [{ id: "target-role", name: "目标角色", revision: 3 }], roleOwnerId: "target", executionScope: { coreId: "core-b", targetDeviceId: "target", resourceOwnerId: "target" } } : { coreId: "core-b", coordinationAvailable: true });
  });

  it("calls the selected device through Core and waits for owner save confirmation", async () => {
    mocks.stream.mockImplementation(async (request: any, _signal: any, event: any) => {
      const executionScope = { coreId: "core-b", targetDeviceId: "target", resourceOwnerId: "target" };
      event({ type: "started", executionScope });
      event({ type: "delta", executionScope, text: "回复" });
      return { reply: "回复", saved: true, executionScope };
    });
    const wrapper = mount(DeviceCoreCall, { props: { visible: true, deviceId: "target", label: "目标" }, global: { stubs } });
    await flushPromises();
    await wrapper.find("input").setValue("检查工作进展");
    await wrapper.findAll("button").find(button => button.text() === "发起调用")!.trigger("click");
    await flushPromises();
    expect(mocks.stream.mock.calls[0][0]).toMatchObject({ targetDeviceId: "target", characterId: "target-role", message: "检查工作进展" });
    expect(mocks.stream.mock.calls[0][0].expectedExecutionScope).toMatchObject({ coreId: "core-b", targetDeviceId: "target", roleId: "target-role", roleRevision: 3 });
    expect(wrapper.text()).toContain("已由 target 确认保存");
    wrapper.unmount();
  });

  it("interrupts a changed provider and displays both provider names", async () => {
    mocks.stream.mockImplementation(async (_request: any, signal: any, event: any) => {
      event({ type: "started", executionScope: { coreId: "core-c", targetDeviceId: "target", resourceOwnerId: "target" } });
      expect(signal.aborted).toBe(true);
      throw new Error("late reply");
    });
    const wrapper = mount(DeviceCoreCall, { props: { visible: true, deviceId: "target" }, global: { stubs } });
    await flushPromises();
    await wrapper.find("input").setValue("执行");
    await wrapper.findAll("button").find(button => button.text() === "发起调用")!.trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("已从「core-b」切换为「core-c」");
    expect(mocks.stream.mock.calls[0][1].aborted).toBe(true);
    expect(wrapper.text()).not.toContain("确认保存");
    wrapper.unmount();
  });

  it("does not invoke Core after target permission is denied", async () => {
    mocks.get.mockImplementation(async (path: string) => {
      if (path.endsWith("/roles")) throw new Error("目标设备未授权此调用能力");
      return { coreId: "core-b", coordinationAvailable: true };
    });
    const wrapper = mount(DeviceCoreCall, { props: { visible: true, deviceId: "target" }, global: { stubs } });
    await flushPromises();
    expect(wrapper.text()).toContain("目标设备未授权");
    expect(wrapper.findAll("button").find(button => button.text() === "发起调用")!.attributes("disabled")).toBeDefined();
    expect(mocks.stream).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it("revokes exactly the caller capability and displayed grant revision", async () => {
    mocks.get.mockImplementation(async (path: string) => path.endsWith("/grants") ? { grants: [{ callerId: "caller", targetId: "target", capability: "ai.chat", allowed: true, revision: 4 }] } : { coreId: "core-b" });
    mocks.put.mockResolvedValue({});
    const wrapper = mount(DeviceCapabilityGrants, { props: { visible: true, deviceId: "target", devices: [{ deviceId: "caller", label: "调用设备", trustState: "trusted" }] }, global: { stubs, directives: { loading: () => {} } } });
    await flushPromises();
    await wrapper.findAll("button").find(button => button.text() === "撤销")!.trigger("click");
    await flushPromises();
    expect(mocks.put).toHaveBeenCalledWith("/api/device-mesh/v1/business/devices/target/grants", { callerId: "caller", capability: "ai.chat", allowed: false, expectedRevision: 4, expectedCoreId: "core-b" });
    wrapper.unmount();
  });
});
