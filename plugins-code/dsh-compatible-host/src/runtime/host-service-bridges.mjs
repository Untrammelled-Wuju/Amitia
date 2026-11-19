import { randomUUID } from "node:crypto";
import { SESSION_STATE_VERSION, isSurfaceEventType, projectSurfaceEvents, restoreSessionEvents } from "./session-events.mjs";

const CREDENTIAL_REFERENCE = /^[A-Za-z_][A-Za-z0-9_]*$/;
const COMMAND_NAME = /^[a-z0-9][a-z0-9._:-]*$/;
const STATE_WRITE_ATTEMPTS = 8;
const CONVERSATION_PAGE_LIMIT = 128;
const CONVERSATION_PAGE_GUARD = 10000;

function text(value) {
  return value === undefined || value === null ? "" : String(value).trim();
}

function fail(message, details) {
  const error = new Error(message);
  error.code = details.code;
  error.details = details;
  return error;
}

function unavailable(operation, detail) {
  return fail(`DSH ${operation} 没有可用的 Amitia 等价执行语义: ${detail}`, {
    code: "DSH_HOST_OPERATION_UNAVAILABLE",
    operation,
    detail,
  });
}

function unavailableMissing(operation, detail, missing) {
  const error = fail(`DSH ${operation} 没有可用的 Amitia 等价执行语义: ${detail}；宿主缺少接口 ${missing}`, {
    code: "DSH_HOST_OPERATION_UNAVAILABLE",
    operation,
    detail,
    missingHostInterface: missing,
  });
  error.missingHostInterface = missing;
  return error;
}

function attachOriginalError(target, error) {
  Object.defineProperty(target, "error", {
    value: error,
    enumerable: false,
    configurable: true,
    writable: true,
  });
  return target;
}

function errorText(error) {
  const message = text(error?.message);
  return message || String(error);
}

async function readStateEntry(call, key) {
  const response = await call("host.state.get", { key });
  return {
    found: response?.found === true,
    value: response?.found ? response.value : undefined,
    version: Number(response?.version) || 0,
  };
}

async function writeStateEntry(call, key, mutate) {
  for (let attempt = 0; attempt < STATE_WRITE_ATTEMPTS; attempt += 1) {
    const current = await readStateEntry(call, key);
    const next = await mutate(current.value, current.found);
    if (next === undefined) return current.value;
    const result = await call("host.state.cas", {
      key,
      expectedVersion: current.version,
      newValue: next,
    });
    if (result?.swapped) return next;
  }
  throw fail(`Amitia host.state.cas 连续 ${STATE_WRITE_ATTEMPTS} 次拒绝 ${key}`, {
    code: "DSH_STATE_WRITE_CONFLICT",
    key,
    attempts: STATE_WRITE_ATTEMPTS,
  });
}

export function createCredentialsService(call) {
  const RECORD_INDEX_KEY = "dsh-credential-record-index";
  const referenceStateKey = (name) => `dsh-credential-reference:${name}`;
  const recordStateKey = (name) => `dsh-credential-record:${name}`;

  const referenceName = (ref) => {
    const name = text(ref);
    if (!name) {
      throw fail("DSH credential reference is required", {
        code: "DSH_CREDENTIAL_REFERENCE_INVALID",
        reference: ref ?? null,
      });
    }
    if (!CREDENTIAL_REFERENCE.test(name)) {
      throw fail(`DSH credential reference "${name}" 不是 POSIX 环境变量名`, {
        code: "DSH_CREDENTIAL_REFERENCE_INVALID",
        reference: name,
      });
    }
    return name;
  };

  const recordName = (key) => {
    const name = text(key);
    if (!name) {
      throw fail("DSH credential record key is required", {
        code: "DSH_CREDENTIAL_RECORD_INVALID",
        key: key ?? null,
      });
    }
    return name;
  };

  const maskSecret = (value) => {
    if (value.length <= 4) return "*".repeat(value.length);
    return `${"*".repeat(Math.max(4, value.length - 2))}${value.slice(-2)}`;
  };

  const storedValue = async (name) => {
    const entry = await readStateEntry(call, referenceStateKey(name));
    if (!entry.found) return undefined;
    const value = typeof entry.value === "string" ? entry.value : "";
    return value === "" ? undefined : value;
  };

  const ambientValue = (name) => {
    const value = process.env[name];
    return typeof value === "string" && value !== "" ? value : undefined;
  };

  const recordKind = (record) => {
    const kind = text(record?.kind);
    return kind || "opaque";
  };

  const readRecordIndex = async () => {
    const entry = await readStateEntry(call, RECORD_INDEX_KEY);
    if (!entry.found) return [];
    if (!Array.isArray(entry.value)) {
      throw fail("Amitia 存储中的 DSH credential record 索引不是数组", {
        code: "DSH_CREDENTIAL_RECORD_INDEX_INVALID",
        value: entry.value ?? null,
      });
    }
    return entry.value.map((item) => text(item)).filter((item) => item !== "");
  };

  const writeRecordIndex = (names) => writeStateEntry(call, RECORD_INDEX_KEY, () => Array.from(new Set(names)));

  return {
    async resolve(ref) {
      const name = referenceName(ref);
      const stored = await storedValue(name);
      if (stored !== undefined) return { value: stored, source: "file" };
      const ambient = ambientValue(name);
      if (ambient !== undefined) return { value: ambient, source: "env" };
      return undefined;
    },
    async describe(ref) {
      const name = referenceName(ref);
      const stored = await storedValue(name);
      if (stored !== undefined) {
        return { configured: true, source: "file", writable: true, masked: maskSecret(stored) };
      }
      if (ambientValue(name) !== undefined) {
        return { configured: true, source: "env", writable: true };
      }
      return { configured: false, writable: true };
    },
    async set(ref, value) {
      const name = referenceName(ref);
      const secret = typeof value === "string" ? value : "";
      if (secret === "") {
        throw fail(`DSH credential ${name} set 需要非空值`, {
          code: "DSH_CREDENTIAL_VALUE_EMPTY",
          reference: name,
        });
      }
      await writeStateEntry(call, referenceStateKey(name), () => secret);
    },
    async unset(ref) {
      const name = referenceName(ref);
      const entry = await readStateEntry(call, referenceStateKey(name));
      if (!entry.found) return;
      await call("host.state.delete", { key: referenceStateKey(name) });
    },
    async readRecord(key) {
      const name = recordName(key);
      const entry = await readStateEntry(call, recordStateKey(name));
      return entry.found ? entry.value : undefined;
    },
    async describeRecord(key) {
      const name = recordName(key);
      const entry = await readStateEntry(call, recordStateKey(name));
      if (!entry.found) return { present: false, writable: true };
      return { present: true, kind: recordKind(entry.value), writable: true };
    },
    async listRecords() {
      const names = await readRecordIndex();
      const entries = [];
      for (const name of names) {
        const entry = await readStateEntry(call, recordStateKey(name));
        if (!entry.found) continue;
        entries.push({ key: name, kind: recordKind(entry.value) });
      }
      return entries;
    },
    async modifyRecord(key, mutate) {
      const name = recordName(key);
      if (typeof mutate !== "function") {
        throw fail("DSH credentials.modifyRecord 需要 mutate 函数", {
          code: "DSH_CREDENTIAL_MUTATE_INVALID",
          key: name,
        });
      }
      const before = await readStateEntry(call, recordStateKey(name));
      const next = await writeStateEntry(call, recordStateKey(name), async (current) => mutate(current));
      if (next !== undefined) {
        const names = await readRecordIndex();
        if (!names.includes(name)) await writeRecordIndex([...names, name]);
      }
      return next === undefined ? before.value : next;
    },
    async deleteRecord(key) {
      const name = recordName(key);
      const entry = await readStateEntry(call, recordStateKey(name));
      if (!entry.found) return;
      await call("host.state.delete", { key: recordStateKey(name) });
      const names = await readRecordIndex();
      if (names.includes(name)) await writeRecordIndex(names.filter((item) => item !== name));
    },
  };
}

