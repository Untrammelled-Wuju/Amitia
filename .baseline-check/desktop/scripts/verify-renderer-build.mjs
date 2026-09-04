import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { resolve, dirname, relative, extname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, "..");
const rendererDir = resolve(projectRoot, "dist/renderer");
const preloadDir = resolve(projectRoot, "dist/preload");
const petMainPath = resolve(rendererDir, "pet-main.js");

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

function stripQueryAndHash(value) {
  return value.split(/[?#]/, 1)[0];
}

function isExternalReference(ref) {
  return /^(?:[a-z][a-z0-9+.-]*:|\/\/|#)/i.test(ref);
}

function resolveLocalReference(fromPath, ref) {
  const clean = stripQueryAndHash(ref);
  if (!clean || isExternalReference(clean)) return null;
  const resolved = clean.startsWith("/")
    ? resolve(rendererDir, `.${clean}`)
    : resolve(dirname(fromPath), clean);
  const rel = relative(rendererDir, resolved);
  if (rel.startsWith("..") || rel === "..") {
    errors.push(
      `ESCAPE: ${relative(projectRoot, fromPath)} references path outside dist/renderer: ${ref}`,
    );
    return null;
  }
  return resolved;
}

function checkHtmlAssetReferences(htmlPath, label) {
  if (!existsSync(htmlPath)) return;
  const content = readFileSync(htmlPath, "utf8");
  const scriptMatches = content.matchAll(/src="([^"]+)"/g);
  const linkMatches = content.matchAll(/href="([^"]+\.css(?:[?#][^"]*)?)"/g);
  const allRefs = [...scriptMatches, ...linkMatches].map((m) => m[1]);

  for (const ref of allRefs) {
    if (ref.startsWith("/@fs")) {
      errors.push(`FORBIDDEN: ${label} contains development /@fs reference: ${ref}`);
      continue;
    }
    const resolved = resolveLocalReference(htmlPath, ref);
    if (resolved && !existsSync(resolved)) {
      errors.push(`BROKEN: ${label} references non-existent asset: ${ref}`);
    }
  }
}

function checkBundleNonEmpty(path, label) {
  if (!existsSync(path)) {
    errors.push(`MISSING: ${label} (${relative(projectRoot, path)})`);
    return false;
  }
  const stat = statSync(path);
  if (stat.size === 0) {
    errors.push(`EMPTY: ${label} bundle is 0 bytes`);
    return false;
  }
  return true;
}

function checkPreloadOutputs() {
  const requiredPreloads = ["index.cjs", "animation-preload.cjs"];
  for (const name of requiredPreloads) {
    const p = resolve(preloadDir, name);
    if (!existsSync(p)) {
      errors.push(`PRELOAD MISSING: ${name}`);
    }
  }

  for (const forbidden of ["pet-preload.cjs", "pet-combined-preload.cjs"]) {
    const p = resolve(preloadDir, forbidden);
    if (existsSync(p)) {
      errors.push(`FORBIDDEN PRELOAD: ${forbidden}`);
    }
  }
}

function extractModuleReferences(source) {
  const refs = new Set();
  const patterns = [
    /\b(?:import|export)\s+(?:[^"']*?\s+from\s*)?["']([^"']+)["']/g,
    /\bimport\s*\(\s*["']([^"']+)["']\s*\)/g,
  ];
  for (const pattern of patterns) {
    for (const match of source.matchAll(pattern)) refs.add(match[1]);
  }
  return refs;
}

function checkRendererModuleGraph(entryPath) {
  if (!checkBundleNonEmpty(entryPath, "pet-main bundle")) return;
  const pending = [entryPath];
  const visited = new Set();

  while (pending.length > 0) {
    const current = pending.pop();
    if (!current || visited.has(current)) continue;
    visited.add(current);

    let source;
    try {
      source = readFileSync(current, "utf8");
    } catch (error) {
      errors.push(
        `UNREADABLE: ${relative(projectRoot, current)} (${error instanceof Error ? error.message : String(error)})`,
      );
      continue;
    }

    if (/\.(?:ts|tsx)(?:[?#]|["'])/.test(source) || source.includes("/@fs/")) {
      errors.push(
        `FORBIDDEN: production renderer module contains TypeScript or /@fs source reference: ${relative(projectRoot, current)}`,
      );
    }

    for (const ref of extractModuleReferences(source)) {
      if (isExternalReference(ref)) continue;
      const resolved = resolveLocalReference(current, ref);
      if (!resolved) continue;
      if (!existsSync(resolved)) {
        errors.push(
          `BROKEN IMPORT: ${relative(projectRoot, current)} -> ${ref}`,
        );
        continue;
      }
      const extension = extname(stripQueryAndHash(resolved)).toLowerCase();
      if (extension === ".js" || extension === ".mjs") {
        pending.push(resolved);
      }
    }
  }

  if (visited.size === 0) {
    errors.push("MISSING: pet-main module graph could not be traversed");
  }
}

function checkPetMainIsViteOutput() {
  if (!existsSync(petMainPath)) return;
  const source = readFileSync(petMainPath, "utf8");
  if (source.includes("../desktop-pet/") || source.includes("../shared/")) {
    errors.push(
      "FORBIDDEN: pet-main.js still contains source-tree relative imports; it was not bundled by Vite/Rollup",
    );
  }
}

function main() {
  console.log("[verify-renderer-build] starting...");

  checkExists(rendererDir, "dist/renderer directory");
  checkExists(resolve(rendererDir, "pet.html"), "dist/renderer/pet.html");
  checkExists(resolve(rendererDir, "index.html"), "dist/renderer/index.html");

  checkNoTsInHtml(resolve(rendererDir, "pet.html"), "pet.html");
  checkNoTsInHtml(resolve(rendererDir, "index.html"), "index.html");

  checkHtmlAssetReferences(resolve(rendererDir, "pet.html"), "pet.html");
  checkHtmlAssetReferences(resolve(rendererDir, "index.html"), "index.html");

  checkRendererModuleGraph(petMainPath);
  checkPetMainIsViteOutput();
  checkPreloadOutputs();

  if (existsSync(rendererDir)) {
    const rootJs = readdirSync(rendererDir).filter((name) => name.endsWith(".js"));
    if (!rootJs.includes("pet-main.js")) {
      errors.push("MISSING: Vite must emit dist/renderer/pet-main.js as the production pet entry");
    }
  }

  if (warnings.length > 0) {
    console.log("[verify-renderer-build] warnings:");
    for (const warning of warnings) console.log(`  WARN: ${warning}`);
  }

  if (errors.length > 0) {
    console.error("[verify-renderer-build] FAILED:");
    for (const error of errors) console.error(`  ERROR: ${error}`);
    process.exit(1);
  }

  console.log("[verify-renderer-build] PASSED");
}

main();
