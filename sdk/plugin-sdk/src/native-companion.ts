/**
 * Public Native Companion runtime contract for trusted service extensions.
 *
 * The host exposes only companions declared in the extension manifest that
 * match the current OS/architecture and pass package + runtime SHA-256 checks.
 * The descriptor list is delivered through AMITIA_NATIVE_COMPANIONS.
 */
export interface NativeCompanionRuntimeDescriptor {
  id: string;
  platform: "windows" | "linux" | "darwin";
  architecture?: "amd64" | "arm64";
  path: string;
  sha256: string;
  executable: boolean;
  args?: string[];
}

export const NATIVE_COMPANIONS_ENV = "AMITIA_NATIVE_COMPANIONS";
export const NATIVE_COMPANIONS_VERSION_ENV = "AMITIA_NATIVE_COMPANIONS_VERSION";
export const NATIVE_COMPANIONS_CONTRACT_VERSION = "1";
export const NATIVE_COMPANION_SPAWN_PERMISSION = "service.process.spawn";

export function parseNativeCompanionsEnv(
  raw: string | undefined = defaultNativeCompanionsEnv(),
  version: string | undefined = defaultNativeCompanionsVersionEnv(),
): NativeCompanionRuntimeDescriptor[] {
  if (!raw || !raw.trim()) return [];
  const contractVersion = (version || "").trim();
  if (contractVersion && contractVersion !== NATIVE_COMPANIONS_CONTRACT_VERSION) {
    throw new Error(`unsupported native companion contract version: ${contractVersion}`);
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    throw new Error(`${NATIVE_COMPANIONS_ENV} is not valid JSON`);
  }
  if (!Array.isArray(parsed)) {
    throw new Error(`${NATIVE_COMPANIONS_ENV} must contain a JSON array`);
  }
  return parsed.map((value, index) => normalizeDescriptor(value, index));
}

export function findNativeCompanion(
  id: string,
  companions: readonly NativeCompanionRuntimeDescriptor[] = parseNativeCompanionsEnv(),
): NativeCompanionRuntimeDescriptor | undefined {
  const target = id.trim();
  return companions.find((item) => item.id === target);
}

export function requireNativeCompanion(
  id: string,
  companions: readonly NativeCompanionRuntimeDescriptor[] = parseNativeCompanionsEnv(),
): NativeCompanionRuntimeDescriptor {
  const item = findNativeCompanion(id, companions);
  if (!item) throw new Error(`native companion is unavailable on this host: ${id}`);
  return item;
}

function normalizeDescriptor(value: unknown, index: number): NativeCompanionRuntimeDescriptor {
  if (!value || typeof value !== "object") throw new Error(`native companion descriptor ${index} is invalid`);
  const row = value as Record<string, unknown>;
  const id = stringField(row, "id", index);
  const platform = stringField(row, "platform", index);
  if (platform !== "windows" && platform !== "linux" && platform !== "darwin") {
    throw new Error(`native companion descriptor ${index} has unsupported platform`);
  }
  const architectureValue = typeof row.architecture === "string" ? row.architecture.trim() : "";
  if (architectureValue && architectureValue !== "amd64" && architectureValue !== "arm64") {
    throw new Error(`native companion descriptor ${index} has unsupported architecture`);
  }
  const sha256 = stringField(row, "sha256", index).toLowerCase();
  if (!/^[0-9a-f]{64}$/.test(sha256)) throw new Error(`native companion descriptor ${index} has invalid sha256`);
  const args = Array.isArray(row.args) ? row.args.map((arg) => String(arg)) : undefined;
  return {
    id,
    platform,
    architecture: architectureValue ? architectureValue as "amd64" | "arm64" : undefined,
    path: stringField(row, "path", index),
    sha256,
    executable: row.executable === true,
    args,
  };
}

function stringField(row: Record<string, unknown>, field: string, index: number): string {
  const value = typeof row[field] === "string" ? row[field].trim() : "";
  if (!value) throw new Error(`native companion descriptor ${index} is missing ${field}`);
  return value;
}

function defaultNativeCompanionsEnv(): string | undefined {
  return runtimeEnvironment()?.[NATIVE_COMPANIONS_ENV];
}

function defaultNativeCompanionsVersionEnv(): string | undefined {
  return runtimeEnvironment()?.[NATIVE_COMPANIONS_VERSION_ENV];
}

function runtimeEnvironment(): Record<string, string | undefined> | undefined {
  const runtimeGlobal = globalThis as typeof globalThis & {
    process?: { env?: Record<string, string | undefined> };
  };
  return runtimeGlobal.process?.env;
}
