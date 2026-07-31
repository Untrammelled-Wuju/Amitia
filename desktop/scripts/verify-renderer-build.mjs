import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { resolve, dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, "..");
const rendererDir = resolve(projectRoot, "dist/renderer");
const preloadDir = resolve(projectRoot, "dist/preload");

const errors = [];
const warnings = [];

function checkExists(path, label) {
  if (!existsSync(path)) {
    errors.push(`MISSING: ${label} (${relative(projectRoot, path)})`);
    return false;
  }
  return true;
}

function checkNoTsInHtml(htmlPath, label) {
  if (!existsSync(htmlPath)) {
    errors.push(`MISSING: ${label} (${relative(projectRoot, htmlPath)})`);
    return;
  }
  const content = readFileSync(htmlPath, "utf8");
  const tsMatches = content.match(/src="[^"]*\.ts"/g);
  if (tsMatches) {
    errors.push(`FORBIDDEN: ${label} references .ts files: ${tsMatches.join(", ")}`);
  }
  const tsxMatches = content.match(/src="[^"]*\.tsx"/g);
  if (tsxMatches) {
    errors.push(`FORBIDDEN: ${label} references .tsx files: ${tsxMatches.join(", ")}`);
  }
}

function checkHtmlAssetReferences(htmlPath, label) {
  if (!existsSync(htmlPath)) return;
  const content = readFileSync(htmlPath, "utf8");
  const scriptMatches = content.matchAll(/src="([^"]+)"/g);
  const linkMatches = content.matchAll(/href="([^"]+\.css)"/g);
  const allRefs = [...scriptMatches, ...linkMatches].map((m) => m[1]);

  for (const ref of allRefs) {
    if (ref.startsWith("http") || ref.startsWith("//")) continue;
    if (ref.startsWith("/@fs")) continue;
    const resolved = resolve(dirname(htmlPath), ref);
    const relToRenderer = relative(rendererDir, resolved);
    if (relToRenderer.startsWith("..")) {
      errors.push(`ESCAPE: ${label} references path outside dist/renderer: ${ref}`);
      continue;
    }
    if (!existsSync(resolved)) {
      errors.push(`BROKEN: ${label} references non-existent asset: ${ref}`);
    }
  }
}

function checkBundleNonEmpty(path, label) {
  if (!existsSync(path)) {
    errors.push(`MISSING: ${label} (${relative(projectRoot, path)})`);
    return;
  }
  const stat = statSync(path);
  if (stat.size === 0) {
    errors.push(`EMPTY: ${label} bundle is 0 bytes`);
  }
}

function checkPreloadOutputs() {
  const requiredPreloads = [
    "index.cjs",
    "pet-preload.cjs",
    "pet-combined-preload.cjs",
    "animation-preload.cjs",
  ];
  for (const name of requiredPreloads) {
    const p = resolve(preloadDir, name);
    if (!existsSync(p)) {
      warnings.push(`PRELOAD MISSING: ${name} (may not be needed for this build)`);
    }
  }
}

function checkPetMainBundle() {
  const candidates = [
    "pet-main.js",
    "assets/pet-main.js",
  ];
  let found = false;
  for (const c of candidates) {
    const p = resolve(rendererDir, c);
    if (existsSync(p)) {
      checkBundleNonEmpty(p, "pet-main bundle");
      found = true;
    }
  }
  if (!found) {
    const files = existsSync(rendererDir) ? readdirSync(rendererDir) : [];
    const jsFiles = files.filter((f) => f.endsWith(".js"));
    const assetDir = resolve(rendererDir, "assets");
    const assetFiles = existsSync(assetDir) ? readdirSync(assetDir) : [];
    const assetJsFiles = assetFiles.filter((f) => f.endsWith(".js"));

    if (jsFiles.length === 0 && assetJsFiles.length === 0) {
      errors.push("MISSING: no JS bundles found in dist/renderer");
    } else {
      warnings.push(`pet-main bundle not found at expected paths, but JS files exist: ${[...jsFiles, ...assetJsFiles.map((f) => "assets/" + f)].join(", ")}`);
    }
  }
}

function main() {
  console.log("[verify-renderer-build] starting...");

  checkExists(rendererDir, "dist/renderer directory");
  checkExists(resolve(rendererDir, "pet.html"), "dist/renderer/pet.html");

  checkNoTsInHtml(resolve(rendererDir, "pet.html"), "pet.html");
  checkNoTsInHtml(resolve(rendererDir, "index.html"), "index.html");

  checkHtmlAssetReferences(resolve(rendererDir, "pet.html"), "pet.html");
  checkHtmlAssetReferences(resolve(rendererDir, "index.html"), "index.html");

  checkPetMainBundle();
  checkPreloadOutputs();

  if (warnings.length > 0) {
    console.log("[verify-renderer-build] warnings:");
    for (const w of warnings) {
      console.log(`  WARN: ${w}`);
    }
  }

  if (errors.length > 0) {
    console.error("[verify-renderer-build] FAILED:");
    for (const e of errors) {
      console.error(`  ERROR: ${e}`);
    }
    process.exit(1);
  }

  console.log("[verify-renderer-build] PASSED");
}

main();
