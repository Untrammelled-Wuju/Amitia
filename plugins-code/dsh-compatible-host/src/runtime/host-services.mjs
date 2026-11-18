import {
  SESSION_STATE_VERSION,
  assertSessionEventBudget,
  assertSessionStateVersion,
  assertSurfaceReplacement,
  createSessionEvent,
  isSurfaceEventType,
  nextSessionEventSeq,
  projectSurfaceEvents,
  restoreSessionEvents,
} from "./session-events.mjs";
import { createCommandsService, createCredentialsService, createSessionQueryService } from "./host-service-bridges.mjs";
import { randomUUID } from "node:crypto";
import { approvalPromptText } from "./approval-bridge.mjs";
import { createDshScheduleDefinition, mapDshScheduleCatalog, updateDshScheduleDefinition } from "./schedule-adapter.mjs";

function text(value) {
  return value == null ? "" : String(value).trim();
}

function logOriginalError(error) {
  console.error("[original-error]", error);
}

function messageText(message) {
  if (typeof message === "string") return message;
  if (message && typeof message === "object") {
    const content = message.content;
    if (typeof content === "string") return content;
    if (Array.isArray(content)) {
      return content.map((block) => {
        if (typeof block === "string") return block;
        if (block && typeof block === "object" && typeof block.text === "string") return block.text;
        return "";
      }).filter(Boolean).join("");
    }
    if (typeof message.text === "string") return message.text;
  }
  return message == null ? "" : JSON.stringify(message);
}

function messageParts(message) {
  const content = message && typeof message === "object" ? message.content : message;
  const blocks = Array.isArray(content)
    ? content
    : content === undefined && message && typeof message === "object" && typeof message.text === "string"
      ? [message]
      : [content];
  return blocks.map((block) => {
    if (typeof block === "string") return { type: "text", content: block };
    if (block && typeof block === "object" && (block.type === "text" || typeof block.text === "string")) {
      return { type: "text", content: text(block.text ?? block.content) };
    }
    throw new Error(`DSH agent message content block is unsupported: ${JSON.stringify(block)}`);
  });
}

function promptEntry(raw, kind) {
  if (!raw || typeof raw !== "object") throw new Error(`DSH systemPrompt.${kind} options must be an object`);
  const name = text(raw.name);
  if (!name) throw new Error(`DSH systemPrompt.${kind} name is required`);
  const order = Number(raw.order);
  if (!Number.isFinite(order)) throw new Error(`DSH systemPrompt.${kind} ${name} order must be finite`);
  if (typeof raw.text !== "string" && typeof raw.text !== "function") {
    throw new Error(`DSH systemPrompt.${kind} ${name} text must be a string or function`);
  }
  return { ...raw, name, order, kind };
}

function promptText(entry, context) {
  const value = typeof entry.text === "function" ? entry.text(context) : entry.text;
  if (typeof value !== "string") throw new Error(`DSH systemPrompt.${entry.kind} ${entry.name} text provider must return a string`);
  return value;
}

function managedCollection(collection, kind) {
  return {
    add(raw) {
      const entry = promptEntry(raw, kind);
      if (collection.some((item) => item.name === entry.name)) {
        throw new Error(`DSH systemPrompt.${kind} ${entry.name} is already registered`);
      }
      collection.push(entry);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        const index = collection.indexOf(entry);
        if (index >= 0) collection.splice(index, 1);
      };
    },
    snapshot(context = {}) {
      return collection
        .map((entry) => ({ ...entry, text: promptText(entry, context) }))
        .sort((left, right) => left.order - right.order || left.name.localeCompare(right.name))
        .map((entry) => ({ ...entry }));
    },
  };
}

export function createAmitiaDshPromptState() {
  return {
    sections: [],
    contexts: [],
    variables: new Map(),
    toolProviders: [],
  };
}

export function snapshotAmitiaDshPromptState(promptState, context = {}) {
  if (!promptState || !Array.isArray(promptState.sections)) throw new Error("DSH prompt state is invalid");
  const sections = managedCollection(promptState.sections, "section").snapshot(context);
  const contexts = managedCollection(promptState.contexts, "context").snapshot(context);
  const variables = Object.fromEntries(Array.from(promptState.variables.entries()).map(([key, provider]) => [key, provider(context)]));
  const tools = promptState.toolProviders.flatMap((provider) => {
    const result = provider(context);
    if (!result || !Array.isArray(result.schemas)) throw new Error("DSH systemPrompt.tools provider must return schemas");
    return result.schemas;
  });
  return { sections, contexts, variables, tools };
}

function unsupportedOperation(operation, detail) {
  const error = new Error(`DSH ${operation} 没有可用的 Amitia 等价执行语义${detail ? `: ${detail}` : ""}`);
  error.code = "DSH_HOST_OPERATION_UNAVAILABLE";
  error.details = { operation, detail };
  return error;
}

function mergeSessionEvents(target, incoming) {
  const existing = new Set(target.map((event) => `${event.seq}:${event.type}:${event.time}`));
  for (const event of incoming) {
    if (!event || typeof event !== "object") continue;
    const identity = `${event.seq}:${event.type}:${event.time}`;
    if (existing.has(identity)) continue;
    existing.add(identity);
    target.push(event);
  }
  target.sort((left, right) => (Number(left.seq) || 0) - (Number(right.seq) || 0) || (Number(left.time) || 0) - (Number(right.time) || 0));
}

function hostSeamError(details) {
  const { method, capability, expectation, missing, request } = details;
  const parts = [`DSH ${method} 不可用`];
  parts.push(`期望能力 ${capability}`);
  if (expectation) parts.push(`语义要求 ${expectation}`);
  parts.push(`宿主缺少接口 ${missing}`);
  if (request !== undefined) parts.push(`入参 ${JSON.stringify(request)}`);
  const error = new Error(parts.join("；"));
  error.code = "DSH_HOST_OPERATION_UNAVAILABLE";
  error.missingHostInterface = missing;
  error.details = {
    operation: method,
    capability,
    expectation: expectation ?? null,
    missingHostInterface: missing,
    ...(request === undefined ? {} : { request }),
  };
  return error;
}

function seamThrower(options) {
  return (request) => {
    throw hostSeamError({ ...options, request });
  };
}

function createUnavailableHostService(options) {
  const capability = options.capability;
  return new Proxy({}, {
    get(_target, property) {
      if (typeof property === "symbol") return undefined;
      if (property === "then" || property === "toJSON" || property === "constructor") return undefined;
      return (...args) => {
        throw hostSeamError({
          ...options,
          method: `${capability}.${property}`,
          request: args.length === 0 ? undefined : args,
        });
      };
    },
  });
}


function sessionIdOf(method, request) {
  const sessionId = text(request?.sessionId);
  if (!sessionId) {
    throw new Error(`DSH ${method} 需要 sessionId：Amitia host.workspace.read 的会话文件操作按 sessionId 解析工作区挂载`);
  }
  return sessionId;
}

function workspaceOperations(call, prefix) {
  const read = (operation, payload) => call("host.workspace.read", { operation, payload });
  return {
    async list(request = {}) {
      return read("file_list", {
        sessionId: sessionIdOf(`${prefix}.list`, request),
        path: text(request.path),
      });
    },
    async search(request = {}) {
      return read("file_search", {
        sessionId: sessionIdOf(`${prefix}.search`, request),
        query: text(request.query),
        excludedPaths: Array.isArray(request.excludedPaths) ? request.excludedPaths : [],
        includeHidden: request.includeHidden === true,
        respectGitignore: request.respectGitignore === true,
      });
    },
    async stat(request = {}) {
      return read("file_stat", {
        sessionId: sessionIdOf(`${prefix}.stat`, request),
        path: text(request.path),
      });
    },
    async read(request = {}) {
      return read("file_read_text", {
        sessionId: sessionIdOf(`${prefix}.read`, request),
        path: text(request.path),
        ...(request.offset === undefined ? {} : { offset: request.offset }),
        ...(request.limit === undefined ? {} : { limit: request.limit }),
      });
    },
    async readBytes(request = {}) {
      return read("file_read_bytes", {
        sessionId: sessionIdOf(`${prefix}.readBytes`, request),
        path: text(request.path),
        ...(text(request.baseFile) ? { baseFile: text(request.baseFile) } : {}),
        ...(request.offset === undefined ? {} : { offset: request.offset }),
        ...(request.length === undefined ? {} : { length: request.length }),
      });
    },
  };
}

function createWorkspaceFilesService(call) {
  const operations = workspaceOperations(call, "workspaceFiles");
  return {
    ...operations,
    changes: seamThrower({
      method: "workspaceFiles.changes",
      capability: "工作区目录与文件变更帧",
      expectation: "持续输出 DSH ready/change 帧",
      missing: "host.workspace.watch（host.workspace.read 只有 file_list/file_search/file_stat/file_read_text/file_read_bytes，没有变更订阅操作）",
    }),
  };
}

function createFileReferencesService(call) {
  const operations = workspaceOperations(call, "fileReferences");
  return {
    list: operations.list,
    search: operations.search,
    stat: operations.stat,
    read: operations.read,
  };
}

function createSessionFileReferencesService(call) {
  const operations = workspaceOperations(call, "sessionFileReferences");
  const scoped = (operation) => (sessionId, request = {}) => operation({ ...request, sessionId });
  return {
    list: scoped(operations.list),
    search: scoped(operations.search),
    stat: scoped(operations.stat),
    read: scoped(operations.read),
  };
}

function skillCatalogUnavailable(method, request) {
  return hostSeamError({
    method,
    capability: "Amitia 技能目录（SKILL.md 分层清单）",
    expectation: "返回全部可用技能定义",
    missing: "host.skill.list / host.skill.read（Amitia kernel 的 agent_skill 目录未暴露为扩展 Host API 方法）",
    request,
  });
}

function createSkillsService(call) {
  const providers = new Map();
  const registered = new Map();
  let catalog = { items: [], state: "idle", error: null };
  const describe = (definition) => ({
    name: text(definition?.displayName) || text(definition?.name) || text(definition?.skillId) || text(definition?.id),
    skillId: text(definition?.skillId) || text(definition?.id),
    description: text(definition?.description),
    version: text(definition?.version),
    extensionId: text(definition?.extensionId),
    enabled: definition?.enabled !== false,
    source: "host",
  });
  const refresh = async () => {
    if (typeof call !== "function") throw skillCatalogUnavailable("skills.list", {});
    let result;
    try {
      result = await call("host.skill.list", { limit: 200 });
    } catch (error) {
      catalog = { items: [], state: "failed", error };
      throw error;
    }
    const items = (Array.isArray(result?.items) ? result.items : []).map(describe).filter((item) => item.name);
    catalog = { items, state: "ready", error: null };
    return catalog;
  };
  const merged = () => {
    const local = [...registered.values()].map((skill) => ({ ...skill, source: "local" }));
    const names = new Set(local.map((skill) => text(skill.name)));
    return [...local, ...catalog.items.filter((item) => !names.has(item.name))];
  };
  return {
    refresh,
    registerProvider(create) {
      if (typeof create !== "function") throw new Error("DSH skills.registerProvider 需要 provider 工厂函数");
      const id = randomUUID();
      providers.set(id, create);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        providers.delete(id);
      };
    },
    register(skill) {
      const name = text(skill?.name);
      if (!name) throw new Error("DSH skills.register 需要技能 name");
      if (registered.has(name)) throw new Error(`DSH 技能 ${name} 已经注册`);
      registered.set(name, skill);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (registered.get(name) === skill) registered.delete(name);
      };
    },
    get(name) {
      const key = text(name);
      const local = registered.get(key);
      if (local) return local;
      const hosted = catalog.items.find((item) => item.name === key || item.skillId === key);
      if (hosted) return hosted;
      throw skillCatalogUnavailable("skills.get", { name: key, registered: registered.size, providers: providers.size });
    },
    list() {
      return merged();
    },
    snapshot() {
      return { items: merged(), state: catalog.state, error: catalog.error };
    },
  };
}

