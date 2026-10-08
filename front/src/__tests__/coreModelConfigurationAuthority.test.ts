import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import { useCoreConfigurationAccess, coreConfigurationRequestConfig } from "../composables/useCoreConfigurationAccess";
import { useModelConfig } from "../views/model-config/composables/useModelConfig";
import { useImmersiveOnboarding } from "../views/onboarding/composables/useImmersiveOnboarding";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn(), warning: vi.fn(), success: vi.fn(), error: vi.fn(), deployment: vi.fn(), saveDeploymentConfig: vi.fn() }));
vi.mock("../composables/useApi", () => ({ useApi: () => mocks }));
vi.mock("../runtime/runtime-adapter", () => ({ getDeploymentConfig: mocks.deployment, saveDeploymentConfig: mocks.saveDeploymentConfig, getApiBaseURL: vi.fn() }));
vi.mock("vue-router", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("element-plus", () => ({ ElMessage: mocks, ElMessageBox: { confirm: vi.fn().mockResolvedValue(undefined) } }));

describe("Core 模型配置授权", () => {
  let status: any;
  const wrappers: ReturnType<typeof mount>[] = [];
  function open<T>(create: () => T): T {
    let state!: T;
    wrappers.push(mount(defineComponent({ setup() { state = create(); return () => null; } })));
    return state;
  }
  beforeEach(() => {
    Object.values(mocks).forEach((mock) => mock.mockReset());
    mocks.deployment.mockResolvedValue({ mode: "cloud", serverURL: "https://core-b.example" });
    status = { coreId: "core-b", canAdminister: true, policy: { coordinated: true, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } };
    mocks.get.mockImplementation(async (path: string) => {
      if (path.endsWith("/coordination/me")) return structuredClone(status);
      if (path.endsWith("/authority")) return { roleAuthority: "a".repeat(64) };
      if (path.endsWith("/configs/7")) return { id: 7, name: "B配置", apiType: "openai", modelName: "B模型" };
      if (path.endsWith("/configs")) return [{ id: 7, name: "B配置", apiType: "openai", modelName: "B模型" }];
      if (path.endsWith("/status")) return { mainModelVision: false };
      return [];
    });
  });
  afterEach(() => { wrappers.splice(0).forEach((wrapper) => wrapper.unmount()); vi.clearAllTimers(); vi.useRealTimers(); });

  it.each([{ canAdminister: false }, { policy: { coordinated: false } }, { policy: undefined }])("普通设备、未统筹及未知状态禁止配置 %j", async (change) => {
    Object.assign(status, change);
    const configuration = open(() => useModelConfig());
    await flushPromises();
    await configuration.showDialog(null);
    await configuration.setActive(7);
    await configuration.assignRoute("chat", 7);
    expect(configuration.dialogVisible.value).toBe(false);
    expect(mocks.post).not.toHaveBeenCalled();
    expect(mocks.put).not.toHaveBeenCalled();
    expect(mocks.get.mock.calls.some(([path]) => path.endsWith("/configs"))).toBe(false);
  });

  it("部署切换立即禁用，旧延迟授权响应不能重新开启", async () => {
    const access = open(() => useCoreConfigurationAccess());
    await flushPromises();
    expect(access.canConfigure.value).toBe(true);
    let finish!: (value: unknown) => void;
    mocks.get.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
    const stale = access.refresh();
    await flushPromises();
    status.canAdminister = false;
    window.dispatchEvent(new CustomEvent("amitia:runtime-connection-changed"));
    expect(access.canConfigure.value).toBe(false);
    finish({ coreId: "core-b", canAdminister: true, policy: { coordinated: true, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } });
    await stale;
    await flushPromises();
    expect(access.canConfigure.value).toBe(false);
  });

  it("权限撤销后重新授予，同 Core 旧表单仍拒绝且原版本头不变", async () => {
    const configuration = open(() => useModelConfig());
    await flushPromises();
    await configuration.showDialog(configuration.configs.value[0]);
    configuration.dialogFormRef.value = { validate: vi.fn().mockResolvedValue(true), clearValidate: vi.fn() } as any;
    status.policy.permissionRevision = 3;
    await configuration.saveConfig();
    expect(mocks.put).not.toHaveBeenCalled();
    expect(coreConfigurationRequestConfig(JSON.stringify(["https://core-b.example", "core-b", 1, 1, 1]))).toEqual({ headers: { "X-Amitia-Expected-Core-ID": "core-b", "X-Amitia-Expected-Configuration-Policy": "1:1:1" } });
    expect(() => coreConfigurationRequestConfig(JSON.stringify(["https://core-b.example", "core-b", 1, 1, 0]))).toThrow("配置权限版本");
  });

  it.each(["/api/model", "/api/vision", "/api/tts", "/api/embedding"])("%s 保存和测试保留原 Core 意图头，同 ID 换 Core 拒绝提交", async (apiBase) => {
    const configuration = open(() => useModelConfig({ apiBase, withScenario: false }));
    await flushPromises();
    await configuration.showDialog(configuration.configs.value[0]);
    configuration.dialogFormRef.value = { validate: vi.fn().mockResolvedValue(true), clearValidate: vi.fn() } as any;
    await configuration.saveConfig();
    expect(mocks.put).toHaveBeenCalledWith(`${apiBase}/configs/7`, expect.any(Object), { headers: { "X-Amitia-Expected-Core-ID": "core-b", "X-Amitia-Expected-Configuration-Policy": "1:1:1" } });
    await configuration.testConfig(7);
    expect(mocks.post).toHaveBeenCalledWith(`${apiBase}/configs/7/test`, { configId: 7 }, { headers: { "X-Amitia-Expected-Core-ID": "core-b", "X-Amitia-Expected-Configuration-Policy": "1:1:1" } });
    await configuration.showDialog(configuration.configs.value[0]);
    status.coreId = "core-c";
    await configuration.saveConfig();
    expect(mocks.put).toHaveBeenCalledTimes(1);
    expect(configuration.dialogVisible.value).toBe(false);
  });

  it("普通绑定引导跳过四类模型及 Core 引导状态修改", async () => {
    vi.useFakeTimers();
    status.canAdminister = false;
    const onboarding = open(() => useImmersiveOnboarding());
    await onboarding.nextStage();
    onboarding.modelApiKey.value = "用户未提交的临时值";
    onboarding.modelName.value = "文本模型";
    onboarding.visionModelKey.value = "临时视觉值";
    onboarding.voiceModelKey.value = "临时语音值";
    onboarding.vectorModelKey.value = "临时向量值";
    await onboarding.detectModel();
    await onboarding.handleEnterAmitia();
    expect(mocks.post).not.toHaveBeenCalled();
    expect(onboarding.onboardingComplete.value).toBe(true);
    expect(onboarding.canConfigureModels.value).toBe(false);
  });

  it("管理员引导四类配置均携带原 Core 头，完成也使用同份 Core 数据", async () => {
    vi.useFakeTimers();
    const onboarding = open(() => useImmersiveOnboarding());
    await onboarding.nextStage();
    onboarding.modelApiKey.value = "临时文本值";
    onboarding.modelName.value = "文本模型";
    onboarding.visionModelKey.value = "临时视觉值";
    onboarding.voiceModelKey.value = "临时语音值";
    onboarding.vectorModelKey.value = "临时向量值";
    await onboarding.handleEnterAmitia();
    expect(mocks.post.mock.calls.map(([path]) => path)).toEqual(["/api/model/configs", "/api/vision/configs", "/api/tts/configs", "/api/embedding/configs", "/api/onboarding/complete"]);
    for (const [, , config] of mocks.post.mock.calls) expect(config).toEqual({ headers: { "X-Amitia-Expected-Core-ID": "core-b", "X-Amitia-Expected-Configuration-Policy": "1:1:1" } });
  });
});
