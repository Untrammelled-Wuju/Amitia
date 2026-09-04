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

export function loadConfig(configPath: string): AmitiaExtensionConfig {
  // Stub for dynamic import — actual implementation resolves .ts/.js file at runtime
  throw new Error(`loadConfig not implemented for path: ${configPath}`);
}