function createSessionSkillCatalogService(call) {
  let catalog = { items: [], state: "idle", error: null };
  const refresh = async () => {
    if (typeof call !== "function") {
      throw seamUnavailable("sessionSkillCatalog", "按会话返回可用技能目录", "host.skill.list 的宿主调用桥不可用");
    }
    let result;
    try {
      result = await call("host.skill.list", { limit: 200 });
    } catch (error) {
      catalog = { items: [], state: "failed", error };
      throw error;
    }
    const items = (Array.isArray(result?.items) ? result.items : []).map((item) => ({
      skillId: text(item?.skillId) || text(item?.id),
      name: text(item?.name),
      description: text(item?.description),
      enabled: item?.enabled !== false,
    })).filter((item) => item.name);
    catalog = { items, state: "ready", error: null };
    return catalog;
  };
  return {
    refresh,
    list() {
      return catalog.items.slice();
    },
    snapshot() {
      return { items: catalog.items.slice(), state: catalog.state, error: catalog.error };
    },
  };
}

function createDirectoryPickerService(call) {
  const requireCall = (method) => {
    if (typeof call !== "function") {
      throw seamUnavailable("directoryPicker", method, "host.directory.* 的宿主调用桥不可用");
    }
    return call;
  };
  const pick = seamThrower({
    method: "directoryPicker.pick",
    capability: "交互式选择宿主目录",
    expectation: "返回用户选中的绝对路径与授权范围",
    missing: "host.directory.pick（原生目录对话框由渲染进程 window.amitiaDesktop.selectDirectory 提供，宿主侧没有对应方法）",
  });
  const list = async (path, signal) => {
    signal?.throwIfAborted?.();
    const target = text(path);
    const result = await requireCall("directoryPicker.list")("host.directory.list", {
      ...(target ? { path: target } : {}),
      limit: 200,
    });
    signal?.throwIfAborted?.();
    return result;
  };
  const createDirectory = async (path, name) => {
    const target = text(path);
    const child = text(name);
    if (!target) throw new Error(`DSH directoryPicker.createDirectory 需要绝对路径：${JSON.stringify({ path: path ?? null })}`);
    const input = { path: target };
    if (child) input.name = child;
    return requireCall("directoryPicker.createDirectory")("host.directory.create", input);
  };
  return {
    pick,
    list,
    createDirectory,
    pickDirectory: pick,
    open: pick,
  };
}

function createDirectoryPickerControllerService() {
  return createUnavailableHostService({
    capability: "directoryPickerController",
    expectation: "把目录选择流程暴露给工作区控制器",
    missing: "host.directory.pick（控制器依赖 directoryPicker，Amitia 未暴露交互式目录选择接口）",
  });
}

function createWorkspaceControllerService() {
  return createUnavailableHostService({
    capability: "workspaceController",
    expectation: "管理宿主工作区打开、切换与持久化",
    missing: "storageDomain / sessionPersistence 官方 seam 与 host.workspace.* 的控制器语义（Amitia 只有 project-create/project-update/project-delete 与 host.workspace.read/write）",
  });
}


function fsTarget(method, value, options) {
  const source = value && typeof value === "object" ? value : {};
  const path = text(typeof value === "object" ? source.path : value);
  if (!path) throw new Error(`DSH ${method} 需要 path`);
  const sessionId = text(options?.sessionId ?? source.sessionId);
  return { path, sessionId };
}

function mountPathParts(method, path, label) {
  const parts = text(path).replace(/\\/g, "/").split("/").filter(Boolean);
  if (parts.length === 0) throw new Error(`DSH ${method} 需要 ${label} 文件路径: ${JSON.stringify(path ?? null)}`);
  return parts;
}

function fsSessionId(method, target) {
  if (!target.sessionId) {
    throw new Error(`DSH ${method} 需要 sessionId：Amitia host.workspace.read 的会话文件操作按 sessionId 解析工作区，请用 fs.resolve(path, { sessionId }) 生成目标`);
  }
  return target.sessionId;
}

function createFsService(call) {
  const read = (operation, payload) => call("host.workspace.read", { operation, payload });
  const write = (operation, payload) => call("host.workspace.write", { operation, payload });
  const sessionRootUri = async (sessionId) => {
    const response = await read("conversation_workspace", { conversationId: sessionId });
    const rootUri = text(response?.rootUri);
    if (!rootUri) throw new Error(`Amitia host.workspace.read conversation_workspace 未返回 rootUri: ${JSON.stringify(response)}`);
    return rootUri;
  };
  const absoluteUri = (rootUri, path) => {
    const relative = text(path).replace(/^[/\\]+/, "");
    return relative ? `${rootUri.replace(/\/+$/, "")}/${relative}` : rootUri;
  };
  const readBytes = (target, offset, length) => read("file_read_bytes", {
    sessionId: fsSessionId("fs.readBytes", target),
    path: target.path,
    ...(offset === undefined ? {} : { offset }),
    ...(length === undefined ? {} : { length }),
  });
  return {
    resolve(value, options = {}) {
      return fsTarget("fs.resolve", value, options);
    },
    processPath(value) {
      return text(value && typeof value === "object" ? value.path : value);
    },
    processPathFromHostPath(value) {
      return text(value) || undefined;
    },
    fileUrl(value) {
      const path = text(value && typeof value === "object" ? value.path : value);
      if (!path) return undefined;
      return path.startsWith("file://") ? path : `file://${path.startsWith("/") ? "" : "/"}${path}`;
    },
    contains(parent, child) {
      const base = text(parent && typeof parent === "object" ? parent.path : parent).replace(/\/+$/, "");
      const target = text(child && typeof child === "object" ? child.path : child);
      if (!base || !target) return false;
      return target === base || target.startsWith(`${base}/`);
    },
    async stat(target) {
      const resolved = fsTarget("fs.stat", target);
      return read("file_stat", { sessionId: fsSessionId("fs.stat", resolved), path: resolved.path });
    },
    async lstat(value, options = {}) {
      const resolved = fsTarget("fs.lstat", value, options);
      return read("file_stat", { sessionId: fsSessionId("fs.lstat", resolved), path: resolved.path });
    },
    async readText(target) {
      const resolved = fsTarget("fs.readText", target);
      return read("file_read_text", { sessionId: fsSessionId("fs.readText", resolved), path: resolved.path });
    },
    async readBytes(target, _signal, maxBytes) {
      const resolved = fsTarget("fs.readBytes", target);
      return readBytes(resolved, undefined, Number(maxBytes) > 0 ? Number(maxBytes) : undefined);
    },
    async readByteRange(target, range = {}) {
      const resolved = fsTarget("fs.readByteRange", target);
      const offset = Number(range?.offset);
      const length = Number(range?.length);
      if (!Number.isFinite(offset) || offset < 0) throw new Error("DSH fs.readByteRange 需要非负 offset");
      if (!Number.isFinite(length) || length <= 0) throw new Error("DSH fs.readByteRange 需要正 length");
      return readBytes(resolved, offset, length);
    },
    async listDir(target) {
      const resolved = fsTarget("fs.listDir", target);
      return read("file_list", { sessionId: fsSessionId("fs.listDir", resolved), path: resolved.path });
    },
    async writeText(target, content, expected) {
      const resolved = fsTarget("fs.writeText", target);
      const rootUri = await sessionRootUri(fsSessionId("fs.writeText", resolved));
      return write("write_file", {
        uri: absoluteUri(rootUri, resolved.path),
        content: typeof content === "string" ? content : String(content ?? ""),
        overwrite: expected?.overwrite === true,
        atomic: true,
      });
    },
    async deleteFile(target) {
      const resolved = fsTarget("fs.deleteFile", target);
      const rootUri = await sessionRootUri(fsSessionId("fs.deleteFile", resolved));
      return write("delete", { uri: absoluteUri(rootUri, resolved.path), recursive: false });
    },
    watch(target, onChange, options = {}) {
      const resolved = fsTarget("fs.watch", target);
      const sessionId = fsSessionId("fs.watch", resolved);
      const intervalMs = Number(options?.intervalMs) > 0 ? Number(options.intervalMs) : 1000;
      let disposed = false;
      let timer = null;
      let last = "";
      const tick = async () => {
        if (disposed) return;
        let stat = null;
        let failure = null;
        try {
          stat = await read("file_stat", { sessionId, path: resolved.path });
        } catch (error) {
          failure = error;
        }
        if (disposed) return;
        if (failure) {
          if (typeof options?.onError === "function") options.onError(failure);
          else console.error("[original-error]", failure);
        } else {
          const signature = `${stat?.size ?? ""}:${stat?.modifiedAt ?? stat?.mtime ?? stat?.updatedAt ?? ""}`;
          if (last === "") {
            last = signature;
          } else if (signature !== last) {
            last = signature;
            if (typeof onChange === "function") {
              try {
                await onChange({ path: resolved.path, stat });
              } catch (error) {
                if (typeof options?.onError === "function") options.onError(error);
                else console.error("[original-error]", error);
              }
            }
          }
        }
        if (!disposed) timer = setTimeout(tick, intervalMs);
      };
      timer = setTimeout(tick, intervalMs);
      return {
        path: resolved.path,
        dispose() {
          disposed = true;
          if (timer) clearTimeout(timer);
          timer = null;
        },
      };
    },
    async editText(target, edit, expected) {
      const resolved = fsTarget("fs.editText", target);
      const sessionId = fsSessionId("fs.editText", resolved);
      const oldString = typeof edit?.oldString === "string" ? edit.oldString : "";
      const newString = typeof edit?.newString === "string" ? edit.newString : "";
      const replaceAll = edit?.replaceAll === true;
      if (oldString === "") {
        throw new Error(`DSH fs.editText 需要非空 oldString：${JSON.stringify({ path: resolved.path, replaceAll })}`);
      }
      const current = await read("file_read_text", { sessionId, path: resolved.path });
      const rawBefore = typeof current === "string" ? current : current?.content;
      if (typeof rawBefore !== "string") {
        throw new Error(`Amitia file_read_text 没有返回文本内容：${JSON.stringify({ path: resolved.path, payload: current ?? null })}`);
      }
      const before = rawBefore.replace(/\r\n/g, "\n");
      const needle = oldString.replace(/\r\n/g, "\n");
      const occurrences = before.split(needle).length - 1;
      if (occurrences === 0) {
        throw new Error(`fs.editText 未匹配到 oldString：${JSON.stringify({ path: resolved.path, occurrences })}`);
      }
      if (occurrences > 1 && !replaceAll) {
        throw new Error(`fs.editText 匹配到 ${occurrences} 处，replaceAll=false 时要求唯一匹配：${JSON.stringify({ path: resolved.path })}`);
      }
      const after = replaceAll ? before.split(needle).join(newString) : before.replace(needle, () => newString);
      const rootUri = await sessionRootUri(sessionId);
      const written = await write("write_file", {
        uri: absoluteUri(rootUri, resolved.path),
        content: after,
        overwrite: true,
        atomic: true,
      });
      void expected;
      return {
        version: text(written?.version) || text(written?.outcome?.version),
        before,
        after,
      };
    },
    async patchText(target, hunks) {
      const list = Array.isArray(hunks) ? hunks : [];
      const resolved = fsTarget("fs.patchText", target);
      const sessionId = fsSessionId("fs.patchText", resolved);
      if (list.length === 0) {
        throw new Error(`DSH fs.patchText 需要至少一个 hunk：${JSON.stringify({ path: resolved.path })}`);
      }
      const current = await read("file_read_text", { sessionId, path: resolved.path });
      const rawBefore = typeof current === "string" ? current : current?.content;
      if (typeof rawBefore !== "string") {
        throw new Error(`Amitia file_read_text 没有返回文本内容：${JSON.stringify({ path: resolved.path, payload: current ?? null })}`);
      }
      const before = rawBefore.replace(/\r\n/g, "\n");
      const trailingNewline = before.endsWith("\n");
      const lines = before.split("\n");
      if (trailingNewline) lines.pop();
      const ordered = list
        .map((hunk, index) => ({ hunk, index }))
        .sort((left, right) => (Number(right.hunk?.lineNumber) || 0) - (Number(left.hunk?.lineNumber) || 0));
      for (const { hunk, index } of ordered) {
        const oldLines = Array.isArray(hunk?.oldLines) ? hunk.oldLines.map(String) : [];
        const newLines = Array.isArray(hunk?.newLines) ? hunk.newLines.map(String) : [];
        const anchor = Math.max((Number(hunk?.lineNumber) || 1) - 1, 0);
        let position = -1;
        if (oldLines.length === 0) {
          position = Math.min(anchor, lines.length);
        } else {
          let best = -1;
          for (let cursor = 0; cursor <= lines.length - oldLines.length; cursor += 1) {
            let matched = true;
            for (let offset = 0; offset < oldLines.length; offset += 1) {
              if (lines[cursor + offset] !== oldLines[offset]) {
                matched = false;
                break;
              }
            }
            if (!matched) continue;
            if (best === -1 || Math.abs(cursor - anchor) < Math.abs(best - anchor)) best = cursor;
          }
          position = best;
        }
        if (position < 0) {
          throw new Error(`fs.patchText 第 ${index + 1} 个 hunk 未匹配到 oldLines：${JSON.stringify({ path: resolved.path, lineNumber: hunk?.lineNumber ?? null, oldLines })}`);
        }
        lines.splice(position, oldLines.length, ...newLines);
      }
      const after = `${lines.join("\n")}${trailingNewline ? "\n" : ""}`;
      const rootUri = await sessionRootUri(sessionId);
      const written = await write("write_file", {
        uri: absoluteUri(rootUri, resolved.path),
        content: after,
        overwrite: true,
        atomic: true,
      });
      return {
        version: text(written?.version) || text(written?.outcome?.version),
        before,
        after,
      };
    },
    async moveFile(value, destination, options = {}) {
      const source = fsTarget("fs.moveFile", value, options);
      const target = fsTarget("fs.moveFile", destination, options);
      const hunks = Array.isArray(options?.hunks) ? options.hunks : [];
      if (hunks.length > 0) {
        throw hostSeamError({
          method: "fs.moveFile",
          capability: "移动并就地改写文件内容",
          expectation: "按 PatchHunk 在移动时应用补丁",
          missing: "host.workspace.patch（host.workspace.write 的 rename/move 只搬移文件，没有补丁应用操作）",
          request: { source: source.path, destination: target.path, hunks: hunks.length },
        });
      }
      const sourceParts = mountPathParts("fs.moveFile", source.path, "source");
      const targetParts = mountPathParts("fs.moveFile", target.path, "destination");
      const sourceName = sourceParts.at(-1);
      const targetName = targetParts.at(-1);
      const sourceRoot = await sessionRootUri(fsSessionId("fs.moveFile", source));
      const targetRoot = await sessionRootUri(fsSessionId("fs.moveFile", target));
      const sourceDir = sourceParts.slice(0, -1).join("/");
      const targetDir = targetParts.slice(0, -1).join("/");
      if (sourceRoot === targetRoot && sourceDir === targetDir) {
        return write("rename", { uri: absoluteUri(sourceRoot, source.path), newName: targetName });
      }
      const moved = await write("move", {
        sourceUri: absoluteUri(sourceRoot, source.path),
        destinationDirUri: targetDir ? absoluteUri(targetRoot, targetDir) : targetRoot,
      });
      if (sourceName === targetName) return moved;
      return write("rename", { uri: absoluteUri(targetRoot, target.path), newName: targetName });
    },
  };
}

