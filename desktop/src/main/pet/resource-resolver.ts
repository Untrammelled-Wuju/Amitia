import { join, dirname, normalize, relative, isAbsolute } from "node:path";
import { PET_PROTOCOL_SCHEME } from "../../shared/animation-ipc";

export function resolveActionFramePath(
  actionConfigPath: string,
  frameFile: string,
  packageRoot: string,
): string {
  if (!frameFile) {
    throw new Error("FRAME_FILE_EMPTY");
  }
  if (isAbsolute(frameFile)) {
    throw new Error(`FRAME_PATH_ABSOLUTE: ${frameFile}`);
  }
  if (frameFile.includes("..")) {
    throw new Error(`FRAME_PATH_TRAVERSAL: ${frameFile}`);
  }

  const actionDir = dirname(actionConfigPath);
  const resolvedRelative = normalize(join(actionDir, frameFile));

  const resolvedAbs = join(packageRoot, resolvedRelative);
  const normalizedRoot = normalize(packageRoot);
  const rel = relative(normalizedRoot, resolvedAbs);
  if (rel.startsWith("..") || isAbsolute(rel)) {
    throw new Error(`FRAME_PATH_OUTSIDE_PACKAGE: ${frameFile}`);
  }

  return resolvedRelative;
}

export function buildPetResourceUrl(
  installationId: string,
  relativePath: string,
): string {
  const cleanPath = relativePath.replace(/\\/g, "/").replace(/^\/+/, "");
  return `${PET_PROTOCOL_SCHEME}://installation/${encodeURIComponent(installationId)}/${cleanPath}`;
}
