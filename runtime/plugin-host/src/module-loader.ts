import * as path from "path";
import * as fs from "fs";
import Module from "module";
import { ExtensionContext } from "./runtime-context";

export interface LoadedExtension {
  activate?(context: ExtensionContext): Promise<void> | void;
  deactivate?(): Promise<void> | void;
}

const FORBIDDEN_BUILTINS: Set<string> = new Set([
  "electron",
  "v8",
  "vm",
]);

const PROCESS_BUILTINS: Set<string> = new Set([
  "child_process",
  "cluster",
  "os",
  "readline",
  "worker_threads",
]);

const NETWORK_BUILTINS: Set<string> = new Set([
  "dgram",
  "dns",
  "http",
  "https",
  "net",
  "tls",
]);

const FILESYSTEM_BUILTINS: Set<string> = new Set([
  "fs",
  "fs/promises",
]);

export interface ModuleLoadOptions {
  networkDisabled?: boolean;
  filesystemDisabled?: boolean;
  processDisabled?: boolean;
}

function builtinName(request: string): string {
  if (request.startsWith("node:")) {
    return request.slice(5).split("/")[0];
  }
  return request;
}

function isForbiddenRequest(request: string, extensionRoot: string, options: ModuleLoadOptions): boolean {
  if (!request || typeof request !== "string") {
    return true;
  }
  const name = builtinName(request);
  if (FORBIDDEN_BUILTINS.has(name)) {
    return true;
  }
  if (options.filesystemDisabled && FILESYSTEM_BUILTINS.has(name)) {
    return true;
  }
  if (options.processDisabled && PROCESS_BUILTINS.has(name)) {
    return true;
  }
  if (options.networkDisabled && NETWORK_BUILTINS.has(name)) {
    return true;
  }
  if (path.isAbsolute(request)) {
    return true;
  }
  if (request.endsWith(".node")) {
    return true;
  }
  const base = request.split("/")[0];
  if (FORBIDDEN_BUILTINS.has(base)) {
    return true;
  }
  if (request.startsWith(".")) {
    const resolved = path.resolve(extensionRoot, request);
    const rel = path.relative(extensionRoot, resolved);
    if (rel.startsWith("..") || path.isAbsolute(rel)) {
      return true;
    }
  }
  return false;
}

export function loadExtension(entryPath: string, options: ModuleLoadOptions = {}): LoadedExtension {
  const absoluteEntry = path.resolve(entryPath);
  if (!fs.existsSync(absoluteEntry)) {
    throw new Error("Extension entry not found: " + absoluteEntry);
  }
  const extensionRoot = path.dirname(absoluteEntry);

  let captured: any = null;
  const previousDefine = (globalThis as any).defineExtension;
  (globalThis as any).defineExtension = (manifest: any) => {
    captured = manifest;
    return manifest;
  };

  try {
    const mod = new Module(absoluteEntry, module);
    mod.filename = absoluteEntry;
    mod.paths = (Module as any)._nodeModulePaths(extensionRoot);

    const originalRequire = mod.require.bind(mod);
    (mod as any).require = function (request: string) {
      if (isForbiddenRequest(request, extensionRoot, options)) {
        throw new Error("Forbidden module in extension sandbox: " + request);
      }
      return originalRequire(request);
    };

    const content = fs.readFileSync(absoluteEntry, "utf-8");
    if (options.networkDisabled && /\bimport\s*\(/.test(content)) {
      throw new Error("Dynamic import is not allowed when network access is disabled");
    }
    (mod as any)._compile(content, absoluteEntry);

    if (!captured && mod.exports) {
      captured = (mod.exports as any).default ?? mod.exports;
    }
    if (!captured) {
      throw new Error("Extension entry did not call defineExtension()");
    }

    return {
      activate: captured.activate,
      deactivate: captured.deactivate,
    };
  } finally {
    if (previousDefine === undefined) {
      delete (globalThis as any).defineExtension;
    } else {
      (globalThis as any).defineExtension = previousDefine;
    }
  }
}