function policyScopeId(value) {
  if (value == null) return "";
  if (typeof value === "object") return text(value.id) || text(value.sessionId) || text(value.session?.id);
  return text(value);
}

function approvalPolicyOf(method, policy) {
  if (!policy || typeof policy !== "object" || Array.isArray(policy)) {
    throw new Error(`DSH ${method} 需要 ApprovalPolicy 对象: ${JSON.stringify(policy ?? null)}`);
  }
  return Object.freeze({ ...policy });
}

function createApprovalService(call) {
  const policies = new Map();
  return {
    async request(request = {}) {
      const callId = text(request?.callId ?? request?.toolCallId);
      const requestId = text(request?.id ?? request?.requestId);
      const conversationId = text(request?.conversationId ?? request?.sessionId);
      const timeoutMs = Number(request?.timeoutMs);
      const riskLevel = text(request?.riskLevel);
      const result = await call("host.approval.request", {
        toolName: text(request?.toolName) || "unknown-tool",
        arguments: approvalPromptText(request),
        toolCallId: callId,
        requestId,
        ...(conversationId ? { conversationId } : {}),
        ...(riskLevel ? { riskLevel } : {}),
        ...(Number.isFinite(timeoutMs) && timeoutMs > 0 ? { timeoutMs: Math.trunc(timeoutMs) } : {}),
      });
      if (result?.approved === true) return "allowed-once";
      if (result?.approved === false) return "rejected";
      throw new Error(`Amitia host.approval.request 返回了非法结果: ${JSON.stringify(result)}`);
    },
    setPolicy(agent, policy) {
      const scopeId = policyScopeId(agent);
      if (!scopeId) throw new Error(`DSH approval.setPolicy 需要带 id 的 agent: ${JSON.stringify(agent ?? null)}`);
      policies.set(scopeId, approvalPolicyOf("approval.setPolicy", policy));
    },
    overrideOf(session) {
      const scopeId = policyScopeId(session);
      if (!scopeId) return undefined;
      return policies.get(scopeId);
    },
  };
}

const DIALOG_PROMPT_KEYS = ["question", "prompt", "message", "title", "text"];
const DIALOG_OPTION_KEYS = ["options", "choices", "buttons"];

function optionLabel(entry) {
  if (typeof entry === "object" && entry !== null) {
    return text(entry.label) || text(entry.title) || text(entry.text) || text(entry.value);
  }
  return text(entry);
}

function questionList(request) {
  return Array.isArray(request?.questions) ? request.questions.filter((entry) => entry && typeof entry === "object") : [];
}

function dialogMessage(request) {
  for (const key of DIALOG_PROMPT_KEYS) {
    const value = text(request?.[key]);
    if (value) return value;
  }
  for (const question of questionList(request)) {
    for (const key of DIALOG_PROMPT_KEYS) {
      const value = text(question?.[key]);
      if (value) return value;
    }
  }
  return "";
}

function dialogButtons(request) {
  for (const key of DIALOG_OPTION_KEYS) {
    const raw = request?.[key];
    if (!Array.isArray(raw)) continue;
    const labels = raw.map(optionLabel).filter(Boolean);
    if (labels.length > 0) return labels;
  }
  for (const question of questionList(request)) {
    for (const key of DIALOG_OPTION_KEYS) {
      const raw = question?.[key];
      if (!Array.isArray(raw)) continue;
      const labels = raw.map(optionLabel).filter(Boolean);
      if (labels.length > 0) return labels;
    }
  }
  return [];
}

function dialogRequest(method, request, fallbackMessage) {
  const message = dialogMessage(request) || text(fallbackMessage);
  if (!message) throw new Error(`DSH ${method} 需要 question/prompt/message 文本: ${JSON.stringify(request ?? null)}`);
  const buttons = dialogButtons(request);
  const dialogId = text(request?.dialogId) || text(request?.id) || text(request?.requestId) || text(request?.callId);
  return {
    ...(dialogId ? { dialogId } : {}),
    message,
    ...(buttons.length > 0 ? { buttons } : {}),
  };
}

function createUserQuestionsService(call) {
  const ask = async (request = {}) => {
    const payload = dialogRequest("userQuestions.ask", request);
    const result = await call("host.ui.dialog", payload);
    const selected = text(result?.result);
    if (!selected) throw new Error(`Amitia host.ui.dialog 返回了非法结果: ${JSON.stringify(result ?? null)}`);
    return {
      question: payload.message,
      selected,
      dialogId: text(result?.dialogId) || text(payload.dialogId),
    };
  };
  return {
    ask,
    async askTimed(request = {}, callId, timeoutMs) {
      const dialogId = text(callId) || text(request?.dialogId) || text(request?.id) || text(request?.requestId);
      const pending = ask({ ...request, ...(dialogId ? { dialogId } : {}) });
      const budget = Number(timeoutMs);
      if (!Number.isFinite(budget) || budget <= 0) return { status: "answered", answer: await pending };
      const expired = Symbol("dsh-user-question-timeout");
      let timer;
      const deadline = new Promise((resolve) => {
        timer = setTimeout(() => resolve(expired), budget);
        if (typeof timer?.unref === "function") timer.unref();
      });
      try {
        const outcome = await Promise.race([pending, deadline]);
        if (outcome === expired) {
          pending.catch(logOriginalError);
          return { status: "timeout" };
        }
        return { status: "answered", answer: outcome };
      } finally {
        clearTimeout(timer);
      }
    },
  };
}

