import { access, readdir, readFile } from "node:fs/promises";
import { resolve, join, extname, relative } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const projectRoot = resolve(__dirname, "..");
const srcRoot = resolve(projectRoot, "src");
const managerPath = resolve(srcRoot, "main", "pet", "manager.ts");
const canonicalBridgePath = resolve(srcRoot, "main", "pet", "animation-player-bridge.ts");
const canonicalEnginePath = resolve(srcRoot, "desktop-pet", "animation", "animation-engine.ts");

const FORBIDDEN_PATTERNS = [
  "ActionPlayer",
  "pet:frame-update",
  "onFrameUpdate",
  "PlayerLike",
];

const SKIP_DIRS = new Set(["node_modules", "dist", "release", ".git"]);
const ALLOW_DOC_PATTERN = /doc|README|\.md$/i;
const errors = [];

async function scanDir(dirPath) {
  const entries = await readdir(dirPath, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = join(dirPath, entry.name);
    if (entry.isDirectory()) {
      if (SKIP_DIRS.has(entry.name)) continue;
      await scanDir(fullPath);
      continue;
    }
    if (!entry.isFile()) continue;
    const ext = extname(entry.name);
    if (ext !== ".ts" && ext !== ".tsx") continue;
    const relPath = relative(projectRoot, fullPath);
    if (ALLOW_DOC_PATTERN.test(relPath)) continue;
    const content = await readFile(fullPath, "utf8");
    for (const pattern of FORBIDDEN_PATTERNS) {
      if (content.includes(pattern)) {
        errors.push(`FORBIDDEN: "${pattern}" found in ${relPath}`);
      }
    }
  }
}

async function requireCanonicalPlayerPath() {
  for (const required of [canonicalBridgePath, canonicalEnginePath, managerPath]) {
    try {
      await access(required);
    } catch {
      errors.push(`REQUIRED: missing canonical desktop-pet player file ${relative(projectRoot, required)}`);
    }
  }

  let manager = "";
  try {
    manager = await readFile(managerPath, "utf8");
  } catch {
    return;
  }

  if (!manager.includes('import { AnimationPlayerBridge } from "./animation-player-bridge"')) {
    errors.push("REQUIRED: DesktopPetManager must import the canonical AnimationPlayerBridge");
  }
  if (!manager.includes("new AnimationPlayerBridge(")) {
    errors.push("REQUIRED: DesktopPetManager must instantiate the canonical AnimationPlayerBridge");
  }
  if (!manager.includes("setAnimationIpc(")) {
    errors.push("REQUIRED: canonical player must be connected to renderer animation IPC");
  }
}

async function main() {
  console.log("[verify-pet-player-singleton] scanning canonical desktop-pet player path...");
  try {
    await scanDir(srcRoot);
    await requireCanonicalPlayerPath();
  } catch (err) {
    console.error("[verify-pet-player-singleton] scan failed:", err.message);
    process.exit(1);
  }

  if (errors.length > 0) {
    console.error("[verify-pet-player-singleton] FAILED:");
    for (const error of errors) console.error(`  ERROR: ${error}`);
    process.exit(1);
  }

  console.log(
    "[verify-pet-player-singleton] PASSED: DesktopPetManager -> AnimationPlayerBridge -> renderer AnimationEngine is the only player path",
  );
}

main();
