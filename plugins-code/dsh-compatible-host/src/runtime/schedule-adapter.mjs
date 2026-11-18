function text(value) {
  return value == null ? "" : String(value).trim();
}

function id(prefix) {
  const value = globalThis.crypto?.randomUUID?.();
  return `${prefix}-${value || `${Date.now()}-${Math.random().toString(16).slice(2)}`}`;
}

function clone(value) {
  return typeof globalThis.structuredClone === "function"
    ? globalThis.structuredClone(value)
    : JSON.parse(JSON.stringify(value));
}

function durationNs(seconds, label, minimum) {
  const value = Number(seconds);
  if (!Number.isSafeInteger(value) || value < minimum) {
    throw new Error(`DSH Schedule ${label} must be an integer of at least ${minimum}`);
  }
  const nanoseconds = value * 1e9;
  if (!Number.isSafeInteger(nanoseconds)) throw new Error(`DSH Schedule ${label} is out of range: ${seconds}`);
  return nanoseconds;
}

function positiveSeconds(seconds, label) {
  const value = Number(seconds);
  if (!Number.isSafeInteger(value) || value < 1) throw new Error(`DSH Schedule ${label} must be a positive safe integer`);
  return value;
}

function explicitInstant(value, label) {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(value)) {
    throw new Error(`DSH Schedule ${label} string must be an RFC 3339 instant with an explicit offset`);
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getTime() <= Date.now()) throw new Error(`DSH Schedule ${label} must be a future RFC 3339 date`);
  return date.toISOString();
}

function validatedTimeZone(value, label) {
  const timeZone = text(value);
  if (!timeZone) throw new Error(`DSH Schedule ${label} requires time_zone`);
  try {
    new Intl.DateTimeFormat("en-US", { timeZone }).format(new Date());
  } catch (error) {
    throw new Error(`DSH Schedule ${label} has invalid time_zone ${timeZone}: ${error.message}`, { cause: error });
  }
  return timeZone;
}

function localParts(instant, timeZone) {
  const values = {};
  const formatter = new Intl.DateTimeFormat("en-US", {
    timeZone,
    hourCycle: "h23",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
  for (const part of formatter.formatToParts(instant)) {
    if (part.type !== "literal") values[part.type] = Number(part.value);
  }
  return values;
}

function localInputInstant(input, label) {
  if (!input || typeof input !== "object") throw new Error(`DSH Schedule LocalAtInput ${label} is required`);
  const date = text(input.date);
  const time = text(input.time);
  const timeZone = validatedTimeZone(input.time_zone, `LocalAtInput ${label}`);
  const dateMatch = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
  const timeMatch = /^(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,3}))?$/.exec(time);
  if (!dateMatch || !timeMatch) {
    throw new Error(`DSH Schedule LocalAtInput ${label} requires date YYYY-MM-DD and time HH:mm:ss`);
  }
  const [, yearText, monthText, dayText] = dateMatch;
  const [, hourText, minuteText, secondText, fractionText = ""] = timeMatch;
  const fields = {
    year: Number(yearText),
    month: Number(monthText),
    day: Number(dayText),
    hour: Number(hourText),
    minute: Number(minuteText),
    second: Number(secondText),
    millisecond: Number(fractionText.padEnd(3, "0")),
  };
  if (fields.hour > 23 || fields.minute > 59 || fields.second > 59) {
    throw new Error(`DSH Schedule LocalAtInput ${label} has an invalid local time`);
  }
  const calendar = new Date(Date.UTC(fields.year, fields.month - 1, fields.day));
  if (calendar.getUTCFullYear() !== fields.year || calendar.getUTCMonth() !== fields.month - 1 || calendar.getUTCDate() !== fields.day) {
    throw new Error(`DSH Schedule LocalAtInput ${label} has an invalid local date`);
  }
  const localAsUtc = Date.UTC(fields.year, fields.month - 1, fields.day, fields.hour, fields.minute, fields.second, fields.millisecond);
  const candidates = new Set();
  for (const probe of [localAsUtc - 86400000, localAsUtc, localAsUtc + 86400000]) {
    const instant = new Date(probe);
    const parts = localParts(instant, timeZone);
    const representedAsUtc = Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour, parts.minute, parts.second, fields.millisecond);
    const candidate = localAsUtc - (representedAsUtc - probe);
    const candidateParts = localParts(new Date(candidate), timeZone);
    if (candidateParts.year === fields.year
      && candidateParts.month === fields.month
      && candidateParts.day === fields.day
      && candidateParts.hour === fields.hour
      && candidateParts.minute === fields.minute
      && candidateParts.second === fields.second) {
      candidates.add(candidate);
    }
  }
  const future = [...candidates].filter((candidate) => candidate > Date.now()).sort((left, right) => left - right);
  if (future.length === 0) {
    if (candidates.size > 0) throw new Error(`DSH Schedule LocalAtInput ${label} must be in the future`);
    throw new Error(`DSH Schedule LocalAtInput ${label} is nonexistent in time zone ${timeZone}`);
  }
  return new Date(future[0]).toISOString();
}

