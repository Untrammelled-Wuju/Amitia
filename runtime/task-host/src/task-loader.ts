import { fileURLToPath, pathToFileURL } from "node:url";
import { createHash } from "node:crypto";
import { closeSync, fstatSync, lstatSync, openSync, readSync, readdirSync } from "node:fs";
import { isAbsolute, join, relative, resolve, sep } from "node:path";
import * as nodeModule from "node:module";
import type { TaskHandler } from "@amitia/plugin-sdk";

type ModuleLoadResult = { format: string; source?: string | ArrayBufferView | null; shortCircuit?: boolean };
type ModuleLoadNext = (url: string, context: unknown) => ModuleLoadResult;
type ModuleResolveResult = { url: string; format?: string; shortCircuit?: boolean };
type ModuleResolveNext = (specifier: string, context: unknown) => ModuleResolveResult;
type ModuleHookRegistry = { registerHooks?: (hooks: { resolve?: (specifier: string, context: unknown, next: ModuleResolveNext) => ModuleResolveResult; load: (url: string, context: unknown, next: ModuleLoadNext) => ModuleLoadResult }) => { deregister: () => void } };

function readVerifiedBytes(path: string, limit: number, expectedHash?: string): Buffer {
  const file = openSync(path, "r");
  try {
    const info = fstatSync(file);
    if (!info.isFile() || info.size > limit) throw new Error("任务模块文件无效或超过限制");
    const bytes = Buffer.alloc(info.size + 1);
    let count = 0;
    while (count < bytes.length) {
      const read = readSync(file, bytes, count, bytes.length - count, null);
      if (read === 0) break;
      count += read;
    }
    const source = bytes.subarray(0, count);
    if (count !== info.size || expectedHash && createHash("sha256").update(source).digest("hex") !== expectedHash) throw new Error("任务模块源码已变化，拒绝执行");
    return source;
  } finally {
    closeSync(file);
  }
}

function bundleManifest(root: string, expectedHash: string): Map<string, string> {
  const hash = expectedHash.replace(/^sha256:/, "").toLowerCase();
  if (!/^[a-f0-9]{64}$/.test(hash) || !lstatSync(root).isDirectory()) throw new Error("任务插件文件树摘要或目录无效");
  const entries = new Map<string, string>();
  let nodes = 0;
  let total = 0;
  const visit = (path: string): void => {
    if (++nodes > 16384) throw new Error("任务插件文件树超过数量限制");
    const info = lstatSync(path);
    if (info.isSymbolicLink()) throw new Error("任务插件文件树不允许符号链接");
    if (info.isDirectory()) {
      for (const name of readdirSync(path)) visit(join(path, name));
      return;
    }
    if (!info.isFile() || info.size > 64 * 1024 * 1024 || (total += info.size) > 256 * 1024 * 1024) throw new Error("任务插件文件树包含无效或超限文件");
    const file = openSync(path, "r");
    try {
      const digest = createHash("sha256");
      const buffer = Buffer.alloc(64 * 1024);
      let count = 0;
      while (true) {
        const read = readSync(file, buffer, 0, buffer.length, null);
        if (!read) break;
        count += read;
        if (count > info.size) throw new Error("任务插件文件在校验时发生变化");
        digest.update(buffer.subarray(0, read));
      }
      if (count !== info.size) throw new Error("任务插件文件在校验时发生变化");
      entries.set(relative(root, path).split(sep).join("/"), digest.digest("hex"));
    } finally {
      closeSync(file);
    }
  };
  visit(root);
  const digest = createHash("sha256");
  for (const path of [...entries.keys()].sort((a, b) => Buffer.compare(Buffer.from(a), Buffer.from(b)))) {
    digest.update(path).update("\0").update(entries.get(path)!).update("\0");
  }
  if (digest.digest("hex") !== hash) throw new Error("任务插件文件树已变化，拒绝执行");
  return entries;
}

