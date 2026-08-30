import { lstatSync, realpathSync } from "node:fs";
import { isAbsolute, join, relative, resolve, sep } from "node:path";
import {
  caseFoldPackagePath,
  decodePackagePathFromUrl,
  encodePackagePathForUrl,
  normalizePackagePath,
  resolveActionResourcePackagePath,
  tryNormalizePackagePath,
} from "../../shared/package-path";

export {
  caseFoldPackagePath,
  decodePackagePathFromUrl,
  encodePackagePathForUrl,
  normalizePackagePath,
  resolveActionResourcePackagePath,
  tryNormalizePackagePath,
};

export function assertPathInsideRoot(root: string, target: string): void {
  const rel = relative(root, target);
  if (rel === "" || rel === ".") return;
  if (rel === ".." || rel.startsWith(`..${sep}`) || isAbsolute(rel)) {
    throw new Error(`PACKAGE_PATH_OUTSIDE_ROOT: ${target}`);
  }
}

function resolveRealPackageRoot(root: string): string {
  if (!isAbsolute(root)) throw new Error(`PACKAGE_ROOT_NOT_ABSOLUTE: ${root}`);
  const lexicalRoot = resolve(root);
  const rootInfo = lstatSync(lexicalRoot);
  if (rootInfo.isSymbolicLink() || !rootInfo.isDirectory()) {
    throw new Error(`PACKAGE_ROOT_NOT_REAL_DIRECTORY: ${root}`);
  }
  return realpathSync(lexicalRoot);
}

function inspectPackagePathComponents(
  realRoot: string,
  canonical: string,
  mustExist: boolean,
): string {
  let current = realRoot;
  const segments = canonical.split("/");
  let missingAncestor = false;

  for (let index = 0; index < segments.length; index += 1) {
    current = join(current, segments[index]);
    assertPathInsideRoot(realRoot, current);

    if (missingAncestor) continue;

    try {
      const info = lstatSync(current);
      if (info.isSymbolicLink()) {
        throw new Error(`PACKAGE_SYMLINK_FORBIDDEN: ${canonical}`);
      }
      if (index < segments.length - 1 && !info.isDirectory()) {
        throw new Error(`PACKAGE_PATH_COMPONENT_NOT_DIRECTORY: ${canonical}`);
      }
    } catch (error) {
      const code = (error as NodeJS.ErrnoException | undefined)?.code;
      if (code === "ENOENT" && !mustExist) {
        missingAncestor = true;
        continue;
      }
      throw error;
    }
  }

  if (mustExist) {
    const realTarget = realpathSync(current);
    assertPathInsideRoot(realRoot, realTarget);
    return realTarget;
  }
  return current;
}

export function resolvePackagePathUnderRoot(
  root: string,
  relativePath: string,
  options?: { mustExist?: boolean },
): string {
  const canonical = normalizePackagePath(relativePath);
  const realRoot = resolveRealPackageRoot(root);
  return inspectPackagePathComponents(realRoot, canonical, options?.mustExist !== false);
}

export function relativePackagePathFromRoot(root: string, fullPath: string): string {
  if (!isAbsolute(fullPath)) throw new Error(`PACKAGE_TARGET_NOT_ABSOLUTE: ${fullPath}`);

  const realRoot = resolveRealPackageRoot(root);
  const lexicalTarget = resolve(fullPath);
  assertPathInsideRoot(realRoot, lexicalTarget);

  const rel = relative(realRoot, lexicalTarget).split(sep).join("/");
  const canonical = normalizePackagePath(rel);
  const resolvedTarget = inspectPackagePathComponents(realRoot, canonical, true);
  const realRequestedTarget = realpathSync(lexicalTarget);
  if (resolvedTarget !== realRequestedTarget) {
    throw new Error(`PACKAGE_PATH_RESOLUTION_MISMATCH: ${fullPath}`);
  }
  return canonical;
}
