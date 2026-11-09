import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { loadConfig } from "./extension-config.js";

function tempConfig(name: string, content: string): string {
  const directory = mkdtempSync(join(tmpdir(), "amitia-config-"));
  const file = join(directory, name);
  writeFileSync(file, content, "utf8");
  return file;
}

describe("loadConfig", () => {
  it("loads JSON and merges defaults", async () => {
    const file = tempConfig("amitia-extension.config.json", JSON.stringify({
      outputDir: "./build",
      targets: ["linux-x64", "macos-arm64"],
    }));
    const config = await loadConfig(file);
    expect(config.manifest).toBe("./manifest.ts");
    expect(config.outputDir).toBe("./build");
    expect(config.targets).toEqual(["linux-x64", "macos-arm64"]);
  });

  it("loads ESM default and named config exports", async () => {
    const defaultFile = tempConfig("default.mjs", "export default { packageDir: './default-package', targets: ['windows-x64'] };\n");
    const namedFile = tempConfig("named.mjs", "export const config = { packageDir: './named-package', targets: ['linux-arm64'] };\n");
    expect((await loadConfig(defaultFile)).packageDir).toBe("./default-package");
    expect((await loadConfig(namedFile)).packageDir).toBe("./named-package");
  });

  it("rejects invalid target values", async () => {
    const file = tempConfig("invalid.json", JSON.stringify({ targets: ["plan9"] }));
    await expect(loadConfig(file)).rejects.toThrow("unknown build target");
  });

  it("rejects unsupported config file types", async () => {
    const file = tempConfig("amitia-extension.config.yaml", "manifest: manifest.ts\n");
    await expect(loadConfig(file)).rejects.toThrow("unsupported config extension");
  });
});
