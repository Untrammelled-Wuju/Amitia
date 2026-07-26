#!/usr/bin/env node
import { runCli } from "../registry";

async function main(): Promise<void> {
  const result = await runCli(process.argv);
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
}

void main();
