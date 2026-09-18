import net from "node:net";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, join, resolve } from "node:path";
import assert from "node:assert/strict";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const SERVICE_TOKEN = "amitia-security-test-token";
const AUTH_HEADERS = { authorization: `Bearer ${SERVICE_TOKEN}` };

async function waitFor(fn, timeout = 6000) {
  const started = Date.now();
  while (Date.now() - started < timeout) {
    try { const value = await fn(); if (value) return value; } catch {}
    await new Promise((resolvePromise) => setTimeout(resolvePromise, 100));
  }
  throw new Error("timeout");
}

function portOpen(port) {
  return new Promise((resolvePromise) => {
    const socket = net.createConnection({ host: "127.0.0.1", port });
    socket.once("connect", () => { socket.destroy(); resolvePromise(true); });
    socket.once("error", () => resolvePromise(false));
    socket.setTimeout(250, () => { socket.destroy(); resolvePromise(false); });
  });
}

const child = spawn(process.execPath, [join(root, "runtime", "service.mjs")], {
  cwd: root,
  env: {
    ...process.env,
    AMITIA_SERVICE_AUTH_TOKEN: SERVICE_TOKEN,
    AMITIA_SERVICE_AUTH_VERSION: "1",
    AMITIA_CORE_URL: "http://127.0.0.1:18899",
  },
  stdio: ["ignore", "pipe", "pipe"],
});
let logs = "";
child.stdout.on("data", (chunk) => { logs += chunk; });
child.stderr.on("data", (chunk) => { logs += chunk; });

try {
  await waitFor(async () => {
    const response = await fetch("http://127.0.0.1:19878/api/health", { headers: AUTH_HEADERS });
    return response.ok;
  });

  const noAuth = await fetch("http://127.0.0.1:19878/api/status");
  assert.equal(noAuth.status, 401);
  assert.equal(noAuth.headers.get("access-control-allow-origin"), null);

  const wrongAuth = await fetch("http://127.0.0.1:19878/api/status", {
    headers: { authorization: "Bearer definitely-wrong" },
  });
  assert.equal(wrongAuth.status, 401);

  const authorized = await fetch("http://127.0.0.1:19878/api/status", { headers: AUTH_HEADERS });
  assert.equal(authorized.status, 200);

  // The legacy unauthenticated hero receiver must not exist in production mode.
  assert.equal(await portOpen(9999), false);

  // A forged local callback without the service token must not enter the pipeline.
  const forged = await fetch("http://127.0.0.1:19878/api/native/callback", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ fromWxid: "attacker", content: "forged", msgId: "evil-1" }),
  });
  assert.equal(forged.status, 401);

  console.log("wechat-personal service security e2e: PASS");
} finally {
  child.kill("SIGTERM");
  await new Promise((resolvePromise) => setTimeout(resolvePromise, 150));
  if (child.exitCode && child.exitCode !== 0) console.error(logs);
}
