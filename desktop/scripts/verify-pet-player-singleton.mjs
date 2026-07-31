import { readdir, readFile } from "node:fs/promises";
import { resolve, join, extname, relative } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const projectRoot = resolve(__dirname, "..");
const srcRoot = resolve(projectRoot, "src");

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
    } else if (entry.isFile()) {
      const ext = extname(entry.name);
      if (ext !== ".ts" && ext !== ".tsx") continue;
      const relPath = relative(projectRoot, fullPath);
      if (ALLOW_DOC_PATTERN.test(relPath)) continue;
      try {
        const content = await readFile(fullPath, "utf8");
        for (const pattern of FORBIDDEN_PATTERNS) {
          if (content.includes(pattern)) {
            errors.push(`FORBIDDEN: "${pattern}" found in ${relPath}`);
          }
        }
      } catch {
        void 0;
      }
    }
  }
}

async function main() {
  console.log("[verify-pet-player-singleton] 开始扫描...");
  try {
    await scanDir(srcRoot);
  } catch (err) {
    console.error("[verify-pet-player-singleton] 扫描失败:", err.message);
    process.exit(1);
  }

  if (errors.length > 0) {
    console.error("[verify-pet-player-singleton] FAILED:");
    for (const e of errors) {
      console.error(`  ERROR: ${e}`);
    }
    process.exit(1);
  }

  console.log("[verify-pet-player-singleton] PASSED");
}

main();