function normalizeCommandInput(input) {
  if (input === undefined) return undefined;
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    throw fail("DSH command input 描述必须是对象", { code: "DSH_COMMAND_INPUT_INVALID", input: input ?? null });
  }
  const hint = text(input.hint);
  if (!hint) throw fail("DSH command input.hint 不能为空", { code: "DSH_COMMAND_INPUT_INVALID", input });
  const descriptor = { hint };
  if (input.attachments !== undefined) {
    if (typeof input.attachments !== "boolean") {
      throw fail("DSH command input.attachments 必须是布尔值", {
        code: "DSH_COMMAND_INPUT_INVALID",
        input,
      });
    }
    descriptor.attachments = input.attachments;
  }
  return Object.freeze(descriptor);
}

function normalizeCommandDefinition(definition) {
  if (!definition || typeof definition !== "object" || Array.isArray(definition)) {
    throw fail("DSH command definition 必须是对象", {
      code: "DSH_COMMAND_DEFINITION_INVALID",
      definition: definition ?? null,
    });
  }
  const name = text(definition.name);
  if (!name) throw fail("DSH command definition.name 不能为空", { code: "DSH_COMMAND_NAME_INVALID", name });
  if (!COMMAND_NAME.test(name)) {
    throw fail(`DSH command name "${name}" 必须是小写名称且不带前导斜杠`, {
      code: "DSH_COMMAND_NAME_INVALID",
      name,
    });
  }
  const description = text(definition.description);
  if (!description) {
    throw fail(`DSH command ${name} 缺少 description`, { code: "DSH_COMMAND_DESCRIPTION_INVALID", name });
  }
  if (typeof definition.handler !== "function") {
    throw fail(`DSH command ${name} 缺少 handler 函数`, { code: "DSH_COMMAND_HANDLER_INVALID", name });
  }
  const normalized = { name, description, handler: definition.handler };
  const definitionId = text(definition.definitionId);
  if (definitionId) normalized.definitionId = definitionId;
  const input = normalizeCommandInput(definition.input);
  if (input) normalized.input = input;
  if (definition.recordInput !== undefined) {
    if (typeof definition.recordInput !== "boolean") {
      throw fail(`DSH command ${name} 的 recordInput 必须是布尔值`, {
        code: "DSH_COMMAND_RECORD_INPUT_INVALID",
        name,
      });
    }
    normalized.recordInput = definition.recordInput;
  }
  return normalized;
}

function commandDescriptor(definition) {
  const descriptor = { name: definition.name, description: definition.description };
  if (definition.definitionId) descriptor.definitionId = definition.definitionId;
  if (definition.input) descriptor.input = definition.input;
  return Object.freeze(descriptor);
}

function parseCommandLine(line) {
  const value = typeof line === "string" ? line : "";
  const match = /^\/([^\s/]+)([\s\S]*)$/.exec(value);
  if (!match) return undefined;
  return { name: match[1], rawInput: match[2] };
}

function commandSession(agent) {
  const session = agent?.session;
  if (!session || typeof session.append !== "function") {
    throw fail("DSH commands.execute 需要携带 session.append 的 agent", {
      code: "DSH_COMMAND_AGENT_INVALID",
      sandbox: false,
    });
  }
  return session;
}

function receiptAttachments(attachments) {
  return attachments.filter((entry) => entry && typeof entry === "object" && !Array.isArray(entry) && ("receiptId" in entry || entry.kind === "file-receipt"));
}

