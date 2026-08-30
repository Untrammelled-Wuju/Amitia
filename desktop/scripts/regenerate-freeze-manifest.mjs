import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const repoRoot = resolve(__dirname, "../..");

const FREEZE_SCOPE = [
  "backend/internal/desktoppet",
  "backend/internal/deviceruntime",
  "backend/internal/runtimeprofile",
  "backend/internal/migration",
  "backend/cmd/server",
  "desktop/src/main",
  "desktop/src/desktop-pet",
  "desktop/src/preload",
  "desktop/src/renderer",
  "desktop/src/shared",
  "front/src/runtime",
  "front/src/composables",
  "front/src/components",
  "mobile_app/lib/features/desktop_pet",
  "mobile_app/lib/core/backend_transport",
  "mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/nativeprovider/desktoppet",
  "desktop/vite.config.ts",
  "desktop/src/desktop-pet/animation/__tests__/animation-engine-readiness.test.ts",
  "desktop/src/desktop-pet/animation/__tests__/player-state-machine.test.ts",
  "desktop/src/desktop-pet/runtime/__tests__/runtime-handler-v2.test.ts",
  "desktop/src/main/pet/__tests__/manager.test.ts",
  "desktop/scripts/verify-desktop-pet-finalization.mjs",
  "desktop/scripts/release-integrity.mjs",
  "scripts/audit/verify-source-hygiene.mjs",
  "scripts/pack-source.ps1",
];

const EXCLUDED_NAMES = new Set([
  "node_modules",
  "dist",
  "__pycache__",
]);

const EXCLUDED_TOP_LEVEL_DIRS = new Set([
  "node_modules",
  "dist",
  "build",
  ".git",
  ".pub-cache",
  ".dart_tool",
  ".gradle",
  ".kotlin",
  ".cxx",
]);

function sha256File(filePath) {
  return createHash("sha256").update(readFileSync(filePath)).digest("hex");
}

function collectFiles(dirPath, result) {
  if (!existsSync(dirPath)) return;
  const entries = readdirSync(dirPath, { withFileTypes: true });
  for (const entry of entries) {
    if (EXCLUDED_NAMES.has(entry.name)) continue;
    const full = join(dirPath, entry.name);
    if (entry.isDirectory()) {
      collectFiles(full, result);
    } else if (entry.isFile()) {
      result.push(full);
    }
  }
}

const files = [];
for (const input of FREEZE_SCOPE) {
  const absolute = resolve(repoRoot, input);
  if (!existsSync(absolute)) {
    console.warn(`[warn] scope path missing: ${input}`);
    continue;
  }
  const stats = statSync(absolute);
  if (stats.isDirectory()) {
    collectFiles(absolute, files);
  } else if (stats.isFile()) {
    files.push(absolute);
  }
}

files.sort((a, b) => relative(repoRoot, a).localeCompare(relative(repoRoot, b)));

const lines = [];
for (const filePath of files) {
  const rel = relative(repoRoot, filePath).replace(/\\/g, "/");
  const hash = sha256File(filePath);
  lines.push(`${hash}  ${rel}`);
}

const outputPath = resolve(repoRoot, "DESKTOP_PET_FINALIZATION_SHA256SUMS.txt");
writeFileSync(outputPath, lines.join("\n") + "\n", "utf8");

console.log(`[freeze-manifest] generated: ${files.length} files -> DESKTOP_PET_FINALIZATION_SHA256SUMS.txt`);
