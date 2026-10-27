import { createHash } from "node:crypto";
import {
  cpSync,
  copyFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { join, relative, resolve, sep } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { computeFreezeSourceGateHash } from "../../scripts/lib/freeze-scope.mjs";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const desktopRoot = resolve(__dirname, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const resourcesRoot = resolve(desktopRoot, "resources");
const coreRoot = resolve(resourcesRoot, "core");
const stageRoot = resolve(resourcesRoot, "release-runtime");
const manifestPath = resolve(stageRoot, ".release-runtime-assets.json");
const powershellRequiredFiles = [
  "pwsh.exe",
  "pwsh.dll",
  "pwsh.runtimeconfig.json",
  "pwsh.deps.json",
  "System.Management.Automation.dll",
  "hostfxr.dll",
  "hostpolicy.dll",
  "coreclr.dll",
  "LICENSE.txt",
  "ThirdPartyNotices.txt",
];

const archiveNames = {
  qdrant: {
    windows: ["qdrant-windows-amd64.zip", "qdrant-windows-arm64.zip", "qdrant.exe.zip", "qdrant.zip"],
    macos: ["qdrant-macos-arm64.zip", "qdrant-macos-amd64.zip", "qdrant-darwin-arm64.zip", "qdrant-darwin-amd64.zip", "qdrant.zip"],
    linux: ["qdrant-linux-arm64.zip", "qdrant-linux-x64.zip", "qdrant-aarch64-unknown-linux-gnu.zip", "qdrant-x86_64-unknown-linux-gnu.zip", "qdrant-linux-arm64.tar.gz", "qdrant-linux-x64.tar.gz", "qdrant.zip"],
  },
  surrealdb: {
    windows: ["surreal-windows-amd64.zip", "surreal-windows-arm64.zip", "surreal.exe.zip", "surreal.zip"],
    macos: ["surreal-macos-arm64.zip", "surreal-macos-amd64.zip", "surreal-darwin-arm64.zip", "surreal-darwin-amd64.zip", "surreal.zip"],
    linux: ["surreal-linux-arm64.zip", "surreal-linux-x64.zip", "surreal-linux-arm64.tgz", "surreal-linux-x64.tgz", "surreal.zip"],
  },
  node: {
    windows: ["node-windows-x64.zip", "node.exe.zip"],
    macos: ["node-macos-arm64.zip", "node-macos-x64.zip", "node.zip"],
    linux: ["node-linux-arm64.zip", "node-linux-x64.zip", "node-linux-arm64.tar.xz", "node-linux-x64.tar.xz", "node.zip"],
  },
};

function normalizePlatform(value) {
  const normalized = String(value || process.platform).trim().toLowerCase();
  if (["win32", "windows"].includes(normalized)) return "windows";
  if (["darwin", "mac", "macos"].includes(normalized)) return "macos";
  return "linux";
}

function normalizeArch(value) {
  const normalized = String(value || process.arch).trim().toLowerCase();
  return ["arm64", "aarch64"].includes(normalized) ? "arm64" : "x64";
}

function runtimeTarget(platform, arch) {
  return `${platform}-${arch}`;
}

function stageBundledPowerShell(platform, arch) {
  if (platform !== "windows") {
    return;
  }
  const override = process.env.AMITIA_POWERSHELL_RUNTIME_PATH;
  const source = override
    ? resolve(override)
    : resolve(resourcesRoot, "runtime", runtimeTarget(platform, arch), "powershell");
  for (const name of powershellRequiredFiles) {
    assertRegularNonEmptyFile(join(source, name), `PowerShell runtime ${name}`);
  }
  const destination = join(stageRoot, "powershell");
  mkdirSync(resolve(destination, ".."), { recursive: true });
  cpSync(source, destination, { recursive: true, force: true });
  for (const name of powershellRequiredFiles) {
    assertRegularNonEmptyFile(join(destination, name), `staged PowerShell runtime ${name}`);
  }
}

function goTarget(platform, arch) {
  return {
    goos: platform === "windows" ? "windows" : platform === "macos" ? "darwin" : "linux",
    goarch: arch === "arm64" ? "arm64" : "amd64",
  };
}

function sha256File(filePath) {
  return createHash("sha256").update(readFileSync(filePath)).digest("hex");
}

function assertRegularNonEmptyFile(filePath, label) {
  if (!existsSync(filePath)) throw new Error(`${label} missing: ${filePath}`);
  const stats = statSync(filePath);
  if (!stats.isFile()) throw new Error(`${label} is not a regular file: ${filePath}`);
  if (stats.size <= 0) throw new Error(`${label} is empty: ${filePath}`);
  return stats;
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd ?? repositoryRoot,
    encoding: options.encoding,
    stdio: options.stdio ?? "inherit",
    shell: false,
  });
  if (result.error) throw result.error;
  if ((result.status ?? 1) !== 0) {
    throw new Error(`${command} exited with code ${result.status ?? 1}`);
  }
  return result;
}