export function createCommandsService(call) {
  const registry = new Map();
  let fileReceiptResolver = null;

  const service = {
    register(definition) {
      const normalized = normalizeCommandDefinition(definition);
      if (registry.has(normalized.name)) {
        throw fail(`DSH command ${normalized.name} 已经注册`, {
          code: "DSH_COMMAND_DUPLICATE",
          name: normalized.name,
        });
      }
      registry.set(normalized.name, normalized);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (registry.get(normalized.name) === normalized) registry.delete(normalized.name);
      };
    },
    registerFileReceiptResolver(resolver) {
      if (typeof resolver !== "function") {
        throw fail("DSH commands.registerFileReceiptResolver 需要函数", {
          code: "DSH_COMMAND_RECEIPT_RESOLVER_INVALID",
        });
      }
      if (fileReceiptResolver) {
        throw fail("DSH command file receipt resolver 已被注册", {
          code: "DSH_COMMAND_RECEIPT_RESOLVER_DUPLICATE",
        });
      }
      fileReceiptResolver = resolver;
      return () => {
        if (fileReceiptResolver === resolver) fileReceiptResolver = null;
      };
    },
    list() {
      return Object.freeze(
        Array.from(registry.values())
          .sort((left, right) => left.name.localeCompare(right.name))
          .map(commandDescriptor),
      );
    },
    find(agent, name) {
      return registry.get(text(name));
    },
    async execute(agent, line, submittedAttachments = [], signal) {
      const parsed = parseCommandLine(line);
      if (!parsed) return undefined;
      const name = text(parsed.name).toLowerCase();
      const definition = registry.get(name);
      if (!definition) return undefined;
      const attachments = Array.isArray(submittedAttachments) ? submittedAttachments : [];
      if (attachments.length > 0 && definition.input?.attachments !== true) {
        return {
          commandId: "",
          result: { kind: "error", text: `/${name} 不接受附件` },
        };
      }
      const receipts = receiptAttachments(attachments);
      let admitted = attachments;
      if (receipts.length > 0) {
        if (!fileReceiptResolver) {
          return {
            commandId: "",
            result: { kind: "error", text: `/${name} 的附件收据没有可用的存储解析器` },
          };
        }
        admitted = await fileReceiptResolver(agent, receipts);
        if (!Array.isArray(admitted)) {
          throw fail("DSH command file receipt resolver 必须返回附件数组", {
            code: "DSH_COMMAND_RECEIPT_RESOLVER_INVALID_RESULT",
            value: admitted ?? null,
          });
        }
      }
      const session = commandSession(agent);
      const commandId = `dsh-command:${randomUUID()}`;
      const run = { commandId, name, source: { kind: "user" } };
      if (definition.recordInput !== false) run.args = parsed.rawInput;
      session.append("command/run", run);
      if (typeof session.whenPersisted === "function") await session.whenPersisted();
      signal?.throwIfAborted();
      let result;
      try {
        result = await definition.handler({
          commandId,
          agent,
          rawInput: parsed.rawInput,
          attachments: admitted,
          signal,
        });
        if (!result || typeof result !== "object" || (result.kind !== "success" && result.kind !== "error")) {
          throw fail(`DSH command ${name} 返回了非法结果`, {
            code: "DSH_COMMAND_RESULT_INVALID",
            name,
            value: result ?? null,
          });
        }
      } catch (error) {
        result = attachOriginalError({ kind: "error", text: errorText(error) }, error);
      }
      const done = { commandId, kind: result.kind };
      if (typeof result.text === "string") done.text = result.text;
      if (result.kind === "success" && Number.isSafeInteger(result.sourceEventSeq)) {
        done.sourceEventSeq = result.sourceEventSeq;
      }
      try {
        session.append("command/done", done);
        if (typeof session.whenPersisted === "function") await session.whenPersisted();
      } catch (appendError) {
        if (result.kind !== "error") throw appendError;
        attachOriginalError(result, result.error ?? appendError);
        Object.defineProperty(result, "doneAppendError", {
          value: appendError,
          enumerable: false,
          configurable: true,
          writable: true,
        });
      }
      return { commandId, result };
    },
  };
  return service;
}

function sessionHeader(record) {
  const id = text(record?.sessionId);
  if (!id) {
    throw fail("Amitia host.conversation.list 返回了没有 sessionId 的会话", {
      code: "DSH_SESSION_HEADER_INVALID",
      record: record ?? null,
    });
  }
  const rawCreatedAt = text(record?.createdAt);
  const createdAt = rawCreatedAt === "" ? NaN : Date.parse(rawCreatedAt);
  if (!Number.isSafeInteger(createdAt) || createdAt < 0) {
    throw fail(`Amitia 会话 ${id} 的 createdAt 无法解析为时间戳: ${JSON.stringify(record?.createdAt ?? null)}`, {
      code: "DSH_SESSION_HEADER_INVALID",
      sessionId: id,
      createdAt: record?.createdAt ?? null,
    });
  }
  const cwd = text(record?.cwd);
  const parentSession = text(record?.parentSession) || text(record?.forkOrigin?.conversationId);
  return {
    version: SESSION_STATE_VERSION,
    id,
    createdAt,
    isSeeded: false,
    ...(cwd ? { cwd } : {}),
    ...(parentSession ? { parentSession } : {}),
  };
}

function sessionRecord(record, isLive) {
  const header = sessionHeader(record);
  const formatStatus = text(record?.formatStatus);
  return {
    header,
    live: isLive(header.id),
    persisted: true,
    ...(formatStatus ? { formatStatus } : {}),
  };
}

function foldSessionTitle(events) {
  for (let index = events.length - 1; index >= 0; index -= 1) {
    const event = events[index];
    if (event?.type !== "session/title") continue;
    const data = event.data && typeof event.data === "object" ? event.data : {};
    return {
      title: text(data.title),
      messageSeqs: Array.isArray(data.messageSeqs) ? data.messageSeqs.slice() : [],
      source: data.source,
      eventSeq: event.seq,
      updatedAt: event.time,
    };
  }
  return undefined;
}

