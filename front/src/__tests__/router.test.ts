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

  it("仅提供当前空间与设备地址", () => {
    expect(router.resolve("/my-space").matched.at(-1)?.redirect).toBe("/my-space/profile");
    for (const section of ["profile", "identity", "runtime"]) {
      expect(router.resolve(`/my-space/${section}`).name).toBe(`mySpace${section[0].toUpperCase()}${section.slice(1)}`);
    }
    for (const removed of ["/user-settings", "/devices", "/my-space/devices"]) {
      expect(router.resolve(removed).name).toBe("catchAll");
    }
  });
});
