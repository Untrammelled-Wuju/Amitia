import { beforeEach, describe, expect, it, vi } from "vitest";
import { canSkipCoreOnboarding } from "../runtime/core-onboarding-access";

const mocks = vi.hoisted(() => ({ get: vi.fn(), deployment: vi.fn(), paired: vi.fn() }));
vi.mock("../composables/useApi", () => ({ apiClient: { get: mocks.get } }));
vi.mock("../runtime/runtime-adapter", () => ({ getDeploymentConfig: mocks.deployment, getApiBaseURL: vi.fn().mockResolvedValue("https://core.example"), isCurrentDevicePaired: mocks.paired }));

describe("已配对普通设备独立完成引导", () => {
  beforeEach(() => {
    mocks.deployment.mockResolvedValue({ mode: "cloud" });
    mocks.paired.mockResolvedValue(true);
    mocks.get.mockResolvedValue({ data: { coreId: "core", canAdminister: false, policy: { coordinated: false } } });
  });
  it("普通已验证设备不改 Core 完成状态即可进入聊天", async () => { expect(await canSkipCoreOnboarding()).toBe(true); });
  it("未配对不跳过引导", async () => { mocks.paired.mockResolvedValue(false); expect(await canSkipCoreOnboarding()).toBe(false); });
  it("状态未知或请求失败不跳过引导", async () => {
    mocks.get.mockResolvedValue({ data: {} });
    expect(await canSkipCoreOnboarding()).toBe(false);
    mocks.get.mockRejectedValue(new Error("offline"));
    expect(await canSkipCoreOnboarding()).toBe(false);
  });
  it("管理员仍按 Core 配置完成状态引导", async () => {
    mocks.get.mockResolvedValue({ data: { coreId: "core", canAdminister: true, policy: { coordinated: true } } });
    expect(await canSkipCoreOnboarding()).toBe(false);
  });
});
