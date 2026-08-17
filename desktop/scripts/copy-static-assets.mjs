import { cpSync, existsSync, mkdirSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, "..");

const STATIC_SOURCES = [
  {
    from: "src/renderer/update-check",
    to: "dist/renderer/update-check",
    recursive: true,
  },
  {
    from: "src/renderer/pet.html",
    to: "dist/renderer/pet.html",
    recursive: false,
  },
  {
    from: "dist-types/src/renderer/pet-main.js",
    to: "dist/renderer/pet-main.js",
    recursive: false,
  },
];

const NEVER_OVERWRITE = [
  "dist/renderer/index.html",
  "dist/renderer/pet.html",
  "dist/renderer/pet-main.js",
  "dist/renderer/assets/",
];

function copyStaticAssets() {
  for (const entry of STATIC_SOURCES) {
    const src = resolve(projectRoot, entry.from);
    const dest = resolve(projectRoot, entry.to);
    if (!existsSync(src)) {
      continue;
    }
    const destDir = entry.recursive ? dest : dirname(dest);
    if (!existsSync(destDir)) {
      mkdirSync(destDir, { recursive: true });
    }
    cpSync(src, dest, { recursive: entry.recursive });
    console.log(`[copy-static-assets] ${entry.from} -> ${entry.to}`);
  }
  console.log("[copy-static-assets] done");
}

copyStaticAssets();
