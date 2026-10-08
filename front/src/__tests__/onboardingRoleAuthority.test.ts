import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent } from "vue";
import { mount } from "@vue/test-utils";
import { useImmersiveOnboarding } from "../views/onboarding/composables/useImmersiveOnboarding";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), saveDeploymentConfig: vi.fn(), error: vi.fn() }));
vi.mock("../composables/useApi", () => ({ useApi: () => mocks }));
vi.mock("../runtime/runtime-adapter", () => ({ saveDeploymentConfig: mocks.saveDeploymentConfig, getApiBaseURL: vi.fn() }));
vi.mock("vue-router", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("element-plus", () => ({ ElMessage: { error: mocks.error }, ElMessageBox: {} }));

describe("引导创建角色归属", () => {
  const source = "a".repeat(64);
  const core = "b".repeat(64);
  beforeEach(() => { Object.values(mocks).forEach((mock) => mock.mockReset()); });
  it("进入编辑阶段捕获归属，提交途中切换 Core 不重新查询并拒绝写入", async () => {
    let onboarding!: ReturnType<typeof useImmersiveOnboarding>;
    let owner = source;
    mocks.get.mockImplementation(async () => ({ roleAuthority: owner }));
    const wrapper = mount(defineComponent({ setup() { onboarding = useImmersiveOnboarding(); return () => null; } }));
    await onboarding.nextStage();
    expect(onboarding.currentStage.value).toBe(1);
    onboarding.identityName.value = "旧归属新角色";
    owner = core;
    mocks.post.mockImplementation(async (path, _data, config) => {
      expect(path).toBe("/api/characters");
      expect(config.headers["X-Amitia-Role-Authority"]).toBe(source);
      throw new Error("角色数据归属已变化，请重新开始引导");
    });
    await onboarding.handleEnterAmitia();
    expect(mocks.get).toHaveBeenCalledTimes(1);
    expect(mocks.post).toHaveBeenCalledTimes(1);
    expect(onboarding.characterCreatedInSession.value).toBe(false);
    expect(onboarding.stageError.value).toBe("角色数据归属已变化，请重新开始引导");
    wrapper.unmount();
  });
});
