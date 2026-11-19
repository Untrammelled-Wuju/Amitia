import { createCordisContext } from "./cordis-context.mjs";
import { dshUnavailableError } from "./operation-error.mjs";
import { createDshThemeRuntime } from "./theme-runtime.mjs";
import { createDshTimerService } from "./timer-runtime.mjs";
import { createSessionEventFeed } from "../client-ui/session-events.mjs";
import { expandConversationData, refreshConversationData, setConversationFlags } from "../client-ui/conversation-data.mjs";

function text(value) {
  return value == null ? "" : String(value).trim();
}

const unsupported = dshUnavailableError;

const reportedGaps = new Set();

function reportGap(operation, reason) {
  const key = text(operation);
  if (!key || reportedGaps.has(key)) return;
  reportedGaps.add(key);
  const error = new Error(`DSH ${key} has no Amitia host capability: ${reason}`);
  console.warn(`[dsh-client-services] ${error.message}\n${error.stack}`);
}

function workspaceBasename(path) {
  const segments = text(path).split(/[\\/]+/).filter(Boolean);
  return segments[segments.length - 1] || "Workspace";
}

function remoteFailure(code, operation, phase, reason) {
  return {
    ok: false,
    error: {
      code,
      message: `DSH ${operation} is unavailable (${phase}): ${reason}`,
      detail: { operation, phase, reason },
    },
  };
}

function remoteSuccess(value) {
  return { ok: true, value };
}

function remoteOriginalFailure(error, operation) {
  const source = error && typeof error === "object" ? error : {};
  const cause = source.cause && typeof source.cause === "object"
    ? {
        name: source.cause.name,
        message: source.cause.message,
        code: source.cause.code,
        stack: source.cause.stack,
      }
    : source.cause;
  return {
    ok: false,
    error: {
      code: text(source.code) || (text(operation).startsWith("fileUpload.") ? "FILE_UPLOAD_FAILED" : "WORKSPACE_FILES_FAILED"),
      message: text(source.message) || String(error),
      details: source.details ?? source.detail,
      detail: source.detail,
      name: source.name,
      stack: source.stack,
      cause,
    },
  };
}

const hostFailureStatuses = new Set(["failed", "failure", "rejected", "denied", "error"]);

function hostBodyText(value) {
  if (value === undefined) return "undefined";
  if (value === null) return "null";
  if (typeof value === "string") return value;
  try {
    const serialized = JSON.stringify(value);
    return serialized === undefined ? String(value) : serialized;
  } catch (error) {
    return `${String(value)}（原始返回体序列化失败：${text(error?.message) || String(error)}）`;
  }
}

function hostErrorText(error) {
  if (error === null || typeof error !== "object") return hostBodyText(error);
  const payload = {};
  for (const [key, value] of Object.entries(error)) {
    payload[key] = value instanceof Error ? { name: value.name, message: value.message, code: value.code } : value;
  }
  if (payload.name === undefined) payload.name = error.name;
  if (payload.message === undefined) payload.message = error.message;
  if (payload.code === undefined && error.code !== undefined) payload.code = error.code;
  if (payload.stack === undefined && error.stack !== undefined) payload.stack = error.stack;
  if (payload.cause === undefined && error.cause !== undefined) {
    payload.cause = error.cause instanceof Error
      ? { name: error.cause.name, message: error.cause.message, code: error.cause.code }
      : error.cause;
  }
  return hostBodyText(payload);
}

function hostResponseBody(error) {
  if (error === null || typeof error !== "object") return undefined;
  for (const key of ["response", "body", "result", "details", "detail", "data"]) {
    if (error[key] !== undefined) return error[key];
  }
  return undefined;
}

function hostCommandFailure(operation, request, error) {
  const failure = new Error(
    `DSH ${operation} 调用 Amitia 宿主 action ${request.action} 失败：${text(error?.message) || String(error)}；原始返回：${hostErrorText(error)}`,
    { cause: error },
  );
  failure.name = "DshHostCommandError";
  failure.code = text(error?.code) || "DSH_HOST_COMMAND_FAILED";
  failure.operation = operation;
  failure.action = request.action;
  failure.request = request;
  const body = hostResponseBody(error);
  if (body !== undefined) failure.response = body;
  return failure;
}

function hostResponseFailure(operation, request, response) {
  const failure = new Error(`DSH ${operation} 的 Amitia 宿主 action ${request.action} 返回失败：${hostBodyText(response)}`);
  failure.name = "DshHostCommandError";
  failure.code = text(response?.error?.code ?? response?.code) || "DSH_HOST_COMMAND_FAILED";
  failure.operation = operation;
  failure.action = request.action;
  failure.request = request;
  failure.response = response;
  return failure;
}

function hostActionMissingError(operation, reason, missingHostAction, request) {
  const error = unsupported(
    operation,
    "P13",
    `${reason}；缺少 host action：${missingHostAction}；调用入参：${hostBodyText(request)}`,
  );
  error.missingHostAction = missingHostAction;
  error.request = request;
  return error;
}

function decodeBytes(value) {
  const encoded = text(value);
  if (!encoded) return new Uint8Array();
  const binary = globalThis.atob(encoded);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}

function throwIfAborted(signal) {
  if (signal?.aborted) throw signal.reason ?? new DOMException("The operation was aborted", "AbortError");
}

function waitForWorkspacePoll(milliseconds, signal) {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(signal.reason ?? new DOMException("The operation was aborted", "AbortError"));
      return;
    }
    const timer = setTimeout(() => {
      signal?.removeEventListener?.("abort", abort);
      resolve();
    }, milliseconds);
    const abort = () => {
      clearTimeout(timer);
      reject(signal?.reason ?? new DOMException("The operation was aborted", "AbortError"));
    };
    signal?.addEventListener?.("abort", abort, { once: true });
  });
}

