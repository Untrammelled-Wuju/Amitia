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
import { buildSessionProjection, foldSessionSurface } from "./session-projection.mjs";
import { sharedDshWebServer } from "./web-server.mjs";
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
  const read = (operation, payload) => call("host.workspace.read", { operation, payload });
  const directoryListingOf = async (sessionId, target, absolutePath) => {
    const listing = await read("file_list", { sessionId, path: target });
    const entries = (Array.isArray(listing?.entries) ? listing.entries : [])
      .map((child) => ({ name: text(child?.name), type: text(child?.type), size: Number(child?.size) }))
      .filter((child) => child.name !== "");
    const children = new Map();
    let cursor = 0;
    const worker = async () => {
      for (;;) {
        const index = cursor;
        cursor += 1;
        const child = entries[index];
        if (!child) return;
        const childPath = joinWatchPath(absolutePath, child.name);
        if (child.type !== "file") {
          children.set(child.name, { version: child.type, absolutePath: childPath });
          continue;
        }
        const stat = await read("file_stat", { sessionId, path: childPath });
        children.set(child.name, { version: text(stat?.version), absolutePath: text(stat?.absolutePath) || childPath });
      }
    };
    await Promise.all(Array.from({ length: Math.min(WORKSPACE_WATCH_STAT_CONCURRENCY, entries.length) }, worker));
    return children;
  };
  const snapshotOf = async (sessionId, target) => {
    const { parent, name } = splitWatchPath(target);
    const listing = await read("file_list", { sessionId, path: parent });
    const entries = Array.isArray(listing?.entries) ? listing.entries : [];
    const entry = entries.find((item) => text(item?.name) === name);
    const absolutePath = joinWatchPath(parent, name);
    if (!entry) return { kind: "absent", absolutePath };
    if (text(entry?.type) !== "directory") {
      const stat = await read("file_stat", { sessionId, path: target });
      return { kind: "file", absolutePath: text(stat?.absolutePath) || absolutePath, version: text(stat?.version) };
    }
    return { kind: "directory", absolutePath, entries: await directoryListingOf(sessionId, target, absolutePath) };
  };
  const diffSnapshots = (previous, next) => {
    if (next.kind === "absent") {
      return previous.kind === "absent" ? [] : [{ absolutePath: next.absolutePath, absent: true }];
    }
    if (next.kind === "file") {
      if (previous.kind === "file" && previous.version === next.version) return [];
      return [{ absolutePath: next.absolutePath, version: next.version }];
    }
    const before = previous.kind === "directory" ? previous.entries : new Map();
    const changes = [];
    for (const [name, entry] of next.entries) {
      const previousEntry = before.get(name);
      if (!previousEntry || previousEntry.version !== entry.version) {
        changes.push({ absolutePath: entry.absolutePath, version: entry.version });
      }
    }
    for (const [name, entry] of before) {
      if (!next.entries.has(name)) changes.push({ absolutePath: entry.absolutePath, absent: true });
    }
    changes.sort((left, right) => left.absolutePath.localeCompare(right.absolutePath));
    return changes;
  };
  return {
    ...operations,
    async *changes(request = {}, signal) {
      const input = typeof request === "string" ? { path: request } : (request ?? {});
      const sessionId = sessionIdOf("workspaceFiles.changes", input);
      const target = text(input.path);
      if (target === "") {
        throw new Error(`DSH workspaceFiles.changes 需要 path：${JSON.stringify(input)}`);
      }
      const requestedInterval = Number(input.intervalMs);
      const interval = Number.isFinite(requestedInterval) && requestedInterval > 0 ? Math.max(100, Math.floor(requestedInterval)) : 1000;
      let previous = await snapshotOf(sessionId, target);
      if (previous.kind === "absent") {
        throw new Error(`DSH workspaceFiles.changes 目标不存在 ${JSON.stringify(target)}：请先确认路径存在再建立监视`);
      }
      yield { kind: "ready" };
      while (!signal?.aborted) {
        await delayWatch(interval, signal);
        if (signal?.aborted) return;
        const next = await snapshotOf(sessionId, target);
        for (const change of diffSnapshots(previous, next)) {
          yield { kind: "change", change };
        }
        previous = next;
      }
    },
  };
}

const WORKSPACE_WATCH_STAT_CONCURRENCY = 8;

function splitWatchPath(target) {
  const normalized = String(target ?? "");
  const index = Math.max(normalized.lastIndexOf("/"), normalized.lastIndexOf("\\"));
  if (index < 0) return { parent: "", name: normalized };
  if (index === 0) return { parent: normalized.slice(0, 1), name: normalized.slice(1) };
  return { parent: normalized.slice(0, index), name: normalized.slice(index + 1) };
}

function joinWatchPath(parent, name) {
  const base = String(parent ?? "");
  if (base === "") return name;
  if (base.endsWith("/") || base.endsWith("\\")) return `${base}${name}`;
  return `${base}/${name}`;
}

