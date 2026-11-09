import { describe, expect, it } from "vitest";
import { validateManifest } from "../manifest";
import { listTemplateKinds } from "./index";
import { getTemplateDescriptor } from "./scaffold";

const input = {
  extensionId: "com.example/scaffold-test",
  publisher: "com.example",
  displayName: "Scaffold Test",
  description: "Scaffold Test",
  version: "0.1.0",
  license: "MIT",
  targets: ["windows-x64", "macos-arm64", "linux-x64"],
  sdkVersion: "1.0.0",
};

describe("tool scaffold", () => {
  it("declares the runtime entry and static tool contribution", () => {
    const descriptor = getTemplateDescriptor("tool", input);
    const manifest = JSON.parse(descriptor.files.find((file) => file.path === "manifest.json")?.content ?? "{}");
    const module = manifest.modules[0];
    expect(module.runtime.entry).toBe("dist/main-tool.js");
    expect(manifest.compatibility.platforms).toEqual(["windows", "macos", "linux"]);
    expect(module.dynamic).toMatchObject({ allowedAdapters: ["tool"], maxContributions: 1, maxTools: 1 });
    expect(module.contributions[0]).toMatchObject({
      kind: "tool",
      adapter: "tool",
      spec: {
        toolId: "com.example/scaffold-test/hello",
        handlerName: "hello",
        runtime: { runtimeType: "javascript", handlerName: "hello" },
      },
    });
  });

  it("binds the SDK handler to the declared manifest handler name", () => {
    const descriptor = getTemplateDescriptor("tool", input);
    const entry = descriptor.files.find((file) => file.path === "src/main-tool.ts")?.content ?? "";
    expect(entry).toContain('metadata: { handlerName: "hello" }');
    expect(entry).toContain("defineTool(");
    expect(entry).toContain("successResult(");
  });

  it.each(listTemplateKinds())("%s template emits a canonical manifest", (kind) => {
    const descriptor = getTemplateDescriptor(kind, input);
    const manifest = JSON.parse(descriptor.files.find((file) => file.path === "manifest.json")?.content ?? "{}");
    expect(validateManifest(manifest)).toEqual([]);
    expect(descriptor.files.some((file) => file.path.startsWith("src/") && file.path.endsWith(".ts"))).toBe(true);
  });
});
