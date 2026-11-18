import { LIFECYCLE_EVENTS, SURFACE_EVENT_TYPES } from "./lifecycle-events.generated.mjs";

export const SESSION_STATE_VERSION = 2;

export const SESSION_EVENT_LIMIT = 100000;

const registeredEvents = new Set(LIFECYCLE_EVENTS.map((entry) => entry.name));
const surfaceEvents = new Set(SURFACE_EVENT_TYPES);

function text(value) {
  return value == null ? "" : String(value).trim();
}

function fail(message, details) {
  const error = new Error(message);
  error.name = "DshSessionEventError";
  error.code = details.code;
  error.details = details;
  return error;
}

export function isSurfaceEventType(type) {
  return surfaceEvents.has(text(type));
}

export function isRegisteredLifecycleEvent(type) {
  return registeredEvents.has(text(type));
}

export function nextSessionEventSeq(events) {
  let max = -1;
  for (const event of events ?? []) {
    const seq = Number(event?.seq);
    if (Number.isSafeInteger(seq) && seq > max) max = seq;
  }
  return max + 1;
}

export function assertSessionStateVersion(value, conversationId) {
  const version = Number(value?.version);
  if (!Number.isSafeInteger(version) || version !== SESSION_STATE_VERSION) {
    throw fail(
      `DSH session ${conversationId} state version ${Number.isFinite(version) ? version : "unknown"} is not supported, expected ${SESSION_STATE_VERSION}`,
      {
        code: "DSH_SESSION_STATE_VERSION_UNSUPPORTED",
        conversationId,
        version: Number.isFinite(version) ? version : null,
        expectedVersion: SESSION_STATE_VERSION,
      },
    );
  }
}

function normalizeSurfaceOp(type, seq, value) {
  if (value === undefined || value === "append") return "append";
  if (value && typeof value === "object" && !Array.isArray(value)) {
    const startSeq = Number(value.startSeq);
    const endSeq = Number(value.endSeq);
    if (value.op !== "replace") {
      throw fail(`DSH session event ${type} surfaceOp ${String(value.op)} is not supported`, {
        code: "DSH_SESSION_EVENT_INVALID_SURFACE_OP",
        type,
        seq,
        surfaceOp: value,
      });
    }
    if (!Number.isSafeInteger(startSeq) || !Number.isSafeInteger(endSeq) || startSeq < 0 || endSeq < startSeq) {
      throw fail(`DSH session event ${type} replace range ${String(value.startSeq)}..${String(value.endSeq)} is invalid`, {
        code: "DSH_SESSION_EVENT_INVALID_SURFACE_RANGE",
        type,
        seq,
        startSeq: value.startSeq ?? null,
        endSeq: value.endSeq ?? null,
      });
    }
    return Object.freeze({ op: "replace", startSeq, endSeq });
  }
  throw fail(`DSH session event ${type} surfaceOp must be "append" or a replace range`, {
    code: "DSH_SESSION_EVENT_INVALID_SURFACE_OP",
    type,
    seq,
    surfaceOp: value ?? null,
  });
}

function normalizeSourceSeqs(type, seq, value) {
  if (value === undefined) return [];
  if (!Array.isArray(value)) {
    throw fail(`DSH session event ${type} sourceEventSeqs must be an array`, {
      code: "DSH_SESSION_EVENT_INVALID_SOURCE_SEQS",
      type,
      seq,
      sourceEventSeqs: value ?? null,
    });
  }
  const sources = [];
  for (const item of value) {
    if (!Number.isSafeInteger(item) || item < 0) {
      throw fail(`DSH session event ${type} sourceEventSeqs entry ${String(item)} is not a non-negative safe integer`, {
        code: "DSH_SESSION_EVENT_INVALID_SOURCE_SEQS",
        type,
        seq,
        entry: item ?? null,
      });
    }
    if (!sources.includes(item)) sources.push(item);
  }
  return sources;
}

export function createSessionEvent(type, data, options = {}) {
  const name = text(type);
  if (!name) {
    throw fail("DSH session event type must be a non-empty string", {
      code: "DSH_SESSION_EVENT_INVALID_TYPE",
      type: type ?? null,
    });
  }
  const seq = options.seq;
  if (!Number.isSafeInteger(seq) || seq < 0) {
    throw fail(`DSH session event ${name} sequence ${String(seq)} is not a non-negative safe integer`, {
      code: "DSH_SESSION_EVENT_INVALID_SEQ",
      type: name,
      seq: seq ?? null,
    });
  }
  const time = options.time;
  if (!Number.isSafeInteger(time) || time <= 0) {
    throw fail(`DSH session event ${name} time ${String(time)} is not an epoch millisecond integer`, {
      code: "DSH_SESSION_EVENT_INVALID_TIME",
      type: name,
      seq,
      time: time ?? null,
    });
  }
  if (data == null || typeof data !== "object" || Array.isArray(data)) {
    throw fail(`DSH session event ${name} data must be a payload object`, {
      code: "DSH_SESSION_EVENT_INVALID_DATA",
      type: name,
      seq,
      dataType: data === null ? "null" : Array.isArray(data) ? "array" : typeof data,
    });
  }
  const ignorable = options.ignorable === true;
  if (!registeredEvents.has(name) && !ignorable) {
    throw fail(
      `DSH session event type "${name}" is not declared in the target ABI catalog; an undeclared writer must mark the event ignorable`,
      {
        code: "DSH_SESSION_EVENT_UNDECLARED",
        type: name,
        seq,
        registeredEventCount: LIFECYCLE_EVENTS.length,
      },
    );
  }
  const surface = surfaceEvents.has(name);
  if (!surface && options.surfaceOp !== undefined) {
    throw fail(`DSH session event ${name} is log-only and must not carry surfaceOp`, {
      code: "DSH_SESSION_EVENT_SURFACE_OP_FORBIDDEN",
      type: name,
      seq,
      surfaceOp: options.surfaceOp ?? null,
    });
  }
  if (!surface && options.sourceEventSeqs !== undefined) {
    throw fail(`DSH session event ${name} is log-only and must not carry sourceEventSeqs`, {
      code: "DSH_SESSION_EVENT_SOURCE_SEQS_FORBIDDEN",
      type: name,
      seq,
      sourceEventSeqs: options.sourceEventSeqs ?? null,
    });
  }
  const event = { type: name, seq, time, data };
  if (ignorable) event.ignorable = true;
  if (surface) {
    event.surfaceOp = normalizeSurfaceOp(name, seq, options.surfaceOp);
    const sources = normalizeSourceSeqs(name, seq, options.sourceEventSeqs);
    if (sources.length > 0) event.sourceEventSeqs = sources;
  }
  return Object.freeze(event);
}

