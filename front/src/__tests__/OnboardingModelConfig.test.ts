import { describe, expect, it } from "vitest";
import onboardingSource from "../views/onboarding/composables/useImmersiveOnboarding.ts?raw";

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
});
