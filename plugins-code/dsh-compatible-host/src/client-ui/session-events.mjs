const EMPTY_WINDOW = Object.freeze({
  revision: 0,
  entries: Object.freeze([]),
  hasMore: false,
  change: Object.freeze({ kind: "replace" }),
});

export const DEFAULT_CONVERSATION_READ_LIMIT = 100;
export const MAX_CONVERSATION_READ_LIMIT = 2000;

export function conversationReadLimit(value) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) return DEFAULT_CONVERSATION_READ_LIMIT;
  return Math.min(Math.trunc(parsed), MAX_CONVERSATION_READ_LIMIT);
}

export function nextConversationReadLimit(value) {
  const current = conversationReadLimit(value);
  return Math.min(current * 2, MAX_CONVERSATION_READ_LIMIT);
}

function text(value) {
  return typeof value === "string" ? value.trim() : "";
}

function timeOf(value) {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  const parsed = Date.parse(text(value));
  return Number.isFinite(parsed) ? parsed : Date.now();
}

function messageOf(message, index, role) {
  const content = typeof message.contentSummary === "string"
    ? message.contentSummary
    : (typeof message.content === "string" ? message.content : "");
  if (!content) return undefined;
  const id = text(message.messageId || message.id) || `${role}-${index}`;
  const blocks = Object.freeze([Object.freeze({ type: "text", text: content })]);
  if (role === "user") {
    return Object.freeze({
      id,
      role: "user",
      content: blocks,
      source: Object.freeze({ kind: "user" }),
    });
  }
  return Object.freeze({
    id,
    role: "assistant",
    content: blocks,
    source: Object.freeze({ kind: "model", provider: "amitia", model: "amitia-conversation" }),
  });
}

export function projectSessionEvents(messages) {
  const entries = [];
  let seq = 0;
  let turn = 0;
  let step = 0;
  let open = false;
  const push = (type, data, surface = false) => {
    seq += 1;
    const time = data.time ?? Date.now();
    const { time: _ignored, ...rest } = data;
    const payload = Object.freeze(rest);
    const event = surface
      ? Object.freeze({ type, seq, time, data: payload, surfaceOp: "append" })
      : Object.freeze({ type, seq, time, data: payload });
    entries.push(Object.freeze({ type: "event", event }));
  };
  const closeTurn = (reason) => {
    if (!open) return;
    push("step/end", { turn, step });
    push("turn/end", { turn, reason });
    open = false;
  };
  (Array.isArray(messages) ? messages : []).forEach((message, index) => {
    if (!message || typeof message !== "object") return;
    const role = text(message.role).toLowerCase() === "user" ? "user" : "assistant";
    const value = messageOf(message, index, role);
    if (!value) return;
    const time = timeOf(message.timestamp ?? message.createdAt);
    if (role === "user") {
      closeTurn(Object.freeze({ kind: "completed" }));
      turn += 1;
      step = 0;
      open = true;
      push("turn/start", { turn, time });
      push("step/start", { turn, step, time });
      push("user/message", { message: value, time }, true);
      return;
    }
    if (!open) {
      turn += 1;
      step = 0;
      open = true;
      push("turn/start", { turn, time });
      push("step/start", { turn, step, time });
    }
    push("assistant/message", { turn, step, message: value, stream: Object.freeze([]), time }, true);
  });
  closeTurn(Object.freeze({ kind: "completed" }));
  return Object.freeze(entries);
}

export function createSessionEventFeed(options) {
  const conversationId = text(options?.conversationId);
  const command = options?.command;
  if (!conversationId) throw new Error("DSH session event feed requires a conversation id");
  if (typeof command !== "function") throw new Error("DSH session event feed requires a host command bridge");

  const listeners = new Set();
  let window = EMPTY_WINDOW;
  let limit = conversationReadLimit(options?.limit);
  let refreshing = null;
  let disposed = false;

  const notify = () => {
    for (const listener of [...listeners]) {
      try {
        listener();
      } catch (error) {
        console.error("[original-error]", error);
      }
    }
  };

  const read = async (requested, force) => {
    const result = await command({ action: "conversation-read", conversationId, limit: requested, offset: 0 });
    if (disposed || requested !== limit) return window;
    const messages = Array.isArray(result?.messages) ? result.messages : [];
    const entries = projectSessionEvents(messages);
    const hasMore = result?.hasMore === true;
    const signature = entries.map((entry) => `${entry.event.seq}:${entry.event.type}`).join("|");
    const previous = window.entries.map((entry) => `${entry.event.seq}:${entry.event.type}`).join("|");
    if (force || signature !== previous || hasMore !== window.hasMore) {
      window = Object.freeze({
        revision: window.revision + 1,
        entries,
        hasMore,
        change: Object.freeze({ kind: "replace" }),
      });
      notify();
    }
    return window;
  };

  const track = (promise) => {
    const tracked = promise.finally(() => {
      if (refreshing === tracked) refreshing = null;
    });
    refreshing = tracked;
    return tracked;
  };

  const refresh = () => {
    if (disposed) return Promise.resolve(window);
    if (refreshing) return refreshing;
    return track(read(limit, false));
  };

  const expandWindow = () => {
    if (disposed) return Promise.resolve(window);
    const previousLimit = limit;
    const requested = nextConversationReadLimit(previousLimit);
    limit = requested;
    return track(read(requested, true).catch((error) => {
      if (limit === requested) limit = previousLimit;
      throw error;
    }));
  };

  void refresh().catch((error) => console.error("[original-error]", error));

  return Object.freeze({
    conversationId,
    getSnapshot: () => window,
    subscribe(listener) {
      if (typeof listener !== "function") throw new Error("DSH session event listener must be a function");
      listeners.add(listener);
      return () => { listeners.delete(listener); };
    },
    refresh,
    expandWindow,
    getLimit: () => limit,
    dispose() {
      disposed = true;
      listeners.clear();
    },
  });
}
