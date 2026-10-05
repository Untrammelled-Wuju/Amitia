import { pathToFileURL } from "node:url";
import { createHash } from "node:crypto";
import { closeSync, fstatSync, openSync, readSync, realpathSync } from "node:fs";
import * as nodeModule from "node:module";
import type { TaskHandler } from "@amitia/plugin-sdk";

type ModuleLoadResult = { format: string; source?: string | ArrayBufferView | null; shortCircuit?: boolean };
type ModuleLoadNext = (url: string, context: unknown) => ModuleLoadResult;
type ModuleHookRegistry = { registerHooks?: (hooks: { load: (url: string, context: unknown, next: ModuleLoadNext) => ModuleLoadResult }) => { deregister: () => void } };

export async function loadTaskHandler(entryPath: string, expectedHash: string): Promise<TaskHandler> {
  const hash = expectedHash.replace(/^sha256:/, "").toLowerCase();
  if (!/^[a-f0-9]{64}$/.test(hash)) throw new Error("任务入口缺少有效的源码完整性校验");
  const fileUrl = pathToFileURL(realpathSync(entryPath)).href;
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
    load(url, context, next) {
      const loaded = next(url, context);
      return url === fileUrl ? { ...loaded, source, shortCircuit: true } : loaded;
    },
  });
  let mod: unknown;
  try {
    mod = await import(fileUrl);
  } finally {
    hook.deregister();
  }
  const handler = extractHandler(mod);
  if (!handler) {
    throw new Error(`task entry module does not export a valid handler: ${entryPath}`);
  }
  return handler;
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
