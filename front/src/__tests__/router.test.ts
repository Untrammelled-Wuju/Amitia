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

  it("旧资料、设备与运行模式地址指向统一空间并保留查询参数", () => {
    const legacyRoutes = [
      ["/user-settings", "/my-space/profile"],
      ["/devices", "/my-space/devices"],
      ["/runtime-mode", "/my-space/runtime"],
      ["/my-space", "/my-space/profile"],
      ["/my-space#space-identity", "/my-space/identity"],
      ["/devices#bound-devices", "/my-space/devices"],
    ] as const;
    for (const [source, destination] of legacyRoutes) {
      const route = router.resolve(`${source}${source.includes("#") ? "" : "?source=legacy"}`);
      const redirect = route.matched.at(-1)?.redirect;
      expect(typeof redirect).toBe("function");
      if (typeof redirect !== "function") continue;
      const result = redirect(route as Parameters<typeof redirect>[0], router.currentRoute.value);
      expect(result).toEqual({
        path: destination,
        query: source.includes("#") ? {} : { source: "legacy" },
      });
    }
    const deployment = router.resolve("/settings/deployment");
    expect(deployment.matched.at(-1)?.redirect).toBe("/my-space/runtime");
    for (const section of ["profile", "identity", "runtime", "devices"]) {
      expect(router.resolve(`/my-space/${section}`).name).toBe(`mySpace${section[0].toUpperCase()}${section.slice(1)}`);
    }
  });
});
