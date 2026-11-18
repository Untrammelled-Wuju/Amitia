import { conversationReadLimit, nextConversationReadLimit } from "./session-events.mjs";

const EMPTY_LIST = Object.freeze([]);
const conversationDataRegistry = new Map();
const conversationFlags = new Map();

export function setConversationFlags(conversationId, flags) {
  const id = text(conversationId);
  if (!id || !flags || typeof flags !== "object") return;
  conversationFlags.set(id, { ...(conversationFlags.get(id) ?? {}), ...flags });
  const data = conversationDataRegistry.get(id);
  if (data && typeof data.notifyFlags === "function") data.notifyFlags();
}

export function registerConversationData(conversationId, data) {
  const id = text(conversationId);
  if (!id || !data) throw new Error("conversation data registration requires an id and a data handle");
  conversationDataRegistry.set(id, data);
  return () => {
    if (conversationDataRegistry.get(id) === data) conversationDataRegistry.delete(id);
  };
}

export function refreshConversationData(conversationId) {
  const data = conversationDataRegistry.get(text(conversationId));
  return data ? data.refresh() : undefined;
}

export function expandConversationData(conversationId) {
  const data = conversationDataRegistry.get(text(conversationId));
  return data ? data.loadOlder() : undefined;
}
const EMPTY_TIMELINE = Object.freeze({ turnOrder: EMPTY_LIST, turns: new Map() });

function text(value) {
  return typeof value === "string" ? value.trim() : "";
}

function timeOf(value) {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  const raw = text(value);
  if (!raw) return 0;
  const parsed = Date.parse(raw);
  return Number.isFinite(parsed) ? parsed : 0;
}

function turnDataStore() {
  return Object.freeze({
    get: () => undefined,
    source: () => Object.freeze({ getSnapshot: () => undefined, subscribe: () => () => {} }),
    values: () => EMPTY_LIST,
  });
}

function locationFor(turnRecord, step) {
  return step === undefined
    ? Object.freeze({ kind: "turn", turn: turnRecord })
    : Object.freeze({ kind: "step", step: Object.freeze({ turn: turnRecord, step }), turn: turnRecord });
}

function messageContent(value) {
  const raw = typeof value === "string" ? value : "";
  if (!raw) return EMPTY_LIST;
  return Object.freeze([Object.freeze({ type: "text", text: raw })]);
}

function assistantBlocks(value) {
  const raw = typeof value === "string" ? value : "";
  if (!raw) return EMPTY_LIST;
  return Object.freeze([Object.freeze({ kind: "text", text: raw })]);
}

function emptyChatSnapshot() {
  return Object.freeze({
    order: EMPTY_LIST,
    nodes: Object.freeze({
      get: () => undefined,
      source: () => Object.freeze({ getSnapshot: () => undefined, subscribe: () => () => {} }),
      bottomSource: () => Object.freeze({ getSnapshot: () => false, subscribe: () => () => {} }),
      turnDataSource: () => Object.freeze({ getSnapshot: () => EMPTY_LIST, subscribe: () => () => {} }),
      processSource: () => Object.freeze({ getSnapshot: () => undefined, subscribe: () => () => {} }),
      values: () => EMPTY_LIST,
    }),
    locations: Object.freeze({ getTurn: () => EMPTY_LIST, getStep: () => EMPTY_LIST }),
    navigation: Object.freeze({ items: () => EMPTY_LIST }),
    timeline: EMPTY_TIMELINE,
    legacy: Object.freeze({
      nodes: EMPTY_LIST,
      turnTimings: new Map(),
      turnEnds: new Map(),
      partial: null,
      runningCalls: EMPTY_LIST,
    }),
  });
}

