import fs from "node:fs";
import path from "node:path";

export type RuntimePlatform = "windows" | "macos" | "linux";
export type RuntimeArch = "x64" | "arm64";
export type DatabaseKind = "qdrant" | "surrealdb";

type ArchiveTable = Record<DatabaseKind, Record<RuntimePlatform, Record<RuntimeArch, string[]>>>;

const DATABASE_ARCHIVES: ArchiveTable = {
  qdrant: {
    windows: {
      x64: ["qdrant.zip", "qdrant.exe.zip", "qdrant-windows-amd64.zip"],
      arm64: ["qdrant.zip", "qdrant-windows-arm64.zip"],
    },
    macos: {
      x64: ["qdrant.zip", "qdrant-darwin-amd64.zip", "qdrant-macos-amd64.zip"],
      arm64: ["qdrant.zip", "qdrant-darwin-arm64.zip", "qdrant-macos-arm64.zip"],
    },
    linux: {
      x64: ["qdrant.zip", "qdrant-linux-x64.zip", "qdrant-x86_64-unknown-linux-gnu.zip"],
      arm64: ["qdrant.zip", "qdrant-linux-arm64.zip", "qdrant-aarch64-unknown-linux-gnu.zip"],
    },
  },
  surrealdb: {
    windows: {
      x64: ["surreal.zip", "surreal.exe.zip", "surreal-windows-amd64.zip"],
      arm64: ["surreal.zip", "surreal-windows-arm64.zip"],
    },
    macos: {
      x64: ["surreal.zip", "surreal-darwin-amd64.zip"],
      arm64: ["surreal.zip", "surreal-darwin-arm64.zip"],
    },
    linux: {
      x64: ["surreal.zip", "surreal-linux-x64.zip"],
      arm64: ["surreal.zip", "surreal-linux-arm64.zip"],
    },
  },
};

export function normalizePlatform(platform: NodeJS.Platform = process.platform): RuntimePlatform {
  if (platform === "win32") return "windows";
  if (platform === "darwin") return "macos";
  return "linux";
}

export function normalizeArch(arch: string = process.arch): RuntimeArch {
  return arch === "arm64" ? "arm64" : "x64";
}

export function runtimeTarget(
  platform: NodeJS.Platform = process.platform,
  arch: string = process.arch,
): string {
  return `${normalizePlatform(platform)}-${normalizeArch(arch)}`;
}

export function coreBinaryName(platform: NodeJS.Platform = process.platform): string {
  return normalizePlatform(platform) === "windows" ? "AmitiaCore.exe" : "AmitiaCore";
}

export function databaseArchiveNames(
  kind: DatabaseKind,
  platform: NodeJS.Platform = process.platform,
  arch: string = process.arch,
): string[] {
  return [...DATABASE_ARCHIVES[kind][normalizePlatform(platform)][normalizeArch(arch)]];
}

export function firstExisting(candidates: string[]): string | null {
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) return candidate;
  }
  return null;
}

function dedupe(values: string[]): string[] {
  return values.filter((value, index) => values.indexOf(value) === index);
}

export function corePathCandidates(
  resourcesRoot: string,
  platform: NodeJS.Platform = process.platform,
  arch: string = process.arch,
): string[] {
  const target = runtimeTarget(platform, arch);
  const name = coreBinaryName(platform);
  return dedupe([
    path.join(resourcesRoot, "core", target, name),
    path.join(resourcesRoot, "core", name),
    path.join(resourcesRoot, "core", "AmitiaCore.exe"),
    path.join(resourcesRoot, "core", "AmitiaCore"),
  ]);
}

export function resolveCorePath(
  resourcesRoot: string,
  platform: NodeJS.Platform = process.platform,
  arch: string = process.arch,
): string {
  const candidates = corePathCandidates(resourcesRoot, platform, arch);
  return firstExisting(candidates) ?? candidates[0];
}

export function databaseSourceCandidates(
  resourcesRoot: string,
  kind: DatabaseKind,
  platform: NodeJS.Platform = process.platform,
  arch: string = process.arch,
  extraRoots: string[] = [],
): string[] {
  const target = runtimeTarget(platform, arch);
  const roots = dedupe([
    path.join(resourcesRoot, "databases", target),
    path.join(resourcesRoot, kind),
    ...extraRoots.map((root) => path.join(root, kind)),
  ]);
  const names = databaseArchiveNames(kind, platform, arch);
  const candidates: string[] = [];
  for (const root of roots) {
    for (const name of names) candidates.push(path.join(root, name));
  }
  return dedupe(candidates);
}
