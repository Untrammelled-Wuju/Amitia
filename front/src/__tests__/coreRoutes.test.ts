import { describe, expect, it } from "vitest";
import { CORE_ROUTE_PATHS, CORE_ROUTE_ROOTS } from "@/router/coreRoutes";

describe("core route boundary", () => {
  it("keeps authentication, onboarding, recovery and error routes in the core shell", () => {
    expect(CORE_ROUTE_PATHS).toEqual([
      "/web-access",
      "/onboarding",
      "/privacy",
      "/usage-boundary",
      "/attention",
      "/404",
    ]);
    expect([...CORE_ROUTE_ROOTS]).toEqual([
      "/web-access",
      "/onboarding",
      "/privacy",
      "/usage-boundary",
      "/attention",
      "/404",
    ]);
  });
});
