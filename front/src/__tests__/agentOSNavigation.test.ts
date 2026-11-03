import { describe, expect, it } from "vitest";
import { appSection, resolveAppSection } from "@/navigation/appSections";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";

describe("Agent OS navigation", () => {
  it("keeps Agent OS as a first-class section", () => {
    expect(resolveAppSection("/agents")).toBe("agents");
  });

  it("places the plugin marketplace inside the extension center", () => {
    const paths = builtinBusinessRoutes.map((route) => route.path);
    expect(paths).toContain("/extensions/market");
    expect(paths).not.toContain("/marketplace");
    expect(appSection("extensions")?.entries[0]).toEqual({
      route: "/extensions/market",
      label: "插件市场",
    });
    expect(builtinBusinessRoutes.find((route) => route.path === "/extensions")?.props).toEqual({
      title: "扩展中心",
      description: "管理扩展包、工作流、MCP 服务与 Agent Skills。",
    });
  });
});
