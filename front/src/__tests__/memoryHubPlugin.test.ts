import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import { shadowsProtectedRoute } from "@/ui-runtime/providerRoutes";

describe("memory core module", () => {
  it("keeps memory routes in core and protects them from plugin replacement", () => {
    const paths = builtinBusinessRoutes.map((route) => String(route.path ?? ""));
    expect(paths).toEqual(expect.arrayContaining([
      "/memory",
      "/memory-manager",
      "/memory-timeline",
      "/episodic",
      "/graph",
      "/knowledge",
      "/profiles",
    ]));
    expect(shadowsProtectedRoute("/memory")).toBe(true);
    expect(shadowsProtectedRoute("/knowledge")).toBe(true);
  });
});