function workspaceDirectoryVersion(value) {
  const serialized = JSON.stringify(value ?? null);
  let hash = 2166136261;
  for (let index = 0; index < serialized.length; index += 1) {
    hash ^= serialized.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return `directory-${(hash >>> 0).toString(16)}-${serialized.length}`;
}

function workspaceTargetMissing(error) {
  const messages = [];
  const visit = (value) => {
    if (!value || typeof value !== "object") return;
    if (value.message) messages.push(String(value.message));
    for (const nested of Array.isArray(value.errors) ? value.errors : []) visit(nested);
    if (value.cause) visit(value.cause);
  };
  visit(error);
  return /not found|no such file|does not exist|not exist/i.test(messages.join("\n"));
}

async function artifactUploadTransport() {
  const desktop = globalThis.window?.amitiaDesktop;
  if (desktop && typeof desktop.getRuntimeEndpoint === "function") {
    const endpoint = await desktop.getRuntimeEndpoint();
    const origin = text(endpoint?.httpBaseURL);
    if (!origin) throw new Error("Amitia desktop runtime endpoint is unavailable");
    const headers = typeof desktop.getBackendAuthHeaders === "function"
      ? await desktop.getBackendAuthHeaders("local")
      : {};
    return {
      url: new URL("/api/artifacts/v1", origin).href,
      headers: { ...(headers || {}) },
      credentials: "omit",
    };
  }
  const origin = text(globalThis.location?.origin);
  if (!origin) throw new Error("Amitia artifact upload origin is unavailable");
  return {
    url: new URL("/api/artifacts/v1", origin).href,
    headers: {},
    credentials: "include",
  };
}

function uploadArtifactBlob(input) {
  return new Promise((resolve, reject) => {
    throwIfAborted(input.signal);
    const form = new FormData();
    form.append("file", input.file, input.filename || "attachment");
    form.append("kind", input.kind);
    form.append("source", "chat_upload");
    const xhr = new XMLHttpRequest();
    xhr.open("POST", input.url, true);
    xhr.withCredentials = input.credentials === "include";
    for (const [name, value] of Object.entries(input.headers || {})) {
      if (text(value)) xhr.setRequestHeader(name, String(value));
    }
    const abort = () => xhr.abort();
    input.signal?.addEventListener?.("abort", abort, { once: true });
    const finish = () => input.signal?.removeEventListener?.("abort", abort);
    xhr.upload.addEventListener("progress", (event) => {
      input.onProgress?.({
        loaded: event.loaded,
        ...(event.lengthComputable ? { total: event.total } : {}),
      });
    });
    xhr.addEventListener("error", () => {
      finish();
      reject(new Error("Amitia artifact upload network error"));
    });
    xhr.addEventListener("abort", () => {
      finish();
      reject(input.signal?.reason ?? new DOMException("The operation was aborted", "AbortError"));
    });
    xhr.addEventListener("load", () => {
      finish();
      let payload;
      try {
        payload = xhr.responseText ? JSON.parse(xhr.responseText) : null;
      } catch (cause) {
        const error = new Error(`Amitia artifact upload returned invalid JSON: ${xhr.responseText}`, { cause });
        error.status = xhr.status;
        reject(error);
        return;
      }
      if (xhr.status < 200 || xhr.status >= 300) {
        const error = new Error(
          text(payload?.message) || text(payload?.error) || text(xhr.statusText) || `Amitia artifact upload failed (${xhr.status})`,
        );
        error.code = text(payload?.error) || "artifact_upload_failed";
        error.status = xhr.status;
        error.details = payload;
        reject(error);
        return;
      }
      resolve(payload);
    });
    xhr.send(form);
  });
}

function consumeEffect(effect, cleanup, pending) {
  if (effect == null) return;
  if (typeof effect?.then === "function") {
    pending.push(Promise.resolve(effect).then((value) => consumeEffect(value, cleanup, pending)));
    return;
  }
  if (typeof effect === "function") {
    cleanup.push(effect);
    return;
  }
  if (typeof effect?.[Symbol.asyncIterator] === "function") {
    pending.push((async () => {
      for await (const item of effect) consumeEffect(item, cleanup, pending);
    })());
    return;
  }
  if (typeof effect?.[Symbol.iterator] === "function" && typeof effect !== "string") {
    for (const item of effect) consumeEffect(item, cleanup, pending);
    return;
  }
  throw new Error(`DSH effect must return a disposer or iterable: ${String(effect)}`);
}

function failureSnapshot(error) {
  const source = error && typeof error === "object" ? error : { message: String(error) };
  return {
    code: text(source.code) || "DSH_CATALOG_UNAVAILABLE",
    message: text(source.message) || String(error),
    stack: source.stack,
    cause: source.cause,
  };
}

function createRuntimeSnapshot(initial, load) {
  let snapshot = Object.freeze(initial);
  let pending = null;
  const listeners = new Set();
  const publish = (value) => {
    snapshot = Object.freeze(value);
    for (const listener of [...listeners]) listener();
  };
  return {
    getSnapshot() {
      return snapshot;
    },
    subscribe(listener) {
      if (typeof listener !== "function") throw new Error("DSH snapshot listener must be a function");
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    refresh() {
      if (pending) return pending;
      publish({ ...snapshot, state: "loading", error: null });
      pending = Promise.resolve()
        .then(load)
        .then((value) => {
          publish(value);
          return value;
        })
        .catch((error) => {
          publish({ ...snapshot, state: "error", error: failureSnapshot(error) });
          throw error;
        })
        .finally(() => {
          pending = null;
        });
      return pending;
    },
    dispose() {
      listeners.clear();
      pending = null;
    },
  };
}

function requestId(prefix) {
  const id = globalThis.crypto?.randomUUID?.();
  return id ? `${prefix}:${id}` : `${prefix}:${Date.now()}:${Math.random().toString(16).slice(2)}`;
}

function conversationIdOf(output) {
  const value = output?.conversationId
    || output?.sessionId
    || output?.id
    || output?.result?.conversationId
    || output?.result?.sessionId
    || output?.result?.id
    || output?.data?.conversationId
    || output?.data?.sessionId
    || output?.data?.id;
  const id = text(value);
  if (!id) throw new Error(`Amitia host.conversation.create returned no conversation id: ${JSON.stringify(output)}`);
  return id;
}

function createConversation(command, input = {}) {
  return command({
    action: "conversation-create",
    title: text(input.title),
    conversationId: text(input.conversationId),
    projectId: text(input.projectId || input.workspaceId),
    channel: text(input.channel),
    peerId: text(input.peerId),
    requestId: text(input.requestId) || requestId("dsh-conversation"),
  }).then(conversationIdOf);
}

function navigationTarget(target) {
  const value = text(target);
  if (!value.startsWith("/") || value.startsWith("//") || value.includes("\\") || value.includes("\0")) {
    throw new Error(`DSH navigation target is invalid: ${value}`);
  }
  return value;
}

function sessionTargetId(target) {
  if (typeof target === "string") return text(target);
  return text(target?.sessionId || target?.conversationId || target?.id);
}

function pluginEntryId(pluginId, entryId) {
  return `${text(pluginId)}#${text(entryId) || "root"}`;
}

function pluginEntries(packages) {
  return packages.flatMap((plugin) => (Array.isArray(plugin.entries) ? plugin.entries : []).map((entry) => {
    const id = text(entry.id) || "root";
    return {
      entryId: pluginEntryId(plugin.pluginId, id),
      moduleName: text(entry.entry),
      enabled: Boolean(plugin.enabled),
      fiberPhase: plugin.enabled ? "active" : null,
      ...(id === "root" ? { patchId: text(plugin.pluginId) } : { readOnlyReason: "management-required" }),
    };
  }));
}

function pluginBundles(packages) {
  return packages.map((plugin) => ({
    name: text(plugin.pluginId),
    official: false,
    availability: "profile",
    ...(text(plugin.version) ? { version: text(plugin.version) } : {}),
    ...(text(plugin.name) ? { description: text(plugin.name) } : {}),
    ...(text(plugin.source) ? { source: text(plugin.source) } : {}),
    enabled: Boolean(plugin.enabled),
    installed: true,
    optional: false,
    removable: true,
    rows: (Array.isArray(plugin.entries) ? plugin.entries : []).map((entry) => {
      const id = text(entry.id) || "root";
      return {
        rowId: id,
        moduleName: text(entry.entry),
        entryId: pluginEntryId(plugin.pluginId, id),
      };
    }),
    overrides: [],
  }));
}

function pluginChange(stage, target, enabled, changed = true) {
  return {
    changed,
    application: "applied",
    stage,
    target: text(target),
    enabled: Boolean(enabled),
  };
}

function createRemoteJobNamespace(command) {
  if (typeof command !== "function") throw new Error("DSH remote.job host command is unavailable");
  const encoder = new TextEncoder();
  const decoder = new TextDecoder();
  const pollInterval = 750;
  const viewOf = (job) => ({
    id: job.id,
    kind: job.kind,
    label: job.label,
    ...(job.owner ? { owner: job.owner } : {}),
    status: job.status,
    ...(job.progress ? { progress: job.progress } : {}),
    ...(job.detail ? { detail: job.detail } : {}),
    startedAt: Number(job.startedAt) || 0,
    ...(job.finishedAt ? { finishedAt: Number(job.finishedAt) } : {}),
    output: {
      earliest: Number(job.output?.earliest) || 0,
      total: Number(job.output?.total) || 0,
    },
  });
  const readJobs = async () => {
    const result = await command({ action: "jobs-list" });
    return Array.isArray(result?.jobs) ? result.jobs : [];
  };
  const pause = (signal) => new Promise((resolve) => {
    const timer = setTimeout(resolve, pollInterval);
    signal?.addEventListener?.("abort", () => {
      clearTimeout(timer);
      resolve();
    }, { once: true });
  });
  return {
    async *list(request = {}, signal) {
      const sessionId = String(request.sessionId || "").trim();
      for (;;) {
        const jobs = (await readJobs())
          .filter((job) => !sessionId || !job.owner || job.owner === sessionId)
          .map(viewOf);
        yield { type: "rows", jobs };
        if (signal?.aborted) return;
        await pause(signal);
        if (signal?.aborted) return;
      }
    },
    async *follow(request = {}, signal) {
      const jobId = String(request.jobId || "").trim();
      if (!jobId) throw new Error("DSH remote.job.follow 需要 jobId");
      let offset = Number.isFinite(Number(request.from)) ? Math.max(0, Number(request.from)) : 0;
      let opened = false;
      for (;;) {
        const job = (await readJobs()).find((item) => item.id === jobId);
        if (!job) throw new Error(`DSH job ${jobId} not found`);
        const projection = viewOf(job);
        if (!opened) {
          yield { type: "opened", job: projection, from: offset };
          opened = true;
        }
        const bytes = encoder.encode(String(job.text ?? ""));
        if (bytes.length > offset) {
          const text = decoder.decode(bytes.subarray(offset));
          yield { type: "output", chunks: [{ at: offset, text }], next: bytes.length };
          offset = bytes.length;
        }
        if (job.status !== "running" && job.status !== "stopping") {
          yield { type: "status", job: projection };
          return;
        }
        if (signal?.aborted) return;
        await pause(signal);
        if (signal?.aborted) return;
      }
    },
    async kill(request = {}) {
      const jobId = String(request.jobId || "").trim();
      if (!jobId) throw new Error("DSH remote.job.kill 需要 jobId");
      const result = await command({ action: "jobs-kill", jobId, reason: "用户停止" });
      return { ok: true, value: { outcome: result?.alreadyFinished ? "already-finished" : "requested" } };
    },
  };
}

function createNotifyService() {
  const emit = (level, payload) => {
    const message = typeof payload === "string" ? payload : text(payload?.message) || text(payload?.title) || JSON.stringify(payload ?? null);
    const toaster = globalThis.amitiaClientPlugins?.ui?.toast;
    if (typeof toaster === "function" && message) {
      try {
        toaster({ type: level === "warn" ? "warning" : level, message });
      } catch (error) {
        console.warn(`[dsh-notify] toast 失败，原始错误: ${error?.stack ?? error}`);
      }
    }
    if (level === "error") console.error(`[dsh-notify] ${message}`);
    else console.warn(`[dsh-notify] ${message}`);
  };
  return {
    info: (payload) => emit("info", payload),
    success: (payload) => emit("success", payload),
    warn: (payload) => emit("warn", payload),
    warning: (payload) => emit("warn", payload),
    error: (payload) => emit("error", payload),
  };
}

export function createDshClientRuntimeServices(options = {}) {
  const command = options.command;
  const navigate = options.navigate;
  const hostActions = options.hostActions || {};
  const uploadArtifact = typeof options.uploadArtifact === "function"
    ? options.uploadArtifact
    : async (input) => uploadArtifactBlob({
        ...(await artifactUploadTransport()),
        ...input,
      });
  if (typeof command !== "function") throw new Error("DSH client command bridge is unavailable");
  if (typeof navigate !== "function") throw new Error("DSH client navigation bridge is unavailable");
  const cleanup = [];
  let disposed = false;
  const workspaceChangesIntervalMs = Math.max(25, Number(options.workspaceChangesIntervalMs) || 750);
  const theme = createDshThemeRuntime({
    emit: options.emit,
    hostActions,
  });
  const timer = createDshTimerService();
  let panelInfoSnapshot = Object.freeze({ activePanelId: null });
  const panelInfoListeners = new Set();
  const panelState = { activePanelId: null, rightbar: null };
  let navigationController = new AbortController();
  const layoutPanelRoutes = Object.freeze({
    conversation: "/chat",
    plugins: "/extensions",
    schedules: "/tasks",
  });
  const layoutPanelsByPath = Object.freeze({
    "/chat": "conversation",
    "/extensions": "plugins",
    "/tasks": "schedules",
  });
  const publishPanelInfo = () => {
    const next = Object.freeze({
      activePanelId: panelState.activePanelId,
      ...(panelState.rightbar ? { rightbar: panelState.rightbar } : {}),
    });
    if (panelInfoSnapshot.activePanelId === next.activePanelId && panelInfoSnapshot.rightbar === next.rightbar) return;
    panelInfoSnapshot = next;
    for (const listener of [...panelInfoListeners]) {
      try {
        listener();
      } catch (error) {
        console.error("[original-error]", error);
      }
    }
  };
  const panelOfCurrentRoute = () => {
    const path = text(globalThis.location?.pathname);
    if (!path) return undefined;
    return Object.hasOwn(layoutPanelsByPath, path) ? layoutPanelsByPath[path] : null;
  };
  const syncPanelFromRoute = () => {
    const panel = panelOfCurrentRoute();
    if (panel === undefined) return;
    panelState.activePanelId = panel;
    publishPanelInfo();
  };
  if (typeof globalThis.addEventListener === "function") {
    const onPopState = () => syncPanelFromRoute();
    globalThis.addEventListener("popstate", onPopState);
    cleanup.push(() => globalThis.removeEventListener("popstate", onPopState));
  }

  const scheduleUnavailable = (operation) => remoteFailure(
    "SCHEDULE_UNAVAILABLE",
    `remote.schedule.${operation}`,
    "P13",
    "Amitia 技术调度与 DSH Session prompt 投递语义尚未完成双向适配",
  );

  const layout = {
    panelInfo: {
      getSnapshot() {
        return panelInfoSnapshot;
      },
      subscribe(listener) {
        if (typeof listener !== "function") throw new Error("DSH layout.panelInfo listener must be a function");
        panelInfoListeners.add(listener);
        return () => panelInfoListeners.delete(listener);
      },
    },
    selectPanel(panelId) {
      navigationController.abort();
      navigationController = new AbortController();
      const id = text(panelId);
      const builtin = id === "" ? layoutPanelRoutes.conversation : (layoutPanelRoutes[id] ?? "");
      panelState.activePanelId = id || null;
      publishPanelInfo();
      if (!builtin) {
        return navigate(navigationTarget(`/main-panel/${encodeURIComponent(id)}`));
      }
      return navigate(navigationTarget(builtin));
    },
    beginNavigation() {
      navigationController.abort();
      navigationController = new AbortController();
      return navigationController.signal;
    },
    toggleSidebar() {
      if (typeof hostActions.toggleSidebar !== "function") {
        throw unsupported(
          "layout.toggleSidebar",
          "P13",
          `Amitia UI provider action toggleSidebar is unavailable；调用入参：${hostBodyText({})}`,
        );
      }
      return hostActions.toggleSidebar();
    },
    openRightbar(track, fullscreen) {
      const state = Object.freeze({ track: Boolean(track), fullscreen: Boolean(fullscreen) });
      if (typeof hostActions.openRightPane !== "function") {
        throw unsupported(
          "layout.openRightbar",
          "P12",
          `Amitia right pane action is unavailable；调用入参：${hostBodyText(state)}`,
        );
      }
      const apply = () => {
        panelState.rightbar = state;
        publishPanelInfo();
      };
      const opened = hostActions.openRightPane(state);
      if (opened && typeof opened.then === "function") {
        return opened.then((value) => {
          apply();
          return value;
        });
      }
      apply();
      return opened;
    },
    closeRightbar() {
      const active = panelState.rightbar;
      if (active && typeof hostActions.closeRightPane !== "function") {
        throw unsupported(
          "layout.closeRightbar",
          "P12",
          `Amitia right pane close action is unavailable；调用入参：${hostBodyText({ rightbar: active })}`,
        );
      }
      panelState.rightbar = null;
      publishPanelInfo();
      return typeof hostActions.closeRightPane === "function" ? hostActions.closeRightPane() : undefined;
    },
  };

  const workspaceByDshID = new Map();
  const workspaceByPath = new Map();
  const searchResultLimit = 20;
  const sessionScopes = new Map();
  const sessionBindings = new Map();
  const sessionFeeds = new Map();
  const sessionRetains = new Map();
  const sessionRetainOwners = new Map();
  const retainInfoHandles = new Map();
  const retainInfoListeners = new Map();
  const retainOwnerKey = (options) => text(options?.source) || text(options?.owner) || "session";
  const retainInfoValue = (sessionId) => {
    const owners = sessionRetainOwners.get(sessionId);
    const retainedBy = {};
    if (owners) for (const [owner, count] of owners) retainedBy[owner] = count;
    return Object.freeze({
      referenceCount: sessionRetains.get(sessionId) ?? 0,
      retainedBy: Object.freeze(retainedBy),
    });
  };
  const notifyRetainInfo = (sessionId) => {
    const listeners = retainInfoListeners.get(sessionId);
    if (!listeners) return;
    for (const listener of [...listeners]) {
      try {
        listener();
      } catch (error) {
        console.error("[original-error]", error);
      }
    }
  };
  const retainObserver = (sessionId) => {
    const existing = retainInfoHandles.get(sessionId);
    if (existing) return existing;
    const handle = Object.freeze({
      sessionId,
      getSnapshot: () => retainInfoValue(sessionId),
      subscribe(listener) {
        if (typeof listener !== "function") {
          throw new Error(`DSH sessions.retainInfo(${sessionId}) listener must be a function`);
        }
        const listeners = retainInfoListeners.get(sessionId) ?? new Set();
        listeners.add(listener);
        retainInfoListeners.set(sessionId, listeners);
        return () => {
          listeners.delete(listener);
          if (listeners.size === 0) retainInfoListeners.delete(sessionId);
        };
      },
    });
    retainInfoHandles.set(sessionId, handle);
    return handle;
  };
  let defaultWorkspaceInitialization = null;
  let sessionCatalogState = {
    ids: [],
    byId: {},
    phase: "pending",
    projectionsBySession: {},
    state: "idle",
    error: null,
  };
  let workspaceCatalogState = {
    items: [],
    archivedSessionIds: [],
    pinnedSessionIds: [],
    state: "idle",
    phase: "pending",
    error: null,
  };
  const sessionCatalog = createRuntimeSnapshot(sessionCatalogState, async () => {
    const value = await command({ action: "conversation-list", limit: 128, offset: 0 });
    const items = Array.isArray(value?.items) ? value.items : [];
    const byId = {};
    const ids = [];
    const projectionsBySession = {};
    workspaceByDshID.clear();
    workspaceByPath.clear();
    const ingest = (item, archived) => {
      const id = text(item?.sessionId);
      if (!id || byId[id]) return;
      const title = text(item?.title || item?.displayTitle || id);
      const updatedAt = Number(item?.updatedAt) || 0;
      const blank = Boolean(item?.blank);
      const running = Boolean(item?.running);
      ids.push(id);
      byId[id] = {
        id,
        title,
        displayTitle: text(item?.displayTitle) || title,
        cwd: text(item?.cwd) || undefined,
        running,
        retainedBy: {},
        blank,
        updatedAt,
        parentId: text(item?.parentSessionId) || undefined,
        origin: text(item?.origin) === "subagent" ? "subagent" : undefined,
        formatStatus: text(item?.formatStatus) === "migration-required" ? "migration-required" : "current",
        projectId: text(item?.projectId) || undefined,
        workspaceId: text(item?.workspaceId) || undefined,
        pinnedAt: text(item?.pinnedAt) || undefined,
        archivedAt: text(item?.archivedAt) || undefined,
        pinned: Boolean(text(item?.pinnedAt)),
        archived: archived === true || Boolean(text(item?.archivedAt)),
      };
      projectionsBySession[id] = {
        values: {
          title,
          sessionListMetadata: {
            blank,
            lastPromptAt: blank ? null : updatedAt,
          },
        },
        state: "ready",
        error: null,
      };
    };
    for (const item of items) ingest(item, false);
    const archivedSessionsValue = await command({ action: "conversation-list", limit: 128, offset: 0, archived: true });
    for (const item of Array.isArray(archivedSessionsValue?.items) ? archivedSessionsValue.items : []) ingest(item, true);
    const workspaceItems = Array.isArray(value?.workspaces) ? value.workspaces : [];
    const registered = await command({ action: "project-workspaces" });
    const registeredItems = Array.isArray(registered?.items) ? registered.items : [];
    const mergedWorkspaces = new Map();
    for (const item of registeredItems) {
      const workspaceId = text(item?.workspaceId);
      if (!workspaceId) continue;
      mergedWorkspaces.set(workspaceId, {
        workspaceId,
        projectId: text(item?.projectId),
        path: text(item?.path),
        title: text(item?.title) || workspaceId,
        sessionIds: [],
        createdAt: text(item?.createdAt),
        updatedAt: text(item?.updatedAt),
      });
    }
    for (const item of workspaceItems) {
      const workspaceId = text(item?.workspaceId);
      if (!workspaceId) continue;
      const existing = mergedWorkspaces.get(workspaceId);
      mergedWorkspaces.set(workspaceId, {
        workspaceId,
        projectId: text(item?.projectId) || text(existing?.projectId),
        path: text(item?.path) || text(existing?.path),
        title: text(item?.title) || text(existing?.title) || workspaceId,
        sessionIds: Array.isArray(item?.sessionIds)
          ? item.sessionIds.map(text).filter(Boolean)
          : (existing?.sessionIds ?? []),
        createdAt: text(item?.createdAt) || text(existing?.createdAt),
        updatedAt: text(item?.updatedAt) || text(existing?.updatedAt),
      });
    }
    const sessionIdsByWorkspace = new Map();
    const sessionIdsByProject = new Map();
    const rememberSession = (index, key, sessionId) => {
      const group = index.get(key) ?? [];
      group.push(sessionId);
      index.set(key, group);
    };
    for (const session of Object.values(byId)) {
      const sessionId = text(session.id);
      const workspaceId = text(session.workspaceId);
      const projectId = text(session.projectId);
      if (!sessionId) continue;
      if (workspaceId) rememberSession(sessionIdsByWorkspace, workspaceId, sessionId);
      if (projectId) rememberSession(sessionIdsByProject, projectId, sessionId);
    }
    for (const item of mergedWorkspaces.values()) {
      const projected = Array.isArray(item.sessionIds) ? item.sessionIds.map(text).filter(Boolean) : [];
      const fromSessions = sessionIdsByWorkspace.get(item.workspaceId) ?? sessionIdsByProject.get(text(item.projectId)) ?? [];
      item.sessionIds = projected.length > 0 ? projected : fromSessions;
    }
    for (const item of mergedWorkspaces.values()) {
      workspaceByDshID.set(item.workspaceId, { projectId: item.projectId, path: item.path });
      if (item.path) workspaceByPath.set(item.path, { projectId: item.projectId, workspaceId: item.workspaceId });
    }
    for (const id of [...sessionScopes.keys()]) {
      if (!byId[id]) sessionScopes.delete(id);
    }
    sessionCatalogState = {
      ids,
      byId,
      phase: "ready",
      projectionsBySession,
      state: "idle",
      error: null,
    };
    const pinnedIds = items
      .filter((item) => text(item?.pinnedAt) !== "" || text(byId[text(item?.sessionId)]?.pinnedAt) !== "")
      .map((item) => text(item?.sessionId))
      .filter(Boolean);
    const archivedValue = await command({ action: "conversation-list", limit: 128, offset: 0, archived: true });
    const archivedIds = (Array.isArray(archivedValue?.items) ? archivedValue.items : [])
      .map((item) => text(item?.sessionId) || text(item?.id))
      .filter(Boolean);
    workspaceCatalogState = {
      items: [...mergedWorkspaces.values()],
      archivedSessionIds: archivedIds,
      pinnedSessionIds: pinnedIds,
      state: "idle",
      phase: "ready",
      error: null,
    };
    return sessionCatalogState;
  });
  const sessions = {
    list: {
      getSnapshot() {
        return sessionCatalog.getSnapshot();
      },
      subscribe(listener) {
        return sessionCatalog.subscribe(listener);
      },
    },
    retain(target, options = {}) {
      const sessionId = sessionTargetId(target);
      if (!sessionId) {
        throw new Error(`DSH sessions.retain requires a resolvable session target: ${JSON.stringify(target ?? null)}`);
      }
      const retainCount = (sessionRetains.get(sessionId) ?? 0) + 1;
      sessionRetains.set(sessionId, retainCount);
      const owner = retainOwnerKey(options);
      const owners = sessionRetainOwners.get(sessionId) ?? new Map();
      owners.set(owner, (owners.get(owner) ?? 0) + 1);
      sessionRetainOwners.set(sessionId, owners);
      notifyRetainInfo(sessionId);
      let released = false;
      const releaseOwner = () => {
        const remaining = (sessionRetains.get(sessionId) ?? 1) - 1;
        if (remaining > 0) sessionRetains.set(sessionId, remaining);
        else sessionRetains.delete(sessionId);
        const currentOwners = sessionRetainOwners.get(sessionId);
        if (currentOwners) {
          const next = (currentOwners.get(owner) ?? 1) - 1;
          if (next > 0) currentOwners.set(owner, next);
          else currentOwners.delete(owner);
          if (currentOwners.size === 0) sessionRetainOwners.delete(sessionId);
        }
        notifyRetainInfo(sessionId);
      };
      return {
        sessionId,
        retainCount,
        ...(owner ? { owner } : {}),
        get binding() {
          return sessions.binding(sessionId);
        },
        get ready() {
          return Promise.resolve(sessions.binding(sessionId));
        },
        release() {
          if (released) return;
          released = true;
          releaseOwner();
        },
      };
    },
    async using(target, options, operation) {
      if (typeof operation !== "function") throw new Error("DSH sessions.using requires an operation");
      const reference = this.retain(target, options);
      try {
        return await operation(reference);
      } finally {
        reference.release();
      }
    },
    retainInfo(sessionId) {
      const id = text(sessionId);
      if (!id) {
        throw new Error(`DSH sessions.retainInfo requires a resolvable session id：${hostBodyText({ sessionId: sessionId ?? null })}`);
      }
      return retainObserver(id);
    },
    async prepare(sessionId, options = {}) {
      const request = { action: "conversation-create", draft: true };
      const id = text(sessionId);
      if (id) request.conversationId = id;
      const title = text(options?.title);
      if (title) request.title = title;
      const prepared = await command(request);
      const preparedId = text(prepared?.conversationId);
      if (!preparedId) throw hostResponseFailure("sessions.prepare", request, prepared);
      return Object.freeze({
        sessionId: preparedId,
        id: preparedId,
        draft: true,
        prepared: true,
        preparedAt: text(prepared?.preparedAt),
        binding: () => sessions.binding(preparedId),
        scope: () => sessions.scope(preparedId),
        async publish() {
          const input = { sessionId: preparedId };
          const publishTitle = text(options?.title);
          if (publishTitle) input.title = publishTitle;
          return sessions.create(input);
        },
        async drop() {
          return command({ action: "conversation-delete", conversationId: preparedId });
        },
      });
    },
    async create(input = {}) {
      const preset = text(input.agentPreset);
      let projectId = "";
      const workspaceId = text(input.workspaceId);
      if (workspaceId) {
        const target = workspaceByDshID.get(workspaceId);
        if (!target?.projectId) throw new Error(`DSH WorkspaceId ${workspaceId} has no verified Amitia ProjectId mapping`);
        projectId = target.projectId;
      }
      const cwd = text(input.cwd);
      if (cwd) {
        const target = workspaceByPath.get(cwd);
        if (!target?.projectId) throw new Error(`DSH cwd ${cwd} has no verified Amitia ProjectId mapping`);
        projectId = target.projectId;
      }
      const id = await createConversation(command, {
        ...input,
        projectId,
        conversationId: text(input.sessionId),
      });
      if (preset) await writeAgentPreset(id, preset, "session");
      await sessionCatalog.refresh();
      return id;
    },
    subagentAddress() {
      return undefined;
    },
    async refreshProjections() {
      await sessionCatalog.refresh();
      for (const feed of sessionFeeds.values()) await feed.refresh();
    },
    async refresh() {
      await sessionCatalog.refresh();
    },
    async search(query, signal) {
      try {
        throwIfAborted(signal);
        return remoteSuccess(await command({
          action: "conversation-search",
          query: text(query),
          limit: searchResultLimit,
        }));
      } catch (error) {
        return remoteOriginalFailure(error, "sessions.search");
      }
    },
    async fork(sessionId, boundary, options = {}) {
      const sourceId = text(sessionId?.sessionId) || text(sessionId);
      if (!sourceId) {
        throw new Error(`DSH sessions.fork 需要源会话标识：${JSON.stringify({ sessionId: sessionId ?? null, boundary: boundary ?? null })}`);
      }
      const title = text(options?.title);
      const request = { action: "conversation-fork", conversationId: sourceId };
      if (title) request.title = title;
      const created = await command(request);
      await sessionCatalog.refresh();
      const forkedId = text(created?.conversationId);
      if (!forkedId) throw hostResponseFailure("sessions.fork", request, created);
      return forkedId;
    },
    scope(sessionId) {
      const id = text(sessionId);
      if (!id) return undefined;
      const existing = sessionScopes.get(id);
      if (existing) return existing;
      const created = createCordisContext({ sessionId: id, conversation: conversationService(id) });
      sessionScopes.set(id, created);
      return created;
    },
    scopeOf(target) {
      const sessionId = sessionTargetId(target);
      if (!sessionId) {
        reportGap("sessions.scopeOf", `no resolvable Session target: ${JSON.stringify(target ?? null)}`);
        return undefined;
      }
      const existing = sessionScopes.get(sessionId);
      if (existing) return existing;
      const scope = createCordisContext({ sessionId });
      sessionScopes.set(sessionId, scope);
      return scope;
    },
    sessionOf(target) {
      const sessionId = sessionTargetId(target);
      if (!sessionId) {
        reportGap("sessions.sessionOf", `no resolvable Session scope: ${JSON.stringify(target ?? null)}`);
        return undefined;
      }
      return sessionId;
    },
    searchResultLimit,
    binding(sessionId) {
      const id = text(sessionId);
      if (!id) return undefined;
      const cached = sessionBindings.get(id);
      if (cached) return cached;
      const feed = createSessionEventFeed({ conversationId: id, command });
      sessionFeeds.set(id, feed);
      const listeners = new Set();
      const projectionFaces = new Map();
      const pendingSubmissions = [];
      let loadStatus = "idle";
      let promptFailure = null;
      let lastAgentError = null;
      let promptAttempted = false;
      let sessionSnapshot = null;
      const projectionValues = () => {
        const values = sessionCatalogState.projectionsBySession?.[id]?.values;
        return values && typeof values === "object" ? values : {};
      };
      const buildSnapshot = () => {
        const summary = sessionCatalogState.byId?.[id];
        const eventWindow = feed.getSnapshot();
        const entries = Array.isArray(eventWindow.entries) ? eventWindow.entries : [];
        const catalogReady = sessionCatalogState.phase === "ready";
        const hasMore = eventWindow.hasMore === true;
        return Object.freeze({
          sessionId: id,
          pendingSubmissions: Object.freeze(pendingSubmissions.map((item) => Object.freeze({ ...item }))),
          running: Boolean(summary?.running),
          subagent: null,
          removed: catalogReady && !summary,
          openState: catalogReady ? "open" : "cold",
          openError: null,
          hasMore,
          historyWasPartial: hasMore,
          loadingOlder: loadStatus === "loading",
          historyLoadStatus: loadStatus,
          promptError: promptFailure,
          blank: Boolean(summary?.blank),
          archived: Boolean(summary?.archived),
          pinned: Boolean(summary?.pinned),
          lastAgentError,
          promptAttempted,
          awaitingFirstTurn: promptAttempted && entries.length === 0,
        });
      };
      const notify = () => {
        sessionSnapshot = buildSnapshot();
        for (const listener of [...listeners]) {
          try {
            listener();
          } catch (error) {
            console.error("[original-error]", error);
          }
        }
      };
      const applyLoadStatus = (status) => {
        loadStatus = status;
        notify();
      };
      const sessionGap = (method, reason, missingHostAction, request) =>
        hostActionMissingError(`sessions.${method}`, reason, missingHostAction, request);
      sessionCatalog.subscribe(notify);
      feed.subscribe(notify);
      sessionSnapshot = buildSnapshot();
      const projections = Object.freeze({
        faceOf(key) {
          const name = text(key);
          if (!name) {
            throw new Error(`DSH sessions.binding(${id}).projections.faceOf requires a projection key：${hostBodyText({ key: key ?? null })}`);
          }
          const existing = projectionFaces.get(name);
          if (existing) return existing;
          const face = Object.freeze({
            key: name,
            getSnapshot: () => projectionValues()[name],
            subscribe(listener) {
              if (typeof listener !== "function") {
                throw new Error(`DSH sessions.binding(${id}).projections.faceOf(${name}) listener must be a function`);
              }
              return sessionCatalog.subscribe(listener);
            },
          });
          projectionFaces.set(name, face);
          return face;
        },
        keys() {
          return Object.keys(projectionValues());
        },
      });
      const loadOlderWindow = async (loadOptions) => {
        applyLoadStatus("loading");
        try {
          const window = await feed.expandWindow();
          await expandConversationData(id);
          applyLoadStatus(loadOptions?.deferConsumption === true ? "pendingConsumption" : "idle");
          return window;
        } catch (error) {
          applyLoadStatus("error");
          throw error;
        }
      };
      const retireSubmission = (requestId) => {
        const index = pendingSubmissions.findIndex((item) => item.requestId === requestId);
        if (index < 0) return false;
        pendingSubmissions.splice(index, 1);
        notify();
        return true;
      };
      const binding = Object.freeze({
        kind: "session",
        sessionId: id,
        session: Object.freeze({
          sessionId: id,
          projections,
          getSnapshot: () => sessionSnapshot,
          subscribe(listener) {
            if (typeof listener !== "function") throw new Error(`DSH sessions.binding(${id}).session listener must be a function`);
            listeners.add(listener);
            return () => {
              listeners.delete(listener);
            };
          },
          items() {
            return Object.keys(projectionValues());
          },
          loadOlder(loadOptions) {
            return loadOlderWindow(loadOptions);
          },
          consumeOlder() {
            if (loadStatus !== "pendingConsumption") return false;
            applyLoadStatus("idle");
            return true;
          },
          async loadThrough(seq) {
            const target = Number(seq);
            if (!Number.isSafeInteger(target) || target < 0) {
              throw new Error(`DSH sessions.loadThrough 需要非负整数 seq：${hostBodyText({ seq: seq ?? null })}`);
            }
            let window = feed.getSnapshot();
            for (let attempt = 0; attempt < 12; attempt += 1) {
              const events = window.entries.filter((entry) => entry.type === "event").map((entry) => entry.event);
              const covered = events.some((event) => Number(event.seq) >= target);
              if (covered || window.hasMore !== true) break;
              window = await feed.expandWindow();
            }
            const entries = window.entries
              .filter((entry) => entry.type === "event")
              .filter((entry) => Number(entry.event.seq) <= target);
            return Object.freeze({
              conversationId: id,
              seq: target,
              entries: Object.freeze(entries),
              hasMore: window.hasMore,
            });
          },
          async readAttachment(attachment) {
            const ref = attachment?.attachment && typeof attachment.attachment === "object" ? attachment.attachment : attachment;
            const uri = text(ref?.uri) || text(ref?.path);
            if (uri === "") {
              throw new Error(`DSH sessions.readAttachment 需要带可读路径的附件引用：${hostBodyText({ attachment: attachment ?? null })}`);
            }
            const filePath = uri.startsWith("file://") ? decodeURIComponent(uri.slice("file://".length)) : uri;
            const read = await command({ action: "attachment-read", path: filePath });
            const bytes = typeof read?.bytesBase64 === "string" ? read.bytesBase64 : "";
            return Object.freeze({
              attachmentId: text(ref?.attachmentId),
              name: text(ref?.name),
              mediaType: text(read?.mediaType) || text(ref?.mediaType),
              size: Number(read?.size) || 0,
              bytesBase64: bytes,
            });
          },
          updateQueue(itemId, action) {
            throw sessionGap(
              "updateQueue",
              "Amitia 发送队列没有 DSH 消息队列编辑、删除与转向的 host action",
              "conversation-queue-update（入参 { conversationId, itemId, action }）",
              { action: "conversation-queue-update", conversationId: id, itemId: itemId ?? null, queueAction: action ?? null },
            );
          },
          async cancel() {
            return command({ action: "conversation-interrupt", conversationId: id });
          },
          async rename(title) {
            const nextTitle = text(title);
            if (nextTitle === "") {
              throw new Error(`DSH sessions.rename 需要非空标题：${hostBodyText({ title: title ?? null })}`);
            }
            const updated = await command({ action: "conversation-update", conversationId: id, title: nextTitle });
            await sessionCatalog.refresh();
            await refreshConversationData(id);
            notify();
            return updated;
          },
          command(line) {
            throw sessionGap(
              "command",
              "Amitia 没有 DSH 斜杠命令路由的 host action",
              "conversation-command（入参 { conversationId, line }）",
              { action: "conversation-command", conversationId: id, line: line ?? null },
            );
          },
          beginSubmission(input) {
            const requestId = text(input?.requestId) || `dsh-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
            const submission = Object.freeze({
              requestId,
              placement: text(input?.mode) === "steer" ? "steer" : "queue",
              time: Date.now(),
              text: String(input?.text ?? ""),
              attachments: Object.freeze(Array.isArray(input?.attachments) ? input.attachments.slice() : []),
            });
            pendingSubmissions.push(submission);
            notify();
            return Object.freeze({
              requestId,
              placement: submission.placement,
              abandon() {
                retireSubmission(requestId);
              },
            });
          },
          async prompt(content, mode, signal, requestId) {
            promptAttempted = true;
            if (signal?.aborted) {
              const error = new Error(`DSH prompt aborted before delivery: ${id}`);
              error.name = "AbortError";
              promptFailure = Object.freeze({ op: "send", error: failureSnapshot(error) });
              notify();
              return { ok: false, error };
            }
            const parts = Array.isArray(content) ? content : [];
            const promptText = parts
              .filter((part) => text(part?.type) === "text")
              .map((part) => String(part.text ?? ""))
              .join("");
            if (!promptText.trim()) {
              const error = new Error("DSH prompt requires non-empty text content");
              promptFailure = Object.freeze({ op: "send", error: failureSnapshot(error) });
              notify();
              return { ok: false, error };
            }
            try {
              const result = await command({
                action: "conversation-message-send",
                conversationId: id,
                content: promptText,
                requestId: text(requestId),
              });
              retireSubmission(text(requestId));
              promptFailure = null;
              lastAgentError = null;
              await feed.refresh();
              await refreshConversationData(id);
              for (const delayMs of [400, 1200, 3000, 6000]) {
                setTimeout(() => {
                  void feed.refresh().catch((error) => console.error("[original-error]", error));
                  void refreshConversationData(id)?.catch?.((error) => console.error("[original-error]", error));
                }, delayMs);
              }
              notify();
              return remoteSuccess({ accepted: true, ...result });
            } catch (error) {
              promptFailure = Object.freeze({ op: "send", error: failureSnapshot(error) });
              lastAgentError = text(error?.message) || String(error);
              notify();
              reportGap("sessions.prompt", String(error?.message ?? error));
              throw error;
            }
          },
        }),
        eventSource: Object.freeze({
          getSnapshot: () => feed.getSnapshot(),
          subscribe: (listener) => feed.subscribe(listener),
        }),
        ctx: createCordisContext(options.context ?? options.ctx ?? {}),
      });
      sessionBindings.set(id, binding);
      return binding;
    },
  };

  const conversationService = (sessionId) => Object.freeze({
    sessionId,
    of(target) {
      const id = text(target?.sessionId ?? target?.id ?? target ?? sessionId) || sessionId;
      return Object.freeze({ sessionId: id, messages: Object.freeze([]), status: Object.freeze({ state: "idle" }) });
    },
    list() {
      return Object.freeze([]);
    },
    getSnapshot() {
      return Object.freeze({ sessionId, messages: Object.freeze([]), status: Object.freeze({ state: "idle" }) });
    },
    subscribe() {
      return () => {};
    },
  });

  const writeAgentPreset = async (targetId, preset, scope = "session") => {
    return command({ action: "agent-preset-write", targetId, preset, scope });
  };

  const composerDraftKey = (projectId) => {
    const project = text(projectId);
    return `webchat_draft:${project === "" ? "new:recent" : `new:project:${project}`}`;
  };
  const readComposerDraft = (projectId) => {
    if (typeof localStorage === "undefined") {
      throw new Error("DSH uiWorkspace.startSession 需要 Amitia 编辑器草稿存储，当前环境没有 localStorage");
    }
    return localStorage.getItem(composerDraftKey(projectId)) ?? "";
  };
  const writeComposerDraft = (projectId, value) => {
    if (typeof localStorage === "undefined") {
      throw new Error("DSH uiWorkspace.startSession 需要 Amitia 编辑器草稿存储，当前环境没有 localStorage");
    }
    const key = composerDraftKey(projectId);
    if (String(value).trim() === "") localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  };
  const resolveDraftMode = (input) => {
    const explicit = text(input?.whenDraftExists);
    if (explicit !== "") {
      if (explicit !== "replace" && explicit !== "append" && explicit !== "keep") {
        throw new Error(`DSH uiWorkspace.startSession 的 whenDraftExists 只支持 replace/append/keep：${JSON.stringify(input?.whenDraftExists)}`);
      }
      return explicit;
    }
    if (input?.clearPreviousDraft === true) return "replace";
    if (input?.clearPreviousDraft !== undefined && input?.clearPreviousDraft !== false) {
      throw new Error(`DSH uiWorkspace.startSession 的 clearPreviousDraft 必须是布尔值：${JSON.stringify(input?.clearPreviousDraft)}`);
    }
    return "keep";
  };

  const mostRecentWorkspaceSession = async (projectId) => {
    const target = text(projectId);
    if (target === "") return "";
    const candidates = [];
    for (let offset = 0; offset < 384; offset += 128) {
      const page = await command({ action: "conversation-list", limit: 128, offset });
      const items = Array.isArray(page?.items) ? page.items : [];
      for (const item of items) {
        if (text(item?.projectId) !== target) continue;
        const sessionId = text(item?.sessionId) || text(item?.id);
        if (sessionId === "") continue;
        candidates.push({ sessionId, updatedAt: Number(item?.updatedAt) || 0 });
      }
      if (items.length < 128) break;
    }
    if (candidates.length === 0) return "";
    candidates.sort((left, right) => right.updatedAt - left.updatedAt || left.sessionId.localeCompare(right.sessionId));
    return candidates[0].sessionId;
  };

  const uiWorkspace = {
    openSession(target) {
      const id = sessionTargetId(target);
      if (!id) {
        throw new Error(`DSH uiWorkspace.openSession requires a session id：${hostBodyText(target ?? null)}`);
      }
      return navigate(`/chat?conversationId=${encodeURIComponent(id)}`);
    },
    async startSession(workspaceId, input = {}) {
      const draftMode = resolveDraftMode(input);
      const targetID = text(workspaceId);
      let projectId = "";
      if (targetID) {
        const target = workspaceByDshID.get(targetID);
        if (!target?.projectId) {
          throw new Error(`DSH WorkspaceId ${targetID} has no verified Amitia ProjectId mapping：${hostBodyText({ workspaceId: targetID })}`);
        }
        projectId = target.projectId;
      }
      const preset = text(input.agentPreset);
      const prompt = text(input.prompt);
      const { agentPreset: _preset, prompt: _prompt, clearPreviousDraft: _clear, whenDraftExists: _mode, ...rest } = input ?? {};
      let content = prompt;
      if (draftMode !== "keep") {
        const draftText = readComposerDraft(projectId);
        if (prompt !== "" && draftMode === "append" && draftText !== "") {
          content = `${draftText}\n${prompt}`;
        }
        writeComposerDraft(projectId, "");
      }
      const id = await createConversation(command, { ...rest, projectId });
      if (preset) await writeAgentPreset(id, preset);
      if (content) {
        await command({ action: "conversation-message-send", conversationId: id, content });
      }
      await navigate(`/chat?conversationId=${encodeURIComponent(id)}`);
      return id;
    },
    async openWorkspace(workspaceId, beforeOpen, options = {}) {
      const id = text(workspaceId);
      const target = workspaceByDshID.get(id);
      if (!target?.projectId) {
        throw new Error(`DSH WorkspaceId ${id} has no verified Amitia ProjectId mapping：${hostBodyText({ workspaceId: id })}`);
      }
      const preset = text(options?.agentPreset);
      if (preset) await writeAgentPreset(target.projectId, preset, "workspace");
      if (typeof beforeOpen === "function") {
        const current = await mostRecentWorkspaceSession(target.projectId);
        if (!current) {
          throw new Error(`DSH uiWorkspace.openWorkspace 的 beforeOpen 需要可投影的 SessionId：工作区 ${JSON.stringify(id)} 没有会话`);
        }
        beforeOpen(current);
      }
      await navigate(`/workspaces?projectId=${encodeURIComponent(target.projectId)}`);
    },
    async connectWorkspace(workspaceId, options = {}) {
      const targetID = text(workspaceId);
      if (!targetID) {
        throw new Error(`DSH uiWorkspace.connectWorkspace requires a WorkspaceId：${hostBodyText({ workspaceId: workspaceId ?? null })}`);
      }
      const target = workspaceByDshID.get(targetID);
      if (!target?.projectId) {
        throw new Error(`DSH WorkspaceId ${targetID} has no verified Amitia ProjectId mapping：${hostBodyText({ workspaceId: targetID })}`);
      }
      const preset = text(options?.agentPreset);
      const id = await createConversation(command, { projectId: target.projectId });
      if (preset) await writeAgentPreset(id, preset, "session");
      await navigate(`/chat?conversationId=${encodeURIComponent(id)}`);
      return id;
    },
    async forkSession(sessionId, onCreated, options = {}) {
      const forkedId = await sessions.fork(sessionId, undefined, {});
      if (typeof onCreated === "function") onCreated(forkedId);
      return forkedId;
    },
    async archiveSession(sessionId, options = {}) {
      return workspaces.archiveSession(sessionId, options);
    },
    async unarchiveSession(sessionId) {
      return workspaces.unarchiveSession(sessionId);
    },
    async pickDirectory() {
      const desktop = globalThis.amitiaDesktop;
      if (desktop && typeof desktop.selectDirectory === "function") {
        const picked = await desktop.selectDirectory();
        return picked === undefined || picked === null ? null : String(picked);
      }
      const picker = typeof hostActions.selectDirectory === "function"
        ? hostActions.selectDirectory
        : hostActions.pickDirectory;
      if (typeof picker !== "function") {
        throw unsupported(
          "uiWorkspace.pickDirectory",
          "P13",
          `Amitia desktop directory picker bridge is unavailable：window.amitiaDesktop.selectDirectory 与 hostActions 都不可用；调用入参：${hostBodyText({})}`,
        );
      }
      const picked = await picker();
      return picked === undefined || picked === null ? null : String(picked);
    },
    async listDirectory(path, signal) {
      throwIfAborted(signal);
      const target = text(path);
      if (typeof command === "function") {
        const listing = await command({ action: "directory-list", ...(target ? { path: target } : {}) });
        throwIfAborted(signal);
        return listing;
      }
      if (typeof hostActions.listDirectory !== "function") {
        throw unsupported(
          "uiWorkspace.listDirectory",
          "P13",
          `Amitia directory listing bridge is unavailable；调用入参：${hostBodyText({ path: path ?? null })}`,
        );
      }
      const listing = await hostActions.listDirectory(target || undefined);
      throwIfAborted(signal);
      return listing;
    },
    async createDirectory(path, name) {
      if (!text(path) || !text(name)) {
        throw new Error(`DSH uiWorkspace.createDirectory requires a path and a name：${hostBodyText({ path: path ?? null, name: name ?? null })}`);
      }
      if (typeof command === "function") {
        return command({ action: "directory-create", path: text(path), name: text(name) });
      }
      if (typeof hostActions.createDirectory !== "function") {
        throw unsupported(
          "uiWorkspace.createDirectory",
          "P13",
          `Amitia directory creation bridge is unavailable；调用入参：${hostBodyText({ path: text(path), name: text(name) })}`,
        );
      }
      return hostActions.createDirectory(text(path), text(name));
    },
  };

  const loadPluginStatus = async () => {
    const status = await command({ action: "status" });
    return {
    notify: createNotifyService(),
      ...status,
      packages: Array.isArray(status?.packages) ? status.packages : [],
    };
  };

  const setPluginBundleEnabled = async (name, enabled) => {
    const pluginId = text(name);
    if (!pluginId) throw new Error("DSH plugin bundle name is required");
    const before = await loadPluginStatus();
    const installed = before.packages.find((plugin) => text(plugin.pluginId) === pluginId);
    if (!installed) throw new Error(`DSH plugin bundle ${pluginId} is not installed`);
    const status = await command({ action: "set-enabled", pluginId, enabled });
    return pluginChange("enable", pluginId, Boolean(status?.package?.enabled ?? enabled), Boolean(installed.enabled) !== Boolean(enabled));
  };

  const pluginManager = {
    async listBundles() {
      try {
        return remoteSuccess(pluginBundles((await loadPluginStatus()).packages));
      } catch (error) {
        return remoteOriginalFailure(error, "remote.pluginManager.listBundles");
      }
    },
    async listPlugins() {
      try {
        return remoteSuccess(pluginEntries((await loadPluginStatus()).packages));
      } catch (error) {
        return remoteOriginalFailure(error, "remote.pluginManager.listPlugins");
      }
    },
    async setBundleEnabled(name, enabled) {
      try {
        return remoteSuccess(await setPluginBundleEnabled(name, enabled));
      } catch (error) {
        return remoteOriginalFailure(error, "remote.pluginManager.setBundleEnabled");
      }
    },
    async setPluginEnabled(id, enabled) {
      const entryId = text(id);
      if (!entryId.endsWith("#root")) {
        return remoteSuccess({
          changed: false,
          application: "failed",
          stage: "enable",
          target: entryId,
          enabled: Boolean(enabled),
          error: {
            code: "management-required",
            diagnostic: `DSH plugin entry ${entryId} is managed together with its bundle`,
          },
        });
      }
      try {
        return remoteSuccess(await setPluginBundleEnabled(entryId.slice(0, -"#root".length), enabled));
      } catch (error) {
        return remoteOriginalFailure(error, "remote.pluginManager.setPluginEnabled");
      }
    },
    async installBundle(spec, options = {}) {
      try {
        const installed = await command({ action: "install", spec });
        const pluginId = text(installed?.package?.pluginId);
        if (!pluginId) {
          throw new Error(`Amitia DSH install returned no package identity: ${JSON.stringify(installed)}`);
        }
        if (!options?.enabled) return remoteSuccess(pluginChange("install", pluginId, false));
        try {
          return remoteSuccess(await setPluginBundleEnabled(pluginId, true));
        } catch (error) {
          return remoteSuccess({
            ...pluginChange("enable", pluginId, false),
            application: "failed",
            error: {
              code: "operation-error",
              diagnostic: text(error?.message) || String(error),
            },
          });
        }
      } catch (error) {
        return remoteOriginalFailure(error, "remote.pluginManager.installBundle");
      }
    },
    async removeBundle(name) {
      try {
        const pluginId = text(name);
        if (!pluginId) throw new Error("DSH plugin bundle name is required");
        await command({ action: "remove", pluginId });
        return remoteSuccess(pluginChange("remove", pluginId, false));
      } catch (error) {
        return remoteOriginalFailure(error, "remote.pluginManager.removeBundle");
      }
    },
    async inspect() {
      return remoteFailure("PLUGIN_INSPECT_UNAVAILABLE", "remote.pluginManager.inspect", "P12", "Amitia DSH host does not yet expose pre-install package inspection");
    },
    async registries() {
      return remoteSuccess({ registry: null, fallbackRegistries: [], resolved: null });
    },
    async listVersions(name) {
      return remoteSuccess({
        status: "refused",
        problem: "not-updatable",
        reason: `DSH plugin ${text(name)} is not backed by an Amitia registry version catalog`,
        attempts: [],
      });
    },
    async updateBundle(name) {
      return remoteFailure("PLUGIN_UPDATE_UNAVAILABLE", "remote.pluginManager.updateBundle", "P12", `Amitia DSH host cannot update ${text(name)} from a registry catalog`);
    },
    async readBundleDetail(name) {
      return remoteFailure("PLUGIN_DETAIL_UNAVAILABLE", "remote.pluginManager.readBundleDetail", "P12", `Amitia DSH host cannot read documentation for ${text(name)}`);
    },
    async waitForInstall() {
      return remoteSuccess(null);
    },
    async cancelInstall() {
      return remoteSuccess({ status: "not-running" });
    },
    async listVersionExemptions() {
      return remoteSuccess({ exemptions: {}, warnings: [] });
    },
    async setVersionExemption(packageVersion) {
      return remoteFailure("PLUGIN_VERSION_EXEMPTION_UNAVAILABLE", "remote.pluginManager.setVersionExemption", "P12", `Amitia DSH host has no compatibility exemption catalog for ${text(packageVersion)}`);
    },
  };

  const pluginInventory = {
    async list() {
      try {
        return remoteSuccess({
          managementAvailable: true,
          entries: pluginEntries((await loadPluginStatus()).packages),
        });
      } catch (error) {
        return remoteOriginalFailure(error, "remote.pluginInventory.list");
      }
    },
  };

  const pluginRegistryProbe = {
    async fastest() {
      return remoteSuccess(null);
    },
  };

  const fileUpload = {
    async upload(sessionId, file, name, signal, onProgress) {
      try {
        throwIfAborted(signal);
        const filename = text(name) || text(file?.name);
        const payload = await uploadArtifact({
          sessionId: text(sessionId),
          file,
          filename,
          kind: "file",
          signal,
          onProgress,
        });
        const artifact = payload?.artifact || payload?.data?.artifact;
        const receiptId = text(artifact?.artifactId);
        if (!receiptId) throw new Error(`Amitia artifact upload returned no artifact identity: ${JSON.stringify(payload)}`);
        return remoteSuccess({ receiptId });
      } catch (error) {
        return remoteOriginalFailure(error, "fileUpload.upload");
      }
    },
    async uploadImage(sessionId, file, mediaType, name, signal, onProgress) {
      try {
        throwIfAborted(signal);
        const mimeType = text(mediaType).toLowerCase();
        if (!["image/png", "image/jpeg", "image/webp", "image/gif"].includes(mimeType)) {
          throw new Error(`DSH image attachment media type is unsupported: ${mimeType || "<empty>"}`);
        }
        const filename = text(name) || text(file?.name);
        const payload = await uploadArtifact({
          sessionId: text(sessionId),
          file,
          filename,
          kind: "image",
          mediaType: mimeType,
          signal,
          onProgress,
        });
        const artifact = payload?.artifact || payload?.data?.artifact;
        const receiptId = text(artifact?.artifactId);
        if (!receiptId) throw new Error(`Amitia image artifact upload returned no artifact identity: ${JSON.stringify(payload)}`);
        return remoteSuccess({ receiptId });
      } catch (error) {
        return remoteOriginalFailure(error, "fileUpload.uploadImage");
      }
    },
  };

  const directoryPicker = {
    async pick(signal) {
      throwIfAborted(signal);
      if (typeof hostActions.selectDirectory !== "function") {
        return remoteFailure("DIRECTORY_PICKER_UNAVAILABLE", "remote.directoryPicker.pick", "P12", "Amitia host directory selection is unavailable");
      }
      try {
        const selected = await hostActions.selectDirectory();
        const path = text(typeof selected === "string" ? selected : selected?.path);
        return remoteSuccess(path || null);
      } catch (error) {
        return remoteOriginalFailure(error, "remote.directoryPicker.pick");
      }
    },
    async list(path, signal) {
      throwIfAborted(signal);
      if (typeof hostActions.listDirectory !== "function") {
        return remoteFailure("DIRECTORY_PICKER_UNAVAILABLE", "remote.directoryPicker.list", "P12", "Amitia host directory browsing is unavailable on this platform");
      }
      try {
        const value = await hostActions.listDirectory(text(path) || undefined, signal);
        return remoteSuccess(value);
      } catch (error) {
        return remoteOriginalFailure(error, "remote.directoryPicker.list");
      }
    },
    async createDirectory(path, name) {
      if (typeof hostActions.createDirectory !== "function") {
        return remoteFailure("DIRECTORY_PICKER_UNAVAILABLE", "remote.directoryPicker.createDirectory", "P12", "Amitia host directory creation is unavailable on this platform");
      }
      try {
        return remoteSuccess(await hostActions.createDirectory(text(path), text(name)));
      } catch (error) {
        return remoteOriginalFailure(error, "remote.directoryPicker.createDirectory");
      }
    },
  };

  const workspaceTargetSnapshot = async (sessionId, path) => {
    const scope = text(sessionId);
    const target = text(path);
    if (!scope) throw new Error("DSH workspaceFiles.changes requires a session id");
    let fileError;
    try {
      const stat = await command({
        action: "workspace-files-stat",
        sessionId: scope,
        path: target,
      });
      return {
        kind: "file",
        absolutePath: text(stat?.absolutePath) || target,
        version: text(stat?.version) || "unknown",
      };
    } catch (error) {
      fileError = error;
    }
    try {
      const listing = await command({
        action: "workspace-files-list",
        sessionId: scope,
        path: target,
      });
      return {
        kind: "directory",
        absolutePath: target,
        version: workspaceDirectoryVersion(listing),
      };
    } catch (directoryError) {
      throw new AggregateError(
        [fileError, directoryError],
        `Amitia workspace target cannot be read as a file or directory: ${target}`,
        { cause: fileError },
      );
    }
  };

  const workspaceFileChanges = async function* (sessionId, path, signal) {
    throwIfAborted(signal);
    let current = await workspaceTargetSnapshot(sessionId, path);
    yield { kind: "ready" };
    while (!signal?.aborted) {
      await waitForWorkspacePoll(workspaceChangesIntervalMs, signal);
      let next;
      try {
        next = await workspaceTargetSnapshot(sessionId, path);
      } catch (error) {
        if (!workspaceTargetMissing(error)) throw error;
        if (current.kind === "absent") continue;
        current = { kind: "absent", absolutePath: text(path) };
        yield {
          kind: "change",
          change: { absolutePath: current.absolutePath, absent: true },
        };
        continue;
      }
      throwIfAborted(signal);
      if (next.kind === current.kind && next.version === current.version) continue;
      current = next;
      yield {
        kind: "change",
        change: {
          absolutePath: next.absolutePath,
          version: next.version,
        },
      };
    }
  };

  const liveToolResult = async (request) => {
    const sessionId = text(request?.sessionId);
    const seq = Number(request?.seq);
    if (!sessionId) throw new Error("DSH toolResults.watch requires a session id");
    if (!Number.isSafeInteger(seq) || seq < 0) throw new Error("DSH toolResults.watch requires a non-negative safe sequence");
    return command({
      action: "tool-result-watch",
      sessionId,
      seq,
    });
  };

  const liveToolResultFrames = async function* (request, signal) {
    throwIfAborted(signal);
    const current = await liveToolResult(request);
    if (signal?.aborted) return;
    yield {
      available: current?.available === true,
      ...(current?.available === true ? { value: current.value } : {}),
    };
    let signature = JSON.stringify(current ?? null);
    while (!signal?.aborted) {
      try {
        await waitForWorkspacePoll(workspaceChangesIntervalMs, signal);
      } catch (error) {
        if (signal?.aborted && error === signal.reason) return;
        throw error;
      }
      const next = await liveToolResult(request);
      if (signal?.aborted) return;
      const nextSignature = JSON.stringify(next ?? null);
      if (nextSignature === signature) continue;
      signature = nextSignature;
      yield {
        available: next?.available === true,
        ...(next?.available === true ? { value: next.value } : {}),
      };
    }
  };

  const mapScheduleCreatedSession = async (request) => {
    if (!request || typeof request !== "object" || !request.target || typeof request.target !== "object") return request;
    const targetKind = text(request.target.kind);
    if (targetKind !== "new" && targetKind !== "new_once") return request;
    const create = request.target.create === undefined
      ? {}
      : request.target.create;
    if (!create || typeof create !== "object" || Array.isArray(create)) {
      throw new Error("DSH Schedule target.create must be an object");
    }
    const cwd = text(create.cwd);
    if (!cwd) return { ...request, target: { ...request.target, create: { ...create } } };
    if (workspaceByPath.size === 0) await sessionCatalog.refresh();
    const workspace = workspaceByPath.get(cwd);
    const projectId = text(workspace?.projectId);
    if (!projectId) throw new Error(`DSH Schedule cwd ${cwd} has no verified Amitia ProjectId mapping`);
    return {
      ...request,
      target: {
        ...request.target,
        create: {
          ...create,
          cwd,
          projectId,
        },
      },
    };
  };

  const remote = {
    $stream(options = {}) {
      const controller = new AbortController();
      let active = true;
      return {
        async *[Symbol.asyncIterator]() {
          const handle = await options.open(controller.signal);
          for await (const frame of handle) {
            if (!active) return;
            yield frame;
          }
        },
        async dispose() {
          if (!active) return;
          active = false;
          controller.abort();
        },
      };
    },
    $on(event, listener) {
      if (!text(event)) throw new Error("DSH remote event type is required");
      if (typeof listener !== "function") throw new Error("DSH remote event listener must be a function");
      return () => {};
    },
    commands: {
      async execute() {
        return remoteFailure("REMOTE_UNAVAILABLE", "remote.commands.execute", "P13", "generated Remote endpoint mapping is unavailable");
      },
    },
    job: createRemoteJobNamespace(command),
    pluginManager,
    pluginInventory,
    pluginRegistryProbe,
    directoryPicker,
    toolResults: {
      watch(request, signal) {
        return liveToolResultFrames(request, signal);
      },
    },
    session: {
      async modelCatalog() {
        return remoteFailure("SESSION_MODEL_CATALOG_UNAVAILABLE", "remote.session.modelCatalog", "P11", "Amitia DSH model catalog projection is unavailable");
      },
      async *events() {
        throw unsupported("remote.session.events", "P13", "generated Remote Session stream mapping is unavailable");
      },
      async page() {
        return remoteFailure("REMOTE_UNAVAILABLE", "remote.session.page", "P13", "generated Remote Session pagination mapping is unavailable");
      },
    },
    credentials: {
      async describe() {
        return remoteFailure("CREDENTIALS_UNAVAILABLE", "remote.credentials.describe", "P11", "Amitia DSH credential directory projection is unavailable");
      },
      async set() {
        return remoteFailure("CREDENTIALS_UNAVAILABLE", "remote.credentials.set", "P11", "Amitia DSH credential write projection is unavailable");
      },
      async unset() {
        return remoteFailure("CREDENTIALS_UNAVAILABLE", "remote.credentials.unset", "P11", "Amitia DSH credential removal projection is unavailable");
      },
    },
    llm: {
      async listProviders() {
        return remoteFailure("LLM_UNAVAILABLE", "remote.llm.listProviders", "P11", "Amitia DSH provider directory projection is unavailable");
      },
      async listConfigurableProviders() {
        return remoteFailure("LLM_UNAVAILABLE", "remote.llm.listConfigurableProviders", "P11", "Amitia DSH configurable provider projection is unavailable");
      },
      async discoverModels() {
        return remoteFailure("LLM_UNAVAILABLE", "remote.llm.discoverModels", "P11", "Amitia DSH model discovery projection is unavailable");
      },
    },
    schedule: {
      async catalog() {
        return remoteSuccess(await command({ action: "schedule-catalog" }));
      },
      async create(request) {
        return remoteSuccess(await command({
          action: "schedule-create",
          request: await mapScheduleCreatedSession(request),
        }));
      },
      async deleteInactive() {
        return remoteSuccess(await command({ action: "schedule-delete-inactive" }));
      },
      async deliveryLimits() {
        return remoteSuccess({ newSessionIntervalSeconds: 60 });
      },
      async edit(request) {
        const action = text(request?.action);
        if (action === "update") {
          return remoteSuccess(await command({
            action: "schedule-update",
            scheduleId: text(request.id),
            patch: {
              ...request.patch,
              ...(request.patch?.target === undefined
                ? {}
                : { target: (await mapScheduleCreatedSession({ target: request.patch.target })).target }),
            },
            expectedUpdatedAt: request.expectedUpdatedAt,
          }));
        }
        if (action !== "delete") return scheduleUnavailable(`edit.${action || "unknown"}`);
        return remoteSuccess(await command({
          action: "schedule-cancel",
          scheduleId: text(request.id),
          expectedUpdatedAt: request.expectedUpdatedAt,
        }));
      },
      async history(request) {
        return remoteSuccess(await command({
          action: "schedule-history",
          scheduleId: text(request?.id),
          limit: Number(request?.limit),
          before: text(request?.before),
        }));
      },
      async list(request = {}) {
        return remoteSuccess(await command({
          action: "schedule-list",
          active: request.active,
          limit: request.limit,
          offset: request.offset,
        }));
      },
      async run(request) {
        return remoteSuccess(await command({ action: "schedule-run", scheduleId: text(request?.id) }));
      },
      async view(request) {
        return remoteSuccess(await command({
          action: "schedule-view",
          scheduleId: text(request?.id),
        }));
      },
    },
    workspaceFiles: {
      async read(sessionId, path, range = {}, signal) {
        try {
          throwIfAborted(signal);
          return remoteSuccess(await command({
            action: "workspace-files-read",
            sessionId: text(sessionId),
            path: text(path),
            offset: range?.offset,
            limit: range?.limit,
          }));
        } catch (error) {
          return remoteOriginalFailure(error, "remote.workspaceFiles.read");
        }
      },
      async readBytes(sessionId, path, options = {}, signal) {
        try {
          throwIfAborted(signal);
          const value = await command({
            action: "workspace-files-read-bytes",
            sessionId: text(sessionId),
            path: text(path),
            baseFile: text(options?.baseFile),
            offset: options?.range?.offset,
            length: options?.range?.length,
          });
          if (!value || typeof value !== "object") {
            throw new Error(`DSH workspaceFiles.readBytes returned an invalid value: ${JSON.stringify(value)}`);
          }
          const { dataBase64, ...metadata } = value;
          return remoteSuccess({ ...metadata, data: decodeBytes(dataBase64) });
        } catch (error) {
          return remoteOriginalFailure(error, "remote.workspaceFiles.readBytes");
        }
      },
      async stat(sessionId, path, signal) {
        try {
          throwIfAborted(signal);
          return remoteSuccess(await command({
            action: "workspace-files-stat",
            sessionId: text(sessionId),
            path: text(path),
          }));
        } catch (error) {
          return remoteOriginalFailure(error, "remote.workspaceFiles.stat");
        }
      },
      async list(sessionId, path, signal) {
        try {
          throwIfAborted(signal);
          return remoteSuccess(await command({
            action: "workspace-files-list",
            sessionId: text(sessionId),
            path: text(path),
          }));
        } catch (error) {
          return remoteOriginalFailure(error, "remote.workspaceFiles.list");
        }
      },
      async search(sessionId, request, signal) {
        try {
          throwIfAborted(signal);
          return remoteSuccess(await command({
            action: "workspace-files-search",
            sessionId: text(sessionId),
            request,
          }));
        } catch (error) {
          return remoteOriginalFailure(error, "remote.workspaceFiles.search");
        }
      },
      changes(sessionId, path, signal) {
        return workspaceFileChanges(sessionId, path, signal);
      },
    },
  };

  const workspaceCatalog = createRuntimeSnapshot(workspaceCatalogState, async () => {
    await sessionCatalog.refresh();
    return workspaceCatalogState;
  });
  const notifyHostSlots = () => {
    globalThis.__dshBumpSlots?.();
    const notifiers = globalThis.__dshSlotNotifiers;
    if (!notifiers || typeof notifiers.forEach !== "function") return;
    notifiers.forEach((notify) => {
      if (typeof notify === "function") notify();
    });
  };
  globalThis.__dshLiveCatalogs = {
    sessions: {
      getSnapshot: () => sessionCatalog.getSnapshot(),
      subscribe: (listener) => sessionCatalog.subscribe(listener),
    },
    workspaces: {
      getSnapshot: () => workspaceCatalog.getSnapshot(),
      subscribe: (listener) => workspaceCatalog.subscribe(listener),
    },
  };
  const workspaceHostCall = async (operation, request) => {
    let response;
    try {
      response = await command(request);
    } catch (error) {
      throw hostCommandFailure(operation, request, error);
    }
    if (response && typeof response === "object") {
      const status = text(response.status).toLowerCase();
      if (response.ok === false || response.success === false || hostFailureStatuses.has(status)) {
        throw hostResponseFailure(operation, request, response);
      }
    }
    return response;
  };
  const workspaces = {
    list: {
      getSnapshot() {
        return workspaceCatalog.getSnapshot();
      },
      subscribe(listener) {
        return workspaceCatalog.subscribe(listener);
      },
    },
    refresh() {
      return workspaceCatalog.refresh();
    },
    async initializeDefault(signal) {
      throwIfAborted(signal);
      const known = workspaceCatalogState.items[0];
      if (known) return known;
      if (!defaultWorkspaceInitialization) {
        defaultWorkspaceInitialization = (async () => {
          await workspaceCatalog.refresh();
          const current = workspaceCatalogState.items[0];
          if (current) return current;
          if (typeof hostActions.listDirectory !== "function") {
            throw unsupported(
              "workspaces.initializeDefault",
              "P13",
              "Amitia host directory discovery is unavailable, so no default workspace can be created",
            );
          }
          const listing = await hostActions.listDirectory();
          const directoryPath = text(listing?.home) || text(listing?.path);
          if (!directoryPath) {
            throw new Error(`Amitia host directory discovery returned no home directory: ${JSON.stringify(listing)}`);
          }
          return await workspaces.create({ path: directoryPath, title: workspaceBasename(directoryPath) });
        })().finally(() => {
          defaultWorkspaceInitialization = null;
        });
      }
      const value = await defaultWorkspaceInitialization;
      throwIfAborted(signal);
      return value;
    },
    async create(input = {}) {
      const directoryPath = text(typeof input === "string" ? input : input?.path);
      if (!directoryPath) throw new Error("DSH workspaces.create requires a path");
      const title = text(typeof input === "string" ? "" : input?.title);
      const request = { action: "project-create", title, directoryPath };
      const created = await workspaceHostCall("workspaces.create", request);
      const workspace = created?.workspace;
      if (!workspace || !text(workspace.workspaceId)) {
        throw new Error(`Amitia project-create 未返回可用工作区：${hostBodyText(created)}`);
      }
      await workspaceCatalog.refresh();
      return workspaceCatalogState.items.find((item) => item.workspaceId === text(workspace.workspaceId)) ?? workspace;
    },
    async rename(workspaceId, title) {
      const id = text(workspaceId);
      const target = workspaceByDshID.get(id);
      if (!target?.projectId) throw new Error(`DSH WorkspaceId ${id} has no verified Amitia ProjectId mapping`);
      const nextTitle = text(title);
      if (!nextTitle) throw new Error(`DSH workspaces.rename 需要非空 title：${hostBodyText({ workspaceId: id, title: title ?? null })}`);
      const request = { action: "project-update", projectId: target.projectId, title: nextTitle };
      const updated = await workspaceHostCall("workspaces.rename", request);
      await workspaceCatalog.refresh();
      const view = workspaceCatalogState.items.find((item) => item.workspaceId === id);
      if (!view) {
        throw new Error(`DSH workspaces.rename 后无法在 Amitia 工作区目录确认工作区 ${id}：${hostBodyText(updated)}`);
      }
      return view;
    },
    async delete(workspaceId) {
      const id = text(workspaceId);
      const target = workspaceByDshID.get(id);
      if (!target?.projectId) throw new Error(`DSH WorkspaceId ${id} has no verified Amitia ProjectId mapping`);
      const request = { action: "project-delete", projectId: target.projectId };
      const removed = await workspaceHostCall("workspaces.delete", request);
      if (removed && typeof removed === "object" && removed.deleted === false) {
        throw hostResponseFailure("workspaces.delete", request, removed);
      }
      await workspaceCatalog.refresh();
      return true;
    },
    async archiveSession(sessionId, options = {}) {
      const id = text(sessionId);
      if (!id) throw new Error("DSH workspaces.archiveSession requires a session id");
      const request = { action: "conversation-archive", conversationId: id, archived: true, stopActivity: options?.stopActivity === true };
      const archived = await workspaceHostCall("workspaces.archiveSession", request);
      if (archived && typeof archived === "object" && archived.archived === false) {
        throw hostResponseFailure("workspaces.archiveSession", request, archived);
      }
      await workspaceCatalog.refresh();
      await sessionCatalog.refresh();
      setConversationFlags(id, { archived: true });
      await refreshConversationData(id);
      notifyHostSlots();
    },
    async unarchiveSession(sessionId) {
      const id = text(sessionId);
      if (!id) throw new Error("DSH workspaces.unarchiveSession requires a session id");
      const request = { action: "conversation-archive", conversationId: id, archived: false };
      const unarchived = await workspaceHostCall("workspaces.unarchiveSession", request);
      if (unarchived && typeof unarchived === "object" && unarchived.archived === true) {
        throw hostResponseFailure("workspaces.unarchiveSession", request, unarchived);
      }
      await workspaceCatalog.refresh();
      await sessionCatalog.refresh();
      setConversationFlags(id, { archived: false });
      await refreshConversationData(id);
      notifyHostSlots();
    },
    async pinSession(sessionId, options = {}) {
      const id = text(sessionId);
      if (!id) throw new Error("DSH workspaces.pinSession requires a session id");
      const request = { action: "conversation-pin", conversationId: id, pinned: true, requestId: text(options?.requestId) || undefined };
      const pinned = await workspaceHostCall("workspaces.pinSession", request);
      if (pinned && typeof pinned === "object" && pinned.pinned === false) {
        throw hostResponseFailure("workspaces.pinSession", request, pinned);
      }
      setConversationFlags(id, { pinned: true });
      await sessionCatalog.refresh();
      await refreshConversationData(id);
      notifyHostSlots();
    },
    async unpinSession(sessionId, options = {}) {
      const id = text(sessionId);
      if (!id) throw new Error("DSH workspaces.unpinSession requires a session id");
      const request = { action: "conversation-pin", conversationId: id, pinned: false, requestId: text(options?.requestId) || undefined };
      const unpinned = await workspaceHostCall("workspaces.unpinSession", request);
      if (unpinned && typeof unpinned === "object" && unpinned.pinned === true) {
        throw hostResponseFailure("workspaces.unpinSession", request, unpinned);
      }
      await sessionCatalog.refresh();
      setConversationFlags(id, { pinned: false });
      await refreshConversationData(id);
      notifyHostSlots();
    },
    async insertBefore(workspaceId, beforeWorkspaceId) {
      const id = text(workspaceId);
      if (!id) throw new Error("DSH workspaces.insertBefore requires a workspace id");
      const target = workspaceByDshID.get(id);
      if (!target?.projectId) throw new Error(`DSH WorkspaceId ${id} has no verified Amitia ProjectId mapping`);
      const beforeId = text(beforeWorkspaceId);
      let beforeProjectId = "";
      if (beforeId) {
        const before = workspaceByDshID.get(beforeId);
        if (!before?.projectId) throw new Error(`DSH WorkspaceId ${beforeId} has no verified Amitia ProjectId mapping`);
        beforeProjectId = before.projectId;
      }
      const request = {
        action: "project-reorder",
        projectId: target.projectId,
        beforeProjectId,
        requestId: `workspaces.insertBefore:${target.projectId}:${beforeProjectId}:${Date.now()}`,
      };
      await workspaceHostCall("workspaces.insertBefore", request);
      await workspaceCatalog.refresh();
      notifyHostSlots();
    },
    async insertSessionBefore(workspaceId, sessionId, beforeSessionId) {
      const id = text(sessionId);
      if (!id) throw new Error("DSH workspaces.insertSessionBefore requires a session id");
      const workspace = text(workspaceId);
      let projectId = "";
      if (workspace) {
        const target = workspaceByDshID.get(workspace);
        if (!target?.projectId) throw new Error(`DSH WorkspaceId ${workspace} has no verified Amitia ProjectId mapping`);
        projectId = target.projectId;
      }
      const request = {
        action: "conversation-reorder",
        conversationId: id,
        beforeConversationId: text(beforeSessionId),
        projectId,
        requestId: `workspaces.insertSessionBefore:${id}:${text(beforeSessionId)}:${Date.now()}`,
      };
      await workspaceHostCall("workspaces.insertSessionBefore", request);
      await workspaceCatalog.refresh();
      await sessionCatalog.refresh();
      notifyHostSlots();
      const view = workspaceCatalogState.items.find((item) => item.workspaceId === workspace);
      if (!view) {
        throw new Error(`DSH workspaces.insertSessionBefore 后无法在 Amitia 工作区目录确认工作区 ${JSON.stringify(workspace)}`);
      }
      return view;
    },
  };

  const runAccessEffect = (callback) => {
    if (typeof callback !== "function") throw new Error("DSH remoteAccess callback is required");
    const effects = [];
    const pending = [];
    consumeEffect(callback(), effects, pending);
    let active = true;
    const dispose = async () => {
      if (!active) return;
      active = false;
      await Promise.all(pending);
      for (let index = effects.length - 1; index >= 0; index -= 1) {
        await effects[index]?.();
      }
    };
    cleanup.push(dispose);
    return dispose;
  };

  const accessSource = (resource, id, action) => {
    const resourceName = text(resource);
    if (!resourceName) throw new Error(`DSH remoteAccess resource is required: ${JSON.stringify({ resource, id, action })}`);
    const snapshot = Object.freeze({
      resource: resourceName,
      id: text(id),
      action: text(action),
      allowed: true,
    });
    return Object.freeze({
      ...snapshot,
      getSnapshot() {
        return snapshot;
      },
      subscribe(listener) {
        if (typeof listener !== "function") throw new Error("DSH remoteAccess.source listener must be a function");
        return () => {};
      },
      whenAccess(callback) {
        return runAccessEffect(callback);
      },
      dispose() {},
    });
  };

  const remoteAccess = {
    source(resource, id, action) {
      return accessSource(resource, id, action);
    },
    whenAccess(_action, callback) {
      return runAccessEffect(callback);
    },
    whenResourceAccess(resource, id, action, callback) {
      accessSource(resource, id, action);
      return runAccessEffect(callback);
    },
  };

  void sessionCatalog.refresh()
    .then(() => disposed ? undefined : workspaceCatalog.refresh())
    .catch((error) => {
      if (!disposed) console.error("[original-error]", error);
    });

  return {
    layout,
    theme,
    timer,
    sessions,
    uiWorkspace,
    remote,
    fileUpload,
    pluginInventory,
    pluginManager,
    pluginRegistryProbe,
    workspaces,
    remoteAccess,
    conversation: {},
    dispose: async () => {
      disposed = true;
      for (let index = cleanup.length - 1; index >= 0; index -= 1) await cleanup[index]?.();
      cleanup.length = 0;
      panelInfoListeners.clear();
      navigationController.abort();
      theme.dispose();
      timer.dispose();
      sessionCatalog.dispose();
      for (const feed of sessionFeeds.values()) feed.dispose();
      sessionFeeds.clear();
      workspaceCatalog.dispose();
    },
  };
}
