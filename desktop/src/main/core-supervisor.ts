export const CORE_SUPERVISOR_FLAG = "--supervise";

export function buildCoreSpawnArgs(platform: NodeJS.Platform, profile: string): string[] {
  const profileArg = `--runtime-profile=${profile}`;
  return platform === "win32"
    ? [CORE_SUPERVISOR_FLAG, profileArg]
    : [profileArg];
}

export function parseCoreChildPid(line: string): number | null {
  const match = /^AMITIA_CORE_PID=(\d+)$/.exec(line.trim());
  if (!match) return null;
  const pid = Number(match[1]);
  return Number.isInteger(pid) && pid > 0 ? pid : null;
}
