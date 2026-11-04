import { describe, expect, it } from "vitest";
import { appSection, resolveAppSection } from "@/navigation/appSections";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";

describe("Agent page removal", () => {
  it("does not expose a standalone Agent section or route", () => {
    expect(resolveAppSection("/agents")).toBeNull();
    expect(appSection("character")?.entries.map((entry) => entry.route)).not.toContain("/agents");
    expect(builtinBusinessRoutes.map((route) => route.path)).not.toContain("/agents");
  });

  it("does not hardcode the plugin marketplace as a host route", () => {
    const paths = builtinBusinessRoutes.map((route) => route.path);
    expect(paths).not.toContain("/extensions/market");
    expect(paths).not.toContain("/marketplace");
    expect(appSection("extensions")?.entries.map((entry) => entry.route)).not.toContain("/extensions/market");
    expect(builtinBusinessRoutes.find((route) => route.path === "/extensions")?.props).toEqual({
      title: "扩展中心",
      description: "管理扩展包、工作流、MCP 服务与 Agent Skills。",
    });
  });
});
