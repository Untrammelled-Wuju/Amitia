import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const repoRoot = resolve(__dirname, "../..");

const FULLY_IGNORED_DIRS = new Set([
  ".git",
  "node_modules",
  ".pub-cache",
  ".dart_tool",
  ".gradle",
  ".kotlin",
  ".cxx",
  ".cache",
]);

const FORBIDDEN_FILE_NAMES = new Set([
  "trace.atrace",
  ".qdrant-initialized",
  "raft_state.json",
  ".DS_Store",
  "Thumbs.db",
]);

const FORBIDDEN_EXTENSIONS = new Set([
  ".atrace",
  ".wal",
  ".db",
  ".db-shm",
  ".db-wal",
  ".db-journal",
]);

const RUNTIME_DATA_DIR_NAMES = new Set([
  "storage",
  "surrealdb",
  "qdrant",
  "collections",
  "snapshots",
  "wal",
]);

const PROTECTED_PATHS = [
  "runtime/build",
  "runtime/validation",
  "runtime/out",
  "runtime/mobile_app",
  "backend/node/node.exe",
  "backend/node/node.exe.zip",
  "backend/log",
  "desktop/resources/core",
  "desktop/resources/qdrant",
  "desktop/resources/surrealdb",
  "backend/pkg/database/qdrant",
  "backend/pkg/database/surrealdb",
  "backend/config/providers/qdrant",
  "config/providers/qdrant",
  "mobile_app/android/amitia-runtime/src/main/jniLibs",
  "mobile_app/android/app/src/main/assets/runtime-package",
  "testplugins",
  "backend/internal/qdrant",
  "backend/cmd/qdrant",
  "backend/internal/desktoppet/storage",
  "backend/internal/desktoppet/release/storage",
  "backend/internal/extension/kernel/storage",
  "backend/internal/extension/kernel/package_security/snapshots",
  "backend/internal/gamehost/storage",
  "backend/internal/gamehost/snapshots",
  "backend/qdrant",
  "backend/surrealdb",
];

function isProtectedPath(relativePath) {
  const normalized = relativePath.replace(/\\/g, "/");
  for (const item of PROTECTED_PATHS) {
    if (normalized.startsWith(item) || normalized === item) {
      return true;
    }
  }
  return false;
}

function isRuntimeDataDir(dirName, relativePath) {
  if (!RUNTIME_DATA_DIR_NAMES.has(dirName)) return false;
  if (isProtectedPath(relativePath)) return false;
  return true;
}

function scanDirectory(dirPath, violations) {
  let entries;
  try {
    entries = readdirSync(dirPath, { withFileTypes: true });
  } catch {
    return;
  }

  for (const entry of entries) {
    const fullPath = join(dirPath, entry.name);
    const relPath = relative(repoRoot, fullPath).replace(/\\/g, "/");

    if (entry.isDirectory()) {
      if (FULLY_IGNORED_DIRS.has(entry.name)) continue;
      if (isProtectedPath(relPath)) continue;
      if (FORBIDDEN_FILE_NAMES.has(entry.name)) {
        violations.push(`forbidden directory: ${relPath}`);
        continue;
      }
      if (isRuntimeDataDir(entry.name, relPath)) {
        violations.push(`forbidden runtime data directory: ${relPath}`);
        continue;
      }
      scanDirectory(fullPath, violations);
    } else if (entry.isFile()) {
      if (isProtectedPath(relPath)) continue;
      if (FORBIDDEN_FILE_NAMES.has(entry.name)) {
        violations.push(`forbidden file: ${relPath}`);
        continue;
      }
      const lastDot = entry.name.lastIndexOf(".");
      if (lastDot >= 0) {
        const ext = entry.name.slice(lastDot);
        if (FORBIDDEN_EXTENSIONS.has(ext)) {
          violations.push(`forbidden file type (${ext}): ${relPath}`);
          continue;
        }
      }
    }
  }
}

const violations = [];
let rootEntries;
try {
  rootEntries = readdirSync(repoRoot, { withFileTypes: true });
} catch {
  rootEntries = [];
}

for (const entry of rootEntries) {
  if (FULLY_IGNORED_DIRS.has(entry.name)) continue;
  if (entry.name === ".gitignore" || entry.name === ".git") continue;

  const fullPath = join(repoRoot, entry.name);
  if (entry.isDirectory()) {
    if (isProtectedPath(entry.name)) continue;
    if (FORBIDDEN_FILE_NAMES.has(entry.name)) {
      violations.push(`forbidden directory: ${entry.name}`);
      continue;
    }
    if (isRuntimeDataDir(entry.name, entry.name)) {
      violations.push(`forbidden runtime data directory: ${entry.name}`);
      continue;
    }
    scanDirectory(fullPath, violations);
  } else if (entry.isFile()) {
    if (isProtectedPath(entry.name)) continue;
    if (FORBIDDEN_FILE_NAMES.has(entry.name)) {
      violations.push(`forbidden file: ${entry.name}`);
      continue;
    }
    const lastDot = entry.name.lastIndexOf(".");
    if (lastDot >= 0) {
      const ext = entry.name.slice(lastDot);
      if (FORBIDDEN_EXTENSIONS.has(ext)) {
        violations.push(`forbidden file type (${ext}): ${entry.name}`);
        continue;
      }
    }
  }
}

if (violations.length > 0) {
  console.error(`[verify-source-hygiene] FAILED: ${violations.length} violation(s) found`);
  for (const v of violations) {
    console.error(`  - ${v}`);
  }
  process.exit(1);
}

console.log(`[verify-source-hygiene] PASSED: source tree is clean (${violations.length} violations)`);
