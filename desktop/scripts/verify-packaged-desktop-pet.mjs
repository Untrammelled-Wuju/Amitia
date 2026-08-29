import { existsSync, readFileSync } from "node:fs";
import { resolve, dirname, relative } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const projectRoot = resolve(__dirname, "..");
const rendererDir = resolve(projectRoot, "dist/renderer");
const unpacked = process.argv[2]
  ? resolve(projectRoot, process.argv[2])
  : resolve(projectRoot, "release-ci", "win-unpacked");
const appAsar = resolve(unpacked, "resources", "app.asar");
const errors = [];

function requireFile(path, label) {
  if (!existsSync(path)) {
    errors.push(`MISSING: ${label} (${relative(projectRoot, path)})`);
    return false;
  }
  return true;
}

function stripQueryAndHash(value) {
  return value.split(/[?#]/, 1)[0];
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

function collectRendererBundleFiles(entryPath) {
  const pending = [entryPath];
  const visited = new Set();
  const result = new Set();
  while (pending.length > 0) {
    const current = pending.pop();
    if (!current || visited.has(current) || !existsSync(current)) continue;
    visited.add(current);
    result.add(relative(rendererDir, current).replace(/\\/g, "/"));
    const source = readFileSync(current, "utf8");
    for (const ref of extractModuleReferences(source)) {
      const clean = stripQueryAndHash(ref);
      if (!clean || !clean.startsWith(".")) continue;
      const resolved = resolve(dirname(current), clean);
      const rel = relative(rendererDir, resolved);
      if (rel.startsWith("..") || rel === "..") continue;
      result.add(rel.replace(/\\/g, "/"));
      if (/\.(?:m?js)$/i.test(clean)) pending.push(resolved);
    }
  }
  return [...result];
}

requireFile(resolve(unpacked, "Amitia.exe"), "packaged Amitia.exe");
if (requireFile(appAsar, "packaged resources/app.asar")) {
  const asar = readFileSync(appAsar);
  const requiredNames = new Set([
    "index.cjs",
    "animation-preload.cjs",
    "pet.html",
    "pet-main.js",
  ]);

  const petMain = resolve(rendererDir, "pet-main.js");
  if (!existsSync(petMain)) {
    errors.push("MISSING: dist/renderer/pet-main.js before package verification");
  } else {
    for (const bundlePath of collectRendererBundleFiles(petMain)) {
      requiredNames.add(bundlePath.split("/").pop());
    }
  }

  for (const name of requiredNames) {
    if (!name || !asar.includes(Buffer.from(name, "utf8"))) {
      errors.push(`ASAR MISSING: ${name}`);
    }
  }
  for (const forbidden of ["pet-preload.cjs", "pet-combined-preload.cjs"]) {
    if (asar.includes(Buffer.from(forbidden, "utf8"))) {
      errors.push(`ASAR FORBIDDEN: ${forbidden}`);
    }
  }
}

if (errors.length > 0) {
  console.error("[verify-packaged-desktop-pet] FAILED:");
  for (const error of errors) console.error(`  ERROR: ${error}`);
  process.exit(1);
}

console.log("[verify-packaged-desktop-pet] PASSED");
