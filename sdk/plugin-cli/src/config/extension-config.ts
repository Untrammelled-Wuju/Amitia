import { readFileSync, statSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

export interface AmitiaExtensionConfig {
  readonly manifest: string;
  readonly outputDir: string;
  readonly packageDir: string;
  readonly cacheDir?: string;
  readonly targets: ReadonlyArray<BuildTarget>;
  readonly sdkVersion?: string;
  readonly minHostVersion?: string;
  readonly bundler?: "esbuild" | "rollup" | "vite";
  readonly signer?: SignerConfig;
  readonly templates?: Record<string, string>;
  readonly strict?: boolean;
}

export type BuildTarget = "windows-x64" | "macos-arm64" | "macos-x64" | "linux-x64" | "linux-arm64";

export interface SignerConfig {
  readonly kind: "local-pem" | "system-keychain" | "external";
  readonly keyId?: string;
  readonly publicKeyPath?: string;
  readonly externalCommand?: string;
}

export function defineAmitiaExtensionConfig(config: AmitiaExtensionConfig): AmitiaExtensionConfig {
  validateConfig(config);
  return Object.freeze(config);
}

export function validateConfig(config: AmitiaExtensionConfig): string[] {
  const errors: string[] = [];
  if (!config.manifest) errors.push("manifest path is required");
  if (!config.outputDir) errors.push("outputDir is required");
  if (!config.packageDir) errors.push("packageDir is required");
  if (!config.targets || config.targets.length === 0) {
    errors.push("at least one build target is required");
  } else {
    const validTargets = new Set<BuildTarget>(["windows-x64", "macos-arm64", "macos-x64", "linux-x64", "linux-arm64"]);
    for (const target of config.targets) {
      if (!validTargets.has(target)) errors.push(`unknown build target: ${target}`);
    }
  }
  if (config.bundler && !["esbuild", "rollup", "vite"].includes(config.bundler)) {
    errors.push(`unknown bundler: ${config.bundler}`);
  }
  if (config.signer) {
    const signerErrors = validateSignerConfig(config.signer);
    errors.push(...signerErrors);
  }
  return errors;
}

function validateSignerConfig(signer: SignerConfig): string[] {
  const errors: string[] = [];
  if (!signer.kind) {
    errors.push("signer.kind is required");
    return errors;
  }
  switch (signer.kind) {
    case "local-pem":
      if (!signer.keyId) errors.push("signer.keyId is required for local-pem signer");
      break;
    case "system-keychain":
      if (!signer.keyId) errors.push("signer.keyId is required for system-keychain signer");
      break;
    case "external":
      if (!signer.externalCommand) errors.push("signer.externalCommand is required for external signer");
      break;
    default:
      errors.push(`unknown signer kind: ${signer.kind as string}`);
  }
  return errors;
}

export const DEFAULT_CONFIG: AmitiaExtensionConfig = {
  manifest: "./manifest.ts",
  outputDir: "./dist",
  packageDir: "./package",
  targets: ["windows-x64"],
  bundler: "esbuild",
  strict: true,
};

export function mergeConfig(base: AmitiaExtensionConfig, override: Partial<AmitiaExtensionConfig>): AmitiaExtensionConfig {
  return {
    ...base,
    ...override,
    targets: override.targets ?? base.targets,
    signer: override.signer ?? base.signer,
  };
}

export async function loadConfig(configPath: string): Promise<AmitiaExtensionConfig> {
  const absolutePath = resolve(configPath);
  const extension = absolutePath.slice(absolutePath.lastIndexOf(".")).toLowerCase();
  let loaded: unknown;
  if (extension === ".json") {
    loaded = JSON.parse(readFileSync(absolutePath, "utf8"));
  } else if ([".js", ".mjs", ".cjs", ".ts", ".mts", ".cts"].includes(extension)) {
    loaded = await import(pathToFileURL(absolutePath).href + `?mtime=${statSync(absolutePath).mtimeMs}`);
  } else {
    throw new Error(`unsupported config extension: ${extension || "(none)"}`);
  }
  return normalizeLoadedConfig(loaded);
}

function normalizeLoadedConfig(value: unknown): AmitiaExtensionConfig {
  const moduleValue = value && typeof value === "object" && "default" in value
    ? (value as { default: unknown }).default
    : value;
  const candidate = moduleValue && typeof moduleValue === "object" && "config" in moduleValue
    ? (moduleValue as { config: unknown }).config
    : moduleValue;
  if (!candidate || typeof candidate !== "object" || Array.isArray(candidate)) {
    throw new Error("config module must export an object");
  }
  const config = mergeConfig(DEFAULT_CONFIG, candidate as Partial<AmitiaExtensionConfig>);
  const errors = validateConfig(config);
  if (errors.length > 0) {
    throw new Error(`invalid Amitia extension config: ${errors.join("; ")}`);
  }
  return Object.freeze(config);
}
