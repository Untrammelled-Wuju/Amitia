import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { beforeEach, describe, expect, it, vi } from "vitest";
import GameCenterView from "../views/game-center/GameCenterView.vue";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  push: vi.fn(),
  replace: vi.fn(),
}));

vi.mock("@/composables/useApi", () => ({
  useApi: () => ({ get: mocks.get, post: mocks.post }),
}));

vi.mock("vue-router", () => ({
  useRoute: () => ({ query: {}, path: "/game-center", hash: "" }),
  useRouter: () => ({ push: mocks.push, replace: mocks.replace }),
}));

vi.mock("@/views/extensions/api", () => ({
  installExtensionPackage: vi.fn(),
  previewExtensionPackage: vi.fn(),
  setGameCenterExtensionEnabled: vi.fn(),
}));

describe("游戏模式页面状态", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.post.mockReset();
    mocks.push.mockReset();
    mocks.replace.mockReset();
  });

  function mountView() {
    return mount(GameCenterView, { global: { plugins: [ElementPlus] } });
  }

  it("未进入时不加载设备游戏插件并显示进入入口", async () => {
    mocks.get.mockResolvedValue({
      session: { spaceId: "space", deviceId: "", state: "inactive", revision: 0 },
      currentDeviceId: "device-a",
      owned: false,
      canAcquire: true,
    });
    const wrapper = mountView();
    await flushPromises();
    expect(wrapper.text()).toContain("进入游戏模式");
    expect(mocks.get).toHaveBeenCalledTimes(1);
    expect(mocks.get).toHaveBeenCalledWith("/api/game-mode");
    wrapper.unmount();
  });

  it("其他设备占用时显示租约所有者且不请求本机插件", async () => {
    mocks.get.mockResolvedValue({
      session: {
        spaceId: "space",
        sessionId: "game-session",
        deviceId: "device-b",
        state: "active",
        gameTitle: "Remote Game",
        revision: 3,
      },
      currentDeviceId: "device-a",
      owned: false,
      canAcquire: false,
    });
    const wrapper = mountView();
    await flushPromises();
    expect(wrapper.text()).toContain("其他设备正在使用");
    expect(wrapper.text()).toContain("device-b");
    expect(wrapper.text()).toContain("Remote Game");
    expect(mocks.get).toHaveBeenCalledTimes(1);
    wrapper.unmount();
  });

  it("本机持有租约后加载插件和运行时", async () => {
    mocks.get.mockImplementation((path: string) => {
      if (path === "/api/game-mode") {
        return Promise.resolve({
          session: {
            spaceId: "space",
            sessionId: "game-session",
            deviceId: "device-a",
            state: "active",
            gamePluginId: "game-a",
            gameTitle: "Local Game",
            revision: 2,
          },
          currentDeviceId: "device-a",
          owned: true,
          canAcquire: true,
        });
      }
      if (path === "/api/game-center/plugins") {
        return Promise.resolve({
          items: [{
            extensionId: "com.example/game-a",
            pluginId: "game-a",
            name: "Local Game",
            version: "1.0.0",
            enabled: true,
            health: "healthy",
            runtimeCount: 1,
            capabilities: ["game.execute"],
          }],
        });
      }
      if (path === "/api/game-center/runtimes") {
        return Promise.resolve({
          items: [{
            runtimeId: "runtime-a",
            pluginId: "game-a",
            state: "running",
            health: "healthy",
            connected: true,
            ready: true,
            controlMode: "observe_only",
          }],
        });
      }
      if (path === "/api/game-center/developer-access") {
        return Promise.resolve({ enabled: false });
      }
      return Promise.reject(new Error(`unexpected path ${path}`));
    });
    const wrapper = mountView();
    await flushPromises();
    expect(wrapper.text()).toContain("本机正在使用");
    expect(wrapper.text()).toContain("Local Game");
    expect(wrapper.text()).toContain("运行连接");
    expect(mocks.get).toHaveBeenCalledWith("/api/game-center/plugins", { page: 1, pageSize: 100 });
    expect(mocks.get).toHaveBeenCalledWith("/api/game-center/runtimes", { page: 1, pageSize: 100 });
    wrapper.unmount();
  });

  it("插件已启用但运行时未启动时卡片显示启动", async () => {
    mocks.get.mockImplementation((path: string) => {
      if (path === "/api/game-mode") {
        return Promise.resolve({
          session: {
            spaceId: "space",
            sessionId: "game-session",
            deviceId: "device-a",
            state: "active",
            gamePluginId: "game-a",
            revision: 2,
          },
          currentDeviceId: "device-a",
          owned: true,
          canAcquire: true,
        });
      }
      if (path === "/api/game-center/plugins") {
        return Promise.resolve({
          items: [{
            extensionId: "com.example/game-a",
            pluginId: "game-a",
            name: "Local Game",
            version: "1.0.0",
            enabled: true,
            health: "degraded",
            runtimeCount: 1,
          }],
        });
      }
      if (path === "/api/game-center/runtimes") {
        return Promise.resolve({
          items: [{
            runtimeId: "runtime-a",
            pluginId: "game-a",
            state: "created",
            health: "degraded",
            connected: false,
            ready: false,
            controlMode: "observe_only",
          }],
        });
      }
      return Promise.resolve({ enabled: false });
    });
    const wrapper = mountView();
    await flushPromises();
    const cardText = wrapper.get(".game-card").text();
    expect(cardText).toContain("启动");
    expect(cardText).not.toContain("停止");
    wrapper.unmount();
  });
});