function atInstant(value, label) {
  const instant = typeof value === "string" ? explicitInstant(value, label) : localInputInstant(value, label);
  return instant;
}

function timingDefinition(timing, now) {
  if (!timing || typeof timing !== "object") throw new Error("DSH Schedule timing is required");
  const kind = text(timing.kind);
  if (kind === "after") {
    const seconds = positiveSeconds(timing.after_seconds, "after_seconds");
    const runAt = new Date(Date.parse(now) + seconds * 1000);
    if (Number.isNaN(runAt.getTime()) || runAt.getUTCFullYear() > 9999) throw new Error("DSH Schedule after_seconds is out of range");
    return {
      timezone: "UTC",
      trigger: {
        type: "one_shot",
        oneShot: { runAt: runAt.toISOString() },
      },
    };
  }
  if (kind === "every") {
    return {
      timezone: "UTC",
      trigger: {
        type: "interval",
        interval: {
          interval: durationNs(timing.every_seconds, "every_seconds", 60),
          anchorAt: now,
        },
      },
    };
  }
  if (kind === "at") {
    return {
      timezone: typeof timing.at === "object" ? validatedTimeZone(timing.at?.time_zone, "LocalAtInput") : "UTC",
      trigger: {
        type: "one_shot",
        oneShot: { runAt: atInstant(timing.at, "at") },
      },
    };
  }
  if (kind === "daily" || kind === "weekly") {
    const input = timing[kind];
    if (!input || typeof input !== "object") throw new Error(`DSH Schedule ${kind} input is required`);
    const timezone = validatedTimeZone(input.time_zone, kind);
    const wallTime = text(input.time);
    if (!timezone || !wallTime) throw new Error(`DSH Schedule ${kind} requires time and time_zone`);
    return {
      timezone,
      trigger: {
        type: "calendar",
        calendar: {
          frequency: kind,
          anchorAt: now,
          wallTime,
          ...(kind === "weekly" ? { weekdays: Array.isArray(input.weekdays) ? input.weekdays.slice() : [] } : {}),
        },
      },
    };
  }
  if (kind === "cron") {
    const input = timing.cron;
    if (!input || typeof input !== "object") throw new Error("DSH Schedule cron input is required");
    const expression = text(input.expression);
    const timezone = validatedTimeZone(input.time_zone, "cron");
    if (expression.split(/\s+/).length !== 5 || !timezone) throw new Error("DSH Schedule cron requires a five-field expression and time_zone");
    return {
      timezone,
      trigger: {
        type: "cron",
        cron: { expression, seconds: false },
      },
    };
  }
  if (kind === "rrule") {
    const rule = text(timing.rrule);
    if (!rule) throw new Error("DSH Schedule rrule input is required");
    const start = /^DTSTART(?:;TZID=([^:]+))?:\d{8}T\d{6}(?:Z)?$/m.exec(rule);
    if (!start) throw new Error("DSH Schedule rrule must contain a valid DTSTART line");
    const timezone = start[1] ? validatedTimeZone(start[1], "rrule") : "UTC";
    return {
      timezone,
      trigger: {
        type: "rrule",
        rrule: { rule },
      },
    };
  }
  throw new Error(`DSH Schedule timing kind ${kind || "<empty>"} is unsupported`);
}

