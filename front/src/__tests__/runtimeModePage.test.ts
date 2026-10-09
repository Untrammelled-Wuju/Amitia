import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { describe, expect, it, vi } from "vitest";
import RuntimeModeView from "../views/runtime-mode/RuntimeModeView.vue";

vi.mock("../views/runtime-mode/api", () => ({
  fetchModeApi: vi.fn(async () => ({
    deployMode: "desktop-local",
    host: "127.0.0.1",
    port: 18080,
    web: undefined,
    bridge: undefined,
    storage: undefined,
  })),
  switchModeApi: vi.fn(),
  validateModeApi: vi.fn(),
}));

describe("运行模式配置健壮性", () => {
  it("接口缺少嵌套配置时保留默认值且页面可以渲染", async () => {
    const wrapper = mount(RuntimeModeView, {
      global: { plugins: [ElementPlus] },
    });
    try {
      await vi.waitFor(() => {
        expect(wrapper.get(".runtime-mode-page").text()).toContain("Bridge 模式");
        expect(wrapper.get(".runtime-mode-page").text()).toContain("当前 Core 的服务端部署形态");
        expect(wrapper.get(".runtime-mode-page").text()).toContain("127.0.0.1:8898");
      });
    } finally {
      wrapper.unmount();
    }
  });
});
