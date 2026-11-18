import assert from "node:assert/strict";
import test from "node:test";
import {
  createDshScheduleDefinition,
  mapDshScheduleCatalog,
  mapDshScheduleEntry,
  updateDshScheduleDefinition,
} from "../src/runtime/schedule-adapter.mjs";

const now = "2026-11-14T00:00:00.000Z";

test("maps DSH session schedules into the unique Amitia schedule target", () => {
  const definition = createDshScheduleDefinition({
    title: "Daily review",
    prompt: "Review the current task",
    timing: {
      kind: "daily",
      daily: { time: "09:30:00", time_zone: "Asia/Shanghai" },
    },
    target: { kind: "session", sessionId: "conversation-1" },
  }, now);
  assert.equal(definition.target.type, "session_prompt");
  assert.equal(definition.target.targetId, "conversation-1");
  assert.equal(definition.trigger.type, "calendar");
  assert.equal(definition.trigger.calendar.frequency, "daily");
  assert.equal(definition.timezone, "Asia/Shanghai");
  assert.deepEqual(definition.scopeRule, { scopeType: "global", scopeIds: [] });
  assert.equal(definition.target.inputTemplate.prompt, "Review the current task");
});

test("maps every supported timing kind without changing DSH shape", () => {
  const base = {
    title: "Timed",
    prompt: "Prompt",
    target: { kind: "session", sessionId: "conversation-1" },
  };
  const cases = [
    [{ kind: "after", after_seconds: 30 }, "one_shot"],
    [{ kind: "every", every_seconds: 120 }, "interval"],
    [{ kind: "at", at: "2099-01-01T00:00:00.000Z" }, "one_shot"],
    [{ kind: "daily", daily: { time: "09:00:00", time_zone: "UTC" } }, "calendar"],
    [{ kind: "weekly", weekly: { time: "09:00:00", time_zone: "UTC", weekdays: [1, 3] } }, "calendar"],
    [{ kind: "cron", cron: { expression: "0 9 * * 1-5", time_zone: "UTC" } }, "cron"],
    [{ kind: "rrule", rrule: "DTSTART;TZID=Asia/Shanghai:20990101T090000\nRRULE:FREQ=DAILY;COUNT=3" }, "rrule"],
  ];
  for (const [timing, type] of cases) {
    assert.equal(createDshScheduleDefinition({ ...base, timing }, now).trigger.type, type);
  }
});

test("maps after to one shot and local absolute time through its IANA zone", () => {
  const after = createDshScheduleDefinition({
    title: "Reminder",
    prompt: "Prompt",
    timing: { kind: "after", after_seconds: 30 },
    target: { kind: "session", sessionId: "conversation-1" },
  }, now);
  assert.equal(after.trigger.type, "one_shot");
  assert.equal(after.trigger.oneShot.runAt, "2026-11-14T00:00:30.000Z");

  const local = createDshScheduleDefinition({
    title: "Local reminder",
    prompt: "Prompt",
    timing: {
      kind: "at",
      at: { date: "2099-01-01", time: "09:00:00", time_zone: "Asia/Shanghai" },
    },
    target: { kind: "session", sessionId: "conversation-1" },
  }, now);
  assert.equal(local.trigger.oneShot.runAt, "2099-01-01T01:00:00.000Z");
  assert.equal(local.timezone, "Asia/Shanghai");
  assert.deepEqual(local.target.inputTemplate.dshTiming.at, {
    date: "2099-01-01",
    time: "09:00:00",
    time_zone: "Asia/Shanghai",
  });
});

test("maps Amitia schedules with authoritative state back to DSH entries", () => {
  const definition = createDshScheduleDefinition({
    title: "Review",
    prompt: "Prompt",
    timing: { kind: "after", after_seconds: 60 },
    target: { kind: "session", sessionId: "conversation-1" },
  }, now);
  const entry = mapDshScheduleEntry(definition, {
    status: "enabled",
    nextScheduledAt: "2026-11-14T01:00:00.000Z",
    updatedAt: "2026-11-14T00:00:00.000Z",
  });
  assert.equal(entry.id, definition.scheduleId);
  assert.equal(entry.kind, "after");
  assert.equal(entry.status, "active");
  assert.deepEqual(entry.target, { kind: "session", sessionId: "conversation-1" });
  assert.deepEqual(entry.involvedSessionIds, ["conversation-1"]);
  const catalog = mapDshScheduleCatalog({
    schedules: [definition],
    states: { [definition.scheduleId]: { status: "paused", lastScheduledAt: "2026-11-14T01:00:00.000Z" } },
  });
  assert.equal(catalog[0].status, "paused");
});

