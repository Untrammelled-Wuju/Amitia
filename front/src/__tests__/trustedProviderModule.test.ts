import { describe, expect, it } from "vitest";
import { canLoadTrustedWebModule, selectProviderEntry, trustedWebModule } from "@/ui-runtime/providerRuntime";
import providerHostSource from "../components/ui-runtime/UIProviderHost.vue?raw";
import type { UIProviderDefinition } from "@/ui-runtime/types";

const provider: UIProviderDefinition = {
  providerId: "channel-page",
  extensionId: "com.example/channel",
  capability: "page.provider",
  mode: "replace",
  trustLevel: "official",
  enabled: true,
  entries: {
    web: {
      type: "web_module",
      contributionId: "channel-page",
      path: "modules/channel/page/index.mjs",
      stylePath: "modules/channel/page/styles.css",
      contentHash: "sha256:test",
      styleHash: "sha256:style",
    },
  },
};

describe("same-page provider modules", () => {
  it("accepts only trusted web modules", () => {
    const entry = selectProviderEntry(provider, "web");
    expect(entry).not.toBeNull();
    expect(canLoadTrustedWebModule(provider, entry!)).toBe(true);
    expect(canLoadTrustedWebModule({ ...provider, trustLevel: "unknown" }, entry!)).toBe(false);
    expect(trustedWebModule(provider, entry!)).not.toBeNull();
  });

  it("passes only the provider action and navigation facade to client modules", () => {
    expect(providerHostSource).toContain(':ui-actions="providerActions"');
    expect(providerHostSource).toContain(':ui-navigate="navigateProvider"');
    expect(providerHostSource).not.toContain("webui/bridge");
  });
});
