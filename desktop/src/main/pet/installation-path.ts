import { isAbsolute, join, normalize, relative, sep } from "node:path";

/**
 * Resolve a backend-persisted desktop-pet installation path into the canonical
 * filesystem root used by Electron. Backend records are normally relative to
 * AmitiaData; absolute roots remain supported for development/tests.
 */
export function resolveDesktopPetInstallationRoot(
  installPath: string,
  dataDir: string,
): string | null {
  if (!installPath || !dataDir) return null;

  const normalizedDataDir = normalize(dataDir);
  if (!isAbsolute(normalizedDataDir)) return null;

  const resolved = normalize(
    isAbsolute(installPath)
      ? installPath
      : join(normalizedDataDir, installPath),
  );
  if (!isAbsolute(resolved)) return null;

  if (!isAbsolute(installPath)) {
    const rel = relative(normalizedDataDir, resolved);
    if (rel === ".." || rel.startsWith(`..${sep}`) || isAbsolute(rel)) {
      return null;
    }
  }

  return resolved;
}
