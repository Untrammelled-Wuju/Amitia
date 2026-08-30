import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import os from "node:os";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const workspaceRoot = path.resolve(scriptDir, "..");

const GLOB_SPEC = [
  { base: "backend", pattern: "**/*.go" },
  { base: "backend", pattern: "go.mod" },
  { base: "backend", pattern: "go.sum" },
  { base: "desktop/src", pattern: "**/*.{ts,tsx,vue}" },
  { base: "desktop/scripts", pattern: "**/*.{mjs,cjs,js}" },
  { base: "desktop/package.json", pattern: null },
  { base: "desktop/pnpm-lock.yaml", pattern: null },
  { base: "desktop/pnpm-workspace.yaml", pattern: null },
  { base: "desktop/electron-builder.yml", pattern: null },
  { base: "desktop/patches", pattern: "**/*" },
  { base: "desktop/resources/config-template", pattern: "**/*" },
  { base: "front/src", pattern: "**/*.{ts,tsx,vue}" },
  { base: "front/package.json", pattern: null },
  { base: "front/pnpm-lock.yaml", pattern: null },
  { base: "mobile_app/lib", pattern: "**/*.dart" },
  { base: "mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/nativeprovider", pattern: "**/*.kt" },
  { base: "mobile_app/pubspec.yaml", pattern: null },
  { base: "scripts", pattern: "**/*.{mjs,cjs,js,ps1}" },
  { base: ".github/workflows", pattern: "**/*.{yml,yaml}" },
  { base: ".tool-versions", pattern: null },
];

const EXCLUDE_DIRS = new Set([
  "node_modules",
  "dist",
  "build",
  "dist-types",
  ".dart_tool",
  ".gradle",
  "__pycache__",
  ".cache",
]);

async function collectFilesByPattern(baseDir, pattern) {
  if (pattern === null) {
    const full = path.join(workspaceRoot, baseDir);
    try {
      const stat = await fs.stat(full);
      if (stat.isFile()) return [full];
    } catch {
      return [];
    }
    return [];
  }

  const base = path.join(workspaceRoot, baseDir);
  const results = [];

  async function visit(dir) {
    let entries;
    try {
      entries = await fs.readdir(dir, { withFileTypes: true });
    } catch {
      return;
    }
    for (const entry of entries) {
      if (EXCLUDE_DIRS.has(entry.name)) continue;
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        await visit(full);
      } else if (entry.isFile()) {
        const relToBase = path.relative(base, full).replaceAll("\\", "/");
        if (matchGlob(relToBase, pattern)) {
          results.push(full);
        }
      }
    }
  }

  await visit(base);
  return results;
}

function matchGlob(relPath, pattern) {
  const pathParts = relPath.split("/");
  const patternParts = pattern.split("/");

  if (patternParts.length === 1) {
    return matchSegment(pathParts[pathParts.length - 1], patternParts[0]);
  }

  if (patternParts[0] !== "**") {
    if (pathParts.length < patternParts.length) return false;
    for (let i = 0; i < patternParts.length - 1; i++) {
      if (!matchSegment(pathParts[i], patternParts[i])) return false;
    }
    return matchSegment(pathParts[pathParts.length - 1], patternParts[patternParts.length - 1]);
  }

  const lastPattern = patternParts[patternParts.length - 1];
  return matchSegment(pathParts[pathParts.length - 1], lastPattern);
}

function matchSegment(name, pattern) {
  if (pattern === "*") return true;
  if (pattern === "**") return true;
  if (pattern.startsWith("*.")) {
    const ext = pattern.slice(1);
    return name.endsWith(ext);
  }
  return name === pattern;
}

async function sha256File(filePath) {
  const data = await fs.readFile(filePath);
  return createHash("sha256").update(data).digest("hex");
}

async function collectWorkspaceScope() {
  const scopeFiles = new Set();

  for (const spec of GLOB_SPEC) {
    const files = await collectFilesByPattern(spec.base, spec.pattern);
    for (const f of files) {
      scopeFiles.add(f);
    }
  }

  const result = new Map();
  for (const absPath of Array.from(scopeFiles).sort()) {
    const relativePath = path.relative(workspaceRoot, absPath).replaceAll("\\", "/");
    const hash = await sha256File(absPath);
    result.set(relativePath, hash);
  }
  return result;
}