export function createSessionQueryService(options) {
  const { call, isLive, sessionView } = options;
  const activeProjections = () => (typeof options.projections === "function" ? options.projections() : []);
  const unavailableMethod = (method, detail, missing) => () => {
    throw unavailableMissing(`sessionQuery.${method}`, detail, missing);
  };

  const readConversationPage = async (offset) => {
    const result = await call("host.conversation.list", { limit: CONVERSATION_PAGE_LIMIT, offset });
    if (!Array.isArray(result?.items)) {
      throw fail("Amitia host.conversation.list 返回结构异常：缺少 items 数组", {
        code: "DSH_CONVERSATION_LIST_SHAPE_INVALID",
        payload: result ?? null,
      });
    }
    return result.items;
  };

  const findConversation = async (sessionId) => {
    let previousFirst = null;
    for (let page = 0, offset = 0; page < CONVERSATION_PAGE_GUARD; page += 1, offset += CONVERSATION_PAGE_LIMIT) {
      const sessions = await readConversationPage(offset);
      if (sessions.length === 0) return undefined;
      const first = text(sessions[0]?.sessionId);
      if (first !== "" && first === previousFirst) {
        throw fail("Amitia host.conversation.list 的 offset 没有推进分页", {
          code: "DSH_CONVERSATION_PAGES_NOT_ADVANCING",
          offset,
        });
      }
      previousFirst = first;
      const match = sessions.find((entry) => text(entry?.sessionId) === sessionId);
      if (match) return match;
      if (sessions.length < CONVERSATION_PAGE_LIMIT) return undefined;
    }
    throw fail(`Amitia host.conversation.list 在 ${CONVERSATION_PAGE_GUARD} 页内没有结束`, {
      code: "DSH_CONVERSATION_PAGE_LIMIT_EXCEEDED",
      limit: CONVERSATION_PAGE_GUARD,
    });
  };

  const requireSessionHeader = async (sessionId) => {
    const id = text(sessionId);
    if (!id) {
      throw fail("DSH sessionQuery 需要 sessionId", { code: "DSH_SESSION_ID_REQUIRED", sessionId: sessionId ?? null });
    }
    const record = await findConversation(id);
    if (!record) {
      throw fail(`Amitia 会话目录中不存在 ${id}`, { code: "DSH_SESSION_NOT_FOUND", sessionId: id });
    }
    return sessionHeader(record);
  };

  const readView = async (sessionId) => {
    const header = await requireSessionHeader(sessionId);
    const view = await sessionView(header.id);
    return { header, view };
  };

  const listAllSessions = async (signal) => {
    const records = [];
    let previousFirst = null;
    for (let page = 0, offset = 0; page < CONVERSATION_PAGE_GUARD; page += 1, offset += CONVERSATION_PAGE_LIMIT) {
      signal?.throwIfAborted();
      const sessions = await readConversationPage(offset);
      if (sessions.length === 0) return records;
      const first = text(sessions[0]?.sessionId);
      if (first !== "" && first === previousFirst) {
        throw fail("Amitia host.conversation.list 的 offset 没有推进分页", {
          code: "DSH_CONVERSATION_PAGES_NOT_ADVANCING",
          offset,
        });
      }
      previousFirst = first;
      for (const session of sessions) records.push(sessionRecord(session, isLive));
      if (sessions.length < CONVERSATION_PAGE_LIMIT) return records;
    }
    throw fail(`Amitia host.conversation.list 在 ${CONVERSATION_PAGE_GUARD} 页内没有结束`, {
      code: "DSH_CONVERSATION_PAGE_LIMIT_EXCEEDED",
      limit: CONVERSATION_PAGE_GUARD,
    });
  };

  return {
    listSessions(signal) {
      return listAllSessions(signal);
    },
    async readSession(sessionId) {
      const { header, view } = await readView(sessionId);
      return { session: header, inheritedEventCount: 0, events: view.events.slice() };
    },
    async listEvents(sessionId) {
      const { header, view } = await readView(sessionId);
      const current = new Set(view.surfaceEvents.map((event) => Number(event.seq)));
      return view.events.map((event) => {
        let surface = "log-only";
        if (isSurfaceEventType(event.type)) surface = current.has(Number(event.seq)) ? "current" : "shadowed";
        return {
          sessionId: header.id,
          seq: event.seq,
          type: event.type,
          time: event.time,
          surface,
        };
      });
    },
    async readSurface(sessionId) {
      const { header, view } = await readView(sessionId);
      const last = view.events.at(-1);
      return {
        session: header,
        inheritedEventCount: 0,
        capturedThroughSeq: last ? last.seq : null,
        events: view.surfaceEvents.slice(),
      };
    },
    async readTitle(sessionId) {
      const { view } = await readView(sessionId);
      return foldSessionTitle(view.events);
    },
    async readTitleSnapshot(sessionId) {
      const { header, view } = await readView(sessionId);
      const title = foldSessionTitle(view.events);
      return title ? { session: header, title } : { session: header };
    },
    async readTitleSnapshots(sessionIds) {
      const unique = Array.from(new Set((sessionIds ?? []).map((id) => text(id)).filter((id) => id !== "")));
      const results = [];
      for (const sessionId of unique) {
        try {
          results.push({ sessionId, status: "fulfilled", value: await this.readTitleSnapshot(sessionId) });
        } catch (error) {
          results.push({ sessionId, status: "rejected", reason: error });
        }
      }
      return results;
    },
    async observeSession(sessionId, options = {}) {
      if (options === null || typeof options !== "object") {
        throw fail(`DSH sessionQuery.observeSession 的 options 必须是对象：${JSON.stringify(options ?? null)}`, {
          code: "DSH_SESSION_OBSERVATION_OPTIONS_INVALID",
          sessionId: sessionId ?? null,
        });
      }
      const { header, view } = await readView(sessionId);
      const events = view.events.slice();
      const cursor = events.length === 0 ? -1 : Number(events.at(-1).seq);
      let disposed = false;
      const lease = () => {
        if (disposed) throw new Error(`session observation "${header.id}" is disposed`);
        return {
          source: "persisted",
          header,
          inheritedEventCount: 0,
          events,
          cursor,
          retain: () => lease(),
          [Symbol.dispose]: () => {
            disposed = true;
          },
        };
      };
      return lease();
    },
    async inspectSession(sessionId, read, signal) {
      if (typeof read !== "function") {
        throw fail(`DSH sessionQuery.inspectSession 需要读取回调：${JSON.stringify(read ?? null)}`, {
          code: "DSH_SESSION_INSPECT_READ_REQUIRED",
          sessionId: sessionId ?? null,
        });
      }
      signal?.throwIfAborted?.();
      const { header, view } = await readView(sessionId);
      const events = view.events.slice();
      const session = Object.freeze({
        id: header.id,
        header,
        events: Object.freeze(events),
        inheritedEventCount: 0,
        capturedThroughSeq: events.length === 0 ? null : Number(events.at(-1).seq),
        surfaceEvents: () => view.surfaceEvents.slice(),
        messages: () => Array.from((view.messages ?? new Map()).values()),
        readSurface: () => ({
          session: header,
          inheritedEventCount: 0,
          capturedThroughSeq: events.length === 0 ? null : Number(events.at(-1).seq),
          events: view.surfaceEvents.slice(),
        }),
        title: () => foldSessionTitle(events),
        append() {
          throw fail("DSH inspectSession 交给读取函数的是只读会话视图，不能追加事件", {
            code: "DSH_SESSION_INSPECT_READ_ONLY",
            sessionId: header.id,
          });
        },
      });
      return read(session);
    },
    async searchSessions(request, exec = {}) {
      exec?.signal?.throwIfAborted?.();
      if (request === null || typeof request !== "object") {
        throw fail(`DSH sessionQuery.searchSessions 的 request 必须是对象：${JSON.stringify(request ?? null)}`, {
          code: "DSH_SESSION_SEARCH_REQUEST_INVALID",
          request: request ?? null,
        });
      }
      const tokens = tokenizeSearchQuery("searchSessions", request.query);
      const limit = readSearchLimit("searchSessions", request.limit);
      const offset = readSearchOffset("searchSessions", request.cursor);
      const sessionClauses = materializeSessionFilters(request.sessionFilters ?? []);
      const eventClauses = materializeEventFilters(request.eventFilters ?? []);
      const records = (await listAllSessions(exec?.signal)).filter((record) => sessionClauses.every((clause) => clause(record)));
      const scored = [];
      for (const record of records) {
        exec?.signal?.throwIfAborted?.();
        const view = await sessionView(record.header.id);
        let best = null;
        for (const document of sessionEventDocuments(record.header, view)) {
          if (!eventClauses.every((clause) => clause(document))) continue;
          const score = scoreSearchText(tokens, document.text);
          if (score === 0) continue;
          const hit = {
            sessionId: document.sessionId,
            seq: document.seq,
            type: document.type,
            time: document.time,
            surface: document.surface,
            snippet: buildSearchSnippet(tokens, document.text),
          };
          if (!best || score > best.score || (score === best.score && hit.time > best.hit.time)) best = { score, hit };
        }
        if (best) scored.push({ score: best.score, hit: { ...record, bestMatch: best.hit } });
      }
      scored.sort((left, right) => right.score - left.score
        || right.hit.bestMatch.time - left.hit.bestMatch.time
        || left.hit.header.id.localeCompare(right.hit.header.id));
      return searchPage(scored.map((entry) => entry.hit), offset, limit, "searchSessions");
    },
    async searchEvents(request, exec = {}) {
      exec?.signal?.throwIfAborted?.();
      if (request === null || typeof request !== "object") {
        throw fail(`DSH sessionQuery.searchEvents 的 request 必须是对象：${JSON.stringify(request ?? null)}`, {
          code: "DSH_SESSION_SEARCH_REQUEST_INVALID",
          request: request ?? null,
        });
      }
      const sessionId = text(request.sessionId);
      if (!sessionId) {
        throw fail(`DSH sessionQuery.searchEvents 需要 sessionId：${JSON.stringify(request)}`, {
          code: "DSH_SESSION_ID_REQUIRED",
          request,
        });
      }
      const tokens = tokenizeSearchQuery("searchEvents", request.query);
      const limit = readSearchLimit("searchEvents", request.limit);
      const offset = readSearchOffset("searchEvents", request.cursor);
      const clauses = materializeEventFilters(request.filters ?? []);
      const { header, view } = await readView(sessionId);
      const scored = [];
      for (const document of sessionEventDocuments(header, view)) {
        if (!clauses.every((clause) => clause(document))) continue;
        const score = scoreSearchText(tokens, document.text);
        if (score === 0) continue;
        scored.push({
          score,
          hit: {
            sessionId: document.sessionId,
            seq: document.seq,
            type: document.type,
            time: document.time,
            surface: document.surface,
            snippet: buildSearchSnippet(tokens, document.text),
          },
        });
      }
      scored.sort((left, right) => right.score - left.score
        || right.hit.time - left.hit.time
        || right.hit.seq - left.hit.seq);
      const page = searchPage(scored.map((entry) => entry.hit), offset, limit, "searchEvents");
      return { session: header, ...page };
    },
    async filterSessions(filters, signal) {
      const clauses = materializeSessionFilters(filters);
      const records = await this.listSessions(signal);
      return records.filter((record) => clauses.every((clause) => clause(record)));
    },
    async filterEvents(sessionId, filters) {
      const clauses = materializeEventFilters(filters);
      const { header, view } = await readView(sessionId);
      return sessionEventDocuments(header, view).filter((document) => clauses.every((clause) => clause(document)));
    },
    async traceSession(sessionId, signal) {
      const id = text(sessionId);
      if (!id) {
        throw fail("DSH sessionQuery.traceSession 需要 sessionId", { code: "DSH_SESSION_ID_REQUIRED", sessionId: sessionId ?? null });
      }
      const records = await this.listSessions(signal);
      return traceSessionLineage(records, id);
    },
    async traceEvent(request, signal) {
      signal?.throwIfAborted?.();
      if (request === null || typeof request !== "object") {
        throw fail(`DSH sessionQuery.traceEvent 的 request 必须是对象：${JSON.stringify(request ?? null)}`, {
          code: "DSH_SESSION_SEARCH_REQUEST_INVALID",
          request: request ?? null,
        });
      }
      const sessionId = text(request.sessionId);
      if (!sessionId) {
        throw fail(`DSH sessionQuery.traceEvent 需要 sessionId：${JSON.stringify(request)}`, {
          code: "DSH_SESSION_ID_REQUIRED",
          request,
        });
      }
      const seq = Number(request.seq);
      if (!Number.isSafeInteger(seq) || seq < 0) {
        throw fail(`DSH sessionQuery.traceEvent 需要非负整数 seq：${JSON.stringify(request)}`, {
          code: "DSH_SESSION_EVENT_SEQ_INVALID",
          request,
        });
      }
      const { header, view } = await readView(sessionId);
      const events = view.events.slice();
      const targetEvent = events.find((event) => Number(event.seq) === seq);
      if (!targetEvent) {
        throw fail(`session ${JSON.stringify(sessionId)} has no event seq ${seq}`, {
          code: "DSH_SESSION_EVENT_NOT_FOUND",
          sessionId,
          seq,
        });
      }
      const current = new Set(view.surfaceEvents.map((event) => Number(event.seq)));
      const recordOf = (event) => ({
        sessionId: header.id,
        seq: Number(event.seq),
        type: event.type,
        time: event.time,
        surface: isSurfaceEventType(event.type)
          ? (current.has(Number(event.seq)) ? "current" : "shadowed")
          : "log-only",
      });
      const replacers = new Map();
      for (const event of events) {
        const op = event.surfaceOp;
        if (op === null || op === undefined || op === "append" || typeof op !== "object") continue;
        const start = Number(op.startSeq);
        const end = Number(op.endSeq);
        if (!Number.isSafeInteger(start) || !Number.isSafeInteger(end)) continue;
        for (let index = start; index <= end; index += 1) {
          if (!replacers.has(index)) replacers.set(index, Number(event.seq));
        }
      }
      const replacementChain = [];
      const visited = new Set([seq]);
      let cursor = replacers.get(seq);
      while (cursor !== undefined) {
        if (visited.has(cursor)) {
          throw fail(`session ${JSON.stringify(sessionId)} event ${seq} replacement chain contains a cycle at seq ${cursor}`, {
            code: "DSH_SESSION_EVENT_TRACE_CYCLE",
            sessionId,
            seq,
            replacementChain,
          });
        }
        visited.add(cursor);
        replacementChain.push(cursor);
        cursor = replacers.get(cursor);
      }
      const targetOp = targetEvent.surfaceOp;
      const replacedEventSeqs = targetOp !== null && targetOp !== undefined && typeof targetOp === "object"
        ? Array.from({ length: Math.max(0, Number(targetOp.endSeq) - Number(targetOp.startSeq) + 1) }, (_value, index) => Number(targetOp.startSeq) + index)
        : [];
      const sourceEventSeqs = Array.isArray(targetEvent.sourceEventSeqs) ? targetEvent.sourceEventSeqs.map((value) => Number(value)) : [];
      const derivedEventSeqs = events
        .filter((event) => Array.isArray(event.sourceEventSeqs) && event.sourceEventSeqs.map((value) => Number(value)).includes(seq))
        .map((event) => Number(event.seq));
      return {
        session: header,
        target: recordOf(targetEvent),
        ...(replacementChain.length === 0 ? {} : { replacedBy: replacementChain[0] }),
        replacementChain,
        replacedEventSeqs,
        sourceEventSeqs,
        derivedEventSeqs,
      };
    },
    async readEvent(request, signal) {
      const sessionId = text(request?.sessionId);
      const seq = Number(request?.seq);
      const before = readWindowSize("before", request?.before);
      const after = readWindowSize("after", request?.after);
      signal?.throwIfAborted?.();
      if (!sessionId) {
        throw fail(`DSH sessionQuery.readEvent 需要 sessionId：${JSON.stringify(request ?? null)}`, {
          code: "DSH_SESSION_ID_REQUIRED",
          request: request ?? null,
        });
      }
      if (!Number.isSafeInteger(seq) || seq < 0) {
        throw fail(`DSH sessionQuery.readEvent 需要非负整数 seq：${JSON.stringify(request ?? null)}`, {
          code: "DSH_SESSION_EVENT_SEQ_INVALID",
          request: request ?? null,
        });
      }
      const { header, view } = await readView(sessionId);
      const index = view.events.findIndex((item) => Number(item.seq) === seq);
      if (index < 0) {
        throw fail(`Amitia 会话 ${sessionId} 没有序号为 ${seq} 的事件`, {
          code: "DSH_SESSION_EVENT_NOT_FOUND",
          sessionId,
          seq,
        });
      }
      const startIndex = Math.max(0, index - before);
      const endIndex = Math.min(view.events.length - 1, index + after);
      return {
        session: header,
        inheritedEventCount: 0,
        target: view.events[index],
        events: view.events.slice(startIndex, endIndex + 1),
        startSeq: Number(view.events[startIndex].seq),
        endSeq: Number(view.events[endIndex].seq),
      };
    },
  };
}

