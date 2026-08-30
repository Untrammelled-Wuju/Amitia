import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";

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
  const parts = pattern.split("/");
  const immediateGlob = parts[0].includes("*");

  async function visit(dir, depth) {
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
        await visit(full, depth + 1);
      } else if (entry.isFile()) {
        const relToBase = path.relative(base, full).replaceAll("\\", "/");
        if (matchGlob(relToBase, pattern)) {
          results.push(full);
        }
      }
    }
  }

  await visit(base, 0);
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
    const extPart = pattern.slice(2);
    if (extPart.startsWith("{") && extPart.endsWith("}")) {
      const exts = extPart.slice(1, -1).split(",");
      return exts.some((ext) => name.endsWith("." + ext));
    }
    return name.endsWith("." + extPart);
  }
  if (pattern.startsWith("{") && pattern.endsWith("}")) {
    const alts = pattern.slice(1, -1).split(",");
    return alts.includes(name);
  }
  return name === pattern;
}

async function sha256File(filePath) {
  const data = await fs.readFile(filePath);
  return createHash("sha256").update(data).digest("hex");
}

async function main() {
  const scopeFiles = new Set();

  for (const spec of GLOB_SPEC) {
    const files = await collectFilesByPattern(spec.base, spec.pattern);
    for (const f of files) {
      scopeFiles.add(f);
    }
  }

  const sortedFiles = Array.from(scopeFiles).sort();
  const entries = [];

  for (const absPath of sortedFiles) {
    const relativePath = path.relative(workspaceRoot, absPath).replaceAll("\\", "/");
    const hash = await sha256File(absPath);
    entries.push({ relativePath, hash });
  }

  if (process.argv.includes("--manifest")) {
    const manifestPath = path.join(workspaceRoot, "FREEZE_SCOPE_MANIFEST.txt");
    const lines = [
      `Freeze Scope generated: ${new Date().toISOString()}`,
      `Total files: ${entries.length}`,
      "",
      ...entries.map((e) => `${e.hash}  ${e.relativePath}`),
    ];
    await fs.writeFile(manifestPath, lines.join("\n") + "\n", "utf8");
    console.log(`Freeze scope manifest written: ${manifestPath}`);
    console.log(`Total files: ${entries.length}`);
  } else if (process.argv.includes("--list")) {
    for (const e of entries) {
      console.log(e.relativePath);
    }
  } else if (process.argv.includes("--sha")) {
    for (const e of entries) {
      console.log(`${e.hash}  ${e.relativePath}`);
    }
  } else {
    console.log(`Freeze scope files: ${entries.length}`);
    for (const e of entries) {
      console.log(`  ${e.hash}  ${e.relativePath}`);
    }
  }
}

main().catch((err) => {
  console.error(`build-freeze-scope failed: ${err.message}`);
  process.exit(1);
});