function scheduleIdOf(method, request) {
  const scheduleId = text(request?.scheduleId ?? request?.id);
  if (!scheduleId) throw new Error(`DSH ${method} 需要 scheduleId`);
  return scheduleId;
}

function createScheduleService(call) {
  const catalog = async () => mapDshScheduleCatalog(await call("host.schedule.list", {}));
  const service = {
    deliveryLimits() {
      return { newSessionIntervalSeconds: 60 };
    },
    async create(request, _actorSessionId, signal) {
      signal?.throwIfAborted?.();
      const definition = createDshScheduleDefinition(request);
      await call("host.schedule.create", definition);
      const items = await catalog();
      const entry = items.find((item) => item.id === definition.scheduleId);
      if (!entry) throw new Error(`Amitia host.schedule.create did not return schedule ${definition.scheduleId}`);
      return entry;
    },
    async list(request = {}) {
      const all = await catalog();
      const filtered = request.active === undefined
        ? all
        : all.filter((item) => (item.status === "active") === Boolean(request.active));
      const limit = Math.min(500, Math.max(1, Number(request.limit) || 100));
      const offset = Math.max(0, Number(request.offset) || 0);
      const items = filtered.slice(offset, offset + limit);
      return { items, shown: items.length, total: filtered.length, limit, offset };
    },
    async cancel(request = {}) {
      const scheduleId = scheduleIdOf("schedule.cancel", request);
      try {
        await call("host.schedule.cancel", {
          scheduleId,
          expectedUpdatedAt: request.expectedUpdatedAt,
        });
      } catch (error) {
        if (error?.code === "resource_not_found") return { id: scheduleId, deleted: false, code: "schedule_not_found" };
        if (error?.code === "state_conflict") return { id: scheduleId, code: "schedule_conflict" };
        throw error;
      }
      return { id: scheduleId, deleted: true };
    },
    async run(request = {}) {
      const scheduleId = scheduleIdOf("schedule.run", request);
      await call("host.schedule.run", { scheduleId });
      return (await catalog()).find((item) => item.id === scheduleId) || { id: scheduleId, code: "schedule_not_found" };
    },
    async catalog() {
      return catalog();
    },
    async view(request = {}) {
      const scheduleId = scheduleIdOf("schedule.view", request);
      const items = await catalog();
      return items.find((item) => item.id === scheduleId) || { id: scheduleId, code: "schedule_not_found" };
    },
    async history(request = {}) {
      const scheduleId = scheduleIdOf("schedule.history", request);
      try {
        return await call("host.schedule.history", {
          scheduleId,
          ...(request.limit === undefined ? {} : { limit: request.limit }),
          ...(text(request.before) ? { before: text(request.before) } : {}),
        });
      } catch (error) {
        if (error?.code === "resource_not_found") return { id: scheduleId, code: "schedule_not_found" };
        throw error;
      }
    },
    async edit(request = {}) {
      const scheduleId = scheduleIdOf("schedule.edit", request);
      const raw = await call("host.schedule.list", {});
      const definition = (raw?.schedules || []).find((item) => item.scheduleId === scheduleId);
      const state = raw?.states?.[scheduleId];
      if (!definition || !state) return { id: scheduleId, code: "schedule_not_found" };
      const expected = request.expectedUpdatedAt === undefined ? state.updatedAt : request.expectedUpdatedAt;
      if (expected !== state.updatedAt) return { id: scheduleId, code: "schedule_conflict" };
      const updated = updateDshScheduleDefinition(definition, request.patch);
      try {
        await call("host.schedule.update", {
          scheduleId,
          expectedUpdatedAt: expected,
          definition: updated.definition,
          enabled: updated.enabled,
        });
      } catch (error) {
        if (error?.code === "state_conflict") return { id: scheduleId, code: "schedule_conflict" };
        throw error;
      }
      const items = await catalog();
      return items.find((item) => item.id === scheduleId) || { id: scheduleId, code: "schedule_not_found" };
    },
    async deleteInactive() {
      const ended = (await catalog()).filter((item) => item.status === "ended");
      for (const item of ended) await call("host.schedule.cancel", { scheduleId: item.id });
      return { deleted: ended.length };
    },
  };
  service.remoteList = (request) => service.list(request);
  service.remoteHistory = (request) => service.history(request);
  service.remoteEdit = (request) => service.edit(request);
  return service;
}

function createLlmService(call) {
  const adapters = new Map();
  const configurableProviders = [];
  const modelDiscoveries = new Map();
  const adapterOf = (method, provider, request) => {
    const id = text(provider);
    if (!id) throw new Error(`DSH llm.${method} 需要 provider: ${JSON.stringify(request ?? null)}`);
    const adapter = adapters.get(id);
    if (!adapter) {
      throw hostSeamError({
        method: `llm.${method}`,
        capability: `provider ${id} 的模型适配器`,
        expectation: "由 llm.registerAdapter 注册的适配器执行",
        missing: `host.model.catalog 中 provider ${id} 的适配器（当前已注册 ${adapters.size} 个 provider）`,
        request: request ?? { provider: id },
      });
    }
    return adapter;
  };
  const delegate = (method, adapterMethod, provider, args, request) => {
    const adapter = adapterOf(method, provider, request);
    const target = adapter[adapterMethod];
    if (typeof target !== "function") {
      throw hostSeamError({
        method: `llm.${method}`,
        capability: `provider ${text(provider)} 适配器的 ${adapterMethod}`,
        expectation: `适配器实现 ${adapterMethod}`,
        missing: `host.model.catalog 中 provider ${text(provider)} 适配器的 ${adapterMethod}（已注册适配器没有该方法）`,
        request: request ?? { provider: text(provider) },
      });
    }
    return target.apply(adapter, args);
  };
  return {
    invoke(input) {
      return call("host.provider.invoke", input);
    },
    registerAdapter(providers, adapter) {
      const list = (Array.isArray(providers) ? providers : [providers]).map(text).filter(Boolean);
      if (list.length === 0) throw new Error(`DSH llm.registerAdapter 需要 provider 列表: ${JSON.stringify(providers ?? null)}`);
      if (!adapter || typeof adapter !== "object") throw new Error(`DSH llm.registerAdapter 需要 adapter 对象: ${JSON.stringify(adapter ?? null)}`);
      for (const id of list) {
        if (adapters.has(id)) throw new Error(`DSH llm provider ${id} 已有适配器注册`);
      }
      let current = adapter;
      for (const id of list) adapters.set(id, current);
      let active = true;
      const remove = () => {
        if (!active) return;
        active = false;
        for (const id of list) if (adapters.get(id) === current) adapters.delete(id);
      };
      const handle = () => remove();
      handle.dispose = remove;
      handle.replace = (next) => {
        if (!next || typeof next !== "object") throw new Error("DSH llm AdapterRegistrationHandle.replace 需要 adapter 对象");
        for (const id of list) if (adapters.get(id) === current) adapters.set(id, next);
        current = next;
      };
      return handle;
    },
    listProviders() {
      return Array.from(adapters.entries()).map(([id, adapter]) => {
        const info = typeof adapter.providerInfo === "function" ? adapter.providerInfo(id) : undefined;
        return info && typeof info === "object" ? { id, ...info } : { id, name: id };
      });
    },
    listModels(provider) {
      return delegate("listModels", "listModels", provider, [provider], { provider: text(provider) });
    },
    resolveModelInfo(provider, model, signal) {
      return delegate("resolveModelInfo", "resolveModel", provider, [provider, model, signal], { provider: text(provider), model: text(model) });
    },
    prepareCall(config = {}, signal) {
      const provider = text(config?.provider);
      const model = text(config?.model);
      return delegate("prepareCall", "prepareCall", provider, [provider, model, signal], { provider, model });
    },
    stream(config = {}, signal) {
      const provider = text(config?.provider);
      const options = { ...config, signal };
      return delegate("stream", "stream", provider, [options], { provider, model: text(config?.model) });
    },
    providerRetryPolicy(provider) {
      return delegate("providerRetryPolicy", "providerRetryPolicy", provider, [provider], { provider: text(provider) });
    },
    imageRequestPricing(provider, model) {
      return delegate("imageRequestPricing", "imageRequestPricing", provider, [provider, model], { provider: text(provider), model: text(model) });
    },
    imageRequestTarget(provider, model, image) {
      return delegate("imageRequestTarget", "imageRequestTarget", provider, [provider, model, image], { provider: text(provider), model: text(model) });
    },
    registerConfigurableProviders(entries) {
      const list = Array.isArray(entries) ? entries : [];
      if (list.length === 0) throw new Error(`DSH llm.registerConfigurableProviders 需要 provider 列表: ${JSON.stringify(entries ?? null)}`);
      for (const entry of list) {
        if (!entry || typeof entry !== "object") throw new Error(`DSH llm.registerConfigurableProviders 的条目必须是对象: ${JSON.stringify(entry ?? null)}`);
      }
      configurableProviders.push(...list);
      let active = true;
      const remove = () => {
        if (!active) return;
        active = false;
        for (const entry of list) {
          const index = configurableProviders.indexOf(entry);
          if (index >= 0) configurableProviders.splice(index, 1);
        }
      };
      const handle = () => remove();
      handle.dispose = remove;
      return handle;
    },
    listConfigurableProviders() {
      return configurableProviders.slice();
    },
    registerModelDiscovery(settingsNs, discover) {
      const key = text(settingsNs);
      if (!key) throw new Error(`DSH llm.registerModelDiscovery 需要 settingsNs: ${JSON.stringify(settingsNs ?? null)}`);
      if (typeof discover !== "function") throw new Error(`DSH llm.registerModelDiscovery 需要 discover 函数: ${JSON.stringify(typeof discover)}`);
      if (modelDiscoveries.has(key)) throw new Error(`DSH llm model discovery ${key} 已注册`);
      modelDiscoveries.set(key, discover);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (modelDiscoveries.get(key) === discover) modelDiscoveries.delete(key);
      };
    },
    discoverModels(settingsNs, request, signal) {
      const key = text(settingsNs);
      const discover = modelDiscoveries.get(key);
      if (!discover) {
        throw hostSeamError({
          method: "llm.discoverModels",
          capability: "远端模型发现",
          expectation: "由 registerModelDiscovery 注册的发现器执行",
          missing: `host.model.discover 中 settingsNs=${key || "<empty>"} 的发现器（当前已注册 ${modelDiscoveries.size} 个）`,
          request: { settingsNs: key, request },
        });
      }
      return discover(request, signal);
    },
    async resolveCallConfig(config = {}) {
      const requested = config && typeof config === "object" ? config : {};
      const defaults = await call("host.model.defaults", {});
      const temperature = typeof requested.temperature === "number"
        ? requested.temperature
        : (typeof defaults?.temperature === "number" ? defaults.temperature : undefined);
      const maxOutputTokens = Number(requested.maxOutputTokens) > 0
        ? Number(requested.maxOutputTokens)
        : (Number(defaults?.maxOutputTokens) > 0 ? Number(defaults.maxOutputTokens) : (Number(defaults?.maxTokens) > 0 ? Number(defaults.maxTokens) : undefined));
      const contextWindow = Number(requested.contextWindow) > 0
        ? Number(requested.contextWindow)
        : (Number(defaults?.contextWindow) > 0 ? Number(defaults.contextWindow) : undefined);
      return {
        ...requested,
        provider: text(requested.provider) || text(defaults?.provider),
        model: text(requested.model) || text(defaults?.model),
        ...(temperature === undefined ? {} : { temperature }),
        ...(maxOutputTokens === undefined ? {} : { maxOutputTokens }),
        ...(contextWindow === undefined ? {} : { contextWindow }),
        reasoningEffort: text(requested.reasoningEffort) || text(defaults?.reasoningEffort) || undefined,
      };
    },
    fileRequestText: seamThrower({
      method: "llm.fileRequestText",
      capability: "解析文件附件引用文本",
      expectation: "返回 FileAttachmentRef 的文本形式",
      missing: "host.resource.stat 的附件引用文本（Amitia 资源只有句柄与 URI，没有 DSH FileAttachmentRef 的展示文本）",
    }),
  };
}

