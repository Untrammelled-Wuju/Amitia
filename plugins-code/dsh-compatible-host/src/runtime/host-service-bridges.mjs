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
  return {
    version: SESSION_STATE_VERSION,
    id,
    createdAt,
    isSeeded: false,
    ...(cwd ? { cwd } : {}),
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

  return {
    async listSessions(signal) {
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
    observeSession: unavailableMethod("observeSession", "Amitia 没有会话级事件观察流与中断 Turn 平衡语义", "host.session.observe（必须提供持续事件流与 Turn 结算）"),
    inspectSession: unavailableMethod("inspectSession", "容器不持有 DSH Session 对象，无法把内存会话交给读取函数", "host.session.handle（需要可传入回调的 DSH Session 实例，现有接口只返回快照）"),
    searchSessions: unavailableMethod("searchSessions", "Amitia host.conversation.search 只返回会话摘要匹配，没有 DSH SessionSearchPage 的命中评分与游标结构", "host.conversation.search 的命中评分与游标语义（现有接口只返回 { items, hasMore }）"),
    searchEvents: unavailableMethod("searchEvents", "Amitia 没有跨会话事件全文检索能力", "host.event.search（跨会话事件检索）"),
    filterSessions: unavailableMethod("filterSessions", "Amitia 会话目录没有 DSH SessionResultFilter 的 cwd 前缀、标题区间与时间区间谓词", "host.conversation.filter（会话目录只支持 limit/offset 分页）"),
    filterEvents: unavailableMethod("filterEvents", "Amitia 没有按 DSH SessionEventResultFilter 折叠事件面的查询能力", "host.session.filter_events（按事件面折叠的过滤查询）"),
    traceSession: unavailableMethod("traceSession", "Amitia 会话没有 fork 血缘，无法生成 DSH SessionLineageTrace", "host.session.lineage（会话派生血缘）"),
    traceEvent: unavailableMethod("traceEvent", "Amitia 会话事件没有 fork 来源引用，无法生成 DSH SessionEventTraceObservation", "host.session.event_trace（事件来源引用）"),
    async readEvent(sessionId, seq) {
      const target = Number(seq);
      if (!Number.isFinite(target)) {
        throw fail(`DSH sessionQuery.readEvent 需要数字 seq：${JSON.stringify({ sessionId: sessionId ?? null, seq: seq ?? null })}`, {
          code: "DSH_SESSION_EVENT_SEQ_INVALID",
          sessionId: sessionId ?? null,
          seq: seq ?? null,
        });
      }
      const { view } = await readView(sessionId);
      const event = view.events.find((item) => Number(item.seq) === target);
      if (!event) {
        throw fail(`Amitia 会话 ${text(sessionId)} 没有序号为 ${target} 的事件`, {
          code: "DSH_SESSION_EVENT_NOT_FOUND",
          sessionId: text(sessionId),
          seq: target,
        });
      }
      return event;
    },
  };
}