export function createDshScheduleDefinition(request, now = new Date().toISOString()) {
  const title = text(request?.title);
  const prompt = text(request?.prompt);
  if (!title || title.length > 120) throw new Error("DSH Schedule title must contain 1 to 120 characters");
  if (!prompt) throw new Error("DSH Schedule prompt is required");
  const requestTarget = request?.target ?? { kind: "new_once" };
  const targetKind = text(requestTarget.kind);
  if (targetKind !== "session" && targetKind !== "new_once" && targetKind !== "new") {
    throw new Error("DSH Schedule target must be session, new_once, or new");
  }
  const sessionId = text(requestTarget.sessionId);
  if (targetKind === "session" && !sessionId) throw new Error("DSH Schedule session target requires sessionId");
  const created = targetKind === "session" ? {} : normalizeCreatedSessionOptions(requestTarget.create);
  const reservedSessionId = targetKind === "new_once" ? id("dsh-session") : "";
  const targetId = targetKind === "session" ? sessionId : reservedSessionId;
  const timing = timingDefinition(request.timing, now);
  const scheduleId = id("dsh-schedule");
  return {
    contributionId: scheduleId,
    moduleId: "",
    scheduleId,
    name: title,
    description: prompt,
    trigger: timing.trigger,
    target: {
      type: "session_prompt",
      targetId,
      inputTemplate: {
        prompt,
        dshTiming: request.timing,
        dshTitle: title,
        dshSessionId: sessionId,
        dshReservedSessionId: reservedSessionId,
        dshTargetKind: targetKind,
        dshCreate: {
          ...(created.cwd ? { cwd: created.cwd } : {}),
          ...(created.agentPreset ? { agentPreset: created.agentPreset } : {}),
        },
        dshProjectId: created.projectId,
      },
      idempotencyMode: "conditionally_idempotent",
    },
    timezone: timing.timezone,
    enabledByDefault: true,
    misfirePolicy: {
      policy: "fire_once",
      maxCatchUp: 1,
      gracePeriod: 86400000000000,
    },
    overlapPolicy: {
      policy: "queue_one",
    },
    retryPolicy: {
      maxAttempts: 1,
      initialBackoff: 0,
      maxBackoff: 0,
      multiplier: 1,
      jitter: 0,
      retryableErrorCodes: [],
    },
    jitterPolicy: {
      enabled: false,
      maxDelay: 0,
      seedMode: "disabled",
    },
    concurrencyPolicy: {
      maxConcurrentRuns: 1,
      perExtensionLimit: 1,
      perTargetLimit: 1,
    },
    scopeRule: {
      scopeType: "global",
      scopeIds: [],
    },
    dependencyRequirements: [],
    dstSpringPolicy: "next_valid_time",
    dstFallPolicy: "fire_once_first",
    executionOwner: "backend",
    definitionHash: "",
    version: "1",
  };
}

export function updateDshScheduleDefinition(definition, patch, now = new Date().toISOString()) {
  if (!definition || definition.target?.type !== "session_prompt") throw new Error("Amitia Schedule is not a DSH session prompt task");
  if (!patch || typeof patch !== "object") throw new Error("DSH Schedule update patch is required");
  const next = clone(definition);
  const input = next.target.inputTemplate && typeof next.target.inputTemplate === "object"
    ? { ...next.target.inputTemplate }
    : {};
  if (patch.title !== undefined) {
    const title = text(patch.title);
    if (!title || title.length > 120) throw new Error("DSH Schedule title must contain 1 to 120 characters");
    next.name = title;
    input.dshTitle = title;
  }
  if (patch.prompt !== undefined) {
    const prompt = text(patch.prompt);
    if (!prompt) throw new Error("DSH Schedule prompt is required");
    next.description = prompt;
    input.prompt = prompt;
  }
  if (patch.timing !== undefined) {
    const timing = timingDefinition(patch.timing, now);
    next.trigger = timing.trigger;
    next.timezone = timing.timezone;
    input.dshTiming = patch.timing;
  }
  if (patch.target !== undefined) {
    const targetKind = text(patch.target?.kind);
    if (targetKind !== "session" && targetKind !== "new_once" && targetKind !== "new") {
      throw new Error("DSH Schedule target must be session, new_once, or new");
    }
    const created = targetKind === "session" ? {} : normalizeCreatedSessionOptions(patch.target.create);
    const sessionId = text(patch.target.sessionId);
    if (targetKind === "session" && !sessionId) throw new Error("DSH Schedule session target requires sessionId");
    const reservedSessionId = targetKind === "new_once" ? id("dsh-session") : "";
    next.target.targetId = targetKind === "session" ? sessionId : reservedSessionId;
    input.dshSessionId = sessionId;
    input.dshReservedSessionId = reservedSessionId;
    input.dshTargetKind = targetKind;
    input.dshCreate = {
      ...(created.cwd ? { cwd: created.cwd } : {}),
      ...(created.agentPreset ? { agentPreset: created.agentPreset } : {}),
    };
    input.dshProjectId = created.projectId;
  }
  next.target.inputTemplate = input;
  return {
    definition: next,
    enabled: patch.active === undefined ? undefined : Boolean(patch.active),
  };
}

