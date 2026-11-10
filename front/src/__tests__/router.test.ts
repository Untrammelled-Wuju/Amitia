import { describe, expect, it, vi } from "vitest";

vi.mock("../runtime/runtime-capabilities", () => ({
  shouldUseHashRouting: () => true,
  isRuntimeRouteAvailable: () => true,
}));

import router from "../router";

describe("router", () => {
  it("uses hash URLs for local desktop files", () => {
    expect(router.resolve("/chat").href).toContain("#/chat");
  });

  it("将未知路径交给 404 页面处理", () => {
    const route = router.resolve("/renderer/index.html");
    expect(route.name).toBe("catchAll");
  });

  it("keeps personal space and settings routes in the fixed core router", () => {
    const sections = {
      profile: "mySpaceProfile",
      identity: "mySpaceIdentity",
      runtime: "mySpaceRuntime",
      devices: "mySpaceDevices",
    };
    for (const [section, name] of Object.entries(sections)) {
      expect(router.resolve(`/my-space/${section}`).name).toBe(name);
    }
    expect(router.resolve("/settings/ui-providers").name).toBe("settingsUIProviders");
    for (const removed of ["/user-settings", "/devices"]) {
      expect(router.resolve(removed).name).toBe("catchAll");
    }
  });
});
