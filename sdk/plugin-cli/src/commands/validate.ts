import * as fs from "node:fs";
import * as path from "node:path";
import type { CliCommand, CliContext, CliCommandResult, CliReport } from "../types";
import { EXIT_CODES } from "../exit-codes";
import { validateManifest, type AmitiaxManifestV2 } from "@amitia/plugin-sdk";

export const validateCommand: CliCommand = {
  name: "validate",
  description: "Validate the extension manifest, schemas, and package layout",
  usage: "amitia-ext validate [options]",
  options: [
    { name: "--manifest", shortName: "-m", description: "Manifest path", takesValue: true, defaultValue: "./manifest.json" },
    { name: "--strict", description: "Treat warnings as errors" },
    { name: "--format", description: "Output format", takesValue: true, choices: ["human", "json", "sarif"], defaultValue: "human" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    const manifestPath = path.resolve(ctx.cwd, opts["--manifest"] ?? "./manifest.json");
    if (!fs.existsSync(manifestPath)) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: `manifest not found: ${manifestPath}`,
        reports: [
          {
            ruleId: "manifest.missing",
            severity: "error",
            message: `manifest not found: ${manifestPath}`,
            file: manifestPath,
          },
        ],
      };
    }

    const reports: CliReport[] = [];
    let manifest: AmitiaxManifestV2 | null = null;
    try {
      const raw = fs.readFileSync(manifestPath, "utf-8");
      manifest = JSON.parse(raw) as AmitiaxManifestV2;
    } catch (cause) {
      reports.push({
        ruleId: "manifest.parse",
        severity: "error",
        message: `failed to parse manifest: ${cause instanceof Error ? cause.message : String(cause)}`,
        file: manifestPath,
      });
      return {
        exitCode: EXIT_CODES.VALIDATION_OR_BUILD_FAILURE,
        message: "manifest parse error",
        reports,
      };
    }

    const errors = validateManifest(manifest);
    for (const err of errors) {
      reports.push({
        ruleId: "manifest.schema",
        severity: "error",
        message: err,
        file: manifestPath,
      });
    }

    reports.push(...validateEntryPaths(ctx, manifest, manifestPath));
    reports.push(...validatePermissions(manifest, manifestPath));
    reports.push(...validatePlatforms(manifest, manifestPath));
    reports.push(...validateModules(manifest, manifestPath));

    const strict = opts["--strict"] !== undefined;
    const hasErrors = reports.some((r) => r.severity === "error");
    const hasWarnings = reports.some((r) => r.severity === "warning");
    const failed = hasErrors || (strict && hasWarnings);

    return {
      exitCode: failed ? EXIT_CODES.VALIDATION_OR_BUILD_FAILURE : EXIT_CODES.SUCCESS,
      message: failed
        ? `validation failed with ${reports.length} issue(s)`
        : `validation passed (${reports.length} informational)`,
      reports,
      data: {
        manifest: manifest.extensionId,
        version: manifest.version,
        modules: manifest.modules.length,
      },
    };
  },
};

function validateEntryPaths(ctx: CliContext, manifest: AmitiaxManifestV2, manifestPath: string): CliReport[] {
  const reports: CliReport[] = [];
  const baseDir = path.dirname(manifestPath);
  for (const module of manifest.modules ?? []) {
    const entry = path.resolve(baseDir, module.entry);
    if (!fs.existsSync(entry)) {
      reports.push({
        ruleId: "module.entry.missing",
        severity: "warning",
        message: `module ${module.moduleId} entry not found: ${path.relative(ctx.cwd, entry)}`,
        file: entry,
      });
    }
  }
  return reports;
}

function validatePermissions(manifest: AmitiaxManifestV2, manifestPath: string): CliReport[] {
  const reports: CliReport[] = [];
  const highRisk = ["desktop.shell", "filesystem.write", "network.request", "process.spawn"];
  for (const perm of manifest.permissions ?? []) {
    if (highRisk.includes(perm.permission) && !perm.reason) {
      reports.push({
        ruleId: "permission.high_risk_no_reason",
        severity: "warning",
        message: `high-risk permission ${perm.permission} requires a reason`,
        file: manifestPath,
      });
    }
  }
  return reports;
}

function validatePlatforms(manifest: AmitiaxManifestV2, manifestPath: string): CliReport[] {
  const reports: CliReport[] = [];
  if (!manifest.platforms || manifest.platforms.length === 0) {
    reports.push({
      ruleId: "platforms.empty",
      severity: "info",
      message: "no platforms declared; extension will be assumed platform-agnostic",
      file: manifestPath,
    });
  }
  return reports;
}

function validateModules(manifest: AmitiaxManifestV2, manifestPath: string): CliReport[] {
  const reports: CliReport[] = [];
  const moduleIds = new Set<string>();
  for (const module of manifest.modules ?? []) {
    if (moduleIds.has(module.moduleId)) {
      reports.push({
        ruleId: "module.duplicate_id",
        severity: "error",
        message: `duplicate module id: ${module.moduleId}`,
        file: manifestPath,
      });
    }
    moduleIds.add(module.moduleId);
    if (module.deprecated && !module.deprecationNote) {
      reports.push({
        ruleId: "module.deprecated_no_note",
        severity: "warning",
        message: `module ${module.moduleId} is deprecated without note`,
        file: manifestPath,
      });
    }
  }
  return reports;
}

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
    } else if (arg?.startsWith("-")) {
      const next = args[i + 1];
      if (next && !next.startsWith("-")) {
        opts[arg] = next;
        i++;
      } else {
        opts[arg] = "true";
      }
    }
  }
  return opts;
}