export function assertSurfaceReplacement(surfaceNodes, event) {
  if (!event || event.surfaceOp == null || event.surfaceOp === "append") return;
  const { startSeq, endSeq } = event.surfaceOp;
  const present = new Set(surfaceNodes.map((node) => Number(node?.seq)));
  const missing = [];
  for (let seq = startSeq; seq <= endSeq; seq += 1) if (!present.has(seq)) missing.push(seq);
  if (missing.length > 0) {
    throw fail(`DSH session event ${event.type} replace range ${startSeq}..${endSeq} references seq ${missing.join(", ")} that is not a surface node`, {
      code: "DSH_SESSION_EVENT_SURFACE_RANGE_MISSING",
      type: event.type,
      seq: event.seq,
      missing,
    });
  }
  const sources = new Set((event.sourceEventSeqs ?? []).map((item) => Number(item)));
  const uncovered = [];
  for (let seq = startSeq; seq <= endSeq; seq += 1) if (!sources.has(seq)) uncovered.push(seq);
  if (uncovered.length > 0) {
    throw fail(`DSH session event ${event.type} replace range ${startSeq}..${endSeq} must list seq ${uncovered.join(", ")} in sourceEventSeqs`, {
      code: "DSH_SESSION_EVENT_SOURCE_SEQS_INCOMPLETE",
      type: event.type,
      seq: event.seq,
      uncovered,
    });
  }
}

export function projectSurfaceEvents(events) {
  const surfaceNodes = [];
  for (const event of events ?? []) {
    if (!surfaceEvents.has(text(event?.type))) continue;
    if (event.surfaceOp && event.surfaceOp !== "append") {
      const { startSeq, endSeq } = event.surfaceOp;
      for (let index = surfaceNodes.length - 1; index >= 0; index -= 1) {
        const seq = Number(surfaceNodes[index]?.seq);
        if (seq >= startSeq && seq <= endSeq) surfaceNodes.splice(index, 1);
      }
    }
    surfaceNodes.push(event);
  }
  return surfaceNodes;
}

export function assertSessionEventBudget(conversationId, eventCount) {
  if (eventCount > SESSION_EVENT_LIMIT) {
    throw fail(`DSH session ${conversationId} exceeded ${SESSION_EVENT_LIMIT} persisted events`, {
      code: "DSH_SESSION_EVENT_LIMIT_EXCEEDED",
      conversationId,
      eventCount,
      limit: SESSION_EVENT_LIMIT,
    });
  }
}

export function restoreSessionEvents(events, conversationId) {
  const restored = [];
  const identities = new Set();
  let tolerated = 0;
  for (const event of events ?? []) {
    if (!event || typeof event !== "object" || Array.isArray(event)) {
      throw fail(`DSH session ${conversationId} persisted a non-object session event`, {
        code: "DSH_SESSION_EVENT_INVALID_RECORD",
        conversationId,
        event: event ?? null,
      });
    }
    const type = text(event.type);
    const seq = Number(event.seq);
    const time = Number(event.time);
    if (!type || !Number.isSafeInteger(seq) || seq < 0 || !Number.isSafeInteger(time)) {
      throw fail(`DSH session ${conversationId} persisted an event without a usable type, seq and time`, {
        code: "DSH_SESSION_EVENT_INVALID_RECORD",
        conversationId,
        type: event.type ?? null,
        seq: event.seq ?? null,
        time: event.time ?? null,
      });
    }
    if (!registeredEvents.has(type)) {
      if (event.ignorable !== true) {
        throw fail(
          `DSH session ${conversationId} contains undeclared required event "${type}" at seq ${seq}; the session cannot be reconstructed`,
          {
            code: "DSH_SESSION_EVENT_UNRECOGNIZED_REQUIRED",
            conversationId,
            type,
            seq,
          },
        );
      }
      tolerated += 1;
    }
    const identity = `${seq}:${type}:${time}`;
    if (identities.has(identity)) continue;
    identities.add(identity);
    restored.push(event);
  }
  restored.sort((left, right) => Number(left.seq) - Number(right.seq) || Number(left.time) - Number(right.time));
  assertSessionEventBudget(conversationId, restored.length);
  return { events: restored, tolerated };
}