const SESSION_QUERY_READ_WINDOW_MAX = 50;

function invalidSessionFilter(detail) {
  return fail(`session ${detail}`, { code: "DSH_SESSION_FILTER_INVALID" });
}

function validateRange(name, range) {
  if (!range || typeof range !== "object" || Array.isArray(range)) {
    throw invalidSessionFilter(`${name} filter requires a range object`);
  }
  if (range.from !== undefined && !Number.isFinite(range.from)) {
    throw invalidSessionFilter(`${name} filter from must be finite`);
  }
  if (range.to !== undefined && !Number.isFinite(range.to)) {
    throw invalidSessionFilter(`${name} filter to must be finite`);
  }
  if (range.from !== undefined && range.to !== undefined && range.from > range.to) {
    throw invalidSessionFilter(`${name} filter from must be less than or equal to to`);
  }
  return range;
}

function matchesRange(value, range) {
  return (range.from === undefined || value >= range.from) && (range.to === undefined || value <= range.to);
}

function stringFilterValues(name, values, allowed) {
  if (!Array.isArray(values)) throw invalidSessionFilter(`${name} filter values must be an array`);
  const copy = values.slice();
  for (const value of copy) {
    if (typeof value !== "string") throw invalidSessionFilter(`${name} filter values must be strings`);
    if (allowed && !allowed.includes(value)) {
      throw invalidSessionFilter(`${name} filter values must be one of ${allowed.join(", ")}`);
    }
  }
  return copy;
}