export function projectConversationMessages(messages) {
  const entries = [];
  (Array.isArray(messages) ? messages : []).forEach((message, index) => {
    if (!message || typeof message !== "object") return;
    const role = text(message.role).toLowerCase();
    const content = typeof message.contentSummary === "string"
      ? message.contentSummary
      : (typeof message.content === "string" ? message.content : "");
    if (!content) return;
    entries.push({
      index,
      role: role === "user" ? "user" : "assistant",
      content,
      time: timeOf(message.timestamp ?? message.createdAt),
      messageId: text(message.messageId || message.id),
    });
  });
  entries.sort((left, right) => (left.time - right.time) || (left.index - right.index));

  const nodes = new Map();
  const order = [];
  const turnOrder = [];
  const turns = new Map();
  const turnKeys = new Map();
  const stepKeys = new Map();
  const navigation = [];
  let current = null;
  let seq = 0;

  const openTurn = (entry) => {
    seq += 1;
    const number = turnOrder.length + 1;
    const record = Object.freeze({
      turn: number,
      status: "closed",
      start: Object.freeze({
        type: "turn/start",
        seq,
        time: entry.time,
        data: Object.freeze({ turn: number }),
      }),
      end: Object.freeze({
        type: "turn/end",
        seq: seq + 1,
        time: entry.time,
        data: Object.freeze({ turn: number, reason: Object.freeze({ kind: "completed" }) }),
      }),
      steps: Object.freeze([]),
      data: turnDataStore(),
    });
    turnOrder.push(number);
    turns.set(number, record);
    turnKeys.set(number, []);
    current = { record, step: 1, prompt: "", response: "", anchorKey: "" };
    navigation.push(current);
    return record;
  };

  for (const entry of entries) {
    if (entry.role === "user" || current === null) openTurn(entry);
    seq += 1;
    const kind = entry.role === "user" ? "user" : "assistant-step";
    const key = `${kind}:${entry.messageId || entry.index}`;
    const node = entry.role === "user"
      ? Object.freeze({
        key,
        kind: "user",
        target: "chat",
        anchorSeq: seq,
        seq,
        visibility: "visible",
        location: locationFor(current.record),
        data: Object.freeze({
          kind: "user",
          ...(entry.messageId ? { messageId: entry.messageId } : {}),
          seq,
          time: entry.time,
          content: messageContent(entry.content),
          source: Object.freeze({ kind: "user", form: null }),
        }),
      })
      : Object.freeze({
        key,
        kind: "assistant-step",
        target: "chat",
        anchorSeq: seq,
        seq,
        visibility: "visible",
        location: locationFor(current.record),
        data: Object.freeze({
          status: "settled",
          turn: current.record.turn,
          step: current.step,
          blocks: assistantBlocks(entry.content),
          time: entry.time,
        }),
      });
    nodes.set(key, node);
    order.push(key);
    turnKeys.get(current.record.turn).push(key);
    const stepKey = `${current.record.turn}:${current.step}`;
    if (!stepKeys.has(stepKey)) stepKeys.set(stepKey, []);
    stepKeys.get(stepKey).push(key);
    if (entry.role === "user") {
      current.prompt = entry.content;
      current.anchorKey = key;
    } else {
      current.response = entry.content;
      if (!current.anchorKey) current.anchorKey = key;
    }
  }

  return Object.freeze({
    nodes,
    order: Object.freeze(order),
    timeline: Object.freeze({ turnOrder: Object.freeze(turnOrder), turns }),
    turnKeys,
    stepKeys,
    navigation: Object.freeze(navigation.map((item) => Object.freeze({
      turn: item.record.turn,
      anchorKey: item.anchorKey,
      prompt: item.prompt.slice(0, 240),
      response: item.response.slice(0, 240),
    }))),
  });
}

