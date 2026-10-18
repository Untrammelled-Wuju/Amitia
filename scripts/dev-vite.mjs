import { spawn } from "node:child_process";
import { createServer } from "node:net";

const [viteBin, cwd, ...extraArgs] = process.argv.slice(2);
if (!viteBin || !cwd) {
  console.error("usage: node scripts/dev-vite.mjs <vite-bin> <cwd> [...args]");
  process.exit(2);
}

const port = await new Promise((resolve, reject) => {
  const server = createServer();
  server.unref();
  server.once("error", reject);
  server.listen(0, "127.0.0.1", () => {
    const address = server.address();
    const value = typeof address === "object" && address ? address.port : 0;
    server.close((error) => {
      if (error) reject(error);
      else if (value > 0) resolve(value);
      else reject(new Error("unable to allocate dev server port"));
    });
  });
});

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
  } catch {
    process.exit(1);
  }
};
process.once("SIGINT", () => forward("SIGINT"));
process.once("SIGTERM", () => forward("SIGTERM"));
child.once("exit", (code, signal) => {
  if (signal) process.kill(process.pid, signal);
  else process.exit(code ?? 0);
});
