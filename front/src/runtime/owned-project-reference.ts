export function ownedProjectReference(ownerId: string, id: string): string {
  if (!ownerId || !id || ownerId.length > 512 || id.length > 512 || ownerId.includes("\0") || id.includes("\0")) throw new Error("项目来源无效");
  return `meshproj1:${encodeURIComponent(ownerId)}:${encodeURIComponent(id)}`;
}

export function parseOwnedProjectReference(reference: string) {
  if (!reference.startsWith("meshproj1:")) return undefined;
  const parts = reference.split(":");
  if (parts.length !== 3) throw new Error("项目来源无效");
  const ownerId = decodeURIComponent(parts[1]);
  const id = decodeURIComponent(parts[2]);
  if (ownedProjectReference(ownerId, id) !== reference) throw new Error("项目来源无效");
  return { ownerId, id };
}
