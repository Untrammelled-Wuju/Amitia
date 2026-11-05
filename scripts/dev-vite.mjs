import { spawn } from "node:child_process";

const [viteBin, cwd, ...extraArgs] = process.argv.slice(2);
if (!viteBin || !cwd) {
  console.error("usage: node scripts/dev-vite.mjs <vite-bin> <cwd> [...args]");
  process.exit(2);
}

const port = Number(process.env.AMITIA_WEB_PORT || 5178);
if (!Number.isSafeInteger(port) || port < 1 || port > 65535 || port === 3000) {
  console.error("项目前端端口必须是 1-65535 之间且不能使用 3000");
  process.exit(2);
}

const child = spawn(
  process.execPath,
  [viteBin, "--host", "127.0.0.1", "--port", String(port), "--strictPort", ...extraArgs],
  {
    cwd,
    stdio: "inherit",
    env: { ...process.env, AMITIA_DEV_SERVER_PORT: String(port) },
  },
);

const forward = (signal) => {
  try {
    child.kill(signal);
  } catch (error) {
  console.error("[original-error]", error);
process.exit(1);
  }
};
process.once("SIGINT", () => forward("SIGINT"));
process.once("SIGTERM", () => forward("SIGTERM"));
child.once("exit", (code, signal) => {
  if (signal) process.kill(process.pid, signal);
  else process.exit(code ?? 0);
});
