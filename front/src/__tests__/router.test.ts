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

  it("keeps personal space routes out of the fixed router", () => {
    expect(router.resolve("/my-space").name).toBe("catchAll");
    for (const section of ["profile", "identity", "runtime", "devices"]) {
      expect(router.resolve(`/my-space/${section}`).name).toBe("catchAll");
    }
    expect(router.resolve("/settings/ui-providers").name).toBe("settingsUIProviders");
    for (const removed of ["/user-settings", "/devices"]) {
      expect(router.resolve(removed).name).toBe("catchAll");
    }
  });
});
