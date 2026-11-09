import { describe, expect, it, vi } from "vitest";

vi.mock("@/runtime/runtime-adapter", () => ({
  getRuntimeConnection: vi.fn(async () => ({ apiBaseURL: "https://core.example/" })),
}));

import { pageModuleComponent, pageModuleURL } from "@/ui-runtime/pageModuleRuntime";

describe("extension page client modules", () => {
  it("builds a capability-scoped module URL without exposing the extension bundle root", async () => {
    const url = await pageModuleURL({
      extensionId: "com.example/page",
      pageId: "main",
      generation: 7,
      entryPath: "modules/page/index.mjs",
      entryHash: "sha256-page",
      moduleToken: "token-value",
    });
    expect(url).toBe(
      "https://core.example/api/public/extensions/ui/page-module/com.example%2Fpage/main/modules/page/index.mjs?generation=7&integrity=sha256-page&accessToken=token-value",
    );
  });

  it("rejects page modules without a host-issued capability", async () => {
    await expect(pageModuleURL({
      extensionId: "com.example/page",
      pageId: "main",
      generation: 7,
      entryPath: "modules/page/index.mjs",
    })).rejects.toThrow("访问令牌");
  });

  it("returns a reusable component for the same page generation", () => {
    const descriptor = {
      extensionId: "com.example/page",
      pageId: "main",
      generation: 1,
      entryPath: "modules/page/index.mjs",
      moduleToken: "token",
    };
    expect(pageModuleComponent(descriptor)).toBe(pageModuleComponent(descriptor));
  });
});
