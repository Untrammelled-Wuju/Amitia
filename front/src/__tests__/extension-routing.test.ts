import { describe, expect, it } from "vitest";
import { isDeviceLocalApiPath } from "@/runtime/runtime-adapter";

describe("extension management routing", () => {
  it("routes the device UI event stream to the device Runtime", () => {
    expect(isDeviceLocalApiPath("/api/proactive-sse")).toBe(true);
  });

  it.each([
    "/api/extension-center/view",
    "/api/extension-center/extension",
    "/api/extensions/packages/artifacts",
    "/api/extensions/updates/check",
    "/api/extensions/versions/rollback",
    "/api/extensions/kernel/extensions/enable",
  ])("routes extension state operation %s to the device Runtime", (path) => {
    expect(isDeviceLocalApiPath(path)).toBe(true);
  });

  it.each([
    "/api/extensions/workflows",
    "/api/extensions/mcp/servers",
    "/api/extensions/kernel/agent-skills",
  ])("keeps independently placed extension capability %s on its selected authority", (path) => {
    expect(isDeviceLocalApiPath(path)).toBe(false);
  });
});
