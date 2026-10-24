import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { describe, expect, it, vi } from "vitest";
import DeploymentPanel from "../views/settings/DeploymentPanel.vue";

const mocks = vi.hoisted(() => ({
  getDeploymentConfig: vi.fn(),
  saveDeploymentConfig: vi.fn(),
  getApiBaseURL: vi.fn(),
  initializeRuntimeCapabilities: vi.fn(),
}));

vi.mock("../runtime/runtime-adapter", () => ({
  getDeploymentConfig: mocks.getDeploymentConfig,
  saveDeploymentConfig: mocks.saveDeploymentConfig,
  getApiBaseURL: mocks.getApiBaseURL,
}));

vi.mock("../runtime/runtime-capabilities", () => ({
  initializeRuntimeCapabilities: mocks.initializeRuntimeCapabilities,
  isDesktopShell: () => true,
}));

describe("云端统筹设置", () => {
  it("地址检查与设备配对完成后才保存云端模式", async () => {
    mocks.getDeploymentConfig.mockResolvedValue({ mode: "local" });
    mocks.getApiBaseURL.mockResolvedValue("http://127.0.0.1:18899");
    mocks.saveDeploymentConfig.mockResolvedValue({ mode: "cloud", serverURL: "https://core.example" });
    mocks.initializeRuntimeCapabilities.mockResolvedValue(undefined);
    const wrapper = mount(DeploymentPanel, {
      global: {
        plugins: [ElementPlus],
        stubs: {
          StageRuntimeSetup: {
            name: "StageRuntimeSetup",
            emits: ["healthCheckDone"],
            props: ["beforeVerify", "afterVerificationFailed"],
            template: `
              <div data-testid="cloud-pairing">
                <button data-testid="cloud-prepare" @click="beforeVerify('https://core.example')" />
                <button data-testid="cloud-failed" @click="afterVerificationFailed('https://core.example', new Error('验证失败'))" />
              </div>
            `,
          },
        },
      },
    });
    try {
      const state = (wrapper.vm as any).$.setupState;
      await state.loadConfig();
      state.formMode = "cloud";
      state.formData.serverURL = "https://core.example";
      await wrapper.vm.$nextTick();
      await state.handleSave();
      await wrapper.vm.$nextTick();
      expect(mocks.saveDeploymentConfig).not.toHaveBeenCalled();
      expect(wrapper.find('[data-testid="cloud-pairing"]').exists()).toBe(true);

      const pairing = wrapper.findComponent({ name: "StageRuntimeSetup" });
      await wrapper.get('[data-testid="cloud-prepare"]').trigger("click");
      await vi.waitFor(() => expect(mocks.saveDeploymentConfig).toHaveBeenCalledWith({
        mode: "cloud",
        serverURL: "https://core.example",
      }));
      await wrapper.get('[data-testid="cloud-failed"]').trigger("click");
      await vi.waitFor(() => expect(mocks.saveDeploymentConfig).toHaveBeenLastCalledWith({ mode: "local" }));
      pairing.vm.$emit("healthCheckDone");
    } finally {
      wrapper.unmount();
    }
  });
});
