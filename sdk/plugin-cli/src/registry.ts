import type { CliCommand, CliContext, CliCommandResult } from "./types";
import { ConsoleLogger } from "./logger";
import {
  initCommand,
  devCommand,
  validateCommand,
  lintCommand,
  testCommand,
  buildCommand,
  packCommand,
  signCommand,
  verifyCommand,
  inspectCommand,
  installCommand,
  uninstallCommand,
  publishCheckCommand,
  migrateCommand,
  doctorCommand,
  sdkCommand,
} from "./commands";

export const commands: ReadonlyArray<CliCommand> = [
  initCommand,
  devCommand,
  validateCommand,
  lintCommand,
  testCommand,
  buildCommand,
  packCommand,
  signCommand,
  verifyCommand,
  inspectCommand,
  installCommand,
  uninstallCommand,
  publishCheckCommand,
  migrateCommand,
  doctorCommand,
  sdkCommand,
];

export function findCommand(name: string): CliCommand | undefined {
  return commands.find((c) => c.name === name || c.aliases?.includes(name));
}

export async function runCli(argv: string[]): Promise<CliCommandResult> {
  const args = argv.slice(2);
  if (args.length === 0 || args[0] === "--help" || args[0] === "-h") {
    return printHelp();
  }
  if (args[0] === "--version" || args[0] === "-v") {
    return { exitCode: 0, message: "amitia-ext 1.0.0" };
  }
  const commandName = args[0];
  const command = findCommand(commandName);
  if (!command) {
    return {
      exitCode: 2,
      message: `unknown command: ${commandName} (see --help)`,
    };
  }
  const rest = args.slice(1);
  const logger = new ConsoleLogger(detectLogLevel(rest));
  const ctx: CliContext = {
    cwd: process.cwd(),
    args: rest,
    logger,
    format: detectFormat(rest),
    verbose: rest.includes("--verbose"),
    quiet: rest.includes("--quiet"),
  };
  try {
    return await command.run(ctx, rest);
  } catch (cause) {
    return {
      exitCode: 7,
      message: `internal CLI error: ${cause instanceof Error ? cause.message : String(cause)}`,
    };
  }
}

function detectLogLevel(args: string[]): "debug" | "info" | "warn" | "error" | "quiet" {
  if (args.includes("--quiet")) return "quiet";
  if (args.includes("--debug")) return "debug";
  if (args.includes("--verbose")) return "debug";
  return "info";
}

function detectFormat(args: string[]): "human" | "json" | "sarif" {
  for (let i = 0; i < args.length; i++) {
    if (args[i] === "--format" && args[i + 1]) {
      const fmt = args[i + 1];
      if (fmt === "json" || fmt === "sarif" || fmt === "human") return fmt;
    }
  }
  return "human";
}

function printHelp(): CliCommandResult {
  const lines: string[] = [];
  lines.push("amitia-ext - Amitia Extension CLI");
  lines.push("");
  lines.push("Usage: amitia-ext <command> [options]");
  lines.push("");
  lines.push("Commands:");
  for (const cmd of commands) {
    lines.push(`  ${cmd.name.padEnd(16)} ${cmd.description}`);
  }
  lines.push("");
  lines.push("Global Options:");
  lines.push("  --verbose        Verbose output");
  lines.push("  --debug          Debug output");
  lines.push("  --quiet          Suppress non-error output");
  lines.push("  --format <fmt>   Output format: human|json|sarif");
  lines.push("  --help, -h       Show this help");
  lines.push("  --version, -v    Show CLI version");
  return { exitCode: 0, message: lines.join("\n") };
}
