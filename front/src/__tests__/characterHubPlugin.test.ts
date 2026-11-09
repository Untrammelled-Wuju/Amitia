import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import { shadowsProtectedRoute } from "@/ui-runtime/providerRoutes";

describe("character core module", () => {
  it("keeps character routes in core and protects them from plugin replacement", () => {
    const paths = builtinBusinessRoutes.map((route) => String(route.path ?? ""));
    expect(paths).toEqual(expect.arrayContaining([
      "/characters",
      "/characters/teams",
      "/character",
      "/character/:id",
      "/character/:id/:tab(life-rules|voice|psyche)",
      "/world-book",
    ]));
    expect(shadowsProtectedRoute("/characters")).toBe(true);
    expect(shadowsProtectedRoute("/character/example/voice")).toBe(true);
  });
});
