import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const repoRoot = resolve(__dirname, "../..");

function gitBinary() {
  if (process.env.GIT_BIN) return process.env.GIT_BIN;
  if (process.platform !== "win32") return "git";
  const configured = "C:\\Code\\Git\\Git\\bin\\git.exe";
  return existsSync(configured) ? configured : "git.exe";
}

function ignoredByGit(paths) {
  const ignored = new Set();
  if (paths.length === 0) return ignored;
  const result = spawnSync(gitBinary(), ["-C", repoRoot, "check-ignore", "--stdin"], {
    input: paths.join("\n"),
    encoding: "utf8",
    shell: false,
  });
  if (result.error) return ignored;
  for (const line of (result.stdout ?? "").split(/\r?\n/)) {
    const value = line.trim().replace(/\\/g, "/");
    if (value) ignored.add(value);
  }
  return ignored;
}

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
  ".p12",
  ".pfx",
  ".keystore",
  ".jks",
]);

const FORBIDDEN_NAME_PATTERNS = [
  /(^|[._-])signing[._-]local([._-]|$)/i,
  /(^|[._-])release[._-]key(store)?([._-]|$)/i,
];

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

function hasForbiddenSensitiveName(fileName) {
  return FORBIDDEN_NAME_PATTERNS.some((pattern) => pattern.test(fileName));
}

function scanDirectory(dirPath, violations) {
  let entries;
  try {
    entries = readdirSync(dirPath, { withFileTypes: true });
  } catch (error) {
  console.error("[original-error]", error);
return;
  }

  for (const entry of entries) {
    const fullPath = join(dirPath, entry.name);
    const relPath = relative(repoRoot, fullPath).replace(/\\/g, "/");

    if (entry.isDirectory()) {
      if (FULLY_IGNORED_DIRS.has(entry.name)) continue;
      if (isProtectedPath(relPath)) continue;
      if (FORBIDDEN_FILE_NAMES.has(entry.name)) {
        violations.push({ path: relPath, message: `forbidden directory: ${relPath}` });
        continue;
      }
      if (isRuntimeDataDir(entry.name, relPath)) {
        violations.push({ path: relPath, message: `forbidden runtime data directory: ${relPath}` });
        continue;
      }
      scanDirectory(fullPath, violations);
    } else if (entry.isFile()) {
      if (isProtectedPath(relPath)) continue;
      if (FORBIDDEN_FILE_NAMES.has(entry.name) || hasForbiddenSensitiveName(entry.name)) {
        violations.push({ path: relPath, message: `forbidden file: ${relPath}` });
        continue;
      }
      const lastDot = entry.name.lastIndexOf(".");
      if (lastDot >= 0) {
        const ext = entry.name.slice(lastDot);
        if (FORBIDDEN_EXTENSIONS.has(ext)) {
          violations.push({ path: relPath, message: `forbidden file type (${ext}): ${relPath}` });
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
} catch (error) {
  console.error("[original-error]", error);
rootEntries = [];
}

for (const entry of rootEntries) {
  if (FULLY_IGNORED_DIRS.has(entry.name)) continue;
  if (entry.name === ".gitignore" || entry.name === ".git") continue;

  const fullPath = join(repoRoot, entry.name);
  if (entry.isDirectory()) {
    if (isProtectedPath(entry.name)) continue;
    if (FORBIDDEN_FILE_NAMES.has(entry.name)) {
      violations.push({ path: entry.name, message: `forbidden directory: ${entry.name}` });
      continue;
    }
    if (isRuntimeDataDir(entry.name, entry.name)) {
      violations.push({ path: entry.name, message: `forbidden runtime data directory: ${entry.name}` });
      continue;
    }
    scanDirectory(fullPath, violations);
  } else if (entry.isFile()) {
    if (isProtectedPath(entry.name)) continue;
    if (FORBIDDEN_FILE_NAMES.has(entry.name) || hasForbiddenSensitiveName(entry.name)) {
      violations.push({ path: entry.name, message: `forbidden file: ${entry.name}` });
      continue;
    }
    const lastDot = entry.name.lastIndexOf(".");
    if (lastDot >= 0) {
      const ext = entry.name.slice(lastDot);
      if (FORBIDDEN_EXTENSIONS.has(ext)) {
        violations.push({ path: entry.name, message: `forbidden file type (${ext}): ${entry.name}` });
        continue;
      }
    }
  }
}

const ignoredPaths = ignoredByGit(violations.map((item) => item.path));
const blocking = violations.filter((item) => !ignoredPaths.has(item.path.replace(/\\/g, "/")));

if (blocking.length > 0) {
  console.error(`[verify-source-hygiene] FAILED: ${blocking.length} violation(s) found`);
  for (const item of blocking) {
    console.error(`  - ${item.message}`);
  }
  process.exit(1);
}

console.log(
  `[verify-source-hygiene] PASSED: source tree is clean (${blocking.length} blocking, ${
    violations.length - blocking.length
  } git-ignored local artifacts skipped)`,
);
