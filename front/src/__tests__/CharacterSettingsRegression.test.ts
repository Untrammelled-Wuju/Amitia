import { describe, expect, it } from "vitest";
import settingsSource from "../views/ai-character-settings/AiCharacterSettingsView.vue?raw";
import characterViewSource from "../views/character/CharacterView.vue?raw";

describe("角色信息设置回归", () => {
  it("使用现行角色接口完成保存、默认切换和重置", () => {
    expect(settingsSource).not.toContain("/api/ai/character");
    expect(settingsSource).toContain("/api/characters/${charId.value}");
    expect(settingsSource).toContain("{ isDefault: val }");
    expect(settingsSource).toContain("normalizePersonalityConfig");
  });

  it("按解包后的接口响应读取音色与配置列表", () => {
    expect(characterViewSource).toContain("Array.isArray(data) ? data : []");
    expect(characterViewSource).not.toContain("r.data?.data || []");
  });

  it("生活设置不再散落在角色设置页并改由扩展标签承载", () => {
    expect(settingsSource).not.toContain("LifestyleTendencySection");
    expect(settingsSource).not.toContain("SleepSettingSection");
    expect(settingsSource).not.toContain("FixedEventsSection");
    expect(settingsSource).not.toContain("SpecialEventsSection");
    expect(settingsSource).not.toContain("WorkProfileSection");
    expect(characterViewSource).toContain('slot-id="character.detail.tab"');
  });
});