function nullableStringFilterValues(name, values) {
  if (!Array.isArray(values)) throw invalidSessionFilter(`${name} filter values must be an array`);
  return values.map((value) => {
    if (value !== null && typeof value !== "string") {
      throw invalidSessionFilter(`${name} filter values must be strings or null`);
    }
    return value;
  });
}

function compileSessionTextFilter(raw) {
  const trimmed = text(raw);
  if (trimmed === "") throw invalidSessionFilter("text filter must contain non-whitespace text");
  const pattern = trimmed
    .split(/\s+/u)
    .map((part) => part.replace(/[.*+?^${}()|[\]\\]/gu, "\\$&"))
    .join("\\s+");
  return new RegExp(pattern, "iu");
}

function materializeSessionFilters(filters) {
  if (!Array.isArray(filters)) throw invalidSessionFilter("filters must be an array");
  return filters.map((filter) => {
    switch (filter?.kind) {
      case "id": {
        const values = stringFilterValues("id", filter.values);
        return (record) => values.includes(record.header.id);
      }
      case "cwd": {
        const values = nullableStringFilterValues("cwd", filter.values);
        return (record) => values.includes(record.header.cwd ?? null);
      }
      case "created-at": {
        const range = validateRange("created-at", filter);
        return (record) => matchesRange(record.header.createdAt, range);
      }
      case "parent": {
        const values = nullableStringFilterValues("parent", filter.values);
        return (record) => values.includes(record.header.parentSession ?? null);
      }
      case "availability": {
        const values = stringFilterValues("availability", filter.values, ["live", "persisted"]);
        return (record) => values.some((value) => (value === "live" ? record.live : record.persisted));
      }
      default:
        throw invalidSessionFilter(`filter kind ${JSON.stringify(filter?.kind ?? null)} is unknown`);
    }
  });
}

