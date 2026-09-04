import { describe, expect, it } from "vitest";
import onboardingSource from "../views/onboarding/composables/useImmersiveOnboarding.ts?raw";
import routerSource from "../router/index.ts?raw";

describe("引导页模型配置保存", () => {
  it("将各类模型写入对应配置接口", () => {
    expect(onboardingSource.match(/post\("\/api\/model\/configs"/g)).toHaveLength(
      1,
    );
    expect(onboardingSource).toContain('post("/api/vision/configs"');
    expect(onboardingSource).toContain('post("/api/tts/configs"');
    expect(onboardingSource).toContain('post("/api/embedding/configs"');
  });

  it("语音配置使用语音模型字段", () => {
    const start = onboardingSource.indexOf('post("/api/tts/configs"');
    const end = onboardingSource.indexOf("});", start);
    const saveVoiceSource = onboardingSource.slice(start, end);
    expect(saveVoiceSource).toContain("resourceId: voiceModelResource.value");
    expect(saveVoiceSource).not.toContain("resource: voiceModelResource.value");
    expect(onboardingSource).not.toContain('apiType: "voice"');
    expect(onboardingSource).not.toContain('apiType: "vector"');
    expect(onboardingSource).not.toContain('apiType: "vision"');
  });

  it("完成引导后调用公开完成接口并进入聊天页", () => {
    expect(onboardingSource).toContain('post("/api/public/onboarding/complete"');
    expect(onboardingSource).toContain('router.push("/chat")');
  });

  it("存在账号但引导未完成时优先进入引导页", () => {
    const loginGuardStart = routerSource.indexOf('if (to.path === "/login")');
    const loginGuardEnd = routerSource.indexOf('if (to.path === "/onboarding")');
    const loginGuardSource = routerSource.slice(loginGuardStart, loginGuardEnd);
    expect(loginGuardSource).toContain('apiClient.get("/api/public/onboarding/status")');
    expect(loginGuardSource).toContain('return next("/onboarding")');
  });
});
