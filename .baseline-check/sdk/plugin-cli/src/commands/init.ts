import * as fs from "node:fs";
import * as path from "node:path";
import type { CliCommand, CliContext, CliCommandResult } from "../types.js";
import { EXIT_CODES } from "../exit-codes.js";
import { getTemplateDescriptor, listTemplateKinds, type TemplateKind, type TemplateScaffoldInput } from "../templates/index.js";

export const initCommand: CliCommand = {
  name: "init",
  description: "Scaffold a new Amitia extension from a template",
  usage: "amitia-ext init [options]",
  options: [
    { name: "--template", shortName: "-t", description: "Template kind", takesValue: true, choices: listTemplateKinds() },
    { name: "--extension-id", description: "Extension ID (publisher/name)", takesValue: true },
    { name: "--publisher", description: "Publisher name", takesValue: true },
    { name: "--display-name", description: "Display name", takesValue: true },
    { name: "--version", description: "Initial version", takesValue: true, defaultValue: "0.1.0" },
    { name: "--license", description: "License SPDX", takesValue: true, defaultValue: "MIT" },
    { name: "--target", description: "Build target (repeatable)", takesValue: true, defaultValue: "windows-x64" },
    { name: "--sdk-version", description: "SDK version", takesValue: true, defaultValue: "1.0.0" },
    { name: "--output", shortName: "-o", description: "Output directory", takesValue: true, defaultValue: "." },
    { name: "--force", description: "Overwrite existing files" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    const template = (opts["--template"] ?? "tool") as TemplateKind;
    const outputDir = path.resolve(ctx.cwd, opts["--output"] ?? ".");
    if (!listTemplateKinds().includes(template)) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: `unknown template: ${template}`,
      };
    }

    const input: TemplateScaffoldInput = {
      extensionId: opts["--extension-id"] ?? "publisher/my-extension",
      publisher: opts["--publisher"] ?? "publisher",
      displayName: opts["--display-name"] ?? "My Extension",
      description: `Amitia extension scaffolded from ${template} template`,
      version: opts["--version"] ?? "0.1.0",
      license: opts["--license"] ?? "MIT",
      targets: (opts["--target"] ?? "windows-x64").split(",").map((t) => t.trim()),
      sdkVersion: opts["--sdk-version"] ?? "1.0.0",
    };

    const descriptor = getTemplateDescriptor(template, input);
    const force = opts["--force"] !== undefined;

    try {
      ensureOutputDir(outputDir, force);
      for (const file of descriptor.files) {
        const target = path.resolve(outputDir, file.path);
        ensureParentDir(target);
        if (fs.existsSync(target) && !force) {
          return {
            exitCode: EXIT_CODES.CONFIGURATION_ERROR,
            message: `file already exists: ${target} (use --force to overwrite)`,
          };
        }
        fs.writeFileSync(target, file.content, { encoding: "utf-8" });
        if (file.executable) {
          fs.chmodSync(target, 0o755);
        }
        ctx.logger.info(`wrote ${path.relative(ctx.cwd, target)}`);
      }
      return {
        exitCode: EXIT_CODES.SUCCESS,
        message: `extension '${input.extensionId}' scaffolded at ${path.relative(ctx.cwd, outputDir)}`,
        data: { template, files: descriptor.files.map((f) => f.path) },
      };
    } catch (cause) {
      return {
        exitCode: EXIT_CODES.ENVIRONMENT_ERROR,
        message: `init failed: ${cause instanceof Error ? cause.message : String(cause)}`,
      };
    }
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

function ensureOutputDir(dir: string, force: boolean): void {
  if (fs.existsSync(dir)) {
    if (!force && fs.readdirSync(dir).length > 0) {
      throw new Error(`output directory not empty: ${dir} (use --force to overwrite)`);
    }
  } else {
    fs.mkdirSync(dir, { recursive: true });
  }
}

function ensureParentDir(filePath: string): void {
  const parent = path.dirname(filePath);
  if (!fs.existsSync(parent)) {
    fs.mkdirSync(parent, { recursive: true });
  }
}
