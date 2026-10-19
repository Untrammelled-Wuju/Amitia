import { describe, expect, it } from "vitest";
import sandboxFrameSource from "../components/extension/SandboxWebUIFrame.vue?raw";
import sandboxSessionCacheSource from "../components/extension/sandboxSessionCache.ts?raw";
import extensionPageHostSource from "../components/extension/ExtensionPageHost.vue?raw";

describe("extension sandbox generation contract", () => {
  it("uses the server session generation for the ready handshake", () => {
    expect(sandboxFrameSource).toContain("if (data.generation !== sessionGeneration.value) return;");
    expect(sandboxFrameSource).toContain("generation: sessionGeneration.value");
    expect(sandboxFrameSource).not.toContain("if (data.generation !== props.contribution.generation) return;");
  });

  it("hydrates session generation from the backend response and cache probe", () => {
    expect(sandboxSessionCacheSource).toContain("generation: number;");
    expect(sandboxSessionCacheSource).toContain(
      'generation: typeof data.generation === "number" ? data.generation : options.contribution.generation',
    );
    expect(sandboxFrameSource).toContain(
      'generation: typeof info.data?.generation === "number" ? info.data.generation : cached.generation',
    );
  });

  it("passes the page session generation into the extension page contribution", () => {
    expect(extensionPageHostSource).toContain("generation: pageSessionGeneration.value");
    expect(extensionPageHostSource).toContain("pageSessionGeneration.value = result.generation ?? 0");
  });

  it("uses the page contribution id without prefixing it twice", () => {
    expect(extensionPageHostSource).toContain("contributionId: pageId.value");
  });

  it("stops page polling after unmount and ignores stale responses", () => {
    expect(extensionPageHostSource).toContain("let pageRequestToken = 0;");
    expect(extensionPageHostSource).toContain("disposed = true;");
    expect(extensionPageHostSource).toContain("void pollPageStatus(token);");
  });
});
