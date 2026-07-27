export * from "./types.js";
export * from "./exit-codes.js";
export * from "./logger.js";
export * from "./config/index.js";
export * from "./templates/index.js";
export * from "./commands/index.js";
export * from "./registry.js";

import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { runCli } from "./registry.js";

const entry = process.argv[1];
if (entry && pathToFileURL(resolve(entry)).href === import.meta.url) {
  runCli(process.argv).then((result) => {
    if (result.message) {
      if (result.exitCode === 0) {
        process.stdout.write(`${result.message}\n`);
      } else {
        process.stderr.write(`${result.message}\n`);
      }
    }
    if (result.data !== undefined && process.env.AMITIA_EXT_EMIT_DATA === "1") {
      process.stdout.write(`${JSON.stringify(result.data)}\n`);
    }
    process.exit(result.exitCode);
  });
}
