import { mount, flushPromises } from "@vue/test-utils";
import { ElMessage } from "element-plus";
import { describe, expect, it, vi } from "vitest";
import BackgroundSettings from "../components/BackgroundSettings.vue";

const mock = vi.hoisted(() => ({ update: vi.fn(), settings: null as any, error: null as any }));
vi.mock("../composables/useBackgroundPreference", async () => {
  const { reactive, ref } = await import("vue");
  mock.settings = reactive({ enabled: false, mode: "default", name: "", opacity: 0.35, blurEnabled: false, blurRadius: 10 });
  mock.error = ref("");
  return { useBackgroundPreference: () => ({
    settings: ref(mock.settings), source: ref(""), loading: ref(false), error: mock.error,
    init: vi.fn().mockResolvedValue(undefined), update: mock.update, selectFile: vi.fn(), clear: vi.fn(),
  }) };
});

const stubs = {
  "el-card": { template: '<section><slot name="header"/><slot/></section>' },
  "el-switch": { props: ["modelValue", "disabled"], template: '<input type="checkbox" :disabled="disabled" />' },
  "el-slider": { props: ["modelValue", "disabled"], template: '<input type="range" :disabled="disabled" />' },
  "el-button": { template: '<button><slot/></button>' },
  "el-alert": true,
  BackgroundMedia: true,
};

describe("background settings feedback", () => {
  it("背景或模糊未启用时解释原因且不写入", async () => {
    const warning = vi.spyOn(ElMessage, "warning").mockImplementation(() => undefined as any);
    const wrapper = mount(BackgroundSettings, { global: { stubs } });
    try {
      await flushPromises();
      expect(wrapper.get('input[type="range"]').attributes("disabled")).toBeDefined();
      await wrapper.findAll(".slider-wrapper")[0].trigger("click");
      expect(warning).toHaveBeenCalledWith("请先选择背景。");
      mock.settings.mode = "color";
      await wrapper.vm.$nextTick();
      await wrapper.findAll(".slider-wrapper")[1].trigger("click");
      expect(warning).toHaveBeenCalledWith("请先开启背景模糊。");
      expect(mock.update).not.toHaveBeenCalled();
    } finally { wrapper.unmount(); warning.mockRestore(); }
  });
  it("读取失败时显示原始错误并提示先重试", async () => {
    const warning = vi.spyOn(ElMessage, "warning").mockImplementation(() => undefined as any);
    mock.error.value = "read-original-detail";
    const wrapper = mount(BackgroundSettings, { global: { stubs } });
    try {
      await flushPromises();
      expect(wrapper.get("details").text()).toContain("read-original-detail");
      await wrapper.findAll(".slider-wrapper")[0].trigger("click");
      expect(warning).toHaveBeenCalledWith("无法读取背景设置，请先重试并查看原始错误。");
      expect(mock.update).not.toHaveBeenCalled();
    } finally { wrapper.unmount(); warning.mockRestore(); mock.error.value = ""; }
  });
});
