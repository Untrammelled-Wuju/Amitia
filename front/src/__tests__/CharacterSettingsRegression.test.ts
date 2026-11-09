import { describe, expect, it } from "vitest";
import settingsSource from "../views/ai-character-settings/AiCharacterSettingsView.vue?raw";
import characterViewSource from "../views/character/CharacterView.vue?raw";
import teamsViewSource from "../views/character/CharacterTeamsView.vue?raw";

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

  it("一级角色入口不自动选择角色", () => {
    expect(characterViewSource).not.toContain("selectDefaultChar");
    expect(characterViewSource).not.toContain("selectDefaultChar(characters.value[0])");
  });

  it("生活设置不再散落在角色设置页并改由扩展标签承载", () => {
    expect(settingsSource).not.toContain("LifestyleTendencySection");
    expect(settingsSource).not.toContain("SleepSettingSection");
    expect(settingsSource).not.toContain("FixedEventsSection");
    expect(settingsSource).not.toContain("SpecialEventsSection");
    expect(settingsSource).not.toContain("WorkProfileSection");
    expect(characterViewSource).toContain('slot-id="character.detail.tab"');
  });

  it("角色详情不再提供记忆、时间线、主动任务和团队职责入口", () => {
    expect(characterViewSource).not.toContain('label="记忆" name="memory"');
    expect(characterViewSource).not.toContain('label="时间线" name="timeline"');
    expect(characterViewSource).not.toContain('label="主动任务"');
    expect(characterViewSource).not.toContain("团队职责");
  });

  it("角色和团队不再提供技术运行配置", () => {
    expect(characterViewSource).not.toContain('label="运行配置" name="runtime"');
    expect(characterViewSource).not.toContain("CharacterRuntimeConfig");
    expect(characterViewSource).not.toContain('endsWith("/runtime")');
    expect(teamsViewSource).not.toContain("团队预算");
    expect(teamsViewSource).not.toContain("新增预算");
  });
});
