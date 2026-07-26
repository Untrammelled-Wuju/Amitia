import type { CliCommand, CliContext, CliCommandResult } from "../types";
import { EXIT_CODES } from "../exit-codes";

export const testCommand: CliCommand = {
  name: "test",
  description: "Run extension unit, contract, and platform tests",
  usage: "amitia-ext test [options]",
  options: [
    { name: "--filter", shortName: "-f", description: "Test filter pattern", takesValue: true },
    { name: "--host-version", description: "Host contract version", takesValue: true },
    { name: "--platform", description: "Target platform", takesValue: true, choices: ["windows", "macos", "linux", "web"] },
    { name: "--arch", description: "Target architecture", takesValue: true, choices: ["x64", "arm64"] },
    { name: "--watch", description: "Watch mode" },
  ],
  async run(ctx: CliContext, _args: string[]): Promise<CliCommandResult> {
    ctx.logger.info("running tests via vitest");
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "tests passed (placeholder; vitest invocation pending)",
      data: { filter: ctx.args },
    };
  },
};

export const buildCommand: CliCommand = {
  name: "build",
  description: "Build the extension TypeScript sources",
  usage: "amitia-ext build [options]",
  options: [
    { name: "--manifest", shortName: "-m", description: "Manifest path", takesValue: true },
    { name: "--output", shortName: "-o", description: "Output directory", takesValue: true },
    { name: "--sourcemap", description: "Generate source maps" },
  ],
  async run(ctx: CliContext, _args: string[]): Promise<CliCommandResult> {
    ctx.logger.info("building via tsc");
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "build succeeded (placeholder; tsc invocation pending)",
    };
  },
};

export const packCommand: CliCommand = {
  name: "pack",
  description: "Pack the extension into a .amitiax package",
  usage: "amitia-ext pack [options]",
  options: [
    { name: "--manifest", shortName: "-m", description: "Manifest path", takesValue: true },
    { name: "--output", shortName: "-o", description: "Output directory", takesValue: true, defaultValue: "./package" },
    { name: "--deterministic", description: "Produce deterministic content tree" },
  ],
  async run(ctx: CliContext, _args: string[]): Promise<CliCommandResult> {
    ctx.logger.info("packing .amitiax");
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "pack succeeded (placeholder; archive assembly pending)",
    };
  },
};

export const signCommand: CliCommand = {
  name: "sign",
  description: "Sign a .amitiax package with a developer key",
  usage: "amitia-ext sign [options]",
  options: [
    { name: "--package", shortName: "-p", description: "Package path", takesValue: true, required: true },
    { name: "--key-id", description: "Signing key id", takesValue: true, required: true },
    { name: "--keychain", description: "Use system keychain" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    if (!opts["--package"] || !opts["--key-id"]) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "missing required options: --package and --key-id",
      };
    }
    ctx.logger.info(`signing ${opts["--package"]} with key ${opts["--key-id"]}`);
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "sign succeeded (placeholder; signing implementation pending)",
    };
  },
};

export const verifyCommand: CliCommand = {
  name: "verify",
  description: "Verify a .amitiax package's signature and integrity",
  usage: "amitia-ext verify [options]",
  options: [
    { name: "--package", shortName: "-p", description: "Package path", takesValue: true, required: true },
    { name: "--public-key", description: "Public key path", takesValue: true },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    if (!opts["--package"]) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "missing required option: --package",
      };
    }
    ctx.logger.info(`verifying ${opts["--package"]}`);
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "verify succeeded (placeholder; verification implementation pending)",
    };
  },
};

export const inspectCommand: CliCommand = {
  name: "inspect",
  description: "Inspect a .amitiax package's contents and metadata",
  usage: "amitia-ext inspect [options]",
  options: [
    { name: "--package", shortName: "-p", description: "Package path", takesValue: true, required: true },
    { name: "--format", description: "Output format", takesValue: true, choices: ["human", "json"], defaultValue: "human" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    if (!opts["--package"]) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "missing required option: --package",
      };
    }
    ctx.logger.info(`inspecting ${opts["--package"]}`);
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "inspect succeeded (placeholder; archive reader pending)",
    };
  },
};

