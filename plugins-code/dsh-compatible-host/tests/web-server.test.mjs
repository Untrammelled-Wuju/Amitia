import assert from "node:assert/strict";
import test from "node:test";
import { createSharedWebServer } from "../src/runtime/web-server.mjs";

async function withServer(run) {
  const server = createSharedWebServer();
  try {
    const origin = await server.ensureStarted();
    await run(server, origin);
  } finally {
    await server.close();
  }
}

test("webServer 注册 exact/prefix 路由并带 CORS", async () => {
  await withServer(async (server, origin) => {
    const disposeExact = server.register({ kind: "exact", path: "/pet/ping", handler: (req, res) => { res.statusCode = 200; res.end("pong"); } });
    const disposePrefix = server.register({ kind: "prefix", path: "/pet", handler: (req, res) => { res.statusCode = 200; res.end(`prefix:${new URL(req.url, origin).pathname}`); } });
    const exact = await fetch(`${origin}/pet/ping`);
    assert.equal(await exact.text(), "pong");
    assert.equal(exact.headers.get("access-control-allow-origin"), "*");
    const nested = await fetch(`${origin}/pet/inner/page`);
    assert.equal(await nested.text(), "prefix:/pet/inner/page");
    disposeExact();
    disposePrefix();
    const missing = await fetch(`${origin}/pet/ping`);
    assert.equal(missing.status, 404);
  });
});

test("webServer 拒绝重复路由并提供兜底与升级路由", async () => {
  await withServer(async (server, origin) => {
    server.register({ kind: "exact", path: "/dup", handler: (req, res) => res.end("one") });
    assert.throws(() => server.register({ kind: "exact", path: "/dup", handler: (req, res) => res.end("two") }), /已经注册/);
    const release = server.registerFallback((req, res) => { res.statusCode = 200; res.end("fallback"); });
    assert.equal(await (await fetch(`${origin}/anything`)).text(), "fallback");
    assert.throws(() => server.registerFallback((req, res) => res.end("x")), /已被占用/);
    release();
    const upgrade = server.registerUpgrade({ path: "/ws", handler: () => {} });
    upgrade();
  });
});

test("webServer 应用 tapIndex 变换并可撤销", async () => {
  await withServer(async (server) => {
    const off = server.tapIndex((html) => html.replace("</head>", "<script src=\"/pet.js\"></script></head>"));
    assert.equal(server.applyIndexTaps("<html><head></head></html>"), "<html><head><script src=\"/pet.js\"></script></head></html>");
    assert.deepEqual(server.collectIndexInjections(), []);
    off();
    assert.equal(server.applyIndexTaps("<head></head>"), "<head></head>");
  });
});