function delayWatch(ms, signal) {
  return new Promise((resolve) => {
    const finish = () => {
      clearTimeout(timer);
      signal?.removeEventListener?.("abort", finish);
      resolve();
    };
    const timer = setTimeout(finish, ms);
    signal?.addEventListener?.("abort", finish, { once: true });
  });
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
  const pick = async (input = {}) => {
    const signal = input && typeof input === "object" && typeof input.aborted === "boolean" && typeof input.addEventListener === "function"
      ? input
      : undefined;
    signal?.throwIfAborted?.();
    const result = await requireCall("directoryPicker.pick")("host.directory.pick", {});
    signal?.throwIfAborted?.();
    const picked = text(result?.path);
    return picked === "" ? null : picked;
  };
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

function createDirectoryPickerControllerService(call) {
  const picker = createDirectoryPickerService(call);
  const pick = (signal) => picker.pick(signal);
  return {
    capability() {
      return { kind: "native", pick };
    },
    async pick(signal) {
      return pick(signal);
    },
    async list(path, signal) {
      return picker.list(path, signal);
    },
    async createDirectory(path, name) {
      return picker.createDirectory(path, name);
    },
  };
}

function createWorkspaceControllerService(host) {
  const call = (method, input = {}) => host.call(method, input);
  const requireCall = (method) => {
    if (typeof call !== "function") {
      throw seamUnavailable("workspaceController", method, "host.project.* 的宿主调用桥不可用");
    }
    return call;
  };
  const callTargeted = (method, input, scopeSnapshotId) => {
    if (typeof host.callTargeted !== "function") {
      throw new Error(`Amitia host.callTargeted 不可用，无法按会话作用域快照调用 ${method}：${JSON.stringify({ scopeSnapshotId })}`);
    }
    return host.callTargeted(method, input, scopeSnapshotId);
  };
  const scopeSnapshotIdOf = async (conversationId) => {
    const state = await requireCall("workspaceController.scope")("host.state.get", { key: `dsh-session:${conversationId}` });
    if (state?.found) {
      const persisted = text(state.value?.scopeSnapshotId);
      if (persisted !== "") return persisted;
    }
    const created = await requireCall("workspaceController.scope")("host.conversation.scope", { conversationId });
    const minted = text(created?.scopeSnapshotId);
    if (minted === "") {
      throw new Error(`Amitia host.conversation.scope returned no scope snapshot id: ${JSON.stringify(created ?? null)}`);
    }
    return minted;
  };
  const unavailable = (method, missing) => {
    throw hostSeamError({
      method: `workspaceController.${method}`,
      capability: "宿主工作区控制器",
      expectation: "按 DSH WorkspaceController 语义读写工作区注册表",
      missing,
    });
  };
  const listProjects = async () => {
    const result = await requireCall("workspaceController.list")("host.project.list", { limit: 200 });
    if (!Array.isArray(result?.projects)) {
      throw new Error(`Amitia host.project.list 返回结构异常：缺少 projects 数组 ${JSON.stringify(result ?? null)}`);
    }
    return result.projects;
  };
  const projectPath = (project) => {
    const workspaces = Array.isArray(project?.workspaces) ? project.workspaces : [];
    const primary = workspaces.find((entry) => text(entry?.id) === text(project?.primaryWorkspaceId)) ?? workspaces[0];
    const rootUri = text(primary?.rootUri);
    if (rootUri === "") return "";
    if (rootUri.startsWith("file://")) {
      try {
        return decodeURIComponent(rootUri.slice("file://".length));
      } catch (error) {
        throw new Error(`无法解析工作区根路径 ${JSON.stringify(rootUri)}`, { cause: error });
      }
    }
    return rootUri;
  };
  const sessionOrderKey = (item) => {
    const value = Number(item?.order);
    if (Number.isFinite(value) && value > 0) return value;
    const created = Date.parse(text(item?.createdAt));
    return Number.isFinite(created) ? created : 1;
  };
  const sessionsByProject = async () => {
    const result = await requireCall("workspaceController.sessions")("host.conversation.list", { limit: 200, offset: 0 });
    const items = Array.isArray(result?.items) ? result.items : [];
    const grouped = new Map();
    for (const item of items) {
      const projectId = text(item?.projectId);
      if (projectId === "") continue;
      const sessionId = text(item?.sessionId) || text(item?.id);
      if (sessionId === "") continue;
      const list = grouped.get(projectId) ?? [];
      list.push({ sessionId, order: sessionOrderKey(item) });
      grouped.set(projectId, list);
    }
    const ordered = new Map();
    for (const [projectId, list] of grouped) {
      ordered.set(projectId, list
        .sort((left, right) => left.order - right.order || left.sessionId.localeCompare(right.sessionId))
        .map((entry) => entry.sessionId));
    }
    return ordered;
  };
  const viewOf = (project, grouped) => ({
    workspaceId: text(project?.id),
    path: projectPath(project),
    title: text(project?.title),
    sessionIds: grouped.get(text(project?.id)) ?? [],
    createdAt: text(project?.createdAt),
    updatedAt: text(project?.updatedAt),
  });
  const notFound = (workspaceId) => {
    const error = new Error(`workspace ${JSON.stringify(text(workspaceId))} not found`);
    error.code = "session/not-found";
    return error;
  };
  const refreshed = async (workspaceId) => {
    const project = (await listProjects()).find((entry) => text(entry?.id) === text(workspaceId));
    if (!project) throw notFound(workspaceId);
    return viewOf(project, await sessionsByProject());
  };
  const controller = {
    async list() {
      const grouped = await sessionsByProject();
      return (await listProjects()).map((project) => viewOf(project, grouped));
    },
    async create(request = {}) {
      const path = text(request?.path);
      if (path === "") {
        throw new Error(`DSH workspaceController.create 需要绝对路径：${JSON.stringify(request ?? null)}`);
      }
      const grouped = await sessionsByProject();
      const existing = (await listProjects()).find((project) => projectPath(project) === path);
      if (existing) return { workspace: viewOf(existing, grouped), created: false };
      await requireCall("workspaceController.create")("host.project.create", {
        title: workspaceBasename(path),
        directoryPath: path,
        requestId: `dsh-workspace-create:${path}`,
      });
      const created = (await listProjects()).find((project) => projectPath(project) === path);
      if (!created) {
        throw new Error(`Amitia host.project.create 没有返回 path 为 ${JSON.stringify(path)} 的项目`);
      }
      return { workspace: viewOf(created, await sessionsByProject()), created: true };
    },
    async rename(request = {}) {
      const workspaceId = text(request?.workspaceId);
      const title = text(request?.title);
      if (workspaceId === "") {
        throw new Error(`DSH workspaceController.rename 需要 workspaceId：${JSON.stringify(request ?? null)}`);
      }
      if (title === "") {
        const error = new Error("Workspace rename requires a non-blank title");
        error.code = "gateway/bad-request";
        throw error;
      }
      const projects = await listProjects();
      const target = projects.find((project) => text(project?.id) === workspaceId);
      if (!target) throw notFound(workspaceId);
      if (projects.some((project) => text(project?.id) !== workspaceId && text(project?.title) === title)) {
        const error = new Error(`Workspace name ${JSON.stringify(title)} is already in use`);
        error.code = "workspace/name-conflict";
        throw error;
      }
      if (text(target.title) !== title) {
        await requireCall("workspaceController.rename")("host.project.update", {
          projectId: workspaceId,
          title,
          requestId: `dsh-workspace-rename:${workspaceId}`,
        });
      }
      return { workspace: await refreshed(workspaceId) };
    },
    async delete(request = {}) {
      const workspaceId = text(request?.workspaceId);
      if (workspaceId === "") {
        throw new Error(`DSH workspaceController.delete 需要 workspaceId：${JSON.stringify(request ?? null)}`);
      }
      await requireCall("workspaceController.delete")("host.project.delete", {
        projectId: workspaceId,
        requestId: `dsh-workspace-delete:${workspaceId}`,
      });
      return { deleted: true };
    },
    async initializeDefault(signal) {
      signal?.throwIfAborted?.();
      const grouped = await sessionsByProject();
      const existing = (await listProjects())[0];
      if (existing) return viewOf(existing, grouped);
      const listing = await requireCall("workspaceController.initializeDefault")("host.directory.list", { limit: 1 });
      const home = text(listing?.path) || text(listing?.home) || text(listing?.entries?.[0]?.path);
      if (home === "") {
        throw new Error(`Amitia host.directory.list 没有返回可用的主目录：${JSON.stringify(listing ?? null)}`);
      }
      signal?.throwIfAborted?.();
      const created = await controller.create({ path: home });
      return created.workspace;
    },
    async insertBefore(request = {}) {
      const workspaceId = text(request?.workspaceId);
      if (workspaceId === "") {
        throw new Error(`DSH workspaceController.insertBefore 需要 workspaceId：${JSON.stringify(request ?? null)}`);
      }
      const beforeWorkspaceId = text(request?.beforeWorkspaceId);
      const projects = await listProjects();
      if (!projects.some((project) => text(project?.id) === workspaceId)) throw notFound(workspaceId);
      if (beforeWorkspaceId !== "" && !projects.some((project) => text(project?.id) === beforeWorkspaceId)) {
        throw notFound(beforeWorkspaceId);
      }
      await requireCall("workspaceController.insertBefore")("host.project.reorder", {
        projectId: workspaceId,
        beforeProjectId: beforeWorkspaceId,
        requestId: `dsh-workspace-order:${workspaceId}:${beforeWorkspaceId}:${Date.now()}`,
      });
      const ordered = await listProjects();
      return { workspaceIds: ordered.map((project) => text(project?.id)).filter((id) => id !== "") };
    },
    async insertSessionBefore(request = {}) {
      const workspaceId = text(request?.workspaceId);
      const sessionId = text(request?.sessionId);
      if (workspaceId === "" || sessionId === "") {
        throw new Error(`DSH workspaceController.insertSessionBefore 需要 workspaceId 与 sessionId：${JSON.stringify(request ?? null)}`);
      }
      const beforeSessionId = text(request?.beforeSessionId);
      const projects = await listProjects();
      if (!projects.some((project) => text(project?.id) === workspaceId)) throw notFound(workspaceId);
      const grouped = await sessionsByProject();
      const members = grouped.get(workspaceId) ?? [];
      if (!members.includes(sessionId)) {
        throw new Error(`session ${JSON.stringify(sessionId)} does not belong to workspace ${JSON.stringify(workspaceId)}: ${JSON.stringify(members)}`);
      }
      if (beforeSessionId !== "" && !members.includes(beforeSessionId)) {
        throw new Error(`session ${JSON.stringify(beforeSessionId)} does not belong to workspace ${JSON.stringify(workspaceId)}: ${JSON.stringify(members)}`);
      }
      const snapshotId = await scopeSnapshotIdOf(sessionId);
      await callTargeted("host.conversation.reorder", {
        conversationId: sessionId,
        beforeConversationId: beforeSessionId,
        projectId: workspaceId,
        requestId: `dsh-session-order:${workspaceId}:${sessionId}:${beforeSessionId}:${Date.now()}`,
      }, snapshotId);
      return { workspace: await refreshed(workspaceId) };
    },
    async archivedSessionIds() {
      const result = await requireCall("workspaceController.archivedSessionIds")("host.conversation.list", { limit: 200, offset: 0, archived: true });
      const items = Array.isArray(result?.items) ? result.items : [];
      return items.map((item) => text(item?.sessionId) || text(item?.id)).filter((id) => id !== "");
    },
    async pinnedSessionIds() {
      const result = await requireCall("workspaceController.pinnedSessionIds")("host.conversation.list", { limit: 200, offset: 0 });
      const items = Array.isArray(result?.items) ? result.items : [];
      return items
        .filter((item) => text(item?.pinnedAt) !== "")
        .sort((left, right) => text(right?.pinnedAt).localeCompare(text(left?.pinnedAt)))
        .map((item) => text(item?.sessionId) || text(item?.id))
        .filter((id) => id !== "");
    },
    async archiveSession(request = {}) {
      const sessionId = text(request?.sessionId);
      if (sessionId === "") {
        throw new Error(`DSH workspaceController.archiveSession 需要 sessionId：${JSON.stringify(request ?? null)}`);
      }
      const snapshotId = await scopeSnapshotIdOf(sessionId);
      await callTargeted("host.conversation.archive", {
        conversationId: sessionId,
        archived: true,
        stopActivity: request?.stopActivity === true,
        requestId: `dsh-workspace-archive:${sessionId}`,
      }, snapshotId);
      return { archivedSessionIds: await controller.archivedSessionIds() };
    },
    async unarchiveSession(request = {}) {
      const sessionId = text(request?.sessionId);
      if (sessionId === "") {
        throw new Error(`DSH workspaceController.unarchiveSession 需要 sessionId：${JSON.stringify(request ?? null)}`);
      }
      const snapshotId = await scopeSnapshotIdOf(sessionId);
      await callTargeted("host.conversation.archive", {
        conversationId: sessionId,
        archived: false,
        requestId: `dsh-workspace-unarchive:${sessionId}`,
      }, snapshotId);
      return { archivedSessionIds: await controller.archivedSessionIds() };
    },
    async pinSession(request = {}) {
      const sessionId = text(request?.sessionId);
      if (sessionId === "") {
        throw new Error(`DSH workspaceController.pinSession 需要 sessionId：${JSON.stringify(request ?? null)}`);
      }
      if ((await controller.archivedSessionIds()).includes(sessionId)) {
        const error = new Error(`Session ${JSON.stringify(sessionId)} is archived; pinning and archival are mutually exclusive`);
        error.code = "gateway/bad-request";
        throw error;
      }
      const snapshotId = await scopeSnapshotIdOf(sessionId);
      await callTargeted("host.conversation.pin", {
        conversationId: sessionId,
        pinned: true,
        requestId: `dsh-workspace-pin:${sessionId}`,
      }, snapshotId);
      return { pinnedSessionIds: await controller.pinnedSessionIds() };
    },
    async unpinSession(request = {}) {
      const sessionId = text(request?.sessionId);
      if (sessionId === "") {
        throw new Error(`DSH workspaceController.unpinSession 需要 sessionId：${JSON.stringify(request ?? null)}`);
      }
      const snapshotId = await scopeSnapshotIdOf(sessionId);
      await callTargeted("host.conversation.pin", {
        conversationId: sessionId,
        pinned: false,
        requestId: `dsh-workspace-unpin:${sessionId}`,
      }, snapshotId);
      return { pinnedSessionIds: await controller.pinnedSessionIds() };
    },
  };
  return controller;
}

function workspaceBasename(path) {
  const segments = text(path).split(/[\\/]+/u).filter((segment) => segment !== "");
  return segments.at(-1) ?? text(path);
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
    fileRequestText(ref) {
      const attachment = normalizeFileAttachmentRef(ref);
      return dshFileHandleText(attachment, attachmentReadPath(ref));
    },
  };
}