function archiveEntries(filePath) {
  const result = run("tar", ["-tf", filePath], { stdio: "pipe", encoding: "utf8" });
  return String(result.stdout || "")
    .split(/\r?\n/)
    .map((entry) => entry.trim().replace(/^\.\//, ""))
    .filter(Boolean);
}

function expectedBinary(kind, platform) {
  if (kind === "node") return platform === "windows" ? "node.exe" : "bin/node";
  const name = kind === "qdrant" ? "qdrant" : "surreal";
  return platform === "windows" ? `${name}.exe` : name;
}

function candidateArchives(kind, platform, arch) {
  const target = runtimeTarget(platform, arch);
  const override = process.env[`AMITIA_${kind.toUpperCase()}_ARCHIVE_PATH`];
  const roots = kind === "node"
    ? [
        resolve(resourcesRoot, "runtime", target, "node"),
        resolve(resourcesRoot, "core", "node"),
        resolve(resourcesRoot, "databases", target, "node"),
      ]
    : [
        resolve(resourcesRoot, "databases", target, kind),
        resolve(resourcesRoot, kind),
      ];
  const candidates = override ? [resolve(override)] : [];
  for (const root of roots) {
    for (const name of archiveNames[kind][platform]) candidates.push(join(root, name));
  }
  return [...new Set(candidates)];
}

function selectArchive(kind, platform, arch) {
  const expected = expectedBinary(kind, platform);
  const inspected = [];
  for (const candidate of candidateArchives(kind, platform, arch)) {
    if (!existsSync(candidate)) continue;
    assertRegularNonEmptyFile(candidate, `${kind} runtime archive`);
    const entries = archiveEntries(candidate);
    inspected.push({ path: candidate, entries });
    const entry = entries.find((item) => item === expected || item.endsWith(`/${expected}`));
    if (entry) return { path: candidate, entries, expected, entry };
  }
  const details = inspected.length
    ? inspected.map((item) => `${item.path}: ${item.entries.join(", ")}`).join("; ")
    : "no candidate archive found";
  throw new Error(`${kind} archive for ${runtimeTarget(platform, arch)} must contain ${expected}; ${details}`);
}

function normalizeDatabaseArchive(kind, selected, platform, arch) {
  const target = runtimeTarget(platform, arch);
  const archiveName = kind === "qdrant" ? "qdrant.zip" : "surreal.zip";
  const destination = join(stageRoot, kind, archiveName);
  mkdirSync(resolve(destination, ".."), { recursive: true });
  if (selected.entries.length === 1 && selected.entries[0] === selected.entry) {
    copyFileSync(selected.path, destination);
    return destination;
  }

  const extractionRoot = join(stageRoot, ".extract", `${kind}-${target}`);
  rmSync(extractionRoot, { recursive: true, force: true });
  mkdirSync(extractionRoot, { recursive: true });
  run("tar", ["-xf", selected.path, "-C", extractionRoot, selected.entry], { stdio: "pipe" });
  const binaryPath = join(extractionRoot, selected.entry);
  assertRegularNonEmptyFile(binaryPath, `${kind} executable`);
  if (process.platform === "win32") {
    run("pwsh", [
      "-NoProfile",
      "-NonInteractive",
      "-Command",
      `Compress-Archive -LiteralPath '${binaryPath.replaceAll("'", "''")}' -DestinationPath '${destination.replaceAll("'", "''")}' -Force`,
    ], { stdio: "pipe" });
  } else {
    run("zip", ["-j", "-q", destination, binaryPath], { stdio: "pipe" });
  }
  rmSync(extractionRoot, { recursive: true, force: true });
  return destination;
}

function normalizeNodeArchive(selected, platform) {
  const destination = join(stageRoot, "core", "node", "node.zip");
  mkdirSync(resolve(destination, ".."), { recursive: true });
  if (selected.path.toLowerCase().endsWith(".zip")) {
    copyFileSync(selected.path, destination);
    return destination;
  }
  const extractionRoot = join(stageRoot, ".extract", `node-${platform}`);
  rmSync(extractionRoot, { recursive: true, force: true });
  mkdirSync(extractionRoot, { recursive: true });
  run("tar", ["-xf", selected.path, "-C", extractionRoot], { stdio: "pipe" });
  const entries = readdirSync(extractionRoot, { withFileTypes: true });
  const sourceRoot = entries.length === 1 && entries[0].isDirectory()
    ? join(extractionRoot, entries[0].name)
    : extractionRoot;
  if (process.platform === "win32") {
    run("pwsh", [
      "-NoProfile",
      "-NonInteractive",
      "-Command",
      `Compress-Archive -Path '${join(sourceRoot, "*").replaceAll("'", "''")}' -DestinationPath '${destination.replaceAll("'", "''")}' -Force`,
    ], { stdio: "pipe" });
  } else {
    run("zip", ["-r", "-q", destination, "."], { cwd: sourceRoot, stdio: "pipe" });
  }
  rmSync(extractionRoot, { recursive: true, force: true });
  return destination;
}

function packagePath(absolutePath) {
  return relative(stageRoot, absolutePath).split(sep).join("/");
}

function collectStageFiles() {
  const output = [];
  const visit = (current) => {
    for (const entry of readdirSync(current, { withFileTypes: true })) {
      const absolute = resolve(current, entry.name);
      if (absolute === manifestPath) continue;
      if (entry.isDirectory()) visit(absolute);
      else if (entry.isFile()) output.push(absolute);
    }
  };
  visit(stageRoot);
  return output.sort((a, b) => packagePath(a).localeCompare(packagePath(b)));
}

function verifyCoreStamp(platform, arch) {
  const stampPath = join(coreRoot, ".amitiacore-build.json");
  assertRegularNonEmptyFile(stampPath, "AmitiaCore build stamp");
  const stamp = JSON.parse(readFileSync(stampPath, "utf8"));
  const expected = goTarget(platform, arch);
  if (stamp.CORE_GOOS !== expected.goos || stamp.CORE_GOARCH !== expected.goarch) {
    throw new Error(`AmitiaCore target mismatch: expected ${expected.goos}/${expected.goarch}, got ${stamp.CORE_GOOS}/${stamp.CORE_GOARCH}`);
  }
  return stamp;
}

export async function prepareReleaseRuntimeAssets() {
  const platform = normalizePlatform(process.env.AMITIA_RELEASE_PLATFORM);
  const arch = normalizeArch(process.env.AMITIA_RELEASE_ARCH);
  const target = runtimeTarget(platform, arch);
  const sourceGateSha256 = await computeFreezeSourceGateHash(repositoryRoot);
  const coreStamp = verifyCoreStamp(platform, arch);

  rmSync(stageRoot, { recursive: true, force: true });
  mkdirSync(stageRoot, { recursive: true });

  const coreName = platform === "windows" ? "AmitiaCore.exe" : "AmitiaCore";
  const coreSource = join(coreRoot, coreName);
  assertRegularNonEmptyFile(coreSource, "AmitiaCore");
  const coreDestination = join(stageRoot, "core", coreName);
  mkdirSync(resolve(coreDestination, ".."), { recursive: true });
  copyFileSync(coreSource, coreDestination);

  normalizeNodeArchive(selectArchive("node", platform, arch), platform);
  normalizeDatabaseArchive("qdrant", selectArchive("qdrant", platform, arch), platform, arch);
  normalizeDatabaseArchive("surrealdb", selectArchive("surrealdb", platform, arch), platform, arch);
  stageBundledPowerShell(platform, arch);

  const entries = collectStageFiles().map((absolute) => {
    const stats = statSync(absolute);
    return {
      path: packagePath(absolute),
      bytes: stats.size,
      sha256: sha256File(absolute),
      provenance: absolute === coreDestination ? "built-core" : "platform-runtime",
    };
  });

  const manifest = {
    schemaVersion: 2,
    platform,
    arch,
    target,
    createdAt: new Date().toISOString(),
    sourceGateSha256,
    coreBuildSha256: coreStamp.CORE_BUILD_SHA256,
    entries,
  };
  writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, "utf8");
  return manifest;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const manifest = await prepareReleaseRuntimeAssets();
    console.log(
      `[release-runtime] PASS: ${manifest.target} staged ${manifest.entries.length} runtime files; source gate ${manifest.sourceGateSha256.slice(0, 12)}...`,
    );
  } catch (error) {
    console.error(`[release-runtime] FAILED: ${error instanceof Error ? error.message : String(error)}`);
    process.exit(1);
  }
}