function createSubagentUnavailableMethods() {
  return {
    prompt: seamThrower({
      method: "subagents.prompt",
      capability: "向子代理会话投递提示",
      expectation: "返回 SubagentPromptReceipt 并带 resources.session",
      missing: "host.conversation.message.append 的子代理发送者语义（Amitia 追加接口没有 DSH SubagentPromptRequest 的 callId/资源映射）",
    }),
    resolveName: seamThrower({
      method: "subagents.resolveName",
      capability: "按名字解析子代理会话",
      expectation: "把 caller 下的名字解析成 SessionId",
      missing: "host.session.subagent.names（Amitia 会话目录没有子代理命名表）",
    }),
  };
}

function createAgentUnavailableMethods() {
  return {
    setFactory: seamThrower({
      method: "agents.setFactory",
      capability: "替换 agent 工厂",
      expectation: "之后 agents.create 使用该工厂驱动",
      missing: "host.agent.factory（容器 create/resume 已绑定 Amitia 会话创建，没有可替换的 agent 工厂接口）",
    }),
    announce: seamThrower({
      method: "agents.announce",
      capability: "广播会话启动",
      expectation: "向监听者发出 source 对应的会话启动事件",
      missing: "host.event.emit 的 session-start 语义（Amitia 事件总线没有 DSH SessionStartSource 映射）",
    }),
  };
}

function createSessionUnavailableMethods() {
  return {
    fork: seamThrower({
      method: "sessions.fork",
      capability: "按边界派生会话",
      expectation: "返回继承事件的新 Session",
      missing: "host.conversation.fork（Amitia 会话目录没有派生接口，也没有 SessionSeq 边界继承）",
    }),
    prepare: seamThrower({
      method: "sessions.prepare",
      capability: "预分配未公布会话",
      expectation: "返回尚未进入会话目录的 Session",
      missing: "host.conversation.prepare（host.conversation.create 立即写入会话目录，没有预分配语义）",
    }),
    enter: seamThrower({
      method: "sessions.enter",
      capability: "进入会话作用域",
      expectation: "返回退出作用域的释放函数",
      missing: "DSH 会话作用域（容器没有 ctx.session 作用域实现）",
    }),
    announce: seamThrower({
      method: "sessions.announce",
      capability: "公布会话",
      expectation: "把会话写入目录并通知监听者",
      missing: "host.conversation.announce（会话创建即公布，没有独立公布阶段）",
    }),
    registerMessageProjection: seamThrower({
      method: "sessions.registerMessageProjection",
      capability: "注册消息投影",
      expectation: "注册后会话消息按该投影折叠",
      missing: "host.conversation.projection（Amitia 会话消息没有投影注册接口）",
    }),
  };
}

