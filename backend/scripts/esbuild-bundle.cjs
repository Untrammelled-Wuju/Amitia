
const esbuild = require("esbuild");
const path = require("path");

(async () => {
    const name = process.argv[2];
    const root = process.argv[3];
    const outDir = process.argv[4];
    const entry = path.join(root, "src/index.ts");
    const out = path.join(outDir, "bundle.mjs");
    console.log("=== Bundling " + name + " ===");
    console.log("Entry: " + entry);
    const result = await esbuild.build({
        entryPoints: [entry],
        bundle: true,
        platform: "node",
        target: "node22",
        format: "esm",
        outfile: out,
        absWorkingDir: root,
        banner: { js: "// SPDX-FileCopyrightText: 2026 \u5F6D\u65ED\n// SPDX-License-Identifier: AGPL-3.0-only" },
        logLevel: "info",
        metafile: true,
    });
    const output = result.metafile.outputs[out];
    console.log("Bundle size: " + output.bytes + " bytes");
    console.log("Inputs: " + Object.keys(result.metafile.inputs).length + " files");
})();
