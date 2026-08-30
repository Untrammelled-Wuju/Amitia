import { isAbsolute, join, normalize, relative, sep } from "node:path";
import { PET_PROTOCOL_SCHEME } from "../../shared/animation-ipc";
import { encodePackagePathForUrl, normalizePackagePath, resolveActionResourcePackagePath } from "./package-path";

export function resolveActionFramePath(actionConfigPath: string, frameFile: string, packageRoot: string): string {
  if (!frameFile) throw new Error("FRAME_FILE_EMPTY");
  if (isAbsolute(frameFile) || frameFile.startsWith("/")) throw new Error(`FRAME_PATH_ABSOLUTE: ${frameFile}`);
  if (frameFile.split("/").includes("..") || frameFile.split("\\").includes("..")) throw new Error(`FRAME_PATH_TRAVERSAL: ${frameFile}`);

  let resolvedRelative: string;
  try {
    normalizePackagePath(actionConfigPath);
    resolvedRelative = resolveActionResourcePackagePath(actionConfigPath, frameFile);
  } catch {
    if (frameFile.split("/").includes("..") || frameFile.split("\\").includes("..")) throw new Error(`FRAME_PATH_TRAVERSAL: ${frameFile}`);
    throw new Error(`FRAME_PATH_OUTSIDE_PACKAGE: ${frameFile}`);
  }

  if (!isAbsolute(packageRoot)) throw new Error(`FRAME_PACKAGE_ROOT_NOT_ABSOLUTE: ${packageRoot}`);
  const normalizedRoot = normalize(packageRoot);
  const resolvedAbs = join(normalizedRoot, ...resolvedRelative.split("/"));
  const rel = relative(normalizedRoot, resolvedAbs);
  if (rel === ".." || rel.startsWith(`..${sep}`) || isAbsolute(rel)) throw new Error(`FRAME_PATH_OUTSIDE_PACKAGE: ${frameFile}`);
  return resolvedRelative;
}

export function buildPetResourceUrl(installationId: string, relativePath: string): string {
  if (!installationId || /[\u0000-\u001f\u007f]/.test(installationId)) throw new Error("INSTALLATION_ID_INVALID");
  const encodedPath = encodePackagePathForUrl(relativePath);
  return `${PET_PROTOCOL_SCHEME}://installation/${encodeURIComponent(installationId)}/${encodedPath}`;
}
