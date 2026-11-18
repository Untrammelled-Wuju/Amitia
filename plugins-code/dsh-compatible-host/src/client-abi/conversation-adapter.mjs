function text(value) {
  return value == null ? "" : String(value).trim();
}

function dshEvent(event) {
  const eventType = text(event?.eventType);
  return {
    ...event,
    id: text(event?.id),
    type: eventType.includes("/") ? eventType : eventType.replaceAll(".", "/"),
    seq: Number(event?.sequence) || 0,
    time: event?.timestamp ?? new Date(0).toISOString(),
    data: event?.payload ?? {},
  };
}

function matcherOf(definition) {
  if (typeof definition.match === "function") return definition.match;
  const table = definition.match && typeof definition.match === "object" ? definition.match : {};
  return (event) => {
    const handler = table[event.type];
    return typeof handler === "function" ? handler(event) : null;
  };
}

function matchView(event, result) {
  return {
    event,
    role: result.role === "update" ? "update" : "start",
    location: { kind: "unresolved" },
  };
}

function contextView(context, start, matches) {
  return {
    key: context.contextId,
    kind: "",
    id: context.contextId,
    matches,
    start,
    state: context.state,
    current: new Map(),
  };
}

export function adaptDshConversationDefinition(pluginId, definition, index) {
  if (!definition || typeof definition !== "object") throw new Error("DSH conversation definition is required");
  const kind = text(definition.kind);
  if (!kind) throw new Error("DSH conversation definition kind is required");
  if (typeof definition.start !== "function" || typeof definition.update !== "function") {
    throw new Error(`DSH conversation definition ${kind} requires start() and update()`);
  }
  const match = matcherOf(definition);
  return {
    id: `${text(pluginId)}:${kind}:${index}`,
    contributionId: `dsh:${text(pluginId)}:${kind}:${index}`,
    extensionId: text(pluginId),
    match(event) {
      const normalized = dshEvent(event);
      const result = match(normalized);
      if (!result || !text(result.id)) return null;
      return {
        contextId: text(result.id),
        phase: result.role === "update" ? "update" : "start",
        data: {
          ...result,
          event: normalized,
          location: { kind: "unresolved" },
        },
      };
    },
    create(event, matched) {
      const normalized = dshEvent(event);
      const start = matchView(normalized, { role: "start" });
      return definition.start(
        contextView({ contextId: matched.contextId, state: undefined }, start, [start]),
        start,
        undefined,
      );
    },
    update(state, event, matched) {
      const normalized = dshEvent(event);
      const current = matchView(normalized, { role: matched.phase });
      return definition.update(
        contextView({ contextId: matched.contextId, state }, undefined, [current]),
        current,
      );
    },
    buildViewNode(context) {
      if (typeof definition.buildViewNode !== "function") return null;
      const start = context.firstEvent
        ? matchView(dshEvent(context.firstEvent), { role: "start" })
        : undefined;
      const matches = context.events.map((event) => matchView(dshEvent(event), { role: "update" }));
      const node = definition.buildViewNode(contextView(context, start, matches));
      if (!node) return null;
      return {
        key: text(node.key) || context.contextId,
        nodeType: text(node.kind) || kind,
        target: text(node.target) || text(definition.target) || "chat",
        visibility: node.visibility === "hidden" ? "hidden" : "visible",
        payload: {
          dsh: node,
          data: node.data,
        },
      };
    },
  };
}

export function registerDshConversationDefinition(runtime, pluginId, definition, index, component) {
  if (!runtime || typeof runtime.register !== "function") throw new Error("Amitia conversation node runtime is unavailable");
  const key = `dsh:${text(pluginId)}:${text(definition?.kind)}:${index}`;
  return runtime.register(key, component, adaptDshConversationDefinition(pluginId, definition, index));
}
