import { createHash } from "node:crypto";
import { promises as fs } from "node:fs";
import path from "node:path";

export const FREEZE_MANIFEST_NAME = "DESKTOP_PET_FINALIZATION_SHA256SUMS.txt";

const DIRECTORY_SPECS = [
  { base: "backend", extensions: null },
  { base: "desktop/src", extensions: [".ts", ".tsx", ".vue"] },
  { base: "desktop/scripts", extensions: [".mjs", ".cjs", ".js"] },
  { base: "desktop/patches", extensions: null },
  { base: "desktop/resources/config-template", extensions: null },
  { base: "desktop/resources/bridge", extensions: null },
  { base: "desktop/resources/migrations", extensions: null },
  { base: "desktop/resources/qdrant/config", extensions: null },
  { base: "front/public", extensions: null },
  { base: "Logo", extensions: null },
  { base: "front/src", extensions: [".ts", ".tsx", ".vue"] },
  { base: "mobile_app/lib", extensions: [".dart"] },
  {
    base: "mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/nativeprovider",
    extensions: [".kt", ".java"],
  },
  { base: "contracts/desktop-pet", extensions: null },
  { base: "scripts", extensions: [".mjs", ".cjs", ".js", ".ps1", ".py", ".sh"] },
  { base: ".github/workflows", extensions: [".yml", ".yaml"] },
];

const EXACT_FILES = [
  ".gitattributes",
  ".gitignore",
  ".tool-versions",
  "backend/go.mod",
  "backend/go.sum",
  "desktop/package.json",
  "desktop/pnpm-lock.yaml",
  "desktop/pnpm-workspace.yaml",
  "desktop/electron-builder.yml",
  "desktop/electron-builder.ci.yml",
  "desktop/tsconfig.json",
  "desktop/vite.config.ts",
  "desktop/release-notes.md",
  "desktop/resources/icon.ico",
  "desktop/resources/icon-dark.ico",
  "desktop/resources/icon-light.ico",
  "desktop/resources/tray.png",
  "desktop/resources/tray-dark.png",
  "desktop/resources/tray-light.png",
  "front/.npmrc",
  "front/index.html",
  "front/pet.html",
  "front/package.json",
  "front/pnpm-lock.yaml",
  "front/pnpm-workspace.yaml",
  "front/tsconfig.json",
  "front/vite-env.d.ts",
  "front/vite.config.ts",
  "front/vitest.config.ts",
  "front/vitest.setup.ts",
  "mobile_app/analysis_options.yaml",
  "mobile_app/pubspec.yaml",
  "mobile_app/pubspec.lock",
  "mobile_app/android/app/src/main/AndroidManifest.xml",
  "mobile_app/android/app/build.gradle.kts",
  "mobile_app/android/build.gradle.kts",
  "mobile_app/android/gradle.properties",
  "mobile_app/android/settings.gradle.kts",
];

const EXCLUDED_RELATIVE_PREFIXES = [
  "backend/data",
  "backend/cmd/data",
  "backend/logs",
  "backend/AmitiaData",
  "backend/dev",
  "backend/tmp",
  "backend/bin",
  "backend/target",
  "backend/qdrant/storage",
  "backend/surrealdb/data",
  "backend/node",
  "backend/qq-sidecar/data",
  "backend/build",
];

const EXCLUDED_RELATIVE_FILES = new Set([
  "backend/qdrant/qdrant.zip",
  "backend/surrealdb/surreal.zip",
  "backend/server_linux_amd64",
  "backend/server_linux_arm64",
  "backend/server",
]);

const EXCLUDED_FILE_SUFFIXES = [
  ".exe", ".dll", ".so", ".dylib", ".db", ".db-shm", ".db-wal", ".db-journal", ".log", ".tmp", ".bak",
];

const EXCLUDED_DIRECTORY_NAMES = new Set([
  ".git",
  ".dart_tool",
  ".gradle",
  ".kotlin",
  ".cxx",
  ".cache",
  "__pycache__",
  "node_modules",
  "dist",
  "dist-types",
  "coverage",
]);

