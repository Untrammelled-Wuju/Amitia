import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import TimeoutSettings from "../components/TimeoutSettings.vue";

const api = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }));
vi.mock("@/composables/useApi", () => ({ apiClient: api }));
vi.mock("element-plus", () => ({ ElMessage: { success: vi.fn() } }));

const stubs = {
  "el-card": { template: '<section><slot name="header"/><slot/></section>' },
  "el-form": { template: '<div><slot/></div>' },
  "el-form-item": { props: ["label"], template: '<div>{{ label }}<slot/></div>' },
  "el-switch": { props: ["modelValue", "disabled"], emits: ["update:modelValue"], template: '<input type="checkbox" :checked="modelValue" :disabled="disabled" @change="$emit(\'update:modelValue\', $event.target.checked)"/>' },
  "el-slider": { props: ["modelValue", "disabled"], emits: ["update:modelValue"], template: '<input type="range" :value="modelValue" :disabled="disabled" @input="$emit(\'update:modelValue\', Number($event.target.value))"/>' },
  "el-button": { props: ["disabled"], template: '<button :disabled="disabled"><slot/></button>' },
  "el-alert": { props: ["title"], template: '<p>{{ title }}</p>' },
};

describe("timeout settings", () => {
  beforeEach(() => {
    api.get.mockReset(); api.put.mockReset();
    api.get.mockResolvedValue({ data: { disabled: false, seconds: 180 }, config: { baseURL: "desktop-test" } });
    api.put.mockImplementation(async (_path, data) => ({ data, config: { baseURL: "desktop-test" } }));
  });
  it("preserves the slider value while disabled and persists the setting", async () => {
    const wrapper = mount(TimeoutSettings, { global: { stubs, directives: { loading: {} } } });
    await flushPromises();
    expect(wrapper.text()).toContain("3 分钟");
    await wrapper.get('input[type="checkbox"]').setValue(true);
    expect(wrapper.get('input[type="range"]').attributes("disabled")).toBeDefined();
    await wrapper.get("button").trigger("click");
    await flushPromises();
    expect(api.put).toHaveBeenCalledWith("/api/runtime/timeout/config", { disabled: true, seconds: 180 });
  });
  it("blocks saving after a failed load and offers retry", async () => {
    api.get.mockRejectedValueOnce(new Error("offline"));
    const wrapper = mount(TimeoutSettings, { global: { stubs, directives: { loading: {} } } });
    await flushPromises();
    expect(wrapper.text()).toContain("无法加载超时设置");
    expect(wrapper.get('input[type="checkbox"]').attributes("disabled")).toBeDefined();
    await wrapper.get("button").trigger("click");
    await flushPromises();
    expect(api.get).toHaveBeenCalledTimes(2);
    expect(api.put).not.toHaveBeenCalled();
  });
});
