import { createHash } from "node:crypto";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { buildPackage, inspectPackage, readZip } from "./archive.js";

const temporaryRoots: string[] = [];

afterEach(() => {
  for (const root of temporaryRoots.splice(0)) {
    rmSync(root, { recursive: true, force: true });
  }
});

function byteOrderedTreeHash(entries: Array<{ path: string; hash: string }>): string {
  const sorted = [...entries].sort((left, right) =>
    Buffer.compare(Buffer.from(left.path, "utf8"), Buffer.from(right.path, "utf8")),
  );
  const hash = createHash("sha256");
  for (const entry of sorted) {
    hash.update(entry.path);
    hash.update(Buffer.from([0]));
    hash.update(entry.hash);
    hash.update(Buffer.from([0]));
  }
  return hash.digest("hex");
}

describe("archive integrity", () => {
  it("uses byte-order path sorting for the content tree hash", () => {
    const root = mkdtempSync(join(tmpdir(), "amitiax-archive-"));
    temporaryRoots.push(root);
    const manifestPath = join(root, "manifest.json");
    writeFileSync(
      manifestPath,
      `${JSON.stringify(
        {
          manifestVersion: 1,
          extension: {
            id: "com.example/case-order",
            name: { default: "Case Order" },
            description: { default: "Case order fixture" },
            version: "1.0.0",
          },
          publisher: { id: "com.example", displayName: "Example" },
          modules: [],
          integrity: { algorithm: "sha256", contentTreeHash: "" },
        },
        null,
        2,
      )}\n`,
    );
    const componentRoot = join(root, "assets", "legacy", "components");
    mkdirSync(componentRoot, { recursive: true });
    writeFileSync(join(root, "assets", "legacy", "ProactiveRules.vue"), "root");
    writeFileSync(join(componentRoot, "ActiveMessageSettings.vue"), "component");

    const outputPath = join(root, "case-order.amitiax");
    buildPackage(root, manifestPath, outputPath);
    const entries = readZip(readFileSync(outputPath));
    const filesDocument = JSON.parse(entries.get("integrity/files.json")?.toString("utf8") ?? "{}") as {
      files?: Record<string, { path: string; hash: string }>;
    };
    const treeDocument = JSON.parse(entries.get("integrity/content-tree.json")?.toString("utf8") ?? "{}") as {
      treeHash?: string;
    };
    const expected = byteOrderedTreeHash(Object.values(filesDocument.files ?? {}));

    expect(treeDocument.treeHash).toBe(expected);
    expect(inspectPackage(outputPath).treeHash).toBe(expected);
  });
});