export function createAmitiaDshHostServices(host, promptSections = [], promptState = {}) {
  if (!host || typeof host.call !== "function") throw new Error("Amitia host API is unavailable");
  const call = (method, input = {}) => host.call(method, input);
  const jobs = new Map();
  const agents = new Map();
  const initiatorStack = [];
  const subagentProviders = new Map();
  const subagentRuns = new Map();
  const mcpResourceProviders = new Map();
  const sessionProjections = new Map();
  const activePromptState = promptState.sections
    ? promptState
    : {
        sections: promptSections,
        contexts: [],
        variables: new Map(),
        toolProviders: [],
      };
  const promptContexts = activePromptState.contexts;
  const promptVariables = activePromptState.variables;
  const promptToolProviders = activePromptState.toolProviders;
  const sections = managedCollection(promptSections, "section");
  const contexts = managedCollection(promptContexts, "context");

  const createSessionBridge = (conversationId) => {
    const events = [];
    const key = `dsh-session:${conversationId}`;
    let targetScopeSnapshotId = "";
    let persistenceError = null;
    let persistenceTail = Promise.resolve();
    const adopt = (value) => {
      if (value === undefined || value === null) return;
      assertSessionStateVersion(value, conversationId);
      const restored = restoreSessionEvents(value.events, conversationId);
      mergeSessionEvents(events, restored.events);
      if (typeof value.scopeSnapshotId === "string") targetScopeSnapshotId = value.scopeSnapshotId;
    };
    const hydrated = call("host.state.get", { key }).then((response) => {
      if (response?.found) adopt(response.value);
    }, (error) => {
      persistenceError = error;
    }).catch((error) => {
      persistenceError = error;
      throw error;
    });
    hydrated.catch(logOriginalError);
    const persist = () => {
      persistenceTail = persistenceTail.then(hydrated).then(async () => {
        for (let attempt = 0; attempt < 8; attempt += 1) {
          const response = await call("host.state.get", { key });
          if (response?.found) adopt(response.value);
          assertSessionEventBudget(conversationId, events.length);
          const next = {
            version: SESSION_STATE_VERSION,
            conversationId,
            scopeSnapshotId: targetScopeSnapshotId,
            events: events.slice(),
            updatedAt: new Date().toISOString(),
          };
          const result = await call("host.state.cas", {
            key,
            expectedVersion: Number(response?.version) || 0,
            newValue: next,
          });
          if (result?.swapped) {
            persistenceError = null;
            return;
          }
        }
        throw new Error(`DSH session ${conversationId} state write conflict`);
      }).catch((error) => {
        persistenceError = error;
        throw error;
      });
      persistenceTail.catch(logOriginalError);
      return persistenceTail;
    };
    return {
      events,
      append(type, data, options = {}) {
        if (persistenceError) throw persistenceError;
        assertSessionEventBudget(conversationId, events.length + 1);
        const event = createSessionEvent(type, data, {
          seq: nextSessionEventSeq(events),
          time: Date.now(),
          ignorable: options.ignorable,
          surfaceOp: options.surfaceOp,
          sourceEventSeqs: options.sourceEventSeqs,
        });
        if (isSurfaceEventType(event.type)) assertSurfaceReplacement(projectSurfaceEvents(events), event);
        events.push(event);
        persist().catch(logOriginalError);
        return event;
      },
      surfaceEvents() {
        return projectSurfaceEvents(events);
      },
      getSnapshot() {
        return { id: conversationId, version: SESSION_STATE_VERSION, events: events.slice() };
      },
      projection(projections) {
        const result = {};
        for (const definition of projections ?? []) {
          let state = definition.init?.({}, 0);
          for (const event of events) state = definition.apply?.(state, event) ?? state;
          result[definition.key] = state;
        }
        return result;
      },
      get hydrated() {
        return hydrated;
      },
      get persistenceError() {
        return persistenceError;
      },
      whenPersisted() {
        return persistenceTail;
      },
      get scopeSnapshotId() {
        return targetScopeSnapshotId;
      },
      setScopeSnapshotId(value) {
        targetScopeSnapshotId = text(value);
        return persist();
      },
    };
  };

  const agentFacade = (sessionId) => {
    const conversationId = text(sessionId);
    if (!conversationId) return undefined;
    if (agents.has(conversationId)) return agents.get(conversationId);
    const sessionBridge = createSessionBridge(conversationId);
    const inbox = {
      nextStep: [],
      nextTurn: [],
      insert(target, message) {
        const list = target === "next-step" ? inbox.nextStep : inbox.nextTurn;
        list.push(message);
      },
      remove(messageId) {
        for (const list of [inbox.nextStep, inbox.nextTurn]) {
          const index = list.findIndex((message) => message?.id === messageId);
          if (index >= 0) {
            list.splice(index, 1);
            return true;
          }
        }
        return false;
      },
      clear() {
        inbox.nextStep.length = 0;
        inbox.nextTurn.length = 0;
      },
    };
    const session = {
      id: conversationId,
      get events() {
        return sessionBridge.events.slice();
      },
      append: sessionBridge.append,
      surfaceEvents: sessionBridge.surfaceEvents,
      getSnapshot: sessionBridge.getSnapshot,
      projection: sessionBridge.projection,
      get hydrated() {
        return sessionBridge.hydrated;
      },
      get persistenceError() {
        return sessionBridge.persistenceError;
      },
      whenPersisted: sessionBridge.whenPersisted,
      get scopeSnapshotId() {
        return sessionBridge.scopeSnapshotId;
      },
      setScopeSnapshotId: sessionBridge.setScopeSnapshotId,
    };
    const callTarget = async (method, input = {}) => {
      await sessionBridge.hydrated;
      if (sessionBridge.persistenceError) throw sessionBridge.persistenceError;
      if (sessionBridge.scopeSnapshotId && typeof host.callTargeted === "function") {
        return host.callTargeted(method, input, sessionBridge.scopeSnapshotId);
      }
      return call(method, input);
    };
    let modelSelection;
    sessionBridge.hydrated.then(() => {
      for (let index = sessionBridge.events.length - 1; index >= 0; index -= 1) {
        const event = sessionBridge.events[index];
        if (event?.type !== "model/selection") continue;
        modelSelection = Object.freeze({ ...event.data });
        break;
      }
    }, logOriginalError);
    const selectModel = (selection) => {
      if (!selection || typeof selection !== "object") throw new Error("DSH agent.model.select selection is required");
      const provider = text(selection.provider);
      const model = text(selection.model);
      if (!provider || !model) throw new Error("DSH agent.model.select requires provider and model");
      const normalized = {
        provider,
        model,
        ...(text(selection.reasoningEffort) ? { reasoningEffort: text(selection.reasoningEffort) } : {}),
      };
      if (modelSelection && modelSelection.provider === normalized.provider && modelSelection.model === normalized.model && modelSelection.reasoningEffort === normalized.reasoningEffort) {
        return;
      }
      modelSelection = Object.freeze(normalized);
      sessionBridge.append("model/selection", normalized);
    };
    let maintenanceController = null;
    let maintenancePromise = null;
    const runMaintenance = (task) => {
      if (typeof task !== "function") throw new Error("DSH agent.runMaintenance task is required");
      if (maintenancePromise) throw new Error(`DSH agent ${conversationId} already owns a maintenance task`);
      const controller = new AbortController();
      maintenanceController = controller;
      const pending = Promise.resolve().then(() => task(controller.signal));
      const final = pending.finally(() => {
        if (maintenanceController === controller) maintenanceController = null;
        if (maintenancePromise === final) maintenancePromise = null;
      });
      maintenancePromise = final;
      return maintenancePromise;
    };
    const cancelAgent = async (cause = { kind: "user" }, options = {}) => {
      const hadMaintenance = Boolean(maintenanceController);
      maintenanceController?.abort(cause);
      let interruptError = null;
      try {
        const result = await callTarget("host.conversation.turn.interrupt", {
          conversationId,
          cause,
          keepInbox: options.keepInbox === true,
        });
        if (result?.interrupted !== true && !hadMaintenance) {
          const error = new Error(`DSH agent.cancel did not interrupt conversation ${conversationId}`);
          error.code = result?.code;
          interruptError = error;
        }
      } catch (error) {
        const noActive = /state_conflict:.*no active turn|conversation .* has no active turn|not the active turn|no longer interruptible/i.test(error?.message);
        if (!hadMaintenance && !noActive) interruptError = error;
      }
      if (interruptError) throw interruptError;
    };
    const deliver = (method, message) => {
      if (method === "host.conversation.message.append") {
        return callTarget(method, {
          conversationId,
          role: "user",
          parts: messageParts(message),
          source: "dsh-plugin",
        });
      }
      return callTarget(method, {
        conversationId,
        content: messageText(message),
        source: "dsh-plugin",
      });
    };
    const facade = {
      id: conversationId,
      options: {
        get provider() {
          return modelSelection?.provider;
        },
        get model() {
          return modelSelection?.model;
        },
        get reasoningEffort() {
          return modelSelection?.reasoningEffort;
        },
      },
      get status() {
        return "idle";
      },
      session,
      inbox,
      send(message, target, wakeup = true) {
        return wakeup
          ? deliver(target === "next-step" ? "host.conversation.message.append" : "host.conversation.message.send", message)
          : deliver("host.conversation.message.append", message);
      },
      followup(message) {
        return deliver("host.conversation.message.send", message);
      },
      steer(message) {
        return deliver("host.conversation.message.append", message);
      },
      inject(message) {
        return deliver("host.conversation.message.append", message);
      },
      async whenIdle() {
        for (;;) {
          const result = await callTarget("host.conversation.turn.await", {
            conversationId,
          });
          if (result?.idle !== true) {
            throw new Error(`DSH agent.whenIdle did not observe an idle conversation ${conversationId}`);
          }
          const pending = maintenancePromise;
          if (!pending) return;
          await pending;
        }
      },
      cancel: cancelAgent,
      runMaintenance,
      model: {
        current() {
          return modelSelection;
        },
        select(selection) {
          selectModel(selection);
        },
      },
    };
    agents.set(conversationId, facade);
    return facade;
  };

  const mcpResources = {
    register(server, provider) {
      const name = text(server);
      if (!name) throw new Error("DSH mcpResources.register server is required");
      if (!provider || typeof provider.request !== "function") {
        throw new Error(`DSH mcpResources.register ${name} provider.request must be a function`);
      }
      if (mcpResourceProviders.has(name)) throw new Error(`DSH mcpResources ${name} is already registered`);
      mcpResourceProviders.set(name, provider);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (mcpResourceProviders.get(name) === provider) mcpResourceProviders.delete(name);
      };
    },
    async request(server, request, execution = {}) {
      const name = text(server);
      const provider = mcpResourceProviders.get(name);
      if (!provider) throw new Error(`DSH mcpResources ${name || "<empty>"} is not registered`);
      return provider.request(request, execution);
    },
    list() {
      return Array.from(mcpResourceProviders.keys()).sort();
    },
  };

  const sessionProjectionService = {
    register(definition) {
      const key = text(definition?.key);
      if (!key) throw new Error("DSH sessionProjections.register key is required");
      if (sessionProjections.has(key)) throw new Error(`DSH session projection ${key} is already registered`);
      sessionProjections.set(key, definition);
      let active = true;
      const dispose = () => {
        if (!active) return;
        active = false;
        if (sessionProjections.get(key) === definition) sessionProjections.delete(key);
      };
      dispose.stateOf = (session) => {
        const events = Array.isArray(session?.events) ? session.events : session?.getSnapshot?.()?.events ?? [];
        let state = definition.init?.({}, 0);
        for (const event of events) state = definition.apply?.(state, event) ?? state;
        return state;
      };
      dispose.dispose = dispose;
      return dispose;
    },
    get(key) {
      return sessionProjections.get(text(key));
    },
    list() {
      return Array.from(sessionProjections.values());
    },
  };

  const cleanupCreatedAgent = async (sessionId) => {
    const errors = [];
    try {
      await call("host.state.delete", { key: `dsh-session:${sessionId}` });
    } catch (error) {
      errors.push(error);
    }
    try {
      await call("host.conversation.delete", {
        conversationId: sessionId,
        requestId: `dsh-agent-delete:${sessionId}`,
      });
    } catch (error) {
      errors.push(error);
    }
    if (agents.get(sessionId)?.id === sessionId) agents.delete(sessionId);
    if (errors.length === 1) throw errors[0];
    if (errors.length > 1) {
      throw new AggregateError(errors, `DSH agent ${sessionId} cleanup failed`, { cause: errors[0] });
    }
  };

  const normalizeAgentOptions = (operation, options = {}) => {
    if (typeof options.setup === "function") {
      throw unsupportedOperation(operation, "setup requires a DSH scoped Agent context");
    }
    const agentOptions = options.agentOptions && typeof options.agentOptions === "object" ? options.agentOptions : {};
    if (agentOptions.maxTokens !== undefined) {
      throw unsupportedOperation(operation, "agentOptions.maxTokens has no Amitia Agent route mapping");
    }
    const provider = text(agentOptions.provider);
    const model = text(agentOptions.model);
    const reasoningEffort = text(agentOptions.reasoningEffort);
    if ((provider || model) && (!provider || !model)) {
      throw new Error(`DSH ${operation} agentOptions requires both provider and model`);
    }
    if (reasoningEffort && (!provider || !model)) {
      throw new Error(`DSH ${operation} reasoningEffort requires provider and model`);
    }
    return { provider, model, reasoningEffort };
  };

  const createAgentHandle = (agent, sessionId) => {
    let disposePromise;
    return {
      agent,
      dispose() {
        if (disposePromise) return disposePromise;
        disposePromise = (async () => {
          try {
            await agent.cancel({ kind: "disposed" }, { keepInbox: false });
            await agent.whenIdle();
          } finally {
            if (agents.get(sessionId) === agent) agents.delete(sessionId);
          }
        })();
        return disposePromise;
      },
    };
  };

  const userQuestions = createUserQuestionsService(call);

  const workingDirectory = {
    async ensure() {
      return process.cwd();
    },
  };

  const fs = createFsService(call);

  const schedule = createScheduleService(call);

  const sessionView = async (sessionId) => {
    const live = agents.get(sessionId);
    if (live) {
      return {
        live: true,
        events: live.session.events,
        surfaceEvents: live.session.surfaceEvents(),
      };
    }
    const state = await call("host.state.get", { key: `dsh-session:${sessionId}` });
    if (!state?.found) return { live: false, events: [], surfaceEvents: [] };
    assertSessionStateVersion(state.value, sessionId);
    const restored = restoreSessionEvents(state.value.events, sessionId).events;
    return {
      live: false,
      events: restored,
      surfaceEvents: projectSurfaceEvents(restored),
    };
  };
  const credentialsService = createCredentialsService(call);
  const commandsService = createCommandsService(call);
  const sessionQueryService = {
    ...createSessionQueryService({
      call,
      isLive: (sessionId) => agents.has(sessionId),
      sessionView,
    }),
    async searchSessions(request = {}, exec = {}) {
      const query = text(request?.query);
      if (!query) throw new Error(`DSH sessionQuery.searchSessions 需要 query: ${JSON.stringify(request ?? null)}`);
      exec?.signal?.throwIfAborted?.();
      const result = await call("host.conversation.search", {
        query,
        ...(request.limit === undefined ? {} : { limit: request.limit }),
      });
      const items = Array.isArray(result?.items) ? result.items : [];
      return {
        hits: items.map((item) => {
          const record = item && typeof item === "object" ? item : { content: text(item) };
          return { ...record, sessionId: text(record.sessionId) || text(record.id) || text(record.conversationId) };
        }),
        hasMore: result?.hasMore === true,
      };
    },
  };

  return {
    systemPrompt: {
      getSectionOrder(name) {
        const orders = {
          HARNESS_IDENTITY: -1000,
          DEPLOYMENT_PERSONA_PREFIX: 0,
          PLAN_POLICY: 500,
          TEAM_POLICY: 600,
          PTC_ONLY: 800,
          FILE_REFERENCE: 900,
          TOOL_BASH: 1000,
          TOOL_PWSH: 1010,
          TOOL_READ: 1100,
          TOOL_WRITE: 1200,
          TOOL_EDIT: 1300,
          TOOL_APPLY_PATCH: 1350,
          TOOL_GLOB: 1400,
          TOOL_GREP: 1500,
          TOOL_JOBS: 1600,
          TOOL_PTY: 1700,
          TOOL_PTY_PANEL: 1710,
          TOOL_WEB_SEARCH: 2000,
          TOOL_WEB_FETCH: 2100,
          TOOL_LSP: 2200,
          TOOL_SESSION_QUERY: 2300,
          TOOL_GOAL: 2400,
          TOOL_SCHEDULE: 2500,
          TOOL_WORKFLOW: 2600,
          TOOL_RALPH: 2700,
          TOOL_SUBAGENT: 2800,
          TOOL_REPORT: 2900,
          TOOL_COMPUTER_USE: 3000,
          MCP_SERVERS: 3100,
          TOOLS_SDK: 5000,
          DELIVERABLE_FILE_REFERENCES: 9000,
          STRUCTURED_OUTPUT: 9900,
          HARNESS_SOURCE: 10000,
          WEB_SURFACE: 10100,
          DEPLOYMENT_PERSONA_SUFFIX: 10200,
        };
        const order = orders[text(name)];
        if (order === undefined) throw new Error(`DSH systemPrompt section order ${name} is unknown`);
        return order;
      },
      getContextOrder(name) {
        const orders = {
          WORKING_DIRECTORY: 100,
          SANDBOX_POLICY: 110,
          APPROVAL_POLICY: 115,
          SUBAGENT_DELEGATION: 120,
          HARNESS_SOURCE: 200,
          WEB_SURFACE: 210,
        };
        const order = orders[text(name)];
        if (order === undefined) throw new Error(`DSH systemPrompt context order ${name} is unknown`);
        return order;
      },
      section(options) {
        return sections.add(options);
      },
      context(options) {
        return contexts.add(options);
      },
      tools(provider) {
        if (typeof provider !== "function") throw new Error("DSH systemPrompt.tools provider must be a function");
        if (promptToolProviders.includes(provider)) throw new Error("DSH systemPrompt.tools provider is already registered");
        promptToolProviders.push(provider);
        let active = true;
        return () => {
          if (!active) return;
          active = false;
          const index = promptToolProviders.indexOf(provider);
          if (index >= 0) promptToolProviders.splice(index, 1);
        };
      },
      variable(name, provider) {
        const key = text(name);
        if (!key) throw new Error("DSH systemPrompt.variable name is required");
        if (promptVariables.has(key)) throw new Error(`DSH systemPrompt.variable ${key} is already registered`);
        promptVariables.set(key, provider);
        let active = true;
        return () => {
          if (!active) return;
          active = false;
          if (promptVariables.get(key) === provider) promptVariables.delete(key);
        };
      },
      assemble(context = {}) {
        return snapshotAmitiaDshPromptState(activePromptState, context);
      },
      snapshot() {
        return snapshotAmitiaDshPromptState(activePromptState);
      },
    },
    network: {
      request(input) {
        return call("host.network.request", input);
      },
    },
    workspace: {
      read(input) {
        return call("host.workspace.read", input);
      },
      write(input) {
        return call("host.workspace.write", input);
      },
    },
    memory: {
      query(input) {
        return call("host.memory.query", input);
      },
    },
    resources: {
      open(input) {
        return call("host.resource.open", input);
      },
      read(input) {
        return call("host.resource.read", input);
      },
      write(input) {
        return call("host.resource.write", input);
      },
      close(input) {
        return call("host.resource.close", input);
      },
      stat(input) {
        return call("host.resource.stat", input);
      },
      link(input) {
        return call("host.resource.link", input);
      },
      delete(input) {
        return call("host.resource.delete", input);
      },
    },
    events: {
      emit(eventType, payload) {
        return call("host.event.emit", { eventType, payload });
      },
      async subscribe(eventType, entry) {
        const result = await call("host.event.subscribe", { eventType, entry });
        const subscriptionId = text(result?.subscriptionId);
        if (!subscriptionId) throw new Error(`Amitia event subscription ${eventType} did not return subscriptionId`);
        return {
          subscriptionId,
          eventTypes: [eventType],
          dispose() {
            return call("host.event.unsubscribe", { subscriptionId });
          },
        };
      },
    },
    approval: {
      ask(input) {
        return call("host.ui.dialog", dialogRequest("approval.ask", input, "插件请求确认"));
      },
      ...createApprovalService(call),
    },
    workspaceFiles: createWorkspaceFilesService(call),
    fileReferences: createFileReferencesService(call),
    sessionFileReferences: createSessionFileReferencesService(call),
    skills: createSkillsService(call),
    sessionSkillCatalog: createSessionSkillCatalogService(call),
    directoryPicker: createDirectoryPickerService(call),
    directoryPickerController: createDirectoryPickerControllerService(),
    workspaceController: createWorkspaceControllerService(),
    sessions: {
      read(input) {
        return call("host.conversation.read", input);
      },
      messages: {
        send(input) {
          return call("host.conversation.message.send", input);
        },
        append(input) {
          return call("host.conversation.message.append", input);
        },
      },
      resume(sessionId) {
        return agentFacade(sessionId)?.session;
      },
      get(sessionId) {
        const id = text(sessionId);
        return id ? agents.get(id)?.session : undefined;
      },
      list() {
        return Array.from(agents.values()).map((agent) => agent.session);
      },
      async flush(session) {
        if (!session || typeof session.whenPersisted !== "function") {
          throw new Error("DSH sessions.flush 需要容器创建的会话对象");
        }
        await session.whenPersisted();
        return true;
      },
      ...createSessionUnavailableMethods(),
      async create(input = {}) {
        const result = await call("host.conversation.create", {
          title: text(input.title),
          projectId: text(input.projectId),
          channel: text(input.channel),
          peerId: text(input.peerId),
          requestId: text(input.requestId),
        });
        const conversationId = text(result?.conversationId || result?.id);
        if (!conversationId) throw new Error("Amitia host.conversation.create did not return conversationId");
        const session = agentFacade(conversationId)?.session;
        const scopeSnapshotId = text(result?.scopeSnapshotId);
        if (session && scopeSnapshotId) await session.setScopeSnapshotId(scopeSnapshotId);
        return session;
      },
    },
    sessionProjections: sessionProjectionService,
    credentials: credentialsService,
    commands: commandsService,
    sessionQuery: sessionQueryService,
    userQuestions,
    workingDirectory,
    fs,
    schedule,
    agents: {
      turnBoundaryOf(session) {
        if (!session || typeof session !== "object") {
          throw new Error(`DSH agents.turnBoundaryOf 需要 Session 对象: ${JSON.stringify(session ?? null)}`);
        }
        const sessionId = text(session.id) || text(session.session?.id);
        const definition = sessionProjectionService.get("turnBoundary");
        if (!definition) {
          throw hostSeamError({
            method: "agents.turnBoundaryOf",
            capability: "会话轮次边界投影",
            expectation: "按官方 turnBoundary 投影定义折叠会话事件",
            missing: `sessionProjections 中 key 为 turnBoundary 的投影（由官方 @deepseek-ai/dsh-agent-loop 经 ctx.sessionProjections.register 注册，当前已注册 ${sessionProjectionService.list().length} 个）`,
            request: { sessionId },
          });
        }
        const events = Array.isArray(session.events) ? session.events : session.getSnapshot?.()?.events ?? [];
        let state = definition.init?.({}, 0);
        for (const event of events) state = definition.apply?.(state, event) ?? state;
        return state;
      },
      ...createAgentUnavailableMethods(),
      currentInitiator() {
        return initiatorStack.at(-1);
      },
      requireInitiator() {
        const agent = initiatorStack.at(-1);
        if (!agent) throw new Error("DSH agents.requireInitiator 当前没有发起者 agent：请在 withInitiator 或 enter 作用域内调用");
        return agent;
      },
      withInitiator(agent, operation) {
        if (typeof operation !== "function") throw new Error("DSH agents.withInitiator 需要同步回调");
        initiatorStack.push(agent);
        try {
          return operation();
        } finally {
          const index = initiatorStack.lastIndexOf(agent);
          if (index >= 0) initiatorStack.splice(index, 1);
        }
      },
      withoutInitiator(operation) {
        if (typeof operation !== "function") throw new Error("DSH agents.withoutInitiator 需要同步回调");
        const saved = initiatorStack.splice(0, initiatorStack.length);
        try {
          return operation();
        } finally {
          initiatorStack.push(...saved);
        }
      },
      register(agent) {
        const id = text(agent?.id);
        if (!id) throw new Error("DSH agents.register 需要带 id 的 agent");
        agents.set(id, agent);
        let active = true;
        const dispose = () => {
          if (!active) return;
          active = false;
          if (agents.get(id) === agent) agents.delete(id);
        };
        dispose.dispose = dispose;
        return dispose;
      },
      enter(agent) {
        const id = text(agent?.id);
        if (!id) throw new Error("DSH agents.enter 需要带 id 的 agent");
        initiatorStack.push(agent);
        let active = true;
        return () => {
          if (!active) return;
          active = false;
          const index = initiatorStack.lastIndexOf(agent);
          if (index >= 0) initiatorStack.splice(index, 1);
        };
      },
      get(sessionId) {
        const id = text(sessionId);
        return id ? agents.get(id) : undefined;
      },
      async create(options = {}) {
        const sessionId = text(options.sessionId);
        if (!sessionId) throw new Error("DSH agents.create sessionId is required");
        if (agents.has(sessionId)) throw new Error(`DSH agent ${sessionId} is already live`);
        const meta = options.meta && typeof options.meta === "object" ? options.meta : {};
        const unsupportedMeta = Object.keys(meta).filter((key) => meta[key] !== undefined);
        if (unsupportedMeta.length > 0) {
          throw unsupportedOperation("agents.create", `session metadata ${unsupportedMeta.join(", ")} requires a DSH Session projection`);
        }
        if (options.seed !== undefined || options.inheritedEventCount !== undefined) {
          throw unsupportedOperation("agents.create", "seed and inheritedEventCount require DSH fork history import");
        }
        const { provider, model, reasoningEffort } = normalizeAgentOptions("agents.create", options);
        options.signal?.throwIfAborted();
        const created = await call("host.conversation.create", {
          conversationId: sessionId,
          requestId: `dsh-agent-create:${sessionId}`,
          title: "",
          projectId: "",
          channel: "extension",
          peerId: "",
        });
        const createdSessionId = text(created?.conversationId || created?.id);
        if (!createdSessionId) throw new Error(`Amitia host.conversation.create returned no conversation id: ${JSON.stringify(created)}`);
        if (createdSessionId !== sessionId) {
          const error = new Error(`Amitia host.conversation.create returned ${createdSessionId} instead of requested ${sessionId}`);
          try {
            await cleanupCreatedAgent(createdSessionId);
          } catch (cleanupError) {
            error.cleanupError = cleanupError;
          }
          throw error;
        }
        const agent = agentFacade(sessionId);
        try {
          const scopeSnapshotId = text(created?.scopeSnapshotId);
          if (scopeSnapshotId) await agent.session.setScopeSnapshotId(scopeSnapshotId);
          await agent.session.hydrated;
          if (agent.session.persistenceError) throw agent.session.persistenceError;
          if (provider && model) {
            agent.model.select({
              provider,
              model,
              ...(reasoningEffort ? { reasoningEffort } : {}),
            });
            await agent.session.whenPersisted();
          }
          options.signal?.throwIfAborted();
        } catch (error) {
          try {
            await cleanupCreatedAgent(sessionId);
          } catch (cleanupError) {
            error.cleanupError = cleanupError;
          }
          throw error;
        }
        return createAgentHandle(agent, sessionId);
      },
      async resume(options = {}) {
        if (!options || typeof options !== "object") throw new Error("DSH agents.resume options must be an object");
        const sessionId = text(options.resumeSessionId);
        if (!sessionId) throw new Error("DSH agents.resume resumeSessionId is required");
        if (agents.has(sessionId)) throw new Error(`DSH agent ${sessionId} is already live`);
        if (options.meta !== undefined || options.seed !== undefined || options.inheritedEventCount !== undefined) {
          throw unsupportedOperation("agents.resume", "session metadata and fork history are only valid for agents.create");
        }
        const { provider, model, reasoningEffort } = normalizeAgentOptions("agents.resume", options);
        options.signal?.throwIfAborted();
        const agent = agentFacade(sessionId);
        await agent.session.hydrated;
        if (agent.session.persistenceError) throw agent.session.persistenceError;
        if (provider && model) {
          agent.model.select({
            provider,
            model,
            ...(reasoningEffort ? { reasoningEffort } : {}),
          });
          await agent.session.whenPersisted();
        }
        options.signal?.throwIfAborted();
        return createAgentHandle(agent, sessionId);
      },
      agentForTool(sessionId) {
        const id = text(sessionId);
        return id ? agentFacade(id) : undefined;
      },
      list() {
        return Array.from(agents.values());
      },
    },
    subagents: {
      ...createSubagentUnavailableMethods(),
      resolveMaxDepth(configured) {
        if (configured === "provider-managed") return undefined;
        if (configured !== undefined) {
          if (!Number.isSafeInteger(configured) || configured < 0) throw new Error("DSH subagents maxDepth must be a non-negative safe integer");
          return configured;
        }
        return 1;
      },
      rootOf(caller) {
        if (!caller) throw new Error("DSH subagents.rootOf caller is required");
        return caller;
      },
      namesOf(agent) {
        if (!agent) throw new Error("DSH subagents.namesOf agent is required");
        return { self: "lead" };
      },
      getProvider(name) {
        return subagentProviders.get(text(name));
      },
      registerProvider(provider) {
        const name = text(provider?.name);
        if (!name) throw new Error("DSH subagents provider name is required");
        if (subagentProviders.has(name)) throw new Error(`DSH subagents provider ${name} is already registered`);
        subagentProviders.set(name, provider);
        let active = true;
        return () => {
          if (!active) return;
          active = false;
          if (subagentProviders.get(name) === provider) subagentProviders.delete(name);
        };
      },
      async startActivation(spec = {}) {
        const providerName = text(spec.provider);
        const provider = subagentProviders.get(providerName);
        if (!provider) throw new Error(`DSH subagents provider ${providerName || "<empty>"} is not registered`);
        if (typeof provider.start !== "function") {
          throw unsupportedOperation("subagents.startActivation", `${providerName} requires a local DSH Agent runtime`);
        }
        if (spec.signal?.aborted) throw spec.signal.reason;
        const request = {
          ...spec,
          cwd: text(spec.cwd) || process.cwd(),
        };
        const run = await provider.start(request);
        if (!run || !text(run.id) || !run.result || typeof run.dispose !== "function") {
          throw new Error(`DSH subagents provider ${providerName} returned an invalid run`);
        }
        let active = true;
        const dispose = async () => {
          if (!active) return;
          active = false;
          subagentRuns.delete(run.id);
          await run.dispose();
        };
        const activation = {
          childId: run.id,
          name: text(spec.name) || `${text(spec.label) || providerName}:${run.id}`,
          messageId: run.messageId,
          result: run.result,
          dispose,
        };
        subagentRuns.set(run.id, { provider: providerName, activation, run });
        return activation;
      },
      async listDescendants() {
        return Array.from(subagentRuns.values()).map(({ provider, activation }) => ({
          id: activation.childId,
          kind: "child",
          mode: "one-shot",
          activity: "running",
          name: activation.name,
          label: activation.name,
          path: activation.name,
          provider,
          hasChildren: false,
        }));
      },
      async sendMessage() {
        throw unsupportedOperation("subagents.sendMessage");
      },
      interrupt(targetSessionId) {
        const active = subagentRuns.get(text(targetSessionId));
        if (!active) return;
        void active.activation.dispose().catch((error) => console.error("[original-error]", error));
      },
      interruptByUser(childId) {
        const active = subagentRuns.get(text(childId));
        if (active) void active.activation.dispose().catch((error) => console.error("[original-error]", error));
        return { accepted: true };
      },
      async waitForTreeWork() {
        return false;
      },
      async drainDescendants() {},
    },
    llm: createLlmService(call),
    jobs: {
      start(options = {}) {
        const jobId = text(options.id) || `dsh-job-${jobs.size + 1}`;
        const controller = new AbortController();
        const output = [];
        const startedAt = Date.now();
        const job = {
          id: jobId,
          sources: Array.isArray(options.output) ? options.output : [],
          sourceOffsets: new Map(),
          sourceTotal: 0,
          result: undefined,
          kind: text(options.kind) || "dsh",
          label: text(options.label) || jobId,
          sessionId: text(options.sessionId || options.owner || options.session),
          detail: text(options.detail) || undefined,
          progress: text(options.progress) || undefined,
          status: "running",
          output,
          controller,
          startedAt,
          finishedAt: undefined,
          done: null,
        };
        const run = typeof options.run === "function" ? options.run : async () => undefined;
        job.done = Promise.resolve().then(() => run({ signal: controller.signal, jobId })).then(async (value) => {
          if (value && typeof value.done?.then === "function") {
            job.handle = value;
            await value.done;
            job.result = await value.done;
          } else {
            job.result = value;
          }
          if (job.status !== "cancelled") {
            job.status = "completed";
            job.finishedAt = Date.now();
          }
          if (value !== undefined && value !== null) {
            output.push(typeof value === "string" ? value : JSON.stringify(value));
          }
          return value;
        }, (error) => {
          if (job.status !== "cancelled") {
            job.status = "failed";
            job.detail = error instanceof Error ? error.message : String(error);
            job.finishedAt = Date.now();
          }
          throw error;
        });
        jobs.set(jobId, job);
        console.error(`[dsh-jobs] start id=${jobId} kind=${job.kind} sources=${job.sources.length}`);
        return {
          kind: "background",
          id: jobId,
          jobId,
          cancel(reason = "cancelled") {
            if (job.handle && typeof job.handle.cancel === "function") job.handle.cancel(reason);
            controller.abort(reason);
            job.status = "cancelled";
            job.detail = text(reason) || job.detail;
            job.finishedAt = Date.now();
          },
          done: job.done,
        };
      },
      get(target) {
        const jobId = target && typeof target === "object" ? text(target.id ?? target.jobId) : text(target);
        const job = jobs.get(jobId);
        return job ? jobView(job) : undefined;
      },
      async wait(target, timeoutMs, caller, signal) {
        const jobId = target && typeof target === "object" ? text(target.id ?? target.jobId) : text(target);
        const job = jobs.get(jobId);
        if (!job) throw new Error(`DSH job ${jobId} not found`);
        const settled = job.done ? job.done.then(() => undefined, () => undefined) : Promise.resolve();
        const budget = Number(timeoutMs);
        if (Number.isFinite(budget) && budget > 0) {
          await Promise.race([settled, new Promise((resolve) => setTimeout(resolve, budget))]);
        } else {
          await settled;
        }
        if (signal && typeof signal.throwIfAborted === "function") signal.throwIfAborted();
        return jobView(job);
      },
      read(target, caller) {
        const jobId = target && typeof target === "object" ? text(target.id ?? target.jobId) : text(target);
        const job = jobs.get(jobId);
        if (!job) throw new Error(`DSH job ${jobId} not found`);
        const chunks = collectJobChunks(job);
        console.error(`[dsh-jobs] read id=${jobId} status=${job.status} chunks=${chunks.length} total=${job.sourceTotal}`);
        const result = job.status === "running" || job.status === "stopping" ? undefined : text(job.result) || undefined;
        return {
          chunks,
          lossy: false,
          ...(result ? { result } : {}),
          next: job.sourceTotal,
          job: jobView(job),
        };
      },
      readAt(target, from, caller) {
        const jobId = target && typeof target === "object" ? text(target.id ?? target.jobId) : text(target);
        const job = jobs.get(jobId);
        if (!job) throw new Error(`DSH job ${jobId} not found`);
        const offset = Number(from);
        if (!Number.isInteger(offset) || offset < 0) throw new Error(`DSH job ${jobId} invalid offset ${from}`);
        return {
          chunks: collectJobChunks(job, offset),
          next: job.sourceTotal,
          lossy: false,
        };
      },
      remove(target) {
        const jobId = target && typeof target === "object" ? text(target.id ?? target.jobId) : text(target);
        return jobs.delete(jobId);
      },
      list() {
        return Array.from(jobs.values()).map((job) => jobView(job));
      },
      cancel(jobId, reason = "cancelled") {
        const job = jobs.get(text(jobId));
        if (!job) throw new Error(`DSH job ${jobId} not found`);
        job.controller.abort(reason);
        job.status = "cancelled";
        job.detail = text(reason) || job.detail;
        job.finishedAt = Date.now();
      },
      kill(target, owner, reason = "killed") {
        const jobId = target && typeof target === "object" ? text(target.id ?? target.jobId) : text(target);
        const job = jobs.get(jobId);
        if (!job) throw new Error(`DSH job ${jobId} not found`);
        const live = job.status === "running" || job.status === "stopping";
        job.controller.abort(text(reason) || "killed");
        job.status = "cancelled";
        job.detail = text(reason) || job.detail;
        job.finishedAt = Date.now();
        return live ? "requested" : "already-finished";
      },
      cancelAll(reason = "plugin deactivated") {
        for (const job of jobs.values()) {
          if (job.status !== "running") continue;
          job.controller.abort(reason);
          job.status = "cancelled";
          job.detail = text(reason) || job.detail;
          job.finishedAt = Date.now();
        }
      },
    },
    characters: {
      read(input) {
        return call("host.character.read", input);
      },
      list(input) {
        return call("host.character.list", input);
      },
    },
    runtime: {
      health(input) {
        return call("host.runtime.health", input);
      },
      snapshot(input) {
        return call("host.runtime.snapshot", input);
      },
    },
    mcpResources,
    mcp: {
      resources: mcpResources,
      async probe() {
        throw unsupportedOperation("mcp.probe");
      },
    },
  };
}

