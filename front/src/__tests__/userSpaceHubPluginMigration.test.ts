import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import { shadowsProtectedRoute } from "@/ui-runtime/providerRoutes";

describe("user space core routes", () => {
  it("owns settings and account routes in the core router", () => {
    const paths = builtinBusinessRoutes.map((route) => String(route.path ?? ""));
    expect(paths).toContain("/settings/:pathMatch(.*)*");
    expect(paths).toContain("/my-space/:pathMatch(.*)*");
    expect(paths).toContain("/settings/ui-providers");
    expect(builtinBusinessRoutes.find((route) => route.path === "/settings/:pathMatch(.*)*")?.meta?.coreSurface).toBe(true);
    expect(builtinBusinessRoutes.find((route) => route.path === "/my-space/:pathMatch(.*)*")?.meta?.coreSurface).toBe(true);
  });

  it("rejects provider ownership of settings and account routes", () => {
    expect(shadowsProtectedRoute("/settings")).toBe(true);
    expect(shadowsProtectedRoute("/settings/system")).toBe(true);
    expect(shadowsProtectedRoute("/my-space")).toBe(true);
    expect(shadowsProtectedRoute("/my-space/profile")).toBe(true);
  });

  it("removes the plugin source", () => {
    expect(existsSync(resolve(__dirname, "../../../plugins-builtin/user-space-hub/amitia-extension.json"))).toBe(false);
  });
});
