import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";

const stubs = {
  "el-card": { template: '<section><slot name="header"/><slot/></section>' },
  "el-switch": { props: ["modelValue"], emits: ["update:modelValue"], template: '<input type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)"/>' },
  "el-color-picker": { props: ["modelValue", "disabled"], emits: ["change"], template: '<input type="color" :value="modelValue" :disabled="disabled" @change="$emit(\'change\', $event.target.value)"/>' },
  "el-select": { props: ["modelValue", "disabled"], emits: ["update:modelValue"], template: '<select :value="modelValue" :disabled="disabled" @change="$emit(\'update:modelValue\', $event.target.value)"><slot/></select>' },
  "el-option": { props: ["value", "label"], template: '<option :value="value">{{ label }}</option>' },
};
describe("custom theme settings", () => {
  beforeEach(() => { vi.resetModules(); localStorage.clear(); });
  it("enables editing, selects custom text, and shows contrast feedback", async () => {
    const Component = (await import("../components/CustomThemeSettings.vue")).default;
    const wrapper = mount(Component, { global: { stubs } });
    expect(wrapper.get('input[type="color"]').attributes("disabled")).toBeDefined();
    await wrapper.get('input[type="checkbox"]').setValue(true);
    expect(wrapper.get('input[type="color"]').attributes("disabled")).toBeUndefined();
    await wrapper.findAll('input[type="color"]')[2].setValue("#ffffff");
    expect((wrapper.get("select").element as HTMLSelectElement).value).toBe("custom");
    expect(wrapper.text()).toContain("1.00:1");
    expect(wrapper.text()).toContain("建议提高对比度");
    await wrapper.get("select").setValue("auto");
    expect(wrapper.text()).toContain("21.00:1");
    await wrapper.get('input[type="checkbox"]').setValue(false);
    expect(wrapper.findAll('input[type="color"]')[2].attributes("disabled")).toBeDefined();
    expect(JSON.parse(localStorage.getItem("ai-companion-appearance")!).customPalette.text).toBe("#FFFFFF");
    wrapper.unmount();
  });
});