function jobView(job) {
  const owner = text(job.sessionId) || text(job.owner);
  const spillPaths = Array.isArray(job.output?.spillPaths) ? job.output.spillPaths : [];
  return {
    id: job.id,
    kind: job.kind,
    label: job.label,
    ...(owner ? { owner } : {}),
    status: job.status,
    ...(text(job.detail) ? { detail: text(job.detail) } : {}),
    output: { spillPaths },
  };
}

function collectJobChunks(job, from = 0) {
  const chunks = [];
  for (const source of job.sources) {
    if (!source || typeof source.read !== "function") continue;
    const cursor = job.sourceOffsets.get(source) ?? 0;
    let pulled;
    try {
      pulled = source.read(cursor);
    } catch {
      continue;
    }
    const data = text(pulled?.text);
    console.error(`[dsh-jobs] source channel=${text(source.channel)} cursor=${cursor} textLen=${data.length} next=${pulled?.nextOffset}`);
    if (!data) continue;
    const next = Number.isFinite(pulled?.nextOffset) ? Number(pulled.nextOffset) : cursor + data.length;
    job.sourceOffsets.set(source, next);
    job.sourceTotal = Math.max(job.sourceTotal, next);
    if (next <= from) continue;
    chunks.push({ channel: text(source.channel) || "stdout", text: data, from: cursor, to: next, at: cursor });
  }
  let offset = job.sourceTotal;
  for (const item of Array.isArray(job.output) ? job.output : []) {
    const data = typeof item === "string" ? item : text(item?.data ?? item?.text);
    if (!data) continue;
    chunks.push({ channel: "stdout", text: data, from: offset, to: offset + data.length, at: offset });
    offset += data.length;
  }
  return chunks;
}
