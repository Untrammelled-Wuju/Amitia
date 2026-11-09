import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import { shadowsProtectedRoute } from "@/ui-runtime/providerRoutes";

describe("chat logs core route", () => {
  it("owns the archive route in the core router", () => {
    const route = builtinBusinessRoutes.find((item) => item.path === "/logs");
    expect(route?.name).toBe("chatLogs");
    expect(route?.meta?.requiresAuth).toBe(true);
    expect(route?.meta?.coreSurface).toBe(true);
  });

  it("rejects plugin ownership of the archive route", () => {
    expect(shadowsProtectedRoute("/logs")).toBe(true);
  });

  it("removes the plugin source", () => {
    expect(existsSync(resolve(__dirname, "../../../plugins-builtin/chat-logs/amitia-extension.json"))).toBe(false);
  });
});
