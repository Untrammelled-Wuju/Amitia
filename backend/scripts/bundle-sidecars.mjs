const { build } = require("esbuild");
const path = require("path");
const fs = require("fs");

async function bundleSidecar(name, rootDir, outDir) {
    const entry = path.join(rootDir, "src/index.ts");
    const outFile = path.join(outDir, "bundle.mjs");
    
    console.log(`=== Bundling ${name} ===`);
    console.log(`Entry: ${entry}`);
    console.log(`Output: ${outFile}`);
    
    const result = await build({
        entryPoints: [entry],
        bundle: true,
        platform: "node",
        target: "node22",
        format: "esm",
        outfile: outFile,
        banner: { js: "// SPDX-FileCopyrightText: 2026 彭旭\n// SPDX-License-Identifier: AGPL-3.0-only" },
        logLevel: "info",
        metafile: true,
        // Keep node: builtins external is wrong - we actually need them bundled or referenced as node: 
        // For ESM bundle, let esbuild handle node: imports
    });
    
    const output = result.metafile.outputs[outFile];
    console.log(`Bundle size: ${output.bytes} bytes`);
    
    // Verify the bundle
    const content = fs.readFileSync(outFile, "utf-8");
    if (content.includes("require(") && !content.includes("node:")) {
        console.log("WARNING: Bundle may contain CommonJS require()");
    }
    return result;
}

const wechatRoot = process.argv[2];
const qqRoot = process.argv[3];
const outBase = process.argv[4];

(async () => {
    await bundleSidecar("WeChat", wechatRoot, path.join(outBase, "sidecar"));
    await bundleSidecar("QQ", qqRoot, path.join(outBase, "qq-sidecar"));
    console.log("=== Both bundles built ===");
})().catch(e => { console.error(e); process.exit(1); });

