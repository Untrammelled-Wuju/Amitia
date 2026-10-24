import { describe, expect, it } from "vitest";
import {
  WORKSPACE_SIDEBAR_DEFAULT_WIDTH,
  WORKSPACE_SIDEBAR_MIN_WIDTH,
  clampWorkspaceSidebarWidth,
  maxWorkspaceSidebarWidth,
} from "../navigation/workspaceSidebarWidth";
import shellSource from "../components/BuiltinAppShell.vue?raw";

describe("第二栏拖拽宽度", () => {
  it("默认宽度和最小宽度都为 240", () => {
    expect(WORKSPACE_SIDEBAR_DEFAULT_WIDTH).toBe(240);
    expect(WORKSPACE_SIDEBAR_MIN_WIDTH).toBe(240);
  });

  it("最大宽度保持为窗口宽度的百分比", () => {
    expect(maxWorkspaceSidebarWidth(1200)).toBe(480);
    expect(maxWorkspaceSidebarWidth(800)).toBe(320);
    expect(maxWorkspaceSidebarWidth(360)).toBe(240);
  });

  it("拖拽结果不会小于 240 或超过窗口上限", () => {
    expect(clampWorkspaceSidebarWidth(80, 1200)).toBe(240);
    expect(clampWorkspaceSidebarWidth(300, 1200)).toBe(300);
    expect(clampWorkspaceSidebarWidth(700, 1200)).toBe(480);
    expect(clampWorkspaceSidebarWidth(300, 500)).toBe(240);
  });

  it("拖拽提示覆盖第二栏原有右边框并贯穿整个高度", () => {
    expect(shellSource).toContain(".workspace-sidebar-resizer::after");
    expect(shellSource).toContain("top: 0;");
    expect(shellSource).toContain("right: 0;");
    expect(shellSource).toContain("bottom: 0;");
    expect(shellSource).toContain("width: 1px;");
    expect(shellSource).not.toContain("inset: 12px 2px;");
  });
});
