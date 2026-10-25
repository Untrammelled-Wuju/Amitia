import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, ref } from "vue";
import { mount } from "@vue/test-utils";
import { useImmersiveOnboarding } from "../views/onboarding/composables/useImmersiveOnboarding";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), refresh: vi.fn(), write: vi.fn(), push: vi.fn() }));
vi.mock("../composables/useApi", () => ({ useApi: () => mocks }));
vi.mock("../composables/useOwnedMemoryManagement", () => ({ useOwnedMemoryManagement: () => ({ write: mocks.write }) }));
vi.mock("../composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => ({ refresh: mocks.refresh, selectInitialRole: () => "role" }) }));
vi.mock("../composables/useCoreConfigurationAccess", () => ({ useCoreConfigurationAccess: () => ({ refresh: async () => "", canConfigure: ref(false), checking: ref(false) }), coreConfigurationRequestConfig: () => ({}) }));
vi.mock("vue-router", () => ({ useRouter: () => ({ push: mocks.push }) }));
vi.mock("element-plus", () => ({ ElMessage: {}, ElMessageBox: {} }));

describe("引导初始记忆接入", () => {
  const scope = { coreId: "core", roleId: "role", memoryRoleId: "role", resourceOwnerId: "phone" };
  beforeEach(() => {
    Object.values(mocks).forEach((mock) => mock.mockReset());
    mocks.refresh.mockResolvedValue(true);
    mocks.get.mockResolvedValue({ executionScope: scope });
  });
  it("通过统一主链保存初始画像，收到保存结果后才完成", async () => {
    vi.useFakeTimers();
    let onboarding!: ReturnType<typeof useImmersiveOnboarding>;
    const wrapper = mount(defineComponent({ setup() { onboarding = useImmersiveOnboarding(); return () => null; } }));
    onboarding.memoryItems.value = ["小明", "简洁", "喜欢茶"];
    await onboarding.handleEnterAmitia();
    expect(mocks.get).toHaveBeenCalledWith("/api/device-mesh/v1/business/memories", expect.objectContaining({ memoryView: "role:role", characterId: "role" }));
    expect(mocks.write).toHaveBeenCalledTimes(3);
    expect(mocks.write).toHaveBeenCalledWith(scope, expect.any(String), expect.objectContaining({ action: "create", partition: "profile", key: "称呼", value: "小明" }));
    expect(mocks.post).not.toHaveBeenCalled();
    expect(onboarding.onboardingComplete.value).toBe(true);
    wrapper.unmount();
    vi.clearAllTimers();
    vi.useRealTimers();
  });
  it("失败不显示完成，同一内容重试使用同一请求标识", async () => {
    let onboarding!: ReturnType<typeof useImmersiveOnboarding>;
    const wrapper = mount(defineComponent({ setup() { onboarding = useImmersiveOnboarding(); return () => null; } }));
    onboarding.memoryItems.value = ["小明"];
    mocks.write.mockRejectedValue(new Error("数据所有者尚未确认保存"));
    await onboarding.handleEnterAmitia();
    expect(onboarding.onboardingComplete.value).toBe(false);
    expect(onboarding.stageError.value).toBe("数据所有者尚未确认保存");
    await onboarding.handleEnterAmitia();
    expect(mocks.write.mock.calls[0][1]).toBe(mocks.write.mock.calls[1][1]);
    expect(mocks.push).not.toHaveBeenCalled();
    wrapper.unmount();
  });
});