export const installCommand: CliCommand = {
  name: "install",
  description: "Install a .amitiax package via the local Developer Host",
  usage: "amitia-ext install [options]",
  options: [
    { name: "--package", shortName: "-p", description: "Package path", takesValue: true, required: true },
    { name: "--host", description: "Developer Host endpoint", takesValue: true, defaultValue: "http://127.0.0.1:18899" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    if (!opts["--package"]) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "missing required option: --package",
      };
    }
    ctx.logger.info(`submitting ${opts["--package"]} to ${opts["--host"] ?? "developer host"}`);
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "install submitted (placeholder; Developer Host client pending)",
    };
  },
};

export const uninstallCommand: CliCommand = {
  name: "uninstall",
  description: "Uninstall an extension by ID via the Developer Host",
  usage: "amitia-ext uninstall <extensionId>",
  options: [
    { name: "--host", description: "Developer Host endpoint", takesValue: true, defaultValue: "http://127.0.0.1:18899" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const extensionId = args[0];
    if (!extensionId) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "extensionId is required",
      };
    }
    ctx.logger.info(`uninstalling ${extensionId}`);
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "uninstall submitted (placeholder; Developer Host client pending)",
    };
  },
};

export const publishCheckCommand: CliCommand = {
  name: "publish-check",
  description: "Pre-publish verification for release readiness",
  usage: "amitia-ext publish-check [options]",
  options: [
    { name: "--package", shortName: "-p", description: "Package path", takesValue: true },
    { name: "--strict", description: "Treat warnings as errors" },
  ],
  async run(ctx: CliContext, _args: string[]): Promise<CliCommandResult> {
    ctx.logger.info("running publish checks");
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "publish-check passed (placeholder; full checklist pending)",
    };
  },
};

export const migrateCommand: CliCommand = {
  name: "migrate",
  description: "Assist in migrating legacy extension projects to v2",
  usage: "amitia-ext migrate [options]",
  options: [
    { name: "--source", shortName: "-s", description: "Legacy source directory", takesValue: true },
    { name: "--report", description: "Write migration report to file", takesValue: true },
  ],
  async run(ctx: CliContext, _args: string[]): Promise<CliCommandResult> {
    ctx.logger.info("generating migration report");
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "migration report generated (placeholder; legacy scanner pending)",
    };
  },
};

export const doctorCommand: CliCommand = {
  name: "doctor",
  description: "Diagnose environment, toolchain, and SDK installation",
  usage: "amitia-ext doctor",
  async run(ctx: CliContext, _args: string[]): Promise<CliCommandResult> {
    const checks: Array<{ name: string; ok: boolean; detail?: string }> = [];
    checks.push({ name: "node", ok: typeof process !== "undefined" && !!process.versions.node, detail: process.versions.node });
    checks.push({ name: "cli", ok: true, detail: "1.0.0" });
    const failed = checks.filter((c) => !c.ok);
    return {
      exitCode: failed.length > 0 ? EXIT_CODES.ENVIRONMENT_ERROR : EXIT_CODES.SUCCESS,
      message: failed.length > 0 ? `doctor failed: ${failed.map((c) => c.name).join(", ")}` : "doctor passed",
      data: { checks },
    };
  },
};

export const sdkCommand: CliCommand = {
  name: "sdk",
  description: "Inspect SDK version and compatibility matrix",
  usage: "amitia-ext sdk [info|version]",
  async run(_ctx: CliContext, _args: string[]): Promise<CliCommandResult> {
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "sdk info",
      data: {
        sdkVersion: "1.0.0",
        supportedManifestVersion: 2,
        supportedHostApiRange: ">=1.0.0 <2.0.0",
        supportedRuntimeRpcRange: ">=1.0.0 <2.0.0",
        minAmitiaVersion: "0.1.0",
      },
    };
  },
};

function parseOptions(args: string[]): Record<string, string | undefined> {
  const opts: Record<string, string | undefined> = {};
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg?.startsWith("--")) {
      const next = args[i + 1];
      if (next && !next.startsWith("--")) {
        opts[arg] = next;
        i++;
      } else {
        opts[arg] = "true";
      }
    }
  }
  return opts;
}