function normalizeFileAttachmentRef(ref) {
  if (!ref || typeof ref !== "object" || Array.isArray(ref)) {
    throw new Error(`DSH llm.fileRequestText 需要 FileAttachmentRef 对象：${JSON.stringify(ref ?? null)}`);
  }
  const attachmentId = text(ref.attachmentId);
  const name = text(ref.name);
  const bytes = Number(ref.bytes);
  if (!attachmentId || !name || !Number.isSafeInteger(bytes) || bytes < 0) {
    throw new Error(`DSH llm.fileRequestText 的 FileAttachmentRef 缺少 attachmentId/name/bytes：${JSON.stringify(ref)}`);
  }
  const mediaType = text(ref.mediaType);
  return { type: "file", attachmentId, name, bytes, ...(mediaType ? { mediaType } : {}) };
}

function dshFileHandleText(ref, readonlyPath) {
  const digest = String(ref.attachmentId).slice(7, 15);
  const identity = `File ${JSON.stringify(ref.name)} (${ref.bytes} bytes, sha256:${digest})`;
  if (readonlyPath === undefined) {
    return `[${identity} was uploaded, but the current execution environment cannot access a readable path. Report that limitation if its contents are needed; do not claim to have read it.]`;
  }
  return `[${identity}: verbatim read-only copy saved at ${JSON.stringify(readonlyPath)}. Read that path with your file tools when its contents are needed; copy it to a writable location before modifying it. When delegating file work, include this saved path in the delegation prompt; only subagents sharing this execution environment can read it.]`;
}

