import { createServer } from "node:http";

const DEFAULT_HOST = "127.0.0.1";

function text(value) {
  return value == null ? "" : String(value).trim();
}

function normalizePath(value) {
  const path = text(value);
  if (path === "") throw new Error("DSH webServer 路由需要 path");
  if (!path.startsWith("/")) throw new Error(`DSH webServer 路由 path 必须以 / 开头：${path}`);
  if (path.length > 1 && path.endsWith("/")) return path.replace(/\/+$/, "");
  return path;
}

function corsHeaders() {
  return {
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Methods": "GET,POST,PUT,PATCH,DELETE,HEAD,OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type,Authorization,Last-Event-ID,X-Request-ID",
    "Access-Control-Max-Age": "600",
  };
}

function errorText(error) {
  return error instanceof Error ? (error.stack ?? error.message) : String(error);
}

export function createSharedWebServer(options = {}) {
  const host = text(options.host) || DEFAULT_HOST;
  const exact = new Map();
  const prefixes = new Map();
  const upgrades = new Map();
  const taps = [];
  let fallback = null;
  let server = null;
  let port = 0;
  let starting = null;
  let closed = false;

  const requestHandler = async (req, res) => {
    try {
      for (const [name, value] of Object.entries(corsHeaders())) res.setHeader(name, value);
      if (req.method === "OPTIONS") {
        res.statusCode = 204;
        res.end();
        return;
      }
      const pathname = new URL(req.url ?? "/", `http://${host}`).pathname;
      const matched = exact.get(pathname) ?? prefixMatch(pathname);
      if (matched) {
        await matched(req, res);
        return;
      }
      if (fallback) {
        await fallback(req, res);
        return;
      }
      res.statusCode = 404;
      res.setHeader("Content-Type", "application/json; charset=utf-8");
      res.end(JSON.stringify({ error: "not found", path: pathname }));
    } catch (error) {
      console.error("[original-error]", error);
      if (!res.headersSent) {
        res.statusCode = 500;
        res.setHeader("Content-Type", "application/json; charset=utf-8");
        res.end(JSON.stringify({ error: errorText(error) }));
      } else {
        res.end();
      }
    }
  };

  const prefixMatch = (pathname) => {
    let best = null;
    let bestLength = -1;
    for (const [prefix, handler] of prefixes) {
      if (pathname === prefix || pathname.startsWith(`${prefix}/`)) {
        if (prefix.length > bestLength) {
          best = handler;
          bestLength = prefix.length;
        }
      }
    }
    return best;
  };

  const start = () => {
    if (closed) return Promise.reject(new Error("DSH webServer 已关闭"));
    if (server && port > 0) return Promise.resolve(port);
    if (starting) return starting;
    starting = new Promise((resolve, reject) => {
      const instance = createServer((req, res) => {
        void requestHandler(req, res);
      });
      instance.on("upgrade", (req, socket, head) => {
        const pathname = new URL(req.url ?? "/", `http://${host}`).pathname;
        const handler = upgrades.get(pathname);
        if (!handler) {
          socket.destroy();
          return;
        }
        Promise.resolve()
          .then(() => handler(req, socket, head))
          .catch((error) => {
            console.error("[original-error]", error);
            socket.destroy();
          });
      });
      instance.on("error", (error) => {
        console.error("[original-error]", error);
      });
      instance.listen(0, host, () => {
        server = instance;
        port = instance.address().port;
        instance.unref();
        resolve(port);
      });
      instance.once("error", (error) => {
        starting = null;
        reject(error);
      });
    });
    return starting;
  };

  const webServer = {
    get port() {
      return port;
    },
    get host() {
      return host;
    },
    get origin() {
      return port > 0 ? `http://${host}:${port}` : "";
    },
    getOrigin() {
      return webServer.origin;
    },
    routePaths() {
      return { exact: [...exact.keys()], prefixes: [...prefixes.keys()] };
    },
    async ensureStarted() {
      await start();
      return webServer.origin;
    },
    register(route = {}) {
      const kind = text(route.kind) || "exact";
      if (kind !== "exact" && kind !== "prefix") {
        throw new Error(`DSH webServer.register 的 kind 只支持 exact/prefix：${kind}`);
      }
      if (typeof route.handler !== "function") {
        throw new Error("DSH webServer.register 需要 handler 函数");
      }
      const path = normalizePath(route.path);
      const table = kind === "exact" ? exact : prefixes;
      if (table.has(path)) {
        throw new Error(`DSH webServer 路由 ${kind}:${path} 已经注册`);
      }
      table.set(path, route.handler);
      void start();
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (table.get(path) === route.handler) table.delete(path);
      };
    },
    registerUpgrade(route = {}) {
      if (typeof route.handler !== "function") {
        throw new Error("DSH webServer.registerUpgrade 需要 handler 函数");
      }
      const path = normalizePath(route.path);
      if (upgrades.has(path)) {
        throw new Error(`DSH webServer 升级路由 ${path} 已经注册`);
      }
      upgrades.set(path, route.handler);
      void start();
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (upgrades.get(path) === route.handler) upgrades.delete(path);
      };
    },
    registerFallback(handler) {
      if (typeof handler !== "function") {
        throw new Error("DSH webServer.registerFallback 需要 handler 函数");
      }
      if (fallback) {
        throw new Error("DSH webServer 兜底路由已被占用");
      }
      fallback = handler;
      void start();
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (fallback === handler) fallback = null;
      };
    },
    tapIndex(transform) {
      if (typeof transform !== "function") {
        throw new Error("DSH webServer.tapIndex 需要 transform 函数");
      }
      taps.push(transform);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        const index = taps.indexOf(transform);
        if (index >= 0) taps.splice(index, 1);
      };
    },
    applyIndexTaps(html) {
      let next = String(html ?? "");
      for (const transform of taps) {
        try {
          next = String(transform(next) ?? next);
        } catch (error) {
          console.error("[original-error]", error);
        }
      }
      return next;
    },
    collectIndexInjections() {
      return [];
    },
    renderIndex(html) {
      return webServer.applyIndexTaps(html);
    },
    async close() {
      closed = true;
      exact.clear();
      prefixes.clear();
      upgrades.clear();
      taps.length = 0;
      fallback = null;
      if (!server) return;
      const closing = server;
      server = null;
      port = 0;
      await new Promise((resolve) => closing.close(() => resolve()));
    },
  };

  return webServer;
}

let sharedWebServer = null;

export function sharedDshWebServer() {
  if (sharedWebServer === null) sharedWebServer = createSharedWebServer();
  return sharedWebServer;
}