function materializeEventFilters(filters) {
  if (!Array.isArray(filters)) throw invalidSessionFilter("filters must be an array");
  return filters.map((filter) => {
    switch (filter?.kind) {
      case "seq": {
        const range = validateRange("seq", filter);
        return (document) => matchesRange(document.seq, range);
      }
      case "time": {
        const range = validateRange("time", filter);
        return (document) => matchesRange(document.time, range);
      }
      case "type": {
        const values = stringFilterValues("type", filter.values);
        return (document) => values.includes(document.type);
      }
      case "surface": {
        const values = stringFilterValues("surface", filter.values, ["current", "shadowed", "log-only"]);
        return (document) => values.includes(document.surface);
      }
      case "text": {
        const pattern = compileSessionTextFilter(filter.text);
        return (document) => pattern.test(document.text);
      }
      default:
        throw invalidSessionFilter(`filter kind ${JSON.stringify(filter?.kind ?? null)} is unknown`);
    }
  });
}

function readWindowSize(name, value) {
  if (value === undefined) return 0;
  if (!Number.isInteger(value) || value < 0 || value > SESSION_QUERY_READ_WINDOW_MAX) {
    throw fail(`${name} must be an integer between 0 and ${SESSION_QUERY_READ_WINDOW_MAX}`, {
      code: "DSH_SESSION_EVENT_WINDOW_INVALID",
      window: name,
      value: value ?? null,
    });
  }
  return value;
}

function cloneSessionRecord(record) {
  return { header: { ...record.header }, live: record.live, persisted: record.persisted };
}

function traceSessionLineage(records, sessionId) {
  const byId = new Map(records.map((record) => [record.header.id, record]));
  const target = byId.get(sessionId);
  if (!target) {
    throw fail(`session ${JSON.stringify(sessionId)} not found`, {
      code: "DSH_SESSION_NOT_FOUND",
      sessionId,
    });
  }
  const ancestors = [];
  const ancestrySeen = new Set([sessionId]);
  let unresolvedParentId;
  let parentId = target.header.parentSession;
  while (parentId !== undefined) {
    if (ancestrySeen.has(parentId)) {
      throw fail(`session lineage contains a cycle at ${JSON.stringify(parentId)}`, {
        code: "DSH_SESSION_INVALID_LINEAGE",
        sessionId,
        parentId,
      });
    }
    ancestrySeen.add(parentId);
    const parent = byId.get(parentId);
    if (!parent) {
      unresolvedParentId = parentId;
      break;
    }
    ancestors.push(parent);
    parentId = parent.header.parentSession;
  }
  const childrenByParent = new Map();
  for (const record of records) {
    const parent = record.header.parentSession;
    if (parent === undefined) continue;
    const children = childrenByParent.get(parent) ?? [];
    children.push(record);
    childrenByParent.set(parent, children);
  }
  for (const children of childrenByParent.values()) {
    children.sort((left, right) => left.header.createdAt - right.header.createdAt || left.header.id.localeCompare(right.header.id));
  }
  const seen = new Set([sessionId]);
  const buildDescendants = (id) => (childrenByParent.get(id) ?? [])
    .filter((child) => {
      if (seen.has(child.header.id)) return false;
      seen.add(child.header.id);
      return true;
    })
    .map((child) => ({ record: cloneSessionRecord(child), children: buildDescendants(child.header.id) }));
  const common = {
    target: cloneSessionRecord(target),
    ancestors: ancestors.map(cloneSessionRecord),
    descendants: buildDescendants(sessionId),
  };
  if (unresolvedParentId !== undefined) {
    return { ...common, complete: false, unresolvedParentId };
  }
  return { ...common, complete: true, root: cloneSessionRecord(ancestors.at(-1) ?? target) };
}

function joinSessionEventText(parts) {
  return parts.map((part) => text(part)).filter((part) => part !== "").join("\n");
}