function attachmentReadPath(ref) {
  const uri = text(ref?.uri);
  if (!uri) return undefined;
  if (uri.startsWith("file://")) {
    try {
      return decodeURIComponent(uri.slice("file://".length));
    } catch (error) {
      throw new Error(`DSH llm.fileRequestText 无法解析附件 URI ${JSON.stringify(uri)}`, { cause: error });
    }
  }
  if (uri.startsWith("/")) return uri;
  return undefined;
}

async function subagentScopeSnapshotId(call, conversationId) {
  const state = await call("host.state.get", { key: `dsh-session:${conversationId}` });
  const persisted = state?.found ? text(state.value?.scopeSnapshotId) : "";
  if (persisted !== "") return persisted;
  const created = await call("host.conversation.scope", { conversationId });
  const minted = text(created?.scopeSnapshotId);
  if (minted === "") {
    throw new Error(`Amitia host.conversation.scope returned no scope snapshot id: ${JSON.stringify(created ?? null)}`);
  }
  return minted;
}

function createSessionUnavailableMethods() {
  return {
    enter: seamThrower({
      method: "sessions.enter",
      capability: "进入会话作用域",
      expectation: "返回退出作用域的释放函数",
      missing: "DSH Session 实例（SessionStore.enter 需要可进入存储的 Session 对象，容器只提供会话快照与 agentFacade）",
    }),
    announce: seamThrower({
      method: "sessions.announce",
      capability: "公布会话",
      expectation: "把会话写入目录并通知监听者",
      missing: "DSH SessionStore.announce（Amitia 会话创建即公布，没有「已创建未公布」阶段）",
    }),
  };
}