export async function loadTaskHandler(entryPath: string, expectedHash: string, bundleRoot = "", bundleHash = ""): Promise<TaskHandler> {
  const hash = expectedHash.replace(/^sha256:/, "").toLowerCase();
  if (!/^[a-f0-9]{64}$/.test(hash)) throw new Error("任务入口缺少有效的源码完整性校验");
  if (lstatSync(entryPath).isSymbolicLink()) throw new Error("任务入口不允许符号链接");
  const fileUrl = pathToFileURL(resolve(entryPath)).href;
  if (Boolean(bundleRoot) !== Boolean(bundleHash)) throw new Error("任务插件缺少完整文件树身份");
  const root = bundleRoot ? resolve(bundleRoot) : "";
  const manifest = root ? bundleManifest(root, bundleHash) : null;
  const verifiedModule = (url: string): Buffer | null => {
    if (!manifest || url.startsWith("node:")) return null;
    if (!url.startsWith("file:")) throw new Error("任务插件不允许加载外部模块");
    const path = fileURLToPath(url);
    const resolvedPath = resolve(path);
    const local = relative(root, resolvedPath);
    if (isAbsolute(local) || local === ".." || local.startsWith(".." + sep) || resolvedPath !== path) throw new Error("任务模块超出固定插件目录");
    let current = root;
    for (const segment of local.split(sep)) {
      current = join(current, segment);
      if (lstatSync(current).isSymbolicLink()) throw new Error("任务模块不允许符号链接");
    }
    const hash = manifest.get(local.split(sep).join("/"));
    if (!hash) throw new Error("任务模块不在固定插件文件树中");
    return readVerifiedBytes(path, 16 * 1024 * 1024, hash);
  };
  verifiedModule(fileUrl);
  const file = openSync(entryPath, "r");
  let source: Buffer;
  try {
    const info = fstatSync(file);
    if (!info.isFile() || info.size > 16 * 1024 * 1024) throw new Error("任务入口文件无效或超过限制");
    const bytes = Buffer.alloc(info.size + 1);
    let count = 0;
    while (count < bytes.length) {
      const read = readSync(file, bytes, count, bytes.length - count, null);
      if (read === 0) break;
      count += read;
    }
    source = bytes.subarray(0, count);
    if (count !== info.size || createHash("sha256").update(source).digest("hex") !== hash) throw new Error("任务入口源码已变化，拒绝执行");
  } finally {
    closeSync(file);
  }
  const registry = nodeModule as unknown as ModuleHookRegistry;
  if (!registry.registerHooks) throw new Error("任务运行时不支持固定源码加载");
  const hook = registry.registerHooks({
    resolve(specifier, context, next) {
      const resolved = next(specifier, context);
      verifiedModule(resolved.url);
      return resolved;
    },
    load(url, context, next) {
      const moduleSource = verifiedModule(url);
      const loaded = next(url, context);
      if (moduleSource && !["module", "commonjs", "json"].includes(loaded.format)) throw new Error("任务插件模块格式不允许");
      return url === fileUrl ? { ...loaded, source, shortCircuit: true } : moduleSource ? { ...loaded, source: moduleSource, shortCircuit: true } : loaded;
    },
  });
  let mod: unknown;
  try {
    mod = await import(fileUrl);
  } catch (error) {
    hook.deregister();
    throw error;
  }
  const handler = extractHandler(mod);
  if (!handler) {
    hook.deregister();
    throw new Error(`task entry module does not export a valid handler: ${entryPath}`);
  }
  if (!manifest) {
    hook.deregister();
    return handler;
  }
  return async (input, context) => {
    try {
      return await handler(input, context);
    } finally {
      hook.deregister();
    }
  };
}

function extractHandler(mod: unknown): TaskHandler | null {
  if (!mod || typeof mod !== "object") {
    return null;
  }

  const record = mod as Record<string, unknown>;

  if (typeof record.default === "function") {
    return record.default as TaskHandler;
  }

  if (typeof record.handler === "function") {
    return record.handler as TaskHandler;
  }

  if (typeof record.run === "function") {
    return record.run as TaskHandler;
  }

  if (record.default && typeof record.default === "object") {
    const def = record.default as Record<string, unknown>;
    if (typeof def.handler === "function") {
      return def.handler as TaskHandler;
    }
  }

  return null;
}
