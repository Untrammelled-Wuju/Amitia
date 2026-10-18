const SEMVER_PATTERN =
  /^\d+\.\d+\.\d+(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

export function isPrereleaseVersion(version: string): boolean {
  const match = SEMVER_PATTERN.exec(version.trim());
  return match?.[1] != null;
}

export function resolveAllowBetaChannel(
  stored: unknown,
  currentVersion: string,
): boolean {
  return typeof stored === "boolean"
    ? stored
    : isPrereleaseVersion(currentVersion);
}

export function desktopUpdateChannel(allowBetaChannel: boolean): string {
  return allowBetaChannel ? "beta" : "latest";
}