export function createAmitiaDshHostServices(host, promptSections = [], promptState = {}, runtimeHooks = {}) {
  if (!host || typeof host.call !== "function") throw new Error("Amitia host API is unavailable");
  const call = (method, input = {}) => host.call(method, input);
  const jobs = new Map();
  const agents = new Map();
  const initiatorStack = [];
  const agentFactories = [];
  const announcedAgents = new Set();
  const subagentProviders = new Map();
  const subagentRuns = new Map();
  let subagentParents = new Map();
  let subagentParentsLoadedAt = 0;
  const refreshSubagentParents = async () => {
    subagentParents = await loadConversationParents(call);
    subagentParentsLoadedAt = Date.now();
    return subagentParents;
  };
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

  const messageProjections = [];
  const registerMessageProjection = (projection) => {
    if (!projection || typeof projection !== "object" || typeof projection.project !== "function") {
      throw new Error(`DSH sessions.registerMessageProjection 需要 { type, project(event, context) }：${JSON.stringify(projection ?? null)}`);
    }
    const type = text(projection.type);
    if (type === "") {
      throw new Error(`DSH sessions.registerMessageProjection 的 type 不能为空：${JSON.stringify(projection.type ?? null)}`);
    }
    if (messageProjections.some((item) => item.type === type)) {
      throw new Error(`DSH 消息投影 ${type} 已经注册`);
    }
    messageProjections.push({ type, project: projection.project });
    let active = true;
    const dispose = () => {
      if (!active) return;
      active = false;
      const index = messageProjections.findIndex((item) => item.type === type && item.project === projection.project);
      if (index >= 0) messageProjections.splice(index, 1);
    };
    return dispose;
  };

  const callInConversation = async (method, input, conversationId) => {
    if (typeof host.callTargeted === "function") {
      const created = await call("host.conversation.scope", { conversationId });
      const scopeSnapshotId = text(created?.scopeSnapshotId);
      if (scopeSnapshotId !== "") return host.callTargeted(method, input, scopeSnapshotId);
    }
    return call(method, input);
  };

  const readAllMessages = async (conversationId) => {
    const messages = [];
    for (let page = 0, offset = 0; page < 200; page += 1, offset += 200) {
      const response = await callInConversation("host.conversation.read", { conversationId, limit: 200, offset }, conversationId);
      const items = Array.isArray(response?.messages) ? response.messages : [];
      messages.push(...items);
      if (items.length < 200 || response?.hasMore !== true) break;
    }
    return messages;
  };

  const conversationProjection = async (conversationId) => {
    const [messages, turnResult] = await Promise.all([
      readAllMessages(conversationId),
      callInConversation("host.conversation.turns", { conversationId, limit: 200 }, conversationId),
    ]);
    const turns = Array.isArray(turnResult?.turns) ? turnResult.turns : [];
    return buildSessionProjection({ conversationId, messages, turns });
  };

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
      const events = live.session.events;
      return {
        live: true,
        events,
        surfaceEvents: live.session.surfaceEvents(),
        messages: foldSessionSurface(events, messageProjections).messages,
      };
    }
    const state = await call("host.state.get", { key: `dsh-session:${sessionId}` });
    let persisted = [];
    if (state?.found) {
      assertSessionStateVersion(state.value, sessionId);
      persisted = restoreSessionEvents(state.value.events, sessionId).events;
    }
    if (persisted.some((event) => isSurfaceEventType(event.type))) {
      return {
        live: false,
        events: persisted,
        surfaceEvents: projectSurfaceEvents(persisted),
        messages: foldSessionSurface(persisted, messageProjections).messages,
      };
    }
    const projection = await conversationProjection(sessionId);
    const base = projection.events.length;
    const events = [
      ...projection.events,
      ...persisted.map((event, index) => Object.freeze({ ...event, seq: base + index })),
    ];
    return {
      live: false,
      events,
      surfaceEvents: projectSurfaceEvents(events),
      messages: foldSessionSurface(events, messageProjections).messages,
    };
  };
  const defaultModelState = { provider: "", model: "" };
  const refreshDefaultModel = async () => {
    const defaults = await call("host.model.defaults", {});
    defaultModelState.provider = text(defaults?.provider);
    defaultModelState.model = text(defaults?.model);
    return { ...defaultModelState };
  };
  let defaultModelPrimed = false;
  const agentDefaultModel = Object.freeze({
    currentSelection() {
      if (!defaultModelPrimed) {
        defaultModelPrimed = true;
        refreshDefaultModel().catch((error) => console.error("[original-error]", error));
      }
      if (defaultModelState.provider === "" && defaultModelState.model === "") {
        throw new Error("Amitia 当前没有配置默认模型，agentDefaultModel.currentSelection 无法给出选择");
      }
      return Object.freeze({ provider: defaultModelState.provider, model: defaultModelState.model });
    },
    async refresh() {
      return refreshDefaultModel();
    },
  });
  const connection = Object.freeze({
    get authenticatedUrl() {
      return undefined;
    },
    get connected() {
      return true;
    },
  });
  const webServer = sharedDshWebServer();
  void webServer.ensureStarted().catch((error) => console.error("[original-error]", error));

  const credentialsService = createCredentialsService(call);
  const commandsService = createCommandsService(call);
  const sessionQueryService = createSessionQueryService({
    call,
    isLive: (sessionId) => agents.has(sessionId),
    sessionView,
    projections: () => messageProjections,
  });

  return {
    webServer,
    agentDefaultModel,
    connection,
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
    directoryPickerController: createDirectoryPickerControllerService(call),
    workspaceController: createWorkspaceControllerService(host),
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
      registerMessageProjection,
      async prepare(sessionId, options = {}) {
        const input = { draft: true };
        const reservedId = text(sessionId);
        if (reservedId !== "") input.conversationId = reservedId;
        const title = text(options?.title);
        if (title !== "") input.title = title;
        const projectId = text(options?.projectId);
        if (projectId !== "") input.projectId = projectId;
        const created = await call("host.conversation.create", input);
        const preparedId = text(created?.conversationId) || text(created?.id);
        if (preparedId === "") {
          throw new Error(`Amitia host.conversation.create 没有返回草稿会话 id：${JSON.stringify(created ?? null)}`);
        }
        const facade = agentFacade(preparedId);
        if (!facade?.session) {
          throw new Error(`Amitia 草稿会话 ${preparedId} 无法建立宿主 Session`);
        }
        const session = facade.session;
        session.draft = true;
        session.publish = async (publishOptions = {}) => {
          const request = { conversationId: preparedId, draft: false };
          const publishTitle = text(publishOptions?.title);
          if (publishTitle !== "") request.title = publishTitle;
          await call("host.conversation.create", request);
          session.draft = false;
          return session;
        };
        session.drop = async () => {
          const removed = await call("host.conversation.delete", { conversationId: preparedId });
          if (removed && typeof removed === "object" && removed.deleted === false) {
            throw new Error(`Amitia 草稿会话 ${preparedId} 未被删除：${JSON.stringify(removed)}`);
          }
          return true;
        };
        return session;
      },
      async fork(sessionId, atSeq) {
        const id = text(sessionId);
        if (id === "") {
          throw new Error(`DSH sessions.fork 需要会话 id：${JSON.stringify(sessionId ?? null)}`);
        }
        if (atSeq !== undefined && atSeq !== null) {
          throw new Error(`DSH sessions.fork 的 atSeq 无法映射：Amitia host.conversation.fork 只能整段派生会话，请求的 atSeq 为 ${String(atSeq)}`);
        }
        const created = await callInConversation("host.conversation.fork", { conversationId: id }, id);
        const forkedId = text(created?.conversationId) || text(created?.id);
        if (forkedId === "") {
          throw new Error(`Amitia host.conversation.fork 没有返回派生会话 id：${JSON.stringify(created ?? null)}`);
        }
        const facade = agentFacade(forkedId);
        if (!facade?.session) {
          throw new Error(`Amitia 派生会话 ${forkedId} 无法建立宿主 Session`);
        }
        return facade.session;
      },
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
      setFactory(factory) {
        if (typeof factory !== "function") {
          throw new Error(`DSH agents.setFactory 需要工厂函数：${JSON.stringify(typeof factory)}`);
        }
        agentFactories.push(factory);
        let active = true;
        return () => {
          if (!active) return;
          active = false;
          const index = agentFactories.lastIndexOf(factory);
          if (index >= 0) agentFactories.splice(index, 1);
        };
      },
      async announce(agent, source, signal) {
        if (!agent || typeof agent !== "object" || !text(agent.id)) {
          throw new Error(`DSH agents.announce 需要带 id 的 agent：${JSON.stringify(agent ?? null)}`);
        }
        if (!text(source)) {
          throw new Error(`DSH agents.announce 需要 source：${JSON.stringify(source ?? null)}`);
        }
        const agentId = text(agent.id);
        if (agents.get(agentId) !== agent) {
          throw new Error(`DSH agents.announce 的 agent "${agentId}" 不在活动注册表中`);
        }
        if (announcedAgents.has(agentId)) {
          throw new Error(`DSH agents.announce 的 agent "${agentId}" 已经公布过`);
        }
        signal?.throwIfAborted?.();
        const containerContext = runtimeHooks?.context;
        if (!containerContext || typeof containerContext.serial !== "function") {
          throw hostSeamError({
            method: "agents.announce",
            capability: "向 DSH 插件广播 agent/created",
            expectation: "在容器 cordis 上下文上串行派发 agent/created",
            missing: "容器 cordis 上下文（announce 未绑定到宿主容器，无法通知已订阅的 DSH 插件）",
            request: { sessionId: agentId, source },
          });
        }
        announcedAgents.add(agentId);
        try {
          await containerContext.serial(agent, "agent/created", {
            agent,
            source,
            ...(signal === undefined ? {} : { signal }),
          });
        } catch (error) {
          announcedAgents.delete(agentId);
          throw error;
        }
      },
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
        const factory = agentFactories.at(-1);
        if (factory) {
          const handle = await factory(options);
          const factoryAgent = handle?.agent;
          if (factoryAgent && text(factoryAgent.id)) agents.set(text(factoryAgent.id), factoryAgent);
          return handle;
        }
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
      async prompt(request = {}, signal) {
        const childSessionId = text(request?.childSessionId);
        const requestId = text(request?.requestId);
        const delivery = text(request?.delivery) || "queue";
        const content = Array.isArray(request?.content) ? request.content : [];
        if (childSessionId === "") {
          throw new Error(`DSH subagents.prompt 需要 childSessionId：${JSON.stringify(request ?? null)}`);
        }
        if (requestId === "") {
          throw new Error(`DSH subagents.prompt 需要 requestId：${JSON.stringify(request ?? null)}`);
        }
        if (delivery !== "queue" && delivery !== "steer") {
          throw new Error(`DSH subagents.prompt 的 delivery 只支持 queue/steer：${JSON.stringify(delivery)}`);
        }
        if (content.length === 0) {
          throw new Error(`DSH subagents.prompt 需要至少一个内容片段：${JSON.stringify(request ?? null)}`);
        }
        const parts = subagentTextParts("prompt", content);
        signal?.throwIfAborted?.();
        await refreshSubagentParents();
        const scopeSnapshotId = await subagentScopeSnapshotId(call, childSessionId);
        if (typeof host.callTargeted !== "function") {
          throw new Error(`Amitia host.callTargeted 不可用，无法按会话作用域向子代理追加消息：${JSON.stringify({ childSessionId })}`);
        }
        const result = await host.callTargeted("host.conversation.message.append", {
          conversationId: childSessionId,
          role: "user",
          source: "dsh-subagent",
          requestId,
          parts,
        }, scopeSnapshotId);
        signal?.throwIfAborted?.();
        const messageId = text(result?.messageId) || text(result?.id);
        if (messageId === "") {
          throw new Error(`Amitia host.conversation.message.append returned no message id: ${JSON.stringify(result ?? null)}`);
        }
        return { messageId };
      },
      resolveMaxDepth(configured) {
        if (configured === "provider-managed") return undefined;
        if (configured !== undefined) {
          if (!Number.isSafeInteger(configured) || configured < 0) throw new Error("DSH subagents maxDepth must be a non-negative safe integer");
          return configured;
        }
        return 1;
      },
      rootOf(caller) {
        const id = text(caller?.session?.id) || text(caller?.id);
        if (id === "") throw new Error(`DSH subagents.rootOf 需要带会话 id 的 caller：${JSON.stringify(caller ?? null)}`);
        if (subagentParentsLoadedAt === 0) {
          throw new Error(`DSH subagents.rootOf 尚未加载会话血缘，无法确定 ${JSON.stringify(id)} 的树根`);
        }
        const root = conversationTreeRoot(subagentParents, id);
        if (root === id) return caller;
        const live = agents.get(root);
        if (!live) {
          throw new Error(`DSH subagents.rootOf 无法解析 ${JSON.stringify(id)} 的树根 ${JSON.stringify(root)}：该会话不在活动 agent 表中`);
        }
        return live;
      },
      namesOf(agent) {
        const id = text(agent?.session?.id) || text(agent?.id);
        if (id === "") throw new Error(`DSH subagents.namesOf 需要带会话 id 的 agent：${JSON.stringify(agent ?? null)}`);
        if (subagentParentsLoadedAt === 0) {
          throw new Error(`DSH subagents.namesOf 尚未加载会话血缘，无法确定 ${JSON.stringify(id)} 的名字路径`);
        }
        const ancestry = [];
        const seen = new Set([id]);
        let current = id;
        for (;;) {
          const parent = subagentParents.get(current);
          if (parent === undefined || parent === "") break;
          if (seen.has(parent)) {
            throw new Error(`会话血缘存在环：${JSON.stringify({ conversationId: id, cycleAt: parent, ancestry })}`);
          }
          seen.add(parent);
          ancestry.push(parent);
          current = parent;
        }
        if (ancestry.length === 0) return { self: "lead" };
        const chain = ancestry.slice().reverse();
        let label = "lead";
        for (const node of chain.slice(1)) {
          const own = text(subagentRuns.get(node)?.activation?.name);
          label = own === "" || label === null ? null : `${label}/${own}`;
        }
        const own = text(subagentRuns.get(id)?.activation?.name);
        const self = own === "" || label === null ? id : `${label}/${own}`;
        const parentId = ancestry[0];
        const parent = (ancestry.length === 1 ? "lead" : label) ?? parentId;
        return { self, parent };
      },
      async resolveName(caller, name, signal) {
        if (signal?.aborted) throw signal.reason;
        const target = text(name);
        if (!target) {
          throw new Error(`DSH subagents.resolveName 需要名字：${JSON.stringify(name ?? null)}`);
        }
        const matches = [];
        for (const [childId, entry] of subagentRuns) {
          if (entry.activation.name === target) matches.push(childId);
        }
        if (matches.length === 1) return matches[0];
        if (matches.length > 1) {
          throw new Error(`DSH subagents.resolveName 名字 ${JSON.stringify(target)} 命中多个子代理：${JSON.stringify(matches)}`);
        }
        throw new Error(`DSH subagents.resolveName 找不到名字 ${JSON.stringify(target)} 的子代理，当前活动子代理：${JSON.stringify(Array.from(subagentRuns.values()).map((entry) => entry.activation.name))}`);
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
        await refreshSubagentParents();
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
      async sendMessage(sender, targetId, content, options = {}) {
        const target = text(targetId);
        if (target === "") {
          throw new Error(`DSH subagents.sendMessage 需要目标会话 id：${JSON.stringify(targetId ?? null)}`);
        }
        const senderId = text(sender?.session?.id) || text(sender?.sessionId) || text(sender?.id);
        if (senderId === "") {
          throw new Error(`DSH subagents.sendMessage 无法从 sender 解析会话 id：${JSON.stringify(sender ?? null)}`);
        }
        const parts = subagentTextParts("sendMessage", content);
        options?.signal?.throwIfAborted?.();
        const parents = await refreshSubagentParents();
        const senderRoot = conversationTreeRoot(parents, senderId);
        const targetRoot = conversationTreeRoot(parents, target);
        if (senderRoot !== targetRoot) {
          throw new Error(`DSH subagents.sendMessage 拒绝跨子代理树投递：sender ${JSON.stringify(senderId)} 树根 ${JSON.stringify(senderRoot)} 与目标 ${JSON.stringify(target)} 树根 ${JSON.stringify(targetRoot)} 不同`);
        }
        const scopeSnapshotId = await subagentScopeSnapshotId(call, target);
        if (typeof host.callTargeted !== "function") {
          throw new Error(`Amitia host.callTargeted 不可用，无法按会话作用域向子代理追加消息：${JSON.stringify({ targetSessionId: target })}`);
        }
        const result = await host.callTargeted("host.conversation.message.append", {
          conversationId: target,
          role: "user",
          source: "dsh-subagent-steer",
          requestId: `dsh-subagent-steer:${target}:${senderId}:${Date.now()}`,
          parts,
        }, scopeSnapshotId);
        options?.signal?.throwIfAborted?.();
        const messageId = text(result?.messageId) || text(result?.id);
        if (messageId === "") {
          throw new Error(`Amitia host.conversation.message.append returned no message id: ${JSON.stringify(result ?? null)}`);
        }
        return messageId;
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
        return subagentRuns.size > 0;
      },
      async drainDescendants() {
        const entries = Array.from(subagentRuns.values());
        if (entries.length === 0) return;
        const results = await Promise.allSettled(entries.map((entry) => entry.activation.dispose()));
        const failure = results.find((result) => result.status === "rejected");
        if (failure) throw failure.reason;
      },
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
          readErrors: [],
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
    ...(job.readErrors.length > 0 ? { readErrors: job.readErrors.map((entry) => ({ channel: entry.channel, cursor: entry.cursor, message: entry.message })) } : {}),
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
    } catch (error) {
      job.readErrors.push({
        channel: text(source.channel) || "unknown",
        cursor,
        message: error instanceof Error ? error.message : String(error),
        error,
      });
      console.error("[original-error]", error);
      continue;
    }
    const data = text(pulled?.text);
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

const SUBAGENT_CONVERSATION_PAGE_LIMIT = 200;
const SUBAGENT_CONVERSATION_PAGE_GUARD = 50;

function subagentTextParts(method, content) {
  const parts = Array.isArray(content) ? content : [];
  if (parts.length === 0) {
    throw new Error(`DSH subagents.${method} 需要至少一个内容片段：${JSON.stringify(content ?? null)}`);
  }
  const unsupported = parts.filter((part) => text(part?.type) !== "text");
  if (unsupported.length > 0) {
    throw new Error(`Amitia host.conversation.message.append 只接受文本片段，${method} 包含不支持的类型：${JSON.stringify(unsupported)}`);
  }
  const normalized = parts
    .map((part) => ({ type: "text", content: text(part?.text) }))
    .filter((part) => part.content !== "");
  if (normalized.length === 0) {
    throw new Error(`DSH subagents.${method} 的文本片段全为空：${JSON.stringify(content ?? null)}`);
  }
  return normalized;
}

async function loadConversationParents(call) {
  const parents = new Map();
  let previousFirst = "";
  for (let page = 0, offset = 0; page < SUBAGENT_CONVERSATION_PAGE_GUARD; page += 1, offset += SUBAGENT_CONVERSATION_PAGE_LIMIT) {
    const result = await call("host.conversation.list", { limit: SUBAGENT_CONVERSATION_PAGE_LIMIT, offset });
    const items = Array.isArray(result?.items) ? result.items : [];
    if (items.length === 0) break;
    const first = text(items[0]?.sessionId) || text(items[0]?.id);
    if (first !== "" && first === previousFirst) {
      throw new Error(`Amitia host.conversation.list 的 offset 没有推进分页：${JSON.stringify({ offset })}`);
    }
    previousFirst = first;
    for (const item of items) {
      const id = text(item?.sessionId) || text(item?.id);
      if (id === "") continue;
      const parent = text(item?.parentSession) || text(item?.forkOrigin?.conversationId);
      if (parent !== "") parents.set(id, parent);
    }
    if (items.length < SUBAGENT_CONVERSATION_PAGE_LIMIT) break;
  }
  return parents;
}

function conversationTreeRoot(parents, conversationId) {
  const ancestry = [];
  const seen = new Set([conversationId]);
  let current = parents.get(conversationId);
  while (current !== undefined && current !== "") {
    if (seen.has(current)) {
      throw new Error(`会话血缘存在环：${JSON.stringify({ conversationId, cycleAt: current, ancestry })}`);
    }
    seen.add(current);
    ancestry.push(current);
    current = parents.get(current);
  }
  return ancestry.length > 0 ? ancestry[ancestry.length - 1] : conversationId;
}
