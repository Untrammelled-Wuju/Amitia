import { build } from "esbuild";
import path from "path";

const name = process.argv[2];
const outDir = process.argv[3];
const rootDir = process.cwd();
const entry = path.join(rootDir, "src/index.ts");
const outFile = path.join(outDir, "bundle.mjs");

console.log("=== Bundling " + name + " ===");
console.log("Root: " + rootDir);
console.log("Entry: " + entry);
const result = await build({
    entryPoints: [entry],
    bundle: true,
    platform: "node",
    target: "node22",
    format: "esm",
    outfile: outFile,
    banner: { js: "// SPDX-FileCopyrightText: 2026 \u5F6D\u65ED\n// SPDX-License-Identifier: AGPL-3.0-only" },
    logLevel: "info",
    metafile: true,
});
const output = result.metafile.outputs[outFile];
console.log("Bundle size: " + output.bytes + " bytes");
console.log("Inputs: " + Object.keys(result.metafile.inputs).length + " files");