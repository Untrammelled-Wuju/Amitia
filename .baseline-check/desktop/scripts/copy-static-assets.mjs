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