function fallbackTiming(definition) {
  const trigger = definition.trigger || {};
  if (trigger.type === "one_shot" && trigger.oneShot?.runAt) {
    return { kind: "at", at: trigger.oneShot.runAt };
  }
  if (trigger.type === "interval" && trigger.interval?.interval) {
    return { kind: "every", every_seconds: Math.round(Number(trigger.interval.interval) / 1e9) };
  }
  if (trigger.type === "calendar" && trigger.calendar) {
    const calendar = trigger.calendar;
    if (calendar.frequency === "weekly") {
      return { kind: "weekly", weekly: { time: calendar.wallTime, time_zone: definition.timezone, weekdays: calendar.weekdays || [] } };
    }
    return { kind: "daily", daily: { time: calendar.wallTime, time_zone: definition.timezone } };
  }
  if (trigger.type === "cron" && trigger.cron?.expression) {
    return { kind: "cron", cron: { expression: trigger.cron.expression, time_zone: definition.timezone } };
  }
  if (trigger.type === "rrule" && trigger.rrule?.rule) {
    return { kind: "rrule", rrule: trigger.rrule.rule };
  }
  throw new Error(`Amitia schedule ${definition.scheduleId} has no DSH timing mapping`);
}

export function mapDshScheduleEntry(definition, state = {}) {
  if (!definition || definition.target?.type !== "session_prompt") return null;
  const input = definition.target.inputTemplate || {};
  const targetKind = text(input.dshTargetKind) || "session";
  const sessionId = text(input.dshSessionId || definition.target.targetId);
  const reservedSessionId = text(input.dshReservedSessionId || (targetKind === "new_once" ? definition.target.targetId : ""));
  if (targetKind === "session" && !sessionId) throw new Error(`Amitia schedule ${definition.scheduleId} has no session prompt target`);
  if (targetKind === "new_once" && !reservedSessionId) throw new Error(`Amitia schedule ${definition.scheduleId} has no reserved new_once session`);
  const status = state.paused || text(state.status) === "paused"
    ? "paused"
    : ["expired", "uninstalled"].includes(text(state.status))
      ? "ended"
      : "active";
  const scheduledAt = text(state.nextScheduledAt || state.lastScheduledAt || state.updatedAt) || new Date().toISOString();
  const create = input.dshCreate && typeof input.dshCreate === "object" ? input.dshCreate : {};
  return {
    id: definition.scheduleId,
    kind: text(input.dshTiming?.kind) || fallbackTiming(definition).kind,
    title: text(input.dshTitle || definition.name),
    prompt: text(input.prompt || definition.description),
    ...(text(input.dshTiming?.kind) ? input.dshTiming : fallbackTiming(definition)),
    target: targetKind === "new"
      ? { kind: "new", create }
      : targetKind === "new_once"
        ? { kind: "new_once", sessionId: reservedSessionId, create }
        : { kind: "session", sessionId },
    involvedSessionIds: targetKind === "new"
      ? []
      : targetKind === "new_once"
        ? [reservedSessionId]
        : [sessionId],
    status,
    scheduledAt,
    ...(text(state.createdAt) ? { createdAt: state.createdAt } : {}),
    ...(text(state.updatedAt) ? { updatedAt: state.updatedAt } : {}),
    ...(state.status === "running" || state.status === "triggering" ? { executionActive: true } : {}),
  };
}

function normalizeCreatedSessionOptions(raw) {
  if (raw === undefined || raw === null) return { projectId: "" };
  if (typeof raw !== "object" || Array.isArray(raw)) throw new Error("DSH Schedule created-session options must be an object");
  const allowed = new Set(["cwd", "agentPreset", "projectId"]);
  const unknown = Object.keys(raw).filter((key) => !allowed.has(key));
  if (unknown.length > 0) throw new Error(`DSH Schedule created-session options contain unsupported fields: ${unknown.join(", ")}`);
  const cwd = text(raw.cwd);
  const agentPreset = text(raw.agentPreset);
  const projectId = text(raw.projectId);
  if (agentPreset) throw new Error(`DSH Schedule agentPreset ${agentPreset} has no verified Amitia preset mapping`);
  if (cwd && !projectId) throw new Error(`DSH Schedule cwd ${cwd} has no verified Amitia ProjectId mapping`);
  return { cwd, agentPreset, projectId };
}

export function mapDshScheduleCatalog(output) {
  const states = output?.states || {};
  return (Array.isArray(output?.schedules) ? output.schedules : [])
    .map((definition) => mapDshScheduleEntry(definition, states[definition.scheduleId]))
    .filter(Boolean)
    .sort((left, right) => String(left.scheduledAt).localeCompare(String(right.scheduledAt)) || left.id.localeCompare(right.id));
}