test("updates schedule description, timing, active state and delivery target", () => {
  const definition = createDshScheduleDefinition({
    title: "Review",
    prompt: "Prompt",
    timing: { kind: "after", after_seconds: 60 },
    target: { kind: "session", sessionId: "conversation-1" },
  }, now);
  const updated = updateDshScheduleDefinition(definition, {
    title: "Updated review",
    prompt: "Updated prompt",
    timing: { kind: "every", every_seconds: 120 },
    target: { kind: "session", sessionId: "conversation-1" },
    active: false,
  }, now);
  assert.equal(updated.definition.name, "Updated review");
  assert.equal(updated.definition.description, "Updated prompt");
  assert.equal(updated.definition.trigger.type, "interval");
  assert.equal(updated.definition.trigger.interval.interval, 120000000000);
  assert.equal(updated.definition.target.inputTemplate.dshTiming.kind, "every");
  assert.equal(updated.enabled, false);
  const reused = updateDshScheduleDefinition(definition, {
    target: { kind: "new_once", create: {} },
  }, now);
  const reserved = reused.definition.target.inputTemplate.dshReservedSessionId;
  assert.equal(reused.definition.target.targetId, reserved);
  assert.equal(mapDshScheduleEntry(reused.definition).target.kind, "new_once");
  const retargeted = updateDshScheduleDefinition(reused.definition, {
    target: { kind: "session", sessionId: "conversation-2" },
  }, now);
  assert.equal(retargeted.definition.target.targetId, "conversation-2");
  assert.deepEqual(mapDshScheduleEntry(retargeted.definition).target, {
    kind: "session",
    sessionId: "conversation-2",
  });
  const fresh = updateDshScheduleDefinition(retargeted.definition, {
    target: { kind: "new", create: {} },
  }, now);
  assert.equal(fresh.definition.target.targetId, "");
  assert.deepEqual(mapDshScheduleEntry(fresh.definition).target, { kind: "new", create: {} });
});

test("maps a new-session target without requiring a reserved session id", () => {
  const definition = createDshScheduleDefinition({
    title: "Fresh session",
    prompt: "Run in a new session",
    timing: { kind: "after", after_seconds: 60 },
    target: { kind: "new" },
  }, now);
  assert.equal(definition.target.targetId, "");
  assert.equal(definition.target.inputTemplate.dshTargetKind, "new");
  const entry = mapDshScheduleEntry(definition, {
    status: "enabled",
    nextScheduledAt: "2026-11-14T01:00:00.000Z",
  });
  assert.deepEqual(entry.target, { kind: "new", create: {} });
  assert.deepEqual(entry.involvedSessionIds, []);
});

test("maps a new-session cwd to a verified Amitia ProjectId without exposing internal ids", () => {
  const definition = createDshScheduleDefinition({
    title: "Project session",
    prompt: "Run in the selected workspace",
    timing: { kind: "after", after_seconds: 60 },
    target: {
      kind: "new",
      create: {
        cwd: "/workspace",
        projectId: "project-1",
      },
    },
  }, now);
  assert.equal(definition.target.inputTemplate.dshProjectId, "project-1");
  assert.deepEqual(definition.target.inputTemplate.dshCreate, { cwd: "/workspace" });
  const entry = mapDshScheduleEntry(definition, {
    status: "enabled",
    nextScheduledAt: "2026-11-14T01:00:00.000Z",
  });
  assert.deepEqual(entry.target, {
    kind: "new",
    create: { cwd: "/workspace" },
  });
  assert.throws(
    () => createDshScheduleDefinition({
      title: "Preset session",
      prompt: "Run in a preset",
      timing: { kind: "after", after_seconds: 60 },
      target: {
        kind: "new",
        create: { agentPreset: "reviewer" },
      },
    }, now),
    /agentPreset reviewer.*no verified Amitia preset mapping/,
  );
});

test("reserves a stable session id for the default new_once target", () => {
  const definition = createDshScheduleDefinition({
    title: "Reusable session",
    prompt: "Run once, then reuse",
    timing: { kind: "every", every_seconds: 60 },
  }, now);
  const reservedSessionId = definition.target.inputTemplate.dshReservedSessionId;
  assert.match(reservedSessionId, /^dsh-session-/);
  assert.equal(definition.target.targetId, reservedSessionId);
  const entry = mapDshScheduleEntry(definition, {
    status: "enabled",
    nextScheduledAt: "2026-11-14T01:00:00.000Z",
  });
  assert.deepEqual(entry.target, { kind: "new_once", sessionId: reservedSessionId, create: {} });
  assert.deepEqual(entry.involvedSessionIds, [reservedSessionId]);
});