export function createConversationData(options) {
  const conversationId = text(options?.conversationId);
  const command = options?.command;
  if (!conversationId) throw new Error("conversation data requires a conversation id");
  if (typeof command !== "function") throw new Error("conversation data requires a host command bridge");

  const listeners = new Set();
  const keyedSources = new Map();
  const keyedSubscribers = new Map();
  let projection = projectConversationMessages(EMPTY_LIST);
  let chat = emptyChatSnapshot();
  let session = null;
  let conversation = null;
  let refreshing = null;
  let disposed = false;
  let limit = conversationReadLimit(options?.limit);
  let hasMore = false;
  let lastError = null;
  let running = false;
  let activeView = "";

  const subscribe = (listener) => {
    listeners.add(listener);
    return () => { listeners.delete(listener); };
  };

  const resources = Object.freeze({
    subscribe,
    getSnapshot: () => chat,
  });

  const conversationSource = Object.freeze({
    subscribe,
    getSnapshot: () => conversation,
  });

  const sessionSource = Object.freeze({
    subscribe,
    getSnapshot: () => session,
  });

  const notify = () => {
    for (const listener of [...listeners]) {
      try {
        listener();
      } catch (error) {
        console.error("[original-error]", error);
      }
    }
  };

  const notifyKey = (key) => {
    const subscribers = keyedSubscribers.get(key);
    if (!subscribers) return;
    for (const listener of [...subscribers]) {
      try {
        listener();
      } catch (error) {
        console.error("[original-error]", error);
      }
    }
  };

  const nodeSource = (key) => {
    const cached = keyedSources.get(key);
    if (cached) return cached;
    const source = Object.freeze({
      getSnapshot: () => projection.nodes.get(key),
      subscribe: (listener) => {
        let subscribers = keyedSubscribers.get(key);
        if (!subscribers) {
          subscribers = new Set();
          keyedSubscribers.set(key, subscribers);
        }
        subscribers.add(listener);
        return () => {
          subscribers.delete(listener);
          if (subscribers.size === 0) keyedSubscribers.delete(key);
        };
      },
    });
    keyedSources.set(key, source);
    return source;
  };

  const rebuild = () => {
    const nodes = Object.freeze({
      get: (key) => projection.nodes.get(key),
      source: (key) => nodeSource(key),
      bottomSource: () => Object.freeze({ getSnapshot: () => false, subscribe: () => () => {} }),
      turnDataSource: () => Object.freeze({ getSnapshot: () => EMPTY_LIST, subscribe: () => () => {} }),
      processSource: () => Object.freeze({ getSnapshot: () => undefined, subscribe: () => () => {} }),
      values: () => Object.freeze([...projection.nodes.values()]),
    });
    chat = Object.freeze({
      order: projection.order,
      nodes,
      locations: Object.freeze({
        getTurn: (turn) => Object.freeze(projection.turnKeys.get(turn) ?? EMPTY_LIST),
        getStep: (turn, step) => Object.freeze(projection.stepKeys.get(`${turn}:${step}`) ?? EMPTY_LIST),
      }),
      navigation: Object.freeze({ items: () => projection.navigation }),
      timeline: projection.timeline,
      legacy: Object.freeze({
        nodes: EMPTY_LIST,
        turnTimings: new Map(),
        turnEnds: new Map(),
        partial: null,
        runningCalls: EMPTY_LIST,
      }),
    });
    session = Object.freeze({
      id: conversationId,
      items: EMPTY_LIST,
      status: new Map([[conversationId, Object.freeze({ state: running ? "running" : "idle" })]]),
      ready: true,
      blank: false,
      awaitingFirstTurn: projection.order.length === 0,
      running,
      promptAttempted: projection.order.length > 0,
      pendingSubmissions: EMPTY_LIST,
      openState: "open",
      subagent: null,
      removed: false,
      archived: Boolean(conversationFlags.get(conversationId)?.archived),
      pinned: Boolean(conversationFlags.get(conversationId)?.pinned),
      promptError: lastError,
      hasMore,
      loadingOlder: false,
      historyLoadStatus: "idle",
      historyWasPartial: false,
    });
    conversation = Object.freeze({
      views: Object.freeze({ get: () => undefined, grouped: () => undefined }),
      activeTargets: new Set(["chat"]),
    });
  };

  rebuild();

  const read = async (requested) => {
    try {
      const result = await command({ action: "conversation-read", conversationId, limit: requested, offset: 0 });
      if (disposed || requested !== limit) return chat;
      const messages = Array.isArray(result?.messages) ? result.messages : [];
      const previousKeys = new Set(projection.nodes.keys());
      projection = projectConversationMessages(messages);
      hasMore = result?.hasMore === true;
      lastError = null;
      rebuild();
      for (const key of previousKeys) if (!projection.nodes.has(key)) notifyKey(key);
      for (const key of projection.nodes.keys()) notifyKey(key);
      notify();
      return chat;
    } catch (error) {
      lastError = error;
      rebuild();
      notify();
      throw error;
    }
  };

  const track = (promise) => {
    const tracked = promise.finally(() => {
      if (refreshing === tracked) refreshing = null;
    });
    refreshing = tracked;
    return tracked;
  };

  const refresh = () => {
    if (disposed) return Promise.resolve(chat);
    if (refreshing) return refreshing;
    return track(read(limit));
  };

  const loadOlder = () => {
    if (disposed) return Promise.resolve(chat);
    const previousLimit = limit;
    const requested = nextConversationReadLimit(previousLimit);
    limit = requested;
    return track(read(requested).catch((error) => {
      if (limit === requested) limit = previousLimit;
      throw error;
    }));
  };

  const send = async (content) => {
    const value = typeof content === "string" ? content : "";
    if (!value.trim()) throw new Error("发送内容不能为空");
    running = true;
    rebuild();
    notify();
    try {
      const result = await command({
        action: "conversation-message-send",
        conversationId,
        content: value,
        requestId: `dsh-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`,
      });
      await refresh();
      return result;
    } finally {
      running = false;
      rebuild();
      notify();
    }
  };

  return Object.freeze({
    conversationId,
    notifyFlags: notify,
    refresh,
    loadOlder,
    getReadLimit: () => limit,
    send,
    chat: resources,
    conversation: conversationSource,
    session: sessionSource,
    target(name) {
      return text(name) === "chat" ? resources : undefined;
    },
    get activeView() {
      return activeView || undefined;
    },
    activate(viewId) {
      const next = text(viewId);
      if (!next || next === activeView) return;
      activeView = next;
      rebuild();
      notify();
    },
    dispose() {
      disposed = true;
      listeners.clear();
      keyedSubscribers.clear();
      keyedSources.clear();
    },
  });
}