function extractArchiveFileList(archivePath, extractDir) {
  const result = spawnSync("tar", ["-xzf", archivePath, "-C", extractDir], {
    encoding: "utf8",
    shell: false,
  });
  if (result.status !== 0) {
    throw new Error(`Failed to extract archive: ${result.stderr || result.error?.message}`);
  }
}

async function findExtractedRoot(extractDir) {
  const entries = await fs.readdir(extractDir, { withFileTypes: true });
  const dirs = entries.filter((e) => e.isDirectory());
  if (dirs.length === 1) {
    return path.join(extractDir, dirs[0].name);
  }
  return extractDir;
}

async function collectArchiveScope(extractedRoot) {
  const result = new Map();

  async function visit(dir) {
    let entries;
    try {
      entries = await fs.readdir(dir, { withFileTypes: true });
    } catch {
      return;
    }
    for (const entry of entries) {
      if (EXCLUDE_DIRS.has(entry.name)) continue;
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        await visit(full);
      } else if (entry.isFile()) {
        const relativePath = path.relative(extractedRoot, full).replaceAll("\\", "/");
        const hash = await sha256File(full);
        result.set(relativePath, hash);
      }
    }
  }

  await visit(extractedRoot);
  return result;
}

async function main() {
  const archivePath = process.argv[2];
  if (!archivePath) {
    console.error("Usage: node scripts/verify-source-archive.mjs <archive.tar.gz>");
    process.exit(1);
  }

  const fullArchivePath = path.resolve(archivePath);
  console.log(`Archive: ${fullArchivePath}`);
  console.log(`Workspace: ${workspaceRoot}`);

  console.log("Collecting workspace freeze scope...");
  const workspaceFiles = await collectWorkspaceScope();
  console.log(`Workspace files: ${workspaceFiles.size}`);

  const extractDir = await fs.mkdtemp(path.join(os.tmpdir(), "archive-verify-"));
  console.log(`Extracting archive to: ${extractDir}`);

  try {
    extractArchiveFileList(fullArchivePath, extractDir);
    const extractedRoot = await findExtractedRoot(extractDir);
    console.log(`Extracted root: ${extractedRoot}`);

    console.log("Collecting archive file scope...");
    const archiveFiles = await collectArchiveScope(extractedRoot);
    console.log(`Archive files: ${archiveFiles.size}`);

    let missing = 0;
    let extra = 0;
    let changed = 0;
    let matched = 0;

    for (const [relPath, wsHash] of workspaceFiles) {
      if (!archiveFiles.has(relPath)) {
        missing++;
        console.log(`MISSING: ${relPath}`);
      } else if (archiveFiles.get(relPath) !== wsHash) {
        changed++;
        console.log(`CHANGED: ${relPath}`);
      } else {
        matched++;
      }
    }

    for (const [relPath] of archiveFiles) {
      if (!workspaceFiles.has(relPath)) {
        extra++;
        console.log(`EXTRA: ${relPath}`);
      }
    }

    console.log("");
    console.log("=== Verification Summary ===");
    console.log(`Archive files: ${archiveFiles.size}`);
    console.log(`Workspace files: ${workspaceFiles.size}`);
    console.log(`SHA matched: ${matched}/${workspaceFiles.size}`);
    console.log(`Missing: ${missing}`);
    console.log(`Extra: ${extra}`);
    console.log(`Changed: ${changed}`);

    if (missing > 0 || extra > 0 || changed > 0) {
      console.log("");
      console.log("FAIL: Workspace/Archive mismatch detected");
      process.exit(1);
    }

    console.log("");
    console.log("PASS: Workspace and archive are identical");
  } finally {
    await fs.rm(extractDir, { recursive: true, force: true }).catch(() => {});
  }
}

main().catch((err) => {
  console.error(`verify-source-archive failed: ${err.message}`);
  process.exit(1);
});