function sessionContentText(content) {
  if (!Array.isArray(content)) return "";
  const parts = [];
  for (const block of content) {
    if (!block || typeof block !== "object") continue;
    if (block.type === "text") parts.push(text(block.text));
    else if (block.type === "tool-call") parts.push(text(block.name), text(block.arguments));
  }
  return joinSessionEventText(parts);
}

function sessionTurnEndText(reason) {
  switch (reason?.kind) {
    case "error": return joinSessionEventText(["error", text(reason?.error?.message)]);
    case "aborted": return "aborted";
    case "max-tokens":
    case "interrupted": return reason.kind;
    default: return "";
  }
}

function sessionEventText(event) {
  const data = event?.data && typeof event.data === "object" ? event.data : {};
  switch (event?.type) {
    case "user/message": return sessionContentText(data.content);
    case "assistant/message": return sessionContentText(data.message?.content);
    case "tool/call": return joinSessionEventText([data.name, data.arguments]);
    case "tool/result": return joinSessionEventText([
      sessionContentText(data.message?.content),
      text(data.error?.name),
      text(data.error?.code),
    ]);
    case "todo/write": {
      const todos = Array.isArray(data.todos) ? data.todos : [];
      return joinSessionEventText(todos.flatMap((todo) => [text(todo?.status), text(todo?.content)]));
    }
    case "turn/end": return sessionTurnEndText(data.reason);
    default: return "";
  }
}

const SESSION_SEARCH_LIMIT_DEFAULT = 20;
const SESSION_SEARCH_LIMIT_MAX = 100;
const SESSION_SEARCH_SNIPPET_LENGTH = 160;

function tokenizeSearchQuery(method, query) {
  const raw = typeof query === "string" ? query : "";
  const tokens = raw.trim().split(/\s+/).filter((token) => token !== "");
  if (tokens.length === 0) {
    throw fail(`DSH sessionQuery.${method} 需要非空检索词：${JSON.stringify(query ?? null)}`, {
      code: "DSH_SESSION_SEARCH_QUERY_EMPTY",
      query: query ?? null,
    });
  }
  if (tokens.some((token) => token.length > SESSION_SEARCH_SNIPPET_LENGTH)) {
    throw fail(`DSH sessionQuery.${method} 的检索词过长：${JSON.stringify(query)}`, {
      code: "DSH_SESSION_SEARCH_QUERY_INVALID",
      query,
    });
  }
  return tokens;
}

function readSearchLimit(method, value) {
  if (value === undefined) return SESSION_SEARCH_LIMIT_DEFAULT;
  if (!Number.isSafeInteger(value) || value < 1) {
    throw fail(`DSH sessionQuery.${method} 的 limit 必须是正整数：${JSON.stringify(value ?? null)}`, {
      code: "DSH_SESSION_SEARCH_LIMIT_INVALID",
      limit: value ?? null,
    });
  }
  return Math.min(value, SESSION_SEARCH_LIMIT_MAX);
}

function readSearchOffset(method, cursor) {
  if (cursor === undefined || cursor === null || cursor === "") return 0;
  let parsed = null;
  try {
    parsed = JSON.parse(Buffer.from(String(cursor), "base64url").toString("utf8"));
  } catch (error) {
    throw fail(`DSH sessionQuery.${method} 的 cursor 无法解析：${JSON.stringify(cursor)}`, {
      code: "DSH_SESSION_SEARCH_CURSOR_INVALID",
      cursor: cursor ?? null,
      reason: String(error?.message ?? error),
    });
  }
  if (!parsed || !Number.isSafeInteger(parsed.offset) || parsed.offset < 0) {
    throw fail(`DSH sessionQuery.${method} 的 cursor 内容无效：${JSON.stringify(parsed ?? null)}`, {
      code: "DSH_SESSION_SEARCH_CURSOR_INVALID",
      cursor: cursor ?? null,
      value: parsed ?? null,
    });
  }
  return parsed.offset;
}

function searchPage(items, offset, limit, method) {
  const slice = items.slice(offset, offset + limit);
  if (offset > 0 && offset >= items.length) {
    throw fail(`DSH sessionQuery.${method} 的 cursor 超出结果范围：${JSON.stringify({ offset, total: items.length })}`, {
      code: "DSH_SESSION_SEARCH_CURSOR_INVALID",
      offset,
      total: items.length,
    });
  }
  const nextOffset = offset + slice.length;
  if (nextOffset >= items.length) return { items: slice };
  return { items: slice, nextCursor: Buffer.from(JSON.stringify({ offset: nextOffset }), "utf8").toString("base64url") };
}

function scoreSearchText(tokens, text) {
  const haystack = String(text ?? "").toLowerCase();
  let score = 0;
  for (const token of tokens) {
    const needle = token.toLowerCase();
    let index = haystack.indexOf(needle);
    while (index >= 0) {
      score += 1;
      index = haystack.indexOf(needle, index + needle.length);
    }
  }
  return score;
}

function buildSearchSnippet(tokens, text) {
  const source = String(text ?? "");
  const haystack = source.toLowerCase();
  let matchIndex = -1;
  for (const token of tokens) {
    const index = haystack.indexOf(token.toLowerCase());
    if (index >= 0 && (matchIndex < 0 || index < matchIndex)) matchIndex = index;
  }
  if (matchIndex < 0) return source.slice(0, SESSION_SEARCH_SNIPPET_LENGTH);
  const start = Math.max(0, matchIndex - Math.floor(SESSION_SEARCH_SNIPPET_LENGTH / 3));
  return source.slice(start, start + SESSION_SEARCH_SNIPPET_LENGTH);
}

function sessionEventDocuments(header, view) {
  const current = new Set(view.surfaceEvents.map((event) => Number(event.seq)));
  const documents = [];
  for (const event of view.events) {
    const eventText = sessionEventText(event);
    if (eventText.length === 0) continue;
    documents.push({
      sessionId: header.id,
      seq: Number(event.seq),
      type: event.type,
      time: event.time,
      surface: isSurfaceEventType(event.type) ? (current.has(Number(event.seq)) ? "current" : "shadowed") : "log-only",
      text: eventText,
    });
  }
  return documents;
}
