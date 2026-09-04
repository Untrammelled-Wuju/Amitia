import { describe, expect, it } from "vitest";
import settingsSource from "../views/ai-character-settings/AiCharacterSettingsView.vue?raw";
import characterViewSource from "../views/character/CharacterView.vue?raw";
import lifestyleSource from "../views/ai-character-settings/components/LifestyleTendencySection.vue?raw";
import fixedEventsSource from "../views/ai-character-settings/components/FixedEventsSection.vue?raw";
import specialEventsSource from "../views/ai-character-settings/components/SpecialEventsSection.vue?raw";

const delayedSections = [
  lifestyleSource,
  fixedEventsSource,
  specialEventsSource,
];

describe("角色信息设置回归", () => {
  it("使用现行角色接口完成保存、默认切换和重置", () => {
    expect(settingsSource).not.toContain("/api/ai/character");
    expect(settingsSource).toContain("/api/characters/${charId.value}");
    expect(settingsSource).toContain("{ isDefault: val }");
    expect(settingsSource).toContain("normalizePersonalityConfig");
  });

  it("按解包后的接口响应读取音色与配置列表", () => {
    expect(characterViewSource).toContain("Array.isArray(r.data) ? r.data : []");
    expect(characterViewSource).not.toContain("r.data?.data || []");
  });

  it("角色 ID 就绪后重新加载依赖角色的设置区块", () => {
    for (const source of delayedSections) {
      expect(source).toContain("watch(() => props.characterId");
      expect(source).toContain("{ immediate: true }");
    }
  });
});
