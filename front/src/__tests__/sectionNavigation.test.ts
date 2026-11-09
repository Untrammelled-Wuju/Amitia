import { describe, expect, it } from "vitest";
import { appSection, resolveAppSection } from "../navigation/appSections";
import shellSource from "../components/BuiltinAppShell.vue?raw";

describe("两级导航映射", () => {
  it("把路由归入对应的一级导航区", () => {
    expect(resolveAppSection("/chat")).toBe("home");
    expect(resolveAppSection("/character/role-1/life-rules")).toBe("character");
    expect(resolveAppSection("/characters/teams")).toBe("character");
    expect(resolveAppSection("/profiles")).toBe("memory");
    expect(resolveAppSection("/memory-manager")).toBe("memory");
    expect(resolveAppSection("/reminders")).toBe("tasks");
    expect(resolveAppSection("/characters/role-1/active-task")).toBe("tasks");
    expect(resolveAppSection("/continuity")).toBe("tasks");
    expect(resolveAppSection("/game-center/plugin")).toBe("game");
    expect(resolveAppSection("/creative-workshop/pet")).toBe("workshop");
    expect(resolveAppSection("/extensions/packages")).toBe("extensions");
    expect(resolveAppSection("/channel-center")).toBe("channels");
    expect(resolveAppSection("/channels/qq")).toBe("channels");
    expect(resolveAppSection("/my-space/profile")).toBe("account");
    expect(resolveAppSection("/settings/system")).toBe("settings");
    expect(resolveAppSection("/plugin-only-route")).toBeNull();
  });

  it("提供任务、角色、记忆、创意工坊和扩展中心的二级入口", () => {
    expect(appSection("tasks")?.entries).toEqual([]);
    expect(appSection("character")?.entries).toEqual([]);
    expect(appSection("memory")?.entries).toEqual([]);
    expect(appSection("workshop")?.entries).toEqual([]);
    expect(appSection("extensions")?.entries).toEqual([]);
  });

  it("第二栏位于圆角主工作区内部", () => {
    const appMain = shellSource.indexOf('<div class="app-main');
    const sectionSidebar = shellSource.indexOf("<SectionSidebar");
    const appContent = shellSource.indexOf('<main class="app-content');
    expect(appMain).toBeGreaterThan(-1);
    expect(sectionSidebar).toBeGreaterThan(appMain);
    expect(appContent).toBeGreaterThan(sectionSidebar);
  });

  it("插件工作区第二栏优先于内置区侧栏", () => {
    expect(shellSource).toContain('capability="app.workspace.sidebar"');
    expect(shellSource.indexOf('capability="app.workspace.sidebar"')).toBeLessThan(
      shellSource.indexOf("<SettingsSidebar"),
    );
    expect(shellSource).toContain('return "none"');
    expect(shellSource).toMatch(/\.extension-workspace-sidebar\s*\{[^}]*padding:\s*0\s+8px\s+18px/s);
  });
});