function normalizeRelativePath(value) {
  return value.replaceAll("\\", "/").replace(/^\.\//, "");
}

function isExplicitlyExcluded(repositoryRoot, absolutePath, isDirectory) {
  const relativePath = normalizeRelativePath(path.relative(repositoryRoot, absolutePath));
  if (EXCLUDED_RELATIVE_FILES.has(relativePath)) return true;
  if (!isDirectory && EXCLUDED_FILE_SUFFIXES.some((suffix) => relativePath.endsWith(suffix))) return true;
  return EXCLUDED_RELATIVE_PREFIXES.some((prefix) =>
    relativePath === prefix || relativePath.startsWith(`${prefix}/`),
  );
}

function shouldIncludeFile(fileName, extensions) {
  if (extensions === null) return true;
  return extensions.some((extension) => fileName.endsWith(extension));
}

async function collectDirectoryFiles(repositoryRoot, spec, output) {
  const basePath = path.resolve(repositoryRoot, spec.base);
  let stat;
  try {
    stat = await fs.stat(basePath);
  } catch {
    throw new Error(`freeze scope directory missing: ${spec.base}`);
  }
  if (!stat.isDirectory()) {
    throw new Error(`freeze scope path is not a directory: ${spec.base}`);
  }

  async function visit(directory) {
    const entries = await fs.readdir(directory, { withFileTypes: true });
    entries.sort((a, b) => a.name.localeCompare(b.name));
    for (const entry of entries) {
      if (entry.isDirectory() && EXCLUDED_DIRECTORY_NAMES.has(entry.name)) continue;
      const absolute = path.join(directory, entry.name);
      if (isExplicitlyExcluded(repositoryRoot, absolute, entry.isDirectory())) continue;
      if (entry.isDirectory()) {
        await visit(absolute);
      } else if (entry.isFile() && shouldIncludeFile(entry.name, spec.extensions)) {
        output.add(absolute);
      }
    }
  }

  await visit(basePath);
}

export async function sha256File(filePath) {
  const data = await fs.readFile(filePath);
  return createHash("sha256").update(data).digest("hex");
}

export async function collectFreezeEntries(repositoryRoot) {
  const root = path.resolve(repositoryRoot);
  const files = new Set();

  for (const spec of DIRECTORY_SPECS) {
    await collectDirectoryFiles(root, spec, files);
  }

  for (const relativePath of EXACT_FILES) {
    const absolute = path.resolve(root, relativePath);
    let stat;
    try {
      stat = await fs.stat(absolute);
    } catch {
      throw new Error(`freeze scope file missing: ${relativePath}`);
    }
    if (!stat.isFile()) {
      throw new Error(`freeze scope path is not a file: ${relativePath}`);
    }
    files.add(absolute);
  }

  const relativePaths = Array.from(files, (absolute) =>
    normalizeRelativePath(path.relative(root, absolute)),
  ).sort((a, b) => a.localeCompare(b));

  const entries = [];
  for (const relativePath of relativePaths) {
    entries.push({
      relativePath,
      hash: await sha256File(path.resolve(root, relativePath)),
    });
  }
  return entries;
}

export function serializeFreezeEntries(entries) {
  const seen = new Set();
  const lines = [];
  for (const entry of entries) {
    const relativePath = normalizeRelativePath(entry.relativePath);
    if (seen.has(relativePath)) {
      throw new Error(`duplicate freeze path: ${relativePath}`);
    }
    seen.add(relativePath);
    if (!/^[0-9a-f]{64}$/.test(entry.hash)) {
      throw new Error(`invalid SHA256 for freeze path: ${relativePath}`);
    }
    lines.push(`${entry.hash}  ${relativePath}`);
  }
  return `${lines.join("\n")}\n`;
}

export function parseFreezeManifest(text) {
  const entries = [];
  const seen = new Set();
  const lines = text.replaceAll("\r\n", "\n").replaceAll("\r", "\n").split("\n");
  for (const rawLine of lines) {
    if (rawLine.trim() === "") continue;
    const match = /^([0-9a-fA-F]{64})\s{2}(.+)$/.exec(rawLine);
    if (!match) {
      throw new Error(`invalid freeze manifest line: ${JSON.stringify(rawLine)}`);
    }
    const relativePath = normalizeRelativePath(match[2].trim());
    if (!relativePath || relativePath.startsWith("../") || path.isAbsolute(relativePath)) {
      throw new Error(`invalid freeze path: ${relativePath}`);
    }
    if (seen.has(relativePath)) {
      throw new Error(`duplicate freeze path: ${relativePath}`);
    }
    seen.add(relativePath);
    entries.push({ hash: match[1].toLowerCase(), relativePath });
  }
  entries.sort((a, b) => a.relativePath.localeCompare(b.relativePath));
  return entries;
}

export async function readFreezeManifest(repositoryRoot) {
  const manifestPath = path.resolve(repositoryRoot, FREEZE_MANIFEST_NAME);
  const text = await fs.readFile(manifestPath, "utf8");
  return parseFreezeManifest(text);
}

export async function writeFreezeManifest(repositoryRoot) {
  const entries = await collectFreezeEntries(repositoryRoot);
  const manifestPath = path.resolve(repositoryRoot, FREEZE_MANIFEST_NAME);
  await fs.writeFile(manifestPath, serializeFreezeEntries(entries), "utf8");
  return { entries, manifestPath };
}

export async function verifyFreezeManifest(repositoryRoot) {
  const expected = await collectFreezeEntries(repositoryRoot);
  const actual = await readFreezeManifest(repositoryRoot);

  const expectedMap = new Map(expected.map((entry) => [entry.relativePath, entry.hash]));
  const actualMap = new Map(actual.map((entry) => [entry.relativePath, entry.hash]));

  const missing = [];
  const extra = [];
  const changed = [];

  for (const [relativePath, hash] of expectedMap) {
    if (!actualMap.has(relativePath)) {
      missing.push(relativePath);
    } else if (actualMap.get(relativePath) !== hash) {
      changed.push(relativePath);
    }
  }
  for (const relativePath of actualMap.keys()) {
    if (!expectedMap.has(relativePath)) extra.push(relativePath);
  }

  if (missing.length || extra.length || changed.length) {
    const details = [
      ...missing.map((item) => `MISSING_FROM_MANIFEST: ${item}`),
      ...extra.map((item) => `EXTRA_IN_MANIFEST: ${item}`),
      ...changed.map((item) => `SHA_MISMATCH: ${item}`),
    ];
    throw new Error(
      `freeze manifest does not match canonical scope (expected=${expected.length}, actual=${actual.length})\n${details.join("\n")}`,
    );
  }

  return { entries: expected, count: expected.length };
}

export async function computeFreezeSourceGateHash(repositoryRoot) {
  const { entries } = await verifyFreezeManifest(repositoryRoot);
  return createHash("sha256").update(serializeFreezeEntries(entries), "utf8").digest("hex");
}
