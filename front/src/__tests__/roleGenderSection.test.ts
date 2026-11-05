import { mount } from "@vue/test-utils";
import { defineComponent, nextTick, reactive } from "vue";
import { describe, expect, it } from "vitest";
import RoleGenderSection from "../views/ai-character-settings/components/RoleGenderSection.vue";

const ElSelect = defineComponent({
  name: "ElSelect",
  props: ["modelValue", "disabled", "size"],
  emits: ["update:modelValue"],
  template: '<select :disabled="disabled"><slot /></select>',
});
const ElOption = defineComponent({
  name: "ElOption",
  props: ["value", "label"],
  template: "<option>{{ label }}</option>",
});
const ElInput = defineComponent({
  name: "ElInput",
  props: ["modelValue", "placeholder", "size"],
  emits: ["update:modelValue"],
  template: '<input :value="modelValue" :placeholder="placeholder" />',
});
const ElSlider = defineComponent({
  name: "ElSlider",
  props: ["modelValue", "min", "max", "step"],
  emits: ["update:modelValue"],
  template: "<div />",
});

function mountSection() {
  const genderForm = reactive({
    gender: "UNSPECIFIED",
    pronoun: "TA",
    genderLabel: "",
    userAddressingStyle: "自然称呼",
    genderExpression: 30,
  });
  const wrapper = mount(RoleGenderSection, {
    props: { genderForm },
    global: {
      stubs: { ElSelect, ElOption, ElInput, ElSlider },
    },
  });
  return { genderForm, wrapper };
}

describe("角色性别设置", () => {
  it("同步性别、代词、称呼风格和表达强度", async () => {
    const { genderForm, wrapper } = mountSection();
    const selects = wrapper.findAllComponents(ElSelect);
    await selects[0].vm.$emit("update:modelValue", "FEMALE");
    await nextTick();
    expect(genderForm.gender).toBe("FEMALE");
    expect(genderForm.pronoun).toBe("她");

    await selects[0].vm.$emit("update:modelValue", "CUSTOM");
    await nextTick();
    const inputs = wrapper.findAllComponents(ElInput);
    expect(inputs).toHaveLength(2);
    await inputs[0].vm.$emit("update:modelValue", "守护者");
    await inputs[1].vm.$emit("update:modelValue", "它");
    await wrapper.findAllComponents(ElSelect)[2].vm.$emit(
      "update:modelValue",
      "亲近一点",
    );
    await wrapper.findComponent(ElSlider).vm.$emit("update:modelValue", 72);
    await nextTick();

    expect(genderForm.genderLabel).toBe("守护者");
    expect(genderForm.pronoun).toBe("它");
    expect(genderForm.userAddressingStyle).toBe("亲近一点");
    expect(genderForm.genderExpression).toBe(72);
    wrapper.unmount();
  });
});
