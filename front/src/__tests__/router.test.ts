import { describe, expect, it, vi } from "vitest";

vi.mock("../runtime/runtime-capabilities", () => ({
  shouldUseHashRouting: () => true,
}));

import router from "../router";

describe("router", () => {
  it("uses hash URLs for local desktop files", () => {
    expect(router.resolve("/chat").href).toContain("#/chat");
  });

  it("redirects unmatched startup paths to chat", () => {
    const route = router.resolve("/renderer/index.html");
    expect(route.matched.at(-1)?.redirect).toBe("/chat");
  });
});
