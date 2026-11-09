import { describe, expect, it } from "vitest";
import { createMemoryHistory, createRouter } from "vue-router";
import { isNavigationAllowed } from "@/navigation/nav-whitelist";

describe("UI host navigation boundary", () => {
  it("allows registered dynamic routes without a hardcoded plugin path list", () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: "/chat", name: "chat", component: { template: "<div />" } },
        { path: "/runtime-debug", name: "runtime-debug", component: { template: "<div />" } },
        { path: "/404", name: "notFound", component: { template: "<div />" } },
        { path: "/:pathMatch(.*)*", name: "catchAll", component: { template: "<div />" } },
      ],
    });
    expect(isNavigationAllowed("/runtime-debug", router)).toBe(true);
    expect(isNavigationAllowed("/chat", router)).toBe(true);
    expect(isNavigationAllowed("/unknown-plugin-page", router)).toBe(false);
  });

  it("keeps onboarding, privacy and recovery navigation isolated", () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: "/onboarding", name: "onboarding", component: { template: "<div />" } },
        { path: "/privacy", name: "privacy", component: { template: "<div />" } },
        { path: "/web-access", name: "webAccess", component: { template: "<div />" } },
      ],
    });
    expect(isNavigationAllowed("/onboarding", router)).toBe(false);
    expect(isNavigationAllowed("/privacy", router)).toBe(false);
    expect(isNavigationAllowed("/web-access", router)).toBe(false);
  });
});
