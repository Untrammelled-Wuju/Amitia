import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import { shadowsProtectedRoute } from "@/ui-runtime/providerRoutes";

describe("active task core module", () => {
  it("keeps task and reminder routes in core", () => {
    const paths = builtinBusinessRoutes.map((route) => String(route.path ?? ""));
    expect(paths).toEqual(expect.arrayContaining([
      "/tasks",
      "/reminders",
      "/continuity",
      "/continuity/:id",
    ]));
    expect(shadowsProtectedRoute("/tasks")).toBe(true);
    expect(shadowsProtectedRoute("/reminders")).toBe(true);
    expect(shadowsProtectedRoute("/characters/example/active-task")).toBe(true);
  });
});
