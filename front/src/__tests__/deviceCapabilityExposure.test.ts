import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { describe, expect, it, vi } from "vitest";
import DeviceCapabilityExposure from "../components/DeviceCapabilityExposure.vue";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
  resolveApiUrl: vi.fn(async (path: string) => `http://local.test${path}`),
  consume: vi.fn(() => new Promise<void>(() => {})),
}));

vi.mock("../composables/useApi", () => ({
  useApi: () => ({ get: mocks.get, put: mocks.put }),
}));

vi.mock("../runtime/runtime-adapter", () => ({
  resolveApiUrl: mocks.resolveApiUrl,
}));

vi.mock("../runtime/request-auth", () => ({
  createAuthenticatedFetchInit: async () => ({}),
}));

vi.mock("../runtime/sse-stream", () => ({
  consumeSSEBody: mocks.consume,
}));

describe("设备能力开放", () => {
  it("显示本机能力并以原版本提交开关，中心未同步时明确提示", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("", {
      status: 200,
      headers: { "Content-Type": "text/event-stream" },
    })));
    const initial = {
      revision: 3,
      capabilities: [
        { id: "ai.chat", label: "文本与对话", description: "允许中心路由文本", enabled: true },
        { id: "task.execute", label: "主动任务", description: "允许中心调度任务", enabled: true },
      ],
      enabledCapabilities: ["ai.chat", "task.execute"],
    };
    mocks.get.mockResolvedValue(initial);
    mocks.put.mockResolvedValue({
      state: {
        ...initial,
        revision: 4,
        capabilities: [
          initial.capabilities[0],
          { ...initial.capabilities[1], enabled: false },
        ],
        enabledCapabilities: ["ai.chat"],
      },
      centerSynced: false,
      syncError: "",
    });
    const wrapper = mount(DeviceCapabilityExposure, {
      global: { plugins: [ElementPlus] },
    });
    await flushPromises();
    expect(wrapper.text()).toContain("文本与对话");
    expect(wrapper.text()).toContain("主动任务");
    const state = (wrapper.vm as any).$.setupState;
    await state.update(initial.capabilities[1], false);
    await flushPromises();
    expect(mocks.put).toHaveBeenCalledWith("/internal/device-mesh/capabilities/task.execute", {
      enabled: false,
      expectedRevision: 3,
    });
    expect(wrapper.text()).toContain("设备下次连接统筹中心时会同步这项设置");
    wrapper.unmount();
    vi.unstubAllGlobals();
  });
});
