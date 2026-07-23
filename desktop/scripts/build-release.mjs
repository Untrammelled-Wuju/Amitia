import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

const cliPath = fileURLToPath(
  new URL("../node_modules/electron-builder/cli.js", import.meta.url),
);
const compressionLevel = process.env.ELECTRON_BUILDER_COMPRESSION_LEVEL || "5";
const args = [
  cliPath,
  "--win",
  "--x64",
  "--publish",
  "never",
  ...process.argv.slice(2),
];
const child = spawn(process.execPath, args, {
  stdio: "inherit",
  env: {
    ...process.env,
    ELECTRON_BUILDER_COMPRESSION_LEVEL: compressionLevel,
  },
});

child.on("error", (error) => {
  console.error(error.message);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});
