let activeHost = null;

function asObject(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

function requireString(value, name) {
  const result = String(value ?? "").trim();
  if (!result) throw new Error(`${name} is required`);
  return result;
}

function methodFor(kind, action) {
  return kind === "proactive_task" ? `host.proactive_task.${action}` : `host.reminder.${action}`;
}

function requestID() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
  return `reminder-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function taskPayload(payload) {
  const task = {
    title: requireString(payload.title, "title"),
    content: String(payload.content || ""),
    channel: String(payload.channel || "web"),
    conversationId: String(payload.conversationId || ""),
    characterId: String(payload.characterId || ""),
    remindAt: requireString(payload.remindAt, "remindAt"),
    repeatRule: String(payload.repeatRule || "none"),
    timezone: String(payload.timezone || "UTC"),
    enabled: payload.enabled !== false,
    systemNotification: payload.systemNotification !== false,
    requestId: String(payload.requestId || requestID()),
  };
  if (payload.generation != null) task.generation = Number(payload.generation);
  if (payload.endAt) task.endAt = String(payload.endAt);
  if (payload.maxOccurrences != null) task.maxOccurrences = Number(payload.maxOccurrences);
  if (payload.recurrence && typeof payload.recurrence === "object") task.recurrence = payload.recurrence;
  if (Array.isArray(payload.channelTargets)) task.channelTargets = payload.channelTargets;
  if (payload.policy && typeof payload.policy === "object") task.policy = payload.policy;
  return task;
}

async function command(host, inputValue) {
  const input = asObject(inputValue);
  const payload = asObject(input.payload);
  const kind = payload.kind === "proactive_task" ? "proactive_task" : "user_reminder";
  switch (String(input.action || "overview")) {
    case "characters": {
      const result = asObject(await host.call("host.character.list", { includeDisabled: false }));
      return (Array.isArray(result.items) ? result.items : []).map((item) => ({
        ...asObject(item),
        label: String(asObject(item).name || asObject(item).id || "未命名角色"),
        value: String(asObject(item).id || ""),
      })).filter((item) => item.value);
    }
    case "overview": {
      const [list, status, queue, capabilities] = await Promise.all([
        host.call(methodFor(kind, "list"), {}),
        host.call(methodFor(kind, "status"), {}),
        host.call(methodFor(kind, "queue"), {}),
        host.call(methodFor(kind, "capabilities"), {}),
      ]);
      return { list, status, queue, capabilities };
    }
    case "list":
      return host.call(methodFor(kind, "list"), {});
    case "status":
      return host.call(methodFor(kind, "status"), {});
    case "queue":
      return host.call(methodFor(kind, "queue"), {});
    case "capabilities":
      return host.call(methodFor(kind, "capabilities"), {});
    case "history":
      return host.call(methodFor(kind, "history"), {
        id: String(payload.id || ""),
        page: Number(payload.page || 1),
        pageSize: Number(payload.pageSize || 20),
        state: String(payload.state || ""),
      });
    case "create":
      return host.call(methodFor(kind, "create"), taskPayload(payload));
    case "update": {
      const task = asObject(payload.task);
      return host.call(methodFor(kind, "update"), {
        ...taskPayload({ ...task, requestId: task.requestId || requestID() }),
        id: requireString(task.id, "id"),
        generation: Number(task.generation || 0),
      });
    }
    case "pause":
    case "resume":
    case "cancel":
    case "execute":
      return host.call(methodFor(kind, input.action), {
        id: requireString(payload.id, "id"),
        generation: Number(payload.generation || 0),
      });
    case "preview":
      return host.call(methodFor(kind, "preview"), { id: requireString(payload.id, "id") });
    case "navigate":
      return host.call("host.ui.navigate", { target: requireString(payload.route, "route") });
    default:
      throw new Error(`未知提醒操作: ${String(input.action)}`);
  }
}

const activeTaskExtension = {
  async activate(context) {
    activeHost = context.host;
    context.handlers.bindTool("command", (handlerInput) => command(activeHost, handlerInput));
    context.log.info("active task plugin activated");
  },
  async deactivate() {
    activeHost = null;
  },
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = activeTaskExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(activeTaskExtension);
}
