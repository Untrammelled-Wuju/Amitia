import type { CliCommand, CliContext, CliCommandResult } from "../types";
import { EXIT_CODES } from "../exit-codes";
import * as fs from "node:fs";
import * as path from "node:path";
import { watch } from "node:fs";

export const devCommand: CliCommand = {
  name: "dev",
  description: "Start development mode with hot reload and Developer Host connection",
  usage: "amitia-ext dev [options]",
  options: [
    { name: "--manifest", shortName: "-m", description: "Manifest path", takesValue: true, defaultValue: "./manifest.json" },
    { name: "--host", description: "Developer Host endpoint", takesValue: true, defaultValue: "http://127.0.0.1:18899" },
    { name: "--watch", shortName: "-w", description: "Watch source for changes" },
    { name: "--no-hot-reload", description: "Disable hot reload" },
    { name: "--port", description: "Dev server port (avoid 3000)", takesValue: true, defaultValue: "18900" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    const manifestPath = path.resolve(ctx.cwd, opts["--manifest"] ?? "./manifest.json");
    if (!fs.existsSync(manifestPath)) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: `manifest not found: ${manifestPath}`,
      };
    }
    const port = parseInt(opts["--port"] ?? "18900", 10);
    if (port === 3000) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "port 3000 is reserved for CCX; use another port",
      };
    }
    const watchSrc = opts["--no-hot-reload"] === undefined;
    ctx.logger.info(`dev mode started`, {
      manifest: path.relative(ctx.cwd, manifestPath),
      host: opts["--host"],
      port,
      watch: watchSrc,
    });

    if (watchSrc) {
      const srcDir = path.resolve(ctx.cwd, "./src");
      if (fs.existsSync(srcDir)) {
        ctx.logger.info(`watching ${path.relative(ctx.cwd, srcDir)} for changes`);
        const watcher = watch(srcDir, { recursive: true }, (event, filename) => {
          if (filename && /\.(ts|tsx|js|mjs)$/.test(filename)) {
            ctx.logger.info(`change detected: ${filename} (${event})`);
          }
        });
        return new Promise<CliCommandResult>((resolve) => {
          const stop = () => {
            watcher.close();
            resolve({
              exitCode: EXIT_CODES.SUCCESS,
              message: "dev session stopped",
            });
          };
          process.once("SIGINT", stop);
          process.once("SIGTERM", stop);
        });
      }
    }
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "dev mode exited",
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
